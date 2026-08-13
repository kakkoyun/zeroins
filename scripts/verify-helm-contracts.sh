#!/usr/bin/env bash
# Render the pinned OBI and profiling Collector charts and verify deployment contracts.
set -euo pipefail

readonly HELM_VERSION='v4.2.3'
readonly OBI_CHART_VERSION='0.10.0'
readonly COLLECTOR_CHART_VERSION='0.166.0'
readonly PROFILER_IMAGE='otel/opentelemetry-collector-ebpf-profiler:0.158.0'

temporary_directory=''
validation_file=''

cleanup() {
  if [[ -n "${validation_file}" ]]; then
    rm -f "${validation_file}"
  fi
  if [[ -n "${temporary_directory}" ]]; then
    rm -rf "${temporary_directory}"
  fi
}

fail() {
  printf 'contract failure: %s\n' "$*" >&2
  exit 1
}

require_match() {
  local pattern=$1
  local file=$2
  grep -Eq -- "${pattern}" "${file}" || fail "${file} does not match ${pattern}"
}

reject_match() {
  local pattern=$1
  local file=$2
  if grep -Eq -- "${pattern}" "${file}"; then
    fail "${file} unexpectedly matches ${pattern}"
  fi
}

extract_relay_config() {
  local manifest=$1
  local output=$2
  awk '
    /^  relay: \|/ { capture = 1; next }
    capture && /^---$/ { exit }
    capture { sub(/^    /, ""); print }
  ' "${manifest}" >"${output}"
  [[ -s "${output}" ]] || fail 'rendered profiler ConfigMap has no relay configuration'
}

