# Preflight and doctor

`zeroins doctor` is the preflight check. Run it before any attach.

## What it checks

- **Tooling** — kubectl, helm, go are installed and on PATH.
- **Cluster** — context is set, API server is reachable, server version
  reported.
- **RBAC** — can-i create DaemonSets, ClusterRoles, patch Deployments.
- **Nodes** — kernel version, OS, architecture.

## Exit codes

- `0` on pass-or-warn.
- `1` on any hard failure. Do not attempt an attach if doctor exits 1.

## Modes

```bash
zeroins doctor
zeroins doctor --strict
zeroins doctor -o json
```

`--strict` promotes warnings to failures. `-o json` produces machine-readable
output for agent consumption.

## When doctor cannot run

If the cluster is unreachable, doctor exits 1. Proceed only while explicitly
recording preflight as unverified. Never claim preflight passed when it did
not run.
