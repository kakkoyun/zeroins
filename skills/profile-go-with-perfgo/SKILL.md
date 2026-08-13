---
name: profile-go-with-perfgo
description: |
  Profile Go code with perfgo, a third-party Apache-2.0 tool for
  microarchitectural analysis. Covers test and attach modes, stat/profile/
  cache-to-cache collection, PMU event specification, and the numbered
  investigation loop. USE WHEN: "perfgo", "cpu profiling", "pmu events",
  "microarchitectural analysis", "perf profiling", or "cache-to-cache".
license: MIT
compatibility: perfgo is a third-party Apache-2.0 tool with no build or module dependency on zeroins. Requires Linux with perf_events support. The attach mode deploys a privileged sidecar.
---

# Profile Go code with perfgo

perfgo is a third-party Apache-2.0 tool for microarchitectural analysis of Go
code. It has no build or module dependency on zeroins.

perfgo answers "why is this code slow at the hardware level?" For boundary
telemetry from a running service, use OBI. For whole-node continuous CPU
profiles, use the eBPF Profiler.

## Install perfgo

```bash
go install github.com/perfgo/perfgo@latest
```

Ensure `$(go env GOPATH)/bin` is on your `PATH`, then verify:

```bash
perfgo --help
```

## Execution modes

- **test** — runs a Go test binary under `perf record`/`perf stat`, collects
  PMU counters, and reports results. No cluster access required.
- **attach** — deploys a privileged sidecar to a Kubernetes pod and profiles
  a running process. Requires cluster-admin-approved privileges.

Do not run `attach` or `detach` until the checklist in
[confirmation gate](references/confirmation-gate.md) is complete and the user
has explicitly approved. See [perfgo attach notes](references/perfgo-attach-notes.md)
for perfgo-specific gate items.

## Collection modes

Three collection modes select what perfgo measures. They are subcommands under
`test` or `attach`:

- **stat** — collects PMU counter statistics (cycles, instructions, cache
  misses, branch misses). Lightweight, no perf record overhead.
- **profile** — collects full stack profiles with `perf record`. Heavier but
  produces call-graph data.
- **cache-to-cache** — measures cache-to-cache transfer latency between cores.
  Specialised for NUMA and inter-core communication analysis.

## PMU event specification

Use `-e` to specify PMU events. Named events (`cache-misses`, `cycles`,
`branch-misses`) and raw hex events (`rNNN`) are supported. See
[PMU events](references/pmu-events.md) for modifiers and raw encodings.

```bash
perfgo test stat -- ./your/package -bench=. -benchmem -run=^$
perfgo test profile -e cache-misses -- ./your/package -bench=. -benchmem -run=^$
```

## Numbered investigation loop

1. **Baseline** — run `perfgo test stat` to establish baseline PMU counters.
   ```bash
   perfgo test stat -- ./your/package -bench=. -benchmem -run=^$
   ```
2. **Profile** — run `perfgo test profile` to collect a call graph.
   ```bash
   perfgo test profile -e cache-misses -- ./your/package -bench=. -benchmem -run=^$
   ```
3. **Drill** — use `-e` with specific PMU counters to classify the bottleneck.
4. **Cache analysis** — if cache misses dominate, run `cache-to-cache`.
   ```bash
   perfgo test cache-to-cache -- ./your/package -bench=BenchmarkName -benchtime=10s -run=^$
   ```
5. **Iterate** — apply a fix, re-run from step 1, compare.

## Boundaries

perfgo measures hardware performance counters. It does not modify source code,
instrument binaries at compile time, or collect distributed traces.
