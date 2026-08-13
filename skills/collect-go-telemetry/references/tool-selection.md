# Tool selection

Ask these questions before proposing a deployment:

1. Does the service run on Linux?
2. Can the team rebuild it or change the build command?
3. Can a privileged observer run on the host or in the workload pod?
4. Does the user need boundary telemetry, supported Go-library semantics, or
   CPU profiles?
5. Where should OTLP traces, metrics, and profiles be sent?

## Decision matrix

| Question | OBI | otelc | Profiler |
| --- | --- | --- | --- |
| Can you rebuild the service? | Not required | Required | Not required |
| Target OS | Linux only | Any | Linux only |
| Privileges needed | eBPF, host PID | None | eBPF, host PID, tracefs |
| Telemetry type | RED metrics, library spans | Spans, metrics, logs | CPU profiles |
| Deploy mode | DaemonSet or sidecar | Build-time | DaemonSet |

## When to use each

Use **OBI** when the service is already running on supported Linux and a
rebuild is not available. OBI observes supported protocols and Go libraries
from outside the process.

Use **otelc** when the team controls the build, needs a non-Linux target, or
cannot run a privileged eBPF observer. otelc requires Go 1.25 or newer.

Add the **eBPF Profiler** when the user needs whole-node CPU profiles. It is a
separate privileged DaemonSet and exports the Alpha OpenTelemetry profiles
signal.

For microarchitectural analysis (CPU caches, PMU counters, cache-to-cache
latency), use **perfgo** instead. perfgo answers "why is this code slow at the
hardware level?" while OBI answers "what is this running service doing?"
