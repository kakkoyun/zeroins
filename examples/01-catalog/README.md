# 01 — Catalog lookup (no cluster required)

This example runs on any OS, including macOS. No Kubernetes cluster, no Linux
kernel, no privileges.

## Requirements

- Go 1.24+
- zeroins installed: `go install github.com/kakkoyun/zeroins/cmd/...@latest`

## Walkthrough

Check whether OBI covers the workload's libraries:

```bash
zeroins obi lookup net/http
zeroins obi lookup google.golang.org/grpc
```

Check whether otelc covers them:

```bash
zeroins otelc lookup net/http
zeroins otelc lookup google.golang.org/grpc
```

Use `-o json` for machine-readable output:

```bash
zeroins obi lookup net/http -o json | jq
```

## Choosing between OBI and otelc

| Question | OBI | otelc |
| --- | --- | --- |
| Can you rebuild the service? | Not required | Required (Go 1.25+) |
| Target OS | Linux only | Any |
| Privileges needed | eBPF, host PID | None |
| Coverage | Supported Go libraries | Supported Go libraries |
| Telemetry type | RED metrics, library spans | Spans, metrics, logs |

If both cover your libraries and you cannot rebuild, choose OBI. If you can
rebuild and need non-Linux support, choose otelc.