main() {
  command -v helm >/dev/null || fail 'helm is required'
  command -v go >/dev/null || fail 'go is required'

  local actual_helm_version
  actual_helm_version=$(helm version --short)
  [[ "${actual_helm_version}" == "${HELM_VERSION}"* ]] ||
    fail "Helm ${HELM_VERSION} is required; found ${actual_helm_version}"

  temporary_directory=$(mktemp -d)
  trap cleanup EXIT
  export HELM_CACHE_HOME="${temporary_directory}/helm/cache"
  export HELM_CONFIG_HOME="${temporary_directory}/helm/config"
  export HELM_DATA_HOME="${temporary_directory}/helm/data"

  helm repo add open-telemetry https://open-telemetry.github.io/opentelemetry-helm-charts >/dev/null
  helm repo update open-telemetry >/dev/null

  local obi_manifest="${temporary_directory}/obi.yaml"
  helm template obi open-telemetry/opentelemetry-ebpf-instrumentation \
    --version "${OBI_CHART_VERSION}" \
    --namespace obi-system \
    --set-string 'env.OTEL_EXPORTER_OTLP_ENDPOINT=https://collector.example:4318' \
    --set-string 'env.OTEL_EXPORTER_OTLP_METRICS_ENDPOINT=https://collector.example:4318/v1/metrics' \
    --set-string 'env.OTEL_EXPORTER_OTLP_TRACES_ENDPOINT=https://collector.example:4318/v1/traces' \
    >"${obi_manifest}"

  # Contract: zeroins obi values must produce values that render identically to the
  # --set-string approach above, so `values` and `--dry-run` cannot drift from reality.
  local obi_values_file="${temporary_directory}/obi-values.json"
  go run ./cmd/zeroins obi values \
    --endpoint=https://collector.example:4318 \
    --mode=daemonset >"${obi_values_file}"
  local obi_manifest_from_values="${temporary_directory}/obi-from-values.yaml"
  helm template obi open-telemetry/opentelemetry-ebpf-instrumentation \
    --version "${OBI_CHART_VERSION}" \
    --namespace obi-system \
    --values "${obi_values_file}" \
    >"${obi_manifest_from_values}"
  if ! diff -u "${obi_manifest}" "${obi_manifest_from_values}" >/dev/null; then
    fail 'OBI manifest from zeroins obi values differs from the --set-string reference manifest'
  fi
  require_match 'kind: DaemonSet' "${obi_manifest}"
  require_match 'privileged: true' "${obi_manifest}"
  require_match 'OTEL_EXPORTER_OTLP_ENDPOINT' "${obi_manifest}"
  require_match 'OTEL_EXPORTER_OTLP_METRICS_ENDPOINT' "${obi_manifest}"
  require_match 'OTEL_EXPORTER_OTLP_TRACES_ENDPOINT' "${obi_manifest}"
  require_match 'https://collector.example:4318/v1/metrics' "${obi_manifest}"
  require_match 'https://collector.example:4318/v1/traces' "${obi_manifest}"
  require_match 'endpoint: http://\$\{HOST_IP\}:4318' "${obi_manifest}"
  require_match 'endpoint: http://\$\{HOST_IP\}:4317' "${obi_manifest}"
  require_match 'v0\.10\.0' "${obi_manifest}"

  local profiler_values="${temporary_directory}/profiler-values.json"
  go run ./tools/render-profiler-values profiles.example:4317 >"${profiler_values}"
  [[ $(stat -f '%Lp' "${profiler_values}" 2>/dev/null || stat -c '%a' "${profiler_values}") == '600' ]] ||
    chmod 0600 "${profiler_values}"

  # Contract: zeroins profiler values must produce byte-identical output to
  # tools/render-profiler-values, so `values` and `--dry-run` cannot drift from reality.
  # tools/render-profiler-values uses insecure=true, so match that here.
  local profiler_values_from_cmd="${temporary_directory}/profiler-values-from-cmd.json"
  go run ./cmd/zeroins profiler values \
    --endpoint=profiles.example:4317 --insecure >"${profiler_values_from_cmd}"
  if ! diff -u "${profiler_values}" "${profiler_values_from_cmd}" >/dev/null; then
    fail 'zeroins profiler values output differs from tools/render-profiler-values'
  fi

  local profiler_manifest="${temporary_directory}/profiler.yaml"
  helm template profiler open-telemetry/opentelemetry-collector \
    --version "${COLLECTOR_CHART_VERSION}" \
    --namespace profiler-system \
    --values "${profiler_values}" \
    >"${profiler_manifest}"
  require_match 'kind: DaemonSet' "${profiler_manifest}"
  require_match 'hostPID: true' "${profiler_manifest}"
  require_match 'privileged: true' "${profiler_manifest}"
  require_match 'mountPath: /sys/kernel/tracing' "${profiler_manifest}"
  require_match "image: \\\"?${PROFILER_IMAGE}\\\"?" "${profiler_manifest}"
  require_match 'otelcol-ebpf-profiler' "${profiler_manifest}"
  require_match 'service\.profilesSupport' "${profiler_manifest}"
  require_match 'profiles.example:4317' "${profiler_manifest}"

  local relay_config="${temporary_directory}/relay.yaml"
  extract_relay_config "${profiler_manifest}" "${relay_config}"
  require_match '^receivers:' "${relay_config}"
  require_match '^  profiling:' "${relay_config}"
  reject_match '^processors:' "${relay_config}"
  require_match '^extensions:' "${relay_config}"
  require_match '^  health_check:' "${relay_config}"
  require_match '^    profiles:' "${relay_config}"
  require_match 'otlp/profiles' "${relay_config}"
  reject_match '^  (traces|metrics|logs):' "${relay_config}"

  if [[ "${ZEROINS_VALIDATE_PROFILER_IMAGE:-0}" == '1' ]]; then
    command -v docker >/dev/null || fail 'docker is required for image validation'
    validation_file="${PWD}/.zeroins-profiler-relay-$$.yaml"
    cp "${relay_config}" "${validation_file}"
    docker run --rm \
      --volume "${PWD}:/workspace:ro" \
      "${PROFILER_IMAGE}" \
      validate --feature-gates=+service.profilesSupport --config="/workspace/$(basename "${validation_file}")"
  fi

  printf 'Helm contracts verified with %s.\n' "${HELM_VERSION}"
}

main "$@"
