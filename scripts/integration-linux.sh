#!/usr/bin/env bash
# Exercise zeroins against a disposable, directly hosted Linux Kubernetes cluster.
set -o errexit
set -o nounset
set -o pipefail

readonly PROFILER_IMAGE='otel/opentelemetry-collector-ebpf-profiler:0.158.0'
readonly FIXTURES_DIR="${PWD}/examples/_fixtures"

temporary_directory=''

cleanup() {
  helm uninstall obi --namespace obi-system --ignore-not-found >/dev/null 2>&1 || true
  helm uninstall profiler --namespace profiler-system --ignore-not-found >/dev/null 2>&1 || true
  kubectl delete deployment telemetry-sink sample-http --namespace default --ignore-not-found >/dev/null 2>&1 || true
  kubectl delete service telemetry-sink sample-http --namespace default --ignore-not-found >/dev/null 2>&1 || true
  kubectl delete configmap telemetry-sink --namespace default --ignore-not-found >/dev/null 2>&1 || true
  kubectl delete pod traffic cpu-burn --namespace default --ignore-not-found >/dev/null 2>&1 || true
  kubectl delete namespace obi-system profiler-system --ignore-not-found --wait=false >/dev/null 2>&1 || true
  if [[ -n "${temporary_directory}" ]]; then
    rm -rf "${temporary_directory}"
  fi
}

fail() {
  printf 'integration failure: %s\n' "$*" >&2
  exit 1
}

wait_for_log() {
  local pattern=$1
  local timeout_seconds=$2
  local since_time=${3:-}
  local elapsed=0
  local -a log_args=(deployment/telemetry-sink --tail=-1)
  if [[ -n "${since_time}" ]]; then
    log_args+=(--since-time="${since_time}")
  fi
  while ((elapsed < timeout_seconds)); do
    # Do not use grep -q here: with pipefail, an early match can close the pipe
    # while kubectl is still writing, turning a successful assertion into SIGPIPE.
    if kubectl logs "${log_args[@]}" 2>&1 | grep -E "${pattern}" >/dev/null; then
      return 0
    fi
    sleep 5
    elapsed=$((elapsed + 5))
  done
  kubectl logs "${log_args[@]}" >&2 || true
  return 1
}

preflight() {
  [[ $(uname -s) == 'Linux' ]] || fail 'the live eBPF gate requires Linux'
  for command_name in kubectl helm go; do
    command -v "${command_name}" >/dev/null || fail "${command_name} is required"
  done
  [[ "${ZEROINS_INTEGRATION_ALLOW_CURRENT_CONTEXT:-0}" == '1' ]] ||
    fail 'set ZEROINS_INTEGRATION_ALLOW_CURRENT_CONTEXT=1 to allow privileged changes to the current Kubernetes context'

  local context server
  context=$(kubectl config current-context) || fail 'cannot read the current Kubernetes context'
  server=$(kubectl config view --minify -o jsonpath='{.clusters[0].cluster.server}') ||
    fail 'cannot read the current Kubernetes API server'
  printf 'Integration target: context=%q server=%q\n' "${context}" "${server}"

  if [[ ! -r /sys/kernel/btf/vmlinux ]]; then
    if [[ "${ZEROINS_ALLOW_UNSUPPORTED_EBPF_SKIP:-0}" == '1' ]]; then
      printf 'SKIP: host does not expose /sys/kernel/btf/vmlinux.\n' >&2
      exit 77
    fi
    fail 'host does not expose /sys/kernel/btf/vmlinux'
  fi
}

deploy_sink_and_workload() {
  kubectl apply -f "${FIXTURES_DIR}/telemetry-sink.yaml"
  kubectl apply -f "${FIXTURES_DIR}/sample-http.yaml"
  kubectl rollout status deployment/telemetry-sink --timeout=180s
  kubectl rollout status deployment/sample-http --timeout=180s
}

start_http_traffic() {
  kubectl delete pod traffic --ignore-not-found --grace-period=1 >/dev/null
  kubectl apply -f "${FIXTURES_DIR}/traffic.yaml"
  kubectl wait --for=condition=Ready pod/traffic --timeout=120s
}

exercise_obi_daemonset() {
  # Use OTLP/HTTP so this gate exercises the complete signal-specific /v1 paths.
  zeroins obi attach --endpoint=http://telemetry-sink.default.svc.cluster.local:4318
  start_http_traffic
  if ! wait_for_log 'ResourceSpans|ResourceMetrics|ScopeSpans|ScopeMetrics' 180; then
    kubectl logs --namespace obi-system --selector app.kubernetes.io/instance=obi \
      --all-containers --prefix --tail=-1 >&2 || true
    fail 'OBI did not export traces or metrics to the sink'
  fi
  kubectl delete pod traffic --wait=true --grace-period=1
  zeroins obi status
  zeroins obi detach
  if helm status obi --namespace obi-system >/dev/null 2>&1; then
    fail 'OBI Helm release still exists after detach'
  fi
}

