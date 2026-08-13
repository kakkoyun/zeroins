# 03 — OBI sidecar attach

Attach OBI as a sidecar to a single deployment. Demonstrates idempotent
re-attach and `shareProcessNamespace` capture and restore.

## Requirements

- Linux Kubernetes cluster with BTF support
- kubectl, helm v4.x
- RBAC to patch deployments in the target namespace
- zeroins installed

## Walkthrough

Apply the sink and workload fixtures:

```bash
kubectl apply -f ../_fixtures/telemetry-sink.yaml
kubectl apply -f ../_fixtures/sample-http.yaml
kubectl rollout status deployment/telemetry-sink --timeout=180s
kubectl rollout status deployment/sample-http --timeout=180s
```

Review the plan:

```bash
zeroins obi attach sample-http --mode=sidecar --dry-run \
  --endpoint=http://telemetry-sink.default.svc.cluster.local:4318
```

Attach (idempotent — running it twice creates exactly one obi container):

```bash
zeroins obi attach sample-http --mode=sidecar \
  --endpoint=http://telemetry-sink.default.svc.cluster.local:4318
```

Re-attach to confirm idempotency:

```bash
zeroins obi attach sample-http --mode=sidecar \
  --endpoint=http://telemetry-sink.default.svc.cluster.local:4318
```

Verify only one obi container exists:

```bash
kubectl get deployment sample-http -o jsonpath='{range .spec.template.spec.containers[*]}{.name}{"\n"}{end}' | grep -c '^obi$'
```

Confirm `shareProcessNamespace` was set:

```bash
kubectl get deployment sample-http -o jsonpath='{.spec.template.spec.shareProcessNamespace}'
```

## Detach

```bash
zeroins obi detach sample-http --mode=sidecar
```

Verify the obi container is gone and `shareProcessNamespace` is restored:

```bash
kubectl get deployment sample-http -o jsonpath='{range .spec.template.spec.containers[*]}{.name}{"\n"}{end}' | grep '^obi$' || echo "no obi container"
kubectl get deployment sample-http -o jsonpath='{.spec.template.spec.shareProcessNamespace}'
```
