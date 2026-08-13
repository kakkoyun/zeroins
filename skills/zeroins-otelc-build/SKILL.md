---
name: zeroins-otelc-build
description: |
  Build Go binaries with otelc compile-time instrumentation. Covers the
  otelc go build command, the Go 1.25+ floor, injection verification, and
  non-Linux targets. USE WHEN: "otelc go build", "compile-time
  instrumentation", "build with otelc", or "instrument without privileges".
license: MIT
compatibility: Requires Go 1.25+ for building the target with otelc go build. No Kubernetes required. Works on any OS.
---

# Build with otelc

otelc instruments Go binaries at compile time. No Kubernetes, no Linux, no
privileges required. The team must control the build command.

## Install otelc

```bash
go install go.opentelemetry.io/otelc/tool/cmd/otelc@v1.0.1
```

## Build

```bash
otelc go build -o ./myapp ./cmd/myapp
```

## Verify injection

After building, check `.otelc-build/matched.json` for the rules that fired.
See [build verification](references/build-verification.md) for what to check and
common failure modes.

```bash
cat .otelc-build/matched.json | jq .
```

## Non-Linux targets

otelc works on any OS because instrumentation is build-time, not runtime. Use
it when the team controls the build or needs a non-Linux target that a
runtime eBPF observer cannot reach.

## Choosing otelc

Use otelc when the team controls the build, needs a non-Linux target, or
cannot run a privileged eBPF observer. otelc requires Go 1.25 or newer. For
runtime observation without a rebuild, use OBI instead.