exercise_obi_daemonset_duration() {
  # Run the bounded attach in the background so traffic can flow while the
  # observer is active. The attach blocks for 90s, then auto-detaches.
  zeroins obi attach --duration=90s \
    --endpoint=http://telemetry-sink.default.svc.cluster.local:4318 &
  local attach_pid=$!

  # Wait for the DaemonSet to become ready before generating traffic.
  local ds_ready=0
  for _ in {1..24}; do
    if kubectl get daemonset -l app.kubernetes.io/instance=obi -n obi-system -o jsonpath='{.items[0].status.numberReady}' 2>/dev/null | grep -E '^[1-9]' >/dev/null; then
      ds_ready=1
      break
    fi
    sleep 5
  done
  [[ ${ds_ready} -eq 1 ]] || fail 'OBI DaemonSet did not become ready during bounded attach'

  start_http_traffic
  if ! wait_for_log 'ResourceSpans|ResourceMetrics|ScopeSpans|ScopeMetrics' 180; then
    fail 'OBI did not export telemetry during bounded attach'
  fi
  kubectl delete pod traffic --wait=true --grace-period=1

  # Wait for the duration timer to fire and auto-detach.
  local elapsed=0
  while ((elapsed < 150)); do
    if ! helm status obi --namespace obi-system >/dev/null 2>&1; then
      printf 'Bounded attach auto-detached after %ds.\n' "${elapsed}"
      wait ${attach_pid} 2>/dev/null || true
      return 0
    fi
    sleep 5
    elapsed=$((elapsed + 5))
  done
  fail 'OBI Helm release still exists after --duration=90s expiry'
}

exercise_obi_sidecar() {
  zeroins obi attach sample-http --mode=sidecar \
    --endpoint=http://telemetry-sink.default.svc.cluster.local:4318
  zeroins obi attach sample-http --mode=sidecar \
    --endpoint=http://telemetry-sink.default.svc.cluster.local:4318

  local obi_count
  obi_count=$(kubectl get deployment sample-http -o jsonpath='{range .spec.template.spec.containers[*]}{.name}{"\n"}{end}' | grep -c '^obi$')
  [[ ${obi_count} -eq 1 ]] || fail "sidecar attach created ${obi_count} obi containers"

  zeroins obi detach sample-http --mode=sidecar
  if kubectl get deployment sample-http -o jsonpath='{range .spec.template.spec.containers[*]}{.name}{"\n"}{end}' | grep '^obi$' >/dev/null; then
    fail 'sidecar detach left the obi container behind'
  fi
  local shared_namespace
  shared_namespace=$(kubectl get deployment sample-http -o jsonpath='{.spec.template.spec.shareProcessNamespace}')
  [[ -z "${shared_namespace}" ]] || fail "sidecar detach restored shareProcessNamespace as ${shared_namespace}, want unset"
}

exercise_profiler() {
  local profile_start
  profile_start=$(date -u +%Y-%m-%dT%H:%M:%SZ)
  if ! zeroins profiler attach \
    --endpoint=telemetry-sink.default.svc.cluster.local:4317 \
    --insecure; then
    kubectl get pods --namespace profiler-system -o wide >&2 || true
    kubectl describe pods --namespace profiler-system >&2 || true
    kubectl logs --namespace profiler-system --selector app.kubernetes.io/instance=profiler \
      --all-containers --prefix --tail=-1 >&2 || true
    fail 'profiling Collector did not become ready'
  fi
  zeroins profiler status

  kubectl delete pod cpu-burn --ignore-not-found >/dev/null
  kubectl apply -f "${FIXTURES_DIR}/cpu-burn.yaml"
  kubectl wait --for=jsonpath='{.status.phase}'=Succeeded pod/cpu-burn --timeout=180s
  wait_for_log 'ResourceProfiles|ScopeProfiles' 240 "${profile_start}" ||
    fail 'profiler did not export a profile batch to the sink'

  zeroins profiler detach
  if helm status profiler --namespace profiler-system >/dev/null 2>&1; then
    fail 'profiler Helm release still exists after detach'
  fi
}

exercise_sessions_reap() {
  # Create a deliberately orphaned expired session by attaching with a short
  # duration and then killing the process with SIGKILL before the timer fires.
  zeroins obi attach --duration=30s \
    --endpoint=http://telemetry-sink.default.svc.cluster.local:4318 &
  local attach_pid=$!

  # Wait until the managed session label and expiry annotation exist on the
  # DaemonSet, so the orphan is discoverable by sessions reap.
  local session_ready=0
  for _ in {1..24}; do
    if kubectl get daemonset -l zeroins.kakkoyun.dev/managed=true -n obi-system -o jsonpath='{.items[0].metadata.annotations.zeroins\.kakkoyun\.dev/expires-at}' 2>/dev/null | grep -E . >/dev/null; then
      session_ready=1
      break
    fi
    sleep 2
  done
  [[ ${session_ready} -eq 1 ]] || fail 'could not create orphaned session: managed label or expires-at annotation never appeared'

  # SIGKILL the attach process so the bounded cleanup handler cannot run
  # and the session is truly orphaned.
  kill -9 "${attach_pid}" 2>/dev/null || true
  wait "${attach_pid}" 2>/dev/null || true

  # Wait for the expiry to pass.
  sleep 35

  # Reap should find and detach the expired orphaned session.
  zeroins sessions reap -A
  if helm status obi --namespace obi-system >/dev/null 2>&1; then
    fail 'sessions reap did not detach the orphaned expired OBI release'
  fi
}

main() {
  preflight

  temporary_directory=$(mktemp -d)
  trap cleanup EXIT
  go build -o "${temporary_directory}/bin/zeroins" ./cmd/zeroins
  go build -o "${temporary_directory}/bin/kubectl-obi" ./cmd/kubectl-obi
  go build -o "${temporary_directory}/bin/kubectl-profiler" ./cmd/kubectl-profiler
  export PATH="${temporary_directory}/bin:${PATH}"

  kubectl cluster-info >/dev/null || fail 'Kubernetes cluster is not available'
  deploy_sink_and_workload
  exercise_obi_daemonset
  exercise_obi_sidecar
  exercise_profiler
  exercise_obi_daemonset_duration
  exercise_sessions_reap

  printf 'Live OBI and profiler integration gate passed on %s with %s.\n' "$(uname -r)" "${PROFILER_IMAGE}"
}

main "$@"
