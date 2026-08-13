---
name: zeroins-obi-attach
description: |
  Attach, inspect, query, and detach OBI in Kubernetes through zeroins.
  Covers DaemonSet and sidecar modes, endpoint rules, traces queries, and
  session cleanup. USE WHEN: "attach OBI", "obi sidecar", "obi daemonset",
  "obi traces", "obi detach", or "obi values".
license: MIT
compatibility: Linux Kubernetes cluster with BTF support. kubectl, Helm v4.x. Cluster-admin-approved privileges for DaemonSets and ClusterRoles. Explicit OTLP HTTP(S) base endpoint (no credentials in the URL).
---

# Attach OBI

OBI requires supported Linux kernels, BTF, process access, and eBPF
privileges. Both modes require an explicit OTLP HTTP(S) base endpoint. OBI
derives the signal paths from it.

Do not run `attach` or `detach` until the checklist in
[confirmation gate](references/confirmation-gate.md) is complete and the user
has explicitly approved.

## Values

```bash
zeroins obi values --endpoint=https://otel-collector.observability.svc:4318
zeroins obi values --mode=sidecar --endpoint=https://otel-collector.observability.svc:4318
```

`zeroins obi values` prints the exact Helm values or sidecar patch that attach
would apply, with no cluster access required.

## Dry-run

```bash
zeroins obi attach --dry-run -o json \
  --endpoint=https://otel-collector.observability.svc:4318
```

`--dry-run` prints the plan (context, namespace, privilege impact, exact
command argv, rendered values) without mutating.

## DaemonSet mode

```bash
zeroins obi attach \
  --endpoint=https://otel-collector.observability.svc:4318 \
  --namespace=obi-system
```

The DaemonSet observes workloads across a node. See
[DaemonSet vs sidecar](references/daemonset-vs-sidecar.md) for when to choose
each mode.

## Sidecar mode

```bash
zeroins obi attach checkout \
  --mode=sidecar \
  --namespace=production \
  --endpoint=https://otel-collector.observability.svc:4318
```

Sidecar attach is idempotent. zeroins annotates the pod template, records the
original `shareProcessNamespace` state, and restores it on detach. It refuses
to remove an `obi` container it did not create.

## Endpoint rules

Endpoint URLs containing userinfo, queries, or fragments are rejected. See
[endpoint rules](references/endpoint-rules.md) for the full validation.

## Bounded attach

Prefer `--duration` for exploratory attaches. The command blocks, prints
remaining time to stderr, and detaches automatically on timer or signal.

```bash
zeroins obi attach --duration=15m \
  --endpoint=https://otel-collector.observability.svc:4318
```

## Confirm telemetry

```bash
zeroins obi traces checkout --namespace=production --tail=20
```

See [traces query](references/traces-query.md) for the Jaeger-compatible API.

## Status and detach

```bash
zeroins obi status
zeroins obi detach
```

Before detaching, list active sessions with `zeroins sessions list -A`.
