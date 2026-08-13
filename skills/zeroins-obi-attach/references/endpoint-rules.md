# Endpoint rules

OBI requires an explicit OTLP HTTP(S) base endpoint. OBI derives the signal
paths from it.

## Validation

Endpoint URLs containing **userinfo**, **queries**, or **fragments** are
rejected. This keeps credentials out of Helm arguments and release values.

- `https://otel-collector.observability.svc:4318` — accepted
- URLs with userinfo (credentials before the host) — rejected
- URLs with a query string — rejected
- URLs with a fragment — rejected

## Credentials

Do not place credentials in an endpoint URL. If the collector requires
authentication, configure it at the collector side, not in the endpoint
passed to zeroins.
