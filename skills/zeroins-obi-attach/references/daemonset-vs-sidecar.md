# DaemonSet vs sidecar

OBI has two deployment modes. Choose based on observation scope.

## DaemonSet

A DaemonSet runs one OBI pod per node. It observes workloads across the entire
node. Use DaemonSet mode when you need node-wide observation and do not want
to modify a specific deployment.

```bash
zeroins obi attach --endpoint=https://otel-collector.observability.svc:4318
```

The DaemonSet runs in `obi-system` by default and requires ClusterRoles for
eBPF and host PID access.

## Sidecar

A sidecar injects an OBI container into a specific deployment's pod template.
Use sidecar mode when you need to observe one deployment and can tolerate a
rollout.

```bash
zeroins obi attach checkout --mode=sidecar \
  --endpoint=https://otel-collector.observability.svc:4318
```

The sidecar sets `OTEL_EBPF_AUTO_TARGET_EXE=*`, runs privileged, and enables
shared process namespaces. zeroins records ownership and the prior
`shareProcessNamespace` value. It refuses to remove an `obi` container that it
did not manage.

## Key differences

| Aspect | DaemonSet | Sidecar |
| --- | --- | --- |
| Scope | Node-wide | One deployment |
| Rollout | No | Yes (restarts the deployment) |
| Namespace | obi-system | Target namespace |
| Cleanup | Delete DaemonSet | Restore pod template |
