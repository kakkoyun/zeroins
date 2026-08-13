# Boundaries

OBI, otelc, and the eBPF Profiler each cover a bounded surface. They do not
infer semantics outside their declared coverage.

## OBI

OBI observes supported protocols and Go libraries from outside the process.
It does not infer business events or unsupported internal functions. Add
manual instrumentation when the requested semantics fall outside OBI's
declared library coverage.

## otelc

otelc instruments its declared library and version rules at compile time. It
does not add telemetry for libraries outside its catalog. Non-Linux targets
work because instrumentation is build-time, not runtime.

## eBPF Profiler

The eBPF Profiler reports CPU profiles but does not make the OpenTelemetry
profiles signal stable. It is a separate privileged DaemonSet and does not
collect distributed traces or RED metrics.

## perfgo

perfgo measures hardware performance counters. It does not modify source
code, instrument binaries at compile time, or collect distributed traces.
For distributed tracing use OBI or otelc. For continuous node-wide CPU
profiles use the eBPF Profiler.
