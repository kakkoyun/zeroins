# Transport and TLS

`zeroins profiler` connects to the OTLP/gRPC profiles endpoint. TLS is
enabled by default.

## TLS

The default transport is TLS. The profiler verifies the server certificate.

## Insecure

Use `--insecure` only when the user explicitly approves plaintext transport.
Name the impact: the profiles data travels unencrypted across the network.

```bash
zeroins profiler attach --insecure \
  --endpoint=profiles-collector.observability.svc:4317
```

Do not default to `--insecure`. Do not infer approval from an earlier step.

## Endpoint format

The endpoint must be in `host:port` form. Do not include a scheme prefix,
path, userinfo, or query string.

- `profiles-collector.svc:4317` — accepted
- URLs with a scheme prefix — rejected
- URLs with a query string — rejected
