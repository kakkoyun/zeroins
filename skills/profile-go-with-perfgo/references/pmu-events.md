# PMU events

Use `-e` to specify PMU events. Named events and raw hex events are supported.

## Named events

```bash
perfgo test profile -e cache-misses -- ./your/package -bench=.
perfgo test profile -e cycles -- ./your/package -bench=.
perfgo test profile -e branch-misses -- ./your/package -bench=.
```

## Raw hex events

Raw events use the `rNNN` format where `NNN` is the hex encoding of the
performance event select register:

```bash
perfgo test profile -e r076 -- ./your/package -bench=.
```

Raw event encodings are CPU/PMU-specific. Consult `perf list` or
`/sys/bus/event_source/devices/cpu/format/` for your CPU's available events.

## Modifiers

Modifiers are appended with `:`:

- `:u` — user-space only
- `:k` — kernel only
- `:p` — precise-event sampling (reduces skid)

Use `perfgo test stat --count N` or `perfgo test profile --count N` to set
the sample period (how often perf takes a sample).

```bash
perfgo test profile -e cache-misses:u -- ./your/package -bench=.
perfgo test profile -e cycles:u -- ./your/package -bench=.
```

Modifiers can be combined: `cycles:u:p` means user-space cycles with precise
event sampling.
