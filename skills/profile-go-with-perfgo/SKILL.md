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
code. It has no build or module dependency on zeroins. This skill covers its
execution modes, collection modes, and the investigation loop.

## Install perfgo

```bash
go install github.com/perfgo/perfgo@latest
```

Ensure `$(go env GOPATH)/bin` is on your `PATH`, then verify:

```bash
perfgo --help
```

## Execution modes

perfgo has two execution modes:

- **test** — runs a Go test binary under `perf record`/`perf stat`, collects
  PMU counters, and reports results. No cluster access required.
- **attach** — deploys a privileged sidecar to a Kubernetes pod and profiles
  a running process. Requires cluster-admin-approved privileges.

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
`branch-misses`) and raw hex events (`rNNN`) are supported:

```bash
perfgo test stat -- ./your/package -bench=. -benchmem -run=^$
perfgo test profile -e cache-misses -- ./your/package -bench=. -benchmem -run=^$
perfgo test profile -e r076 -- ./your/package -bench=.
```

Modifiers appended with `:`:

- `:u` — user-space only
- `:k` — kernel only
- `:p<N>` — sample period (e.g. `:p100000`)

```bash
perfgo test profile -e cache-misses:u -- ./your/package -bench=.
perfgo test profile -e cycles:u:p100000 -- ./your/package -bench=.
```

Raw event encodings are CPU/PMU-specific. Consult `perf list` or
`/sys/bus/event_source/devices/cpu/format/` for your CPU's available events.

## Numbered investigation loop

1. **Baseline** — run `perfgo test stat` to establish baseline PMU
   counters for the hot path.
   ```bash
   perfgo test stat -- ./your/package -bench=. -benchmem -run=^$
   ```
2. **Profile** — run `perfgo test profile` to collect a call graph and
   identify the hottest functions.
   ```bash
   perfgo test profile -e cache-misses -- ./your/package -bench=. -benchmem -run=^$
   ```
3. **Drill** — use `-e` with specific PMU counters (cache-misses,
   branch-misses) to classify the bottleneck.
4. **Cache analysis** — if cache misses dominate, run `cache-to-cache`
   to measure inter-core transfer latency.
   ```bash
   perfgo test cache-to-cache -- ./your/package -bench=BenchmarkName -benchtime=10s -run=^$
   ```
5. **Iterate** — apply a fix, re-run from step 1, compare.

## Per-mode requirements

| Mode | Cluster | Privileges | Kernel |
| --- | --- | --- | --- |
| test | No | None | Linux with perf_events |
| attach | Yes | Privileged sidecar | Linux with perf_events |

## Consent gate for attach

`perfgo attach` deploys a privileged sidecar. Before running it, obtain
explicit user confirmation against the same checklist as `zeroins obi attach`:

- the cluster context;
- the target namespace and pod;
- the privilege impact: privileged container, host PID, perf_events access;
- the profiling duration.

Do not infer consent from an earlier test run.

## Choosing between perfgo and zeroins

| Need | Tool |
| --- | --- |
| Microarchitectural analysis of code you can build | perfgo |
| Boundary telemetry from running services | zeroins obi |
| Whole-node continuous CPU profiles | zeroins profiler |
| Compile-time instrumentation without privileges | zeroins otelc |

perfgo answers "why is this code slow at the hardware level?" zeroins/OBI
answers "what is this running service doing?" zeroins/profiler answers "which
functions are consuming CPU across the whole node?"

## Boundaries

perfgo measures hardware performance counters. It does not modify source code,
instrument binaries at compile time, or collect distributed traces. For
distributed tracing use zeroins OBI or otelc. For continuous node-wide CPU
profiles use zeroins profiler.
