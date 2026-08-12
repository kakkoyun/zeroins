---
name: collect-go-telemetry
description: |
  Choose and operate zero-code Go telemetry tools. Compares OBI runtime
  instrumentation, otelc compile-time instrumentation, and the OpenTelemetry
  eBPF Profiler. USE WHEN: "collect go telemetry", "instrument a Go service
  without code changes", "attach OBI", "build with otelc", or "profile Go with eBPF".
license: MIT
compatibility: Requires Go 1.24+ for zeroins. Kubernetes actions also require kubectl, Helm, Linux eBPF support, and cluster-admin-approved privileges. otelc requires Go 1.25+.
---

# Collect Go telemetry

Use this workflow to observe a Go service without editing its source.

## Install zeroins

Install all four commands:

```bash
go install github.com/kakkoyun/zeroins/cmd/...@latest
```

Every example also works without installation:

```bash
go run github.com/kakkoyun/zeroins/cmd/obi-integration@latest net/http
go run github.com/kakkoyun/zeroins/cmd/otelc-aspect@latest net/http
```

## Choose the instrumentation path

Ask these questions before proposing a deployment:

1. Does the service run on Linux?
2. Can the team rebuild it or change the build command?
3. Can a privileged observer run on the host or in the workload pod?
4. Does the user need boundary telemetry, supported Go-library semantics, or CPU profiles?
5. Where should OTLP traces, metrics, and profiles be sent?

Use OBI when the service is already running on supported Linux and a rebuild is
not available. OBI observes supported protocols and Go libraries from outside
the process.

Use otelc when the team controls the build, needs a non-Linux target, or cannot
run a privileged eBPF observer. otelc requires Go 1.25 or newer.

Add the eBPF Profiler when the user needs whole-node CPU profiles. It is a
separate privileged DaemonSet and exports the Alpha OpenTelemetry profiles
signal.

## Safe catalog lookups

The lookup commands use embedded, release-pinned data. They do not contact the
network or mutate a cluster, so an agent may run them without confirmation.

```bash
obi-integration net/http
obi-integration gin
otelc-aspect net/http
otelc-aspect google.golang.org/grpc
```

The same lookups can run through `go run`:

```bash
go run github.com/kakkoyun/zeroins/cmd/obi-integration@latest grpc
go run github.com/kakkoyun/zeroins/cmd/otelc-aspect@latest github.com/gin-gonic/gin
```

`obi-integration` is pinned to OBI v0.10.0. `otelc-aspect` is pinned to otelc
v1.0.1. An unknown result means the path is absent from that snapshot, not that
newer upstream versions will never support it.

## Build with otelc

```bash
go install go.opentelemetry.io/otelc/tool/cmd/otelc@v1.0.1
otelc go build -o ./myapp ./...
```

Verify that instrumentation ran and that the configured OTLP receiver receives
telemetry:

```bash
otelc go build -v ./... 2>&1 | grep -i inject
```

## Kubernetes confirmation gate

`kubectl-obi` and `kubectl-profiler` are experimental wrappers. They are not
production installers. They deploy privileged node observers and can restart
application pods.

Before an agent runs any `attach` or `detach` command, show the user all of the
following values and obtain explicit confirmation:

- the output of `kubectl config current-context`;
- the target namespace;
- the attachment mode and target deployment, if any;
- the complete telemetry endpoint, without credentials;
- whether transport uses TLS or plaintext;
- the privilege impact: host PID visibility, privileged containers, eBPF, and
  tracefs access;
- for sidecar mode, the application rollout that the command will trigger.

Do not infer consent from an earlier catalog lookup. Do not place credentials in
an endpoint URL.

## Attach OBI

DaemonSet mode requires an explicit OTLP HTTP(S) endpoint and instruments all
eligible workloads on each node:

```bash
kubectl obi attach \
  --endpoint=https://otel-collector.observability.svc:4318 \
  --namespace=obi-system
```

Sidecar mode targets one deployment and restarts its pods:

```bash
kubectl obi attach checkout \
  --mode=sidecar \
  --namespace=production \
  --endpoint=https://otel-collector.observability.svc:4318
```

The sidecar sets `OTEL_EBPF_AUTO_TARGET_EXE=*`, runs privileged, and enables
shared process namespaces. zeroins records ownership and the prior
`shareProcessNamespace` value. It refuses to remove an `obi` container that it
did not manage.

Check or remove OBI only after the confirmation gate:

```bash
kubectl obi status --namespace=obi-system
kubectl obi detach --namespace=obi-system
kubectl obi detach checkout --mode=sidecar --namespace=production
```

## Attach the eBPF Profiler

The profiler endpoint is OTLP/gRPC `host:port`. TLS is the default:

```bash
kubectl profiler attach \
  --endpoint=profiles-collector.observability.svc:4317 \
  --namespace=profiler-system
```

Use `--insecure` only when the user explicitly approves plaintext transport:

```bash
kubectl profiler attach \
  --endpoint=profiles-collector.observability.svc:4317 \
  --insecure
```

The command uses Collector chart 0.166.0, the dedicated
`otel/opentelemetry-collector-ebpf-profiler:0.158.0` image, and profiler receiver
v0.0.202632. It enables only a profiles pipeline. It does not install a backend,
configure authentication headers, or skip certificate verification.

After confirmation, inspect or remove the release:

```bash
kubectl profiler status --namespace=profiler-system
kubectl profiler detach --namespace=profiler-system
```

## Boundaries

OBI does not infer business events or unsupported internal functions. otelc only
instruments its declared library and version rules. The eBPF Profiler reports
CPU profiles but does not make the OpenTelemetry profiles signal stable. Add
manual instrumentation when the requested semantics fall outside those limits.
