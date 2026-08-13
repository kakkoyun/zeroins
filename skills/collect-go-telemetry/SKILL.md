---
name: collect-go-telemetry
description: |
  Choose and operate zero-code Go telemetry tools through the unified zeroins
  command. This is the spine: it owns the mandatory 7-step workflow, tool
  selection, preflight, session audit, and boundaries. Load a sub-skill for
  tool-specific detail. USE WHEN: "collect go telemetry", "instrument a Go
  service without code changes", "attach OBI", "build with otelc", "profile Go
  with eBPF", or "zeroins".
license: MIT
compatibility: Requires Go 1.24+ for zeroins. Kubernetes actions also require kubectl, Helm, Linux eBPF support, and cluster-admin-approved privileges. otelc requires Go 1.25+.
---

# Collect Go telemetry

Use this workflow to observe a Go service without editing its source. All
commands route through the unified `zeroins` binary.

## Install zeroins

```bash
go install github.com/kakkoyun/zeroins/cmd/...@latest
```

The deprecated `obi-integration` and `otelc-aspect` binaries still work but
print a deprecation notice. Use `zeroins obi lookup` and
`zeroins otelc lookup` instead.

## Mandatory agent workflow

Follow these steps in order. Do not skip preflight or the confirmation gate.

### 1. Preflight

Run `zeroins doctor -o json` first. Abort on any hard failure. See
[preflight and doctor](references/preflight-doctor.md) for what it checks.

```bash
zeroins doctor -o json
```

### 2. Coverage

Check whether the workload's libraries are covered. These lookups are offline
and require no confirmation. See [tool selection](references/tool-selection.md)
for when to choose OBI, otelc, or the eBPF Profiler.

```bash
zeroins obi lookup net/http -o json
zeroins otelc lookup net/http -o json
```

### 3. Plan review

Run `attach --dry-run -o json` and present the plan object to the user. The
plan contains the resolved context, namespace, mode, endpoint, privilege
impact, exact command argv, and rendered Helm values or sidecar patch.

### 4. Confirmation gate

Do not run `attach` or `detach` until the checklist in
[confirmation gate](references/confirmation-gate.md) is complete and the user
has explicitly approved.

### 5. Bounded attach

Prefer `--duration` for exploratory attaches. The command blocks, prints
remaining time to stderr, and detaches automatically when the timer fires or on
SIGINT/SIGTERM.

### 6. Confirm telemetry

```bash
zeroins obi traces checkout --namespace=production --tail=20
```

### 7. Audit and clean up

Before proposing any detach, list active sessions. See
[sessions](references/sessions.md) for the full lifecycle.

```bash
zeroins sessions list -A -o json
zeroins sessions reap -A
```

## Sub-skills

Load these for tool-specific commands and detail:

- **zeroins-catalog-lookup** — `obi lookup`, `otelc lookup`, output modes,
  deprecated shims
- **zeroins-otelc-build** — `otelc go build`, Go 1.25+ floor, injection
  verification
- **zeroins-obi-attach** — `obi values/attach/status/traces/detach`,
  DaemonSet vs sidecar, endpoint rules
- **zeroins-ebpf-profiler-attach** — `profiler values/attach/status/detach`,
  transport, TLS, pinned components
- **profile-go-with-perfgo** — perfgo microarchitectural analysis

## Boundaries

OBI, otelc, and the eBPF Profiler each cover a bounded surface. See
[boundaries](references/boundaries.md) for what they cannot infer or collect.
Add manual instrumentation when the requested semantics fall outside those
limits.
