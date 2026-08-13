# Perfgo attach notes

`perfgo attach` deploys a privileged sidecar. The confirmation gate in
[confirmation gate](confirmation-gate.md) applies. The items below are
perfgo-specific additions to that checklist, not replacements.

## perf_event_paranoid

perfgo attach requires access to `perf_event_open`. The kernel's
`perf_event_paranoid` setting controls access:

- `2` (default) — limited access, may block perfgo.
- `1` — allows per-process profiling.
- `0` — allows system-wide profiling.
- `-1` — no restrictions, but exposes the node to broader perf access.

Do not propose setting `perf_event_paranoid=-1` on the node without naming the
host-level impact: it lowers the bar for perf_events access for all processes
on that node, not just the target.

## Host-level impact

The privileged sidecar has:

- **Privileged container** — full host access.
- **Host PID** — can see and profile any process on the node.
- **perf_events access** — reads CPU performance counters from the kernel.

## Profiling duration

State the profiling duration explicitly. An unbounded attach needs a manual
detach and a `sessions` entry. Prefer `--duration` for exploratory runs.

## Per-mode requirements

| Mode | Cluster | Privileges | Kernel |
| --- | --- | --- | --- |
| test | No | None | Linux with perf_events |
| attach | Yes | Privileged sidecar | Linux with perf_events |
