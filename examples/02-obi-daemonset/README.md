# 02 — OBI DaemonSet attach

Attach OBI as a node-wide DaemonSet, generate HTTP traffic, and confirm
telemetry arrives at a sink.

## Requirements

- Linux Kubernetes cluster with BTF support
- kubectl, helm v4.x
- Cluster-admin-approved privileges for DaemonSets and ClusterRoles
- zeroins installed

## Walkthrough

Apply the sink and workload fixtures:

```bash
kubectl apply -f ../_fixtures/telemetry-sink.yaml
kubectl apply -f ../_fixtures/sample-http.yaml
kubectl rollout status deployment/telemetry-sink --timeout=180s
kubectl rollout status deployment/sample-http --timeout=180s
```

Review the plan before mutating:

```bash
zeroins obi attach --dry-run \
  --endpoint=http://telemetry-sink.default.svc.cluster.local:4318
```

Attach with a 10-minute bound:

```bash
zeroins obi attach --duration=10m \
  --endpoint=http://telemetry-sink.default.svc.cluster.local:4318
```

In another terminal, generate traffic:

```bash
kubectl apply -f ../_fixtures/traffic.yaml
kubectl wait --for=condition=Ready pod/traffic --timeout=120s
```

Confirm telemetry arrives:

```bash
kubectl logs deployment/telemetry-sink --tail=-1 | grep -E 'ResourceSpans|ResourceMetrics|ScopeSpans|ScopeMetrics'
```

List active sessions:

```bash
zeroins sessions list -A
```

## Detach

The `--duration=10m` attach detaches automatically when the timer fires. To
detach manually:

```bash
zeroins obi detach
```

To reap expired sessions:

```bash
zeroins sessions reap -A
```
