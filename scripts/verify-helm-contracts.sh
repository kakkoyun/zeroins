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
    --set-string 'env.OTEL_EXPORTER_OTLP_METRICS_ENDPOINT=https://collector.example:4318' \
    --set-string 'env.OTEL_EXPORTER_OTLP_TRACES_ENDPOINT=https://collector.example:4318' \
    >"${obi_manifest}"
  require_match 'kind: DaemonSet' "${obi_manifest}"
  require_match 'privileged: true' "${obi_manifest}"
  require_match 'OTEL_EXPORTER_OTLP_ENDPOINT' "${obi_manifest}"
  require_match 'OTEL_EXPORTER_OTLP_METRICS_ENDPOINT' "${obi_manifest}"
  require_match 'OTEL_EXPORTER_OTLP_TRACES_ENDPOINT' "${obi_manifest}"
  require_match 'https://collector.example:4318' "${obi_manifest}"
  require_match 'v0\.10\.0' "${obi_manifest}"

  local profiler_values="${temporary_directory}/profiler-values.json"
  go run ./tools/render-profiler-values profiles.example:4317 >"${profiler_values}"
  [[ $(stat -f '%Lp' "${profiler_values}" 2>/dev/null || stat -c '%a' "${profiler_values}") == '600' ]] ||
    chmod 0600 "${profiler_values}"

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
