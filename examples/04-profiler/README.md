# 04 — eBPF Profiler

Deploy the dedicated profiling Collector, generate CPU load, and confirm the
profiles pipeline.

## Requirements

- Linux Kubernetes cluster with BTF support
- kubectl, helm v4.x
- Cluster-admin-approved privileges for DaemonSets
- zeroins installed

## Walkthrough

Apply the sink fixture:

```bash
kubectl apply -f ../_fixtures/telemetry-sink.yaml
kubectl rollout status deployment/telemetry-sink --timeout=180s
```

Review the plan:

```bash
zeroins profiler attach --dry-run \
  --endpoint=telemetry-sink.default.svc.cluster.local:4317 \
  --insecure
```

Attach:

```bash
zeroins profiler attach \
  --endpoint=telemetry-sink.default.svc.cluster.local:4317 \
  --insecure
```

Generate CPU load:

```bash
kubectl apply -f ../_fixtures/cpu-burn.yaml
kubectl wait --for=jsonpath='{.status.phase}'=Succeeded pod/cpu-burn --timeout=180s
```

Confirm profiles arrive:

```bash
kubectl logs deployment/telemetry-sink --tail=-1 | grep -E 'ResourceProfiles|ScopeProfiles'
```

## Detach

```bash
zeroins profiler detach
```

To reap expired sessions:

```bash
zeroins sessions reap -A
```
