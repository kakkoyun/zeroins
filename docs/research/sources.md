# Focused source bibliography

> **Provenance:** Trimmed from `kakkoyun/gopherconuk-26` at commit
> `12762cd9e147148ff310007a895e91dc31f47c2f`. The original author relicensed
> this bibliography from Apache-2.0 to MIT for zeroins. Access dates and source
> notes come from the talk research; immutable links replace mutable links where
> the zeroins command contract depends on a release.

## OBI

| Key | Source | Pin or access date | Supports |
| --- | --- | --- | --- |
| S-OBI-01 | [Grafana Beyla project](https://grafana.com/oss/beyla-ebpf/) | 2026-07-22 | Donation and project relationship |
| S-OBI-02 | [Grafana Beyla repository](https://github.com/grafana/beyla) | 2026-07-22 | Upstream OBI relationship |
| S-OBI-03 | [OBI package index](https://pkg.go.dev/go.opentelemetry.io/obi@v0.10.0) | v0.10.0 | Module and release identity |
| S-OBI-04 | [OBI documentation](https://opentelemetry.io/docs/zero-code/obi/) | 2026-08-10 | Platform, architecture, signals, and maturity |
| S-OBI-05 | [OBI Kubernetes setup](https://opentelemetry.io/docs/zero-code/obi/setup/kubernetes/) | 2026-08-10 | DaemonSet and sidecar requirements |
| S-OBI-06 | [OBI support matrix](https://github.com/open-telemetry/opentelemetry-ebpf-instrumentation/blob/v0.10.0/SUPPORT_MATRIX.md) | v0.10.0 | Go library baselines embedded by `obi-integration` |
| S-OBI-07 | [OBI security](https://opentelemetry.io/docs/zero-code/obi/security/) | 2026-08-10 | Capabilities and kernel restrictions |
| S-OBI-08 | [OBI trace-log correlation](https://opentelemetry.io/docs/zero-code/obi/trace-log-correlation/) | 2026-08-10 | Correlation scope and log-export boundary |
| S-OBI-09 | [OBI donation tracking](https://github.com/open-telemetry/community/issues/2406) | closed 2025 | Donation transfer |
| S-OBI-10 | [OBI Helm chart values](https://github.com/open-telemetry/opentelemetry-helm-charts/blob/opentelemetry-ebpf-instrumentation-0.10.0/charts/opentelemetry-ebpf-instrumentation/values.yaml) | chart 0.10.0 | Deployment and exporter values |

## otelc and Orchestrion

| Key | Source | Pin or access date | Supports |
| --- | --- | --- | --- |
| S-O-01 | [Datadog Orchestrion project](https://opensource.datadoghq.com/projects/orchestrion/) | 2026-07-22 | Orchestrion provenance |
| S-O-02 | [golang/go#69887](https://github.com/golang/go/issues/69887) | 2026-07-22 | `-toolexec` and aspect-oriented framing |
| S-O-03 | [DataDog/orchestrion](https://github.com/DataDog/orchestrion/tree/v1.11.0) | v1.11.0 | Standalone project status and usage |
| S-O-04 | [Go compile-time instrumentation v1](https://opentelemetry.io/blog/2026/go-compile-time-instrumentation-v1/) | 2026-07-16 | otelc v1 release and mechanism |
| S-O-05 | [otelc repository](https://github.com/open-telemetry/opentelemetry-go-compile-instrumentation/tree/v1.0.1) | v1.0.1 | CLI, Go floor, and build behavior |
| S-O-06 | [dd-trace-go](https://github.com/DataDog/dd-trace-go) | v2 branch, 2026-07-22 | Orchestrion integration comparison |
| S-O-07 | [otelc supported libraries](https://opentelemetry.io/docs/zero-code/go/compile-time/supported-libraries/) | reviewed 2026-08-10 against v1.0.1 rules | Catalog names and operations |
| S-O-08 | [otelc testing strategy](https://github.com/open-telemetry/opentelemetry-go-compile-instrumentation/blob/v1.0.1/docs/testing.md) | v1.0.1 | Platform test coverage |
| S-O-09 | [OTel Go compile-time SIG announcement](https://opentelemetry.io/blog/2025/go-compile-time-instrumentation/) | 2025-01-24 | Founding organizations and convergence |
| S-O-10 | [otelc instrumentation rules](https://github.com/open-telemetry/opentelemetry-go-compile-instrumentation/tree/v1.0.1/instrumentation) | v1.0.1 | Immutable source for `otelc-aspect` |
| S-TH-05a | [dd-trace-go GLS aspect](https://github.com/DataDog/dd-trace-go/blob/main/internal/orchestrion/gls.orchestrion.yml) | verified 2026-07-22 | Runtime field injection |
| S-TH-05b | [dd-trace-go GLS consumer](https://github.com/DataDog/dd-trace-go/blob/main/internal/orchestrion/gls.go) | verified 2026-07-22 | Tracer-side accessors |
| S-TH-06 | [golang/go#72032](https://github.com/golang/go/issues/72032) | 2026-07-22 | `go:linkname` variable fragility |
| S-O-11 | [dave/dst](https://github.com/dave/dst) | 2026-07-22 | Decorated syntax trees used by Orchestrion |

## OpenTelemetry eBPF Profiler

| Key | Source | Pin or access date | Supports |
| --- | --- | --- | --- |
| S-P-01 | [Elastic profiling-agent contribution](https://opentelemetry.io/blog/2024/elastic-contributes-continuous-profiling-agent/) | 2024-06 | Completed donation |
| S-P-02 | [CNCF profiling announcement](https://www.cncf.io/blog/2024/03/19/opentelemetry-announces-support-for-profiling/) | 2024-03-19 | Donation pledge |
| S-P-03 | [OpenTelemetry profiles Alpha](https://opentelemetry.io/blog/2026/profiles-alpha/) | 2026 | Public Alpha and Collector distribution |
| S-P-04 | [Donation tracking issue](https://github.com/open-telemetry/community/issues/1918) | closed 2024-06 | Transfer record |
| S-P-05 | [eBPF Profiler repository](https://github.com/open-telemetry/opentelemetry-ebpf-profiler/tree/v0.0.202633) | v0.0.202633 | Architecture and current upstream tag |
| S-P-06 | [Go pclntab unwinding](https://github.com/open-telemetry/opentelemetry-ebpf-profiler/blob/v0.0.202633/doc/gopclntab.md) | v0.0.202633 | Go stack unwinding |
| S-P-07 | [Profiles specification](https://opentelemetry.io/docs/specs/otel/profiles/) | 2026-07-22 | Alpha signal status |
| S-P-08 | [OTLP specification](https://opentelemetry.io/docs/specs/otlp/) | OTLP 1.11.0 | Profiles development status |
| S-P-09 | [OTEP 4947](https://github.com/open-telemetry/opentelemetry-specification/blob/main/oteps/profiles/4947-thread-ctx.md#alternative-for-go-support) | 2026-08-10 | Proposed Go context-sharing path |
| S-P-10 | [Profiling Collector manifest](https://github.com/open-telemetry/opentelemetry-collector-releases/blob/v0.158.0/distributions/otelcol-ebpf-profiler/manifest.yaml) | Collector 0.158.0 | Receiver v0.0.202632 and included components |
| S-P-11 | [Collector chart profiling preset](https://github.com/open-telemetry/opentelemetry-helm-charts/tree/opentelemetry-collector-0.166.0/charts/opentelemetry-collector) | chart 0.166.0 | host PID, privileges, tracefs, and config injection |
| S-P-12 | [Collector batch processor](https://github.com/open-telemetry/opentelemetry-collector/tree/v0.158.0/processor/batchprocessor) | Collector 0.158.0 | Supports traces, metrics, and logs, but not profiles |
