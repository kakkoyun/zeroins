#!/usr/bin/env bash
# Exercise zeroins against a disposable, directly hosted Linux Kubernetes cluster.
set -o errexit
set -o nounset
set -o pipefail

readonly PROFILER_IMAGE='otel/opentelemetry-collector-ebpf-profiler:0.158.0'

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
  kubectl apply -f - <<'YAML'
apiVersion: v1
kind: ConfigMap
metadata:
  name: telemetry-sink
  namespace: default
data:
  relay.yaml: |
    receivers:
      otlp:
        protocols:
          grpc:
            endpoint: 0.0.0.0:4317
    exporters:
      debug:
        verbosity: detailed
    service:
      pipelines:
        traces:
          receivers: [otlp]
          exporters: [debug]
        metrics:
          receivers: [otlp]
          exporters: [debug]
        profiles:
          receivers: [otlp]
          exporters: [debug]
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: telemetry-sink
  namespace: default
spec:
  replicas: 1
  selector:
    matchLabels:
      app: telemetry-sink
  template:
    metadata:
      labels:
        app: telemetry-sink
    spec:
      containers:
        - name: collector
          image: otel/opentelemetry-collector-contrib:0.158.0
          args:
            - --feature-gates=+service.profilesSupport
            - --config=/conf/relay.yaml
          ports:
            - name: otlp-grpc
              containerPort: 4317
          volumeMounts:
            - name: config
              mountPath: /conf
      volumes:
        - name: config
          configMap:
            name: telemetry-sink
---
apiVersion: v1
kind: Service
metadata:
  name: telemetry-sink
  namespace: default
spec:
  selector:
    app: telemetry-sink
  ports:
    - name: otlp-grpc
      port: 4317
      targetPort: otlp-grpc
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: sample-http
  namespace: default
spec:
  replicas: 1
  selector:
    matchLabels:
      app: sample-http
  template:
    metadata:
      labels:
        app: sample-http
    spec:
      containers:
        - name: nginx
          image: nginx:1.27-alpine
          ports:
            - name: http
              containerPort: 80
---
apiVersion: v1
kind: Service
metadata:
  name: sample-http
  namespace: default
spec:
  selector:
    app: sample-http
  ports:
    - name: http
      port: 80
      targetPort: http
YAML
  kubectl rollout status deployment/telemetry-sink --timeout=180s
  kubectl rollout status deployment/sample-http --timeout=180s
}

start_http_traffic() {
  kubectl delete pod traffic --ignore-not-found --grace-period=1 >/dev/null
  # Keep traffic flowing while OBI discovers and instruments the workload. A short
  # burst can finish before the asynchronous process-discovery poll completes.
  # shellcheck disable=SC2016 # Expansion belongs to the pod shell.
  kubectl run traffic --image=curlimages/curl:8.17.0 --restart=Never --command -- \
    sh -c 'while true; do curl -fsS http://sample-http/ >/dev/null; sleep 0.1; done'
  kubectl wait --for=condition=Ready pod/traffic --timeout=120s
}

exercise_obi_daemonset() {
  kubectl obi attach --endpoint=http://telemetry-sink.default.svc.cluster.local:4317
  start_http_traffic
  if ! wait_for_log 'ResourceSpans|ResourceMetrics|ScopeSpans|ScopeMetrics' 180; then
    kubectl logs --namespace obi-system --selector app.kubernetes.io/instance=obi \
      --all-containers --prefix --tail=-1 >&2 || true
    fail 'OBI did not export traces or metrics to the sink'
  fi
  kubectl delete pod traffic --wait=true --grace-period=1
  kubectl obi status
  kubectl obi detach
  if helm status obi --namespace obi-system >/dev/null 2>&1; then
    fail 'OBI Helm release still exists after detach'
  fi
}

exercise_obi_sidecar() {
  kubectl obi attach sample-http --mode=sidecar \
    --endpoint=http://telemetry-sink.default.svc.cluster.local:4317
  kubectl obi attach sample-http --mode=sidecar \
    --endpoint=http://telemetry-sink.default.svc.cluster.local:4317

  local obi_count
  obi_count=$(kubectl get deployment sample-http -o jsonpath='{range .spec.template.spec.containers[*]}{.name}{"\n"}{end}' | grep -c '^obi$')
  [[ ${obi_count} -eq 1 ]] || fail "sidecar attach created ${obi_count} obi containers"

  kubectl obi detach sample-http --mode=sidecar
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
  if ! kubectl profiler attach \
    --endpoint=telemetry-sink.default.svc.cluster.local:4317 \
    --insecure; then
    kubectl get pods --namespace profiler-system -o wide >&2 || true
    kubectl describe pods --namespace profiler-system >&2 || true
    kubectl logs --namespace profiler-system --selector app.kubernetes.io/instance=profiler \
      --all-containers --prefix --tail=-1 >&2 || true
    fail 'profiling Collector did not become ready'
  fi
  kubectl profiler status

  kubectl delete pod cpu-burn --ignore-not-found >/dev/null
  # shellcheck disable=SC2016 # Expansion belongs to the pod shell.
  kubectl run cpu-burn --image=busybox:1.37 --restart=Never --command -- \
    sh -c 'i=0; while [ "$i" -lt 10000000 ]; do i=$((i+1)); done'
  kubectl wait --for=jsonpath='{.status.phase}'=Succeeded pod/cpu-burn --timeout=180s
  wait_for_log 'ResourceProfiles|ScopeProfiles' 240 "${profile_start}" ||
    fail 'profiler did not export a profile batch to the sink'

  kubectl profiler detach
  if helm status profiler --namespace profiler-system >/dev/null 2>&1; then
    fail 'profiler Helm release still exists after detach'
  fi
}

main() {
  preflight

  temporary_directory=$(mktemp -d)
  trap cleanup EXIT
  go build -o "${temporary_directory}/bin/kubectl-obi" ./cmd/kubectl-obi
  go build -o "${temporary_directory}/bin/kubectl-profiler" ./cmd/kubectl-profiler
  export PATH="${temporary_directory}/bin:${PATH}"

  kubectl cluster-info >/dev/null || fail 'Kubernetes cluster is not available'
  deploy_sink_and_workload
  exercise_obi_daemonset
  exercise_obi_sidecar
  exercise_profiler

  printf 'Live OBI and profiler integration gate passed on %s with %s.\n' "$(uname -r)" "${PROFILER_IMAGE}"
}

main "$@"
