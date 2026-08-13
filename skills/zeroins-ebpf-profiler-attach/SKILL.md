---
name: zeroins-ebpf-profiler-attach
description: |
  Deploy, inspect, and remove the OpenTelemetry eBPF Profiler through
  zeroins. Covers the OTLP/gRPC profiles endpoint, TLS and insecure
  transport, pinned chart and image versions, and session cleanup. USE WHEN:
  "profiler attach", "eBPF profiler", "CPU profiles", "profiler detach", or
  "profiler values".
license: MIT
compatibility: Linux Kubernetes cluster with BTF support. kubectl, Helm v4.x. Cluster-admin-approved privileges for DaemonSets. OTLP/gRPC profiles endpoint in host:port form.
---

# Attach the eBPF Profiler

`zeroins profiler` requires an OTLP/gRPC profiles endpoint in `host:port`
form. TLS is enabled by default. Use `--insecure` only for an explicitly
approved plaintext destination.

Do not run `attach` or `detach` until the checklist in
[confirmation gate](references/confirmation-gate.md) is complete and the user
has explicitly approved.

## Values

```bash
zeroins profiler values --endpoint=profiles-collector.observability.svc:4317
```

`zeroins profiler values` prints the exact Helm values that attach would
apply, with no cluster access required.

## Dry-run

```bash
zeroins profiler attach --dry-run -o json \
  --endpoint=profiles-collector.observability.svc:4317
```

## Attach

```bash
zeroins profiler attach \
  --endpoint=profiles-collector.observability.svc:4317 \
  --namespace=profiler-system
```

The command deploys one dedicated profiling Collector per node. See
[pinned components](references/pinned-components.md) for the chart, image, and
receiver versions. See [transport and TLS](references/transport-and-tls.md)
for the `--insecure` flag.

## Bounded attach

```bash
zeroins profiler attach --duration=15m \
  --endpoint=profiles-collector.observability.svc:4317
```

## Status and detach

```bash
zeroins profiler status
zeroins profiler detach
```

Before detaching, list active sessions with `zeroins sessions list -A`.

## What the profiler does not do

zeroins does not install a profiles backend, manage credentials, add custom
headers, or disable certificate verification. The eBPF Profiler reports CPU
profiles but does not make the OpenTelemetry profiles signal stable.
