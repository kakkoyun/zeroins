# Traces query

Query a Jaeger-compatible HTTP API with `OTEL_BACKEND`:

```bash
OTEL_BACKEND=https://jaeger.example \
  zeroins obi traces checkout --namespace=production --tail=20
```

The namespace becomes a `k8s.namespace.name` Jaeger tag filter.

## Options

- `--tail` — number of recent traces to fetch.
- `--follow` — stream new traces as they arrive.
- `-o json` — machine-readable output.

## What traces show

OBI produces RED metrics and library spans for supported Go libraries. Traces
show the request flow through instrumented libraries. OBI does not infer
business events or unsupported internal functions.
