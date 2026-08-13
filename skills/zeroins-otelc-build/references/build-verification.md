# Build verification

After running `otelc go build`, verify that instrumentation was injected.

## Check the matched rules artifact

otelc writes the rules that fired to `.otelc-build/matched.json`:

```bash
cat .otelc-build/matched.json | jq .
```

The JSON lists the instrumentation rules that matched packages in your module.
If the file is missing or empty, instrumentation did not run.

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
