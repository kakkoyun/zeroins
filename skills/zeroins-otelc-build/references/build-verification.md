# Build verification

After running `otelc go build`, verify that instrumentation was injected.

## Check the build output

```bash
otelc go build -v ./... 2>&1 | grep -i inject
```

The verbose output should show injection messages for the packages in the
otelc catalog. If no injection messages appear, the build may have silently
skipped instrumentation.

## Common failure modes

- **Go version too old** — otelc requires Go 1.25 or newer. An older Go
  produces a normal binary without instrumentation.
- **Library not in catalog** — otelc only instruments its declared library
  and version rules. A library outside the catalog is not instrumented.
- **Build tags exclude instrumentation** — some build tags may prevent the
  instrumentation code from being compiled.

## What otelc does not do

otelc does not add telemetry for libraries outside its catalog. It does not
collect CPU profiles or RED metrics from runtime observation. For runtime
observation use OBI. For CPU profiles use the eBPF Profiler.
