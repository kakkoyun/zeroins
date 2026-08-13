---
name: collect-go-telemetry
description: |
  Choose and operate zero-code Go telemetry tools through the unified zeroins
  command. Compares OBI runtime instrumentation, otelc compile-time
  instrumentation, and the OpenTelemetry eBPF Profiler. USE WHEN: "collect go
  telemetry", "instrument a Go service without code changes", "attach OBI",
  "build with otelc", "profile Go with eBPF", or "zeroins".
license: MIT
compatibility: Requires Go 1.24+ for zeroins. Kubernetes actions also require kubectl, Helm, Linux eBPF support, and cluster-admin-approved privileges. otelc requires Go 1.25+.
---

# Collect Go telemetry

Use this workflow to observe a Go service without editing its source. All
commands route through the unified `zeroins` binary.

## Install zeroins

```bash
go install github.com/kakkoyun/zeroins/cmd/...@latest
```

The deprecated `obi-integration` and `otelc-aspect` binaries still work but
print a deprecation notice on stderr. Use `zeroins obi lookup` and
`zeroins otelc lookup` instead.

## Mandatory agent workflow

Follow these steps in order. Do not skip the preflight or confirmation gate.

### 1. Preflight

Run `zeroins doctor -o json` first. Abort on any hard failure. Do not attempt
an attach if the cluster cannot run eBPF observers.

```bash
zeroins doctor -o json
```

### 2. Coverage

Check whether the workload's libraries are covered. These lookups are offline
and require no confirmation.

```bash
zeroins obi lookup net/http -o json
zeroins otelc lookup net/http -o json
```

### 3. Plan review

Run `attach --dry-run -o json` and present the plan object to the user. The
plan contains the resolved context, namespace, mode, endpoint, privilege
impact, exact command argv, and rendered Helm values or sidecar patch.

```bash
zeroins obi attach --dry-run -o json \
  --endpoint=https://otel-collector.observability.svc:4318
```

### 4. Confirmation gate

Before running any `attach` or `detach` command, obtain explicit user
confirmation against this checklist, now backed by the machine-readable
plan object:

- the cluster context (from the plan);
- the target namespace;
- the attachment mode and target deployment, if any;
- the complete telemetry endpoint, without credentials;
- whether transport uses TLS or plaintext;
- the privilege impact: host PID, privileged containers, eBPF, tracefs;
- for sidecar mode, the application rollout that the command will trigger.

Do not infer consent from an earlier catalog lookup. Do not place credentials
in an endpoint URL.

### 5. Bounded attach

Prefer `--duration` for exploratory attaches. The command blocks, prints
remaining time to stderr, and detaches automatically when the timer fires
or on SIGINT/SIGTERM.

```bash
zeroins obi attach --duration=15m \
  --endpoint=https://otel-collector.observability.svc:4318
```

Without `--duration`, attach returns immediately.

### 6. Confirm telemetry

```bash
zeroins obi traces checkout --namespace=production --tail=20
```

### 7. Audit and clean up

Before proposing any detach, list active sessions:

```bash
zeroins sessions list -A -o json
```

Then detach or reap:

```bash
zeroins obi detach
zeroins sessions reap -A
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

## Build with otelc

```bash
go install go.opentelemetry.io/otelc/tool/cmd/otelc@v1.0.1
otelc go build -o ./myapp ./...
```

Verify that instrumentation ran:

```bash
otelc go build -v ./... 2>&1 | grep -i inject
```

## Attach OBI

DaemonSet mode:

```bash
zeroins obi attach \
  --endpoint=https://otel-collector.observability.svc:4318 \
  --namespace=obi-system
```

Sidecar mode:

```bash
zeroins obi attach checkout \
  --mode=sidecar \
  --namespace=production \
  --endpoint=https://otel-collector.observability.svc:4318
```

The sidecar sets `OTEL_EBPF_AUTO_TARGET_EXE=*`, runs privileged, and enables
shared process namespaces. zeroins records ownership and the prior
`shareProcessNamespace` value. It refuses to remove an `obi` container that it
did not manage.

## Attach the eBPF Profiler

```bash
zeroins profiler attach \
  --endpoint=profiles-collector.observability.svc:4317 \
  --namespace=profiler-system
```

Use `--insecure` only when the user explicitly approves plaintext transport.
The command uses Collector chart 0.166.0, the dedicated
`otel/opentelemetry-collector-ebpf-profiler:0.158.0` image, and profiler
receiver v0.0.202632. It enables only a profiles pipeline.

## Boundaries

OBI does not infer business events or unsupported internal functions. otelc
only instruments its declared library and version rules. The eBPF Profiler
reports CPU profiles but does not make the OpenTelemetry profiles signal
stable. Add manual instrumentation when the requested semantics fall
outside those limits.
