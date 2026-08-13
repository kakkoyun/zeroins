# zeroins

`zeroins` is a small toolkit for observing Go services without changing their
source. It supports release-pinned catalog lookups, OBI deployment helpers,
and the OpenTelemetry eBPF Profiler.

The two Kubernetes commands are experimental. They deploy privileged eBPF
observers and are not production installers. Review the rendered resources,
cluster context, telemetry destination, and security impact before using them.

## Install

Go 1.24 or newer is required to build zeroins.

```bash
go install github.com/kakkoyun/zeroins/cmd/...@latest
```

This installs five commands:

| Command | Purpose |
| --- | --- |
| `zeroins` | Unified entry point: doctor, obi, profiler, otelc, sessions, version |
| `kubectl-obi` | Attach, inspect, query, or detach OBI in Kubernetes (plugin) |
| `kubectl-profiler` | Deploy, inspect, or remove the eBPF profiling Collector (plugin) |
| `obi-integration` | Deprecated; use `zeroins obi lookup` |
| `otelc-aspect` | Deprecated; use `zeroins otelc lookup` |

Kubectl discovers executables named `kubectl-<name>` as plugins. After
installation, use `kubectl obi ...` and `kubectl profiler ...`.

Run any command without installing it:

```bash
go run github.com/kakkoyun/zeroins/cmd/zeroins@latest obi lookup net/http
go run github.com/kakkoyun/zeroins/cmd/zeroins@latest version
```

## Available Skills

The repository follows the Agent Skills directory convention:

```bash
npx skills add kakkoyun/zeroins --all
```

| Skill | Path | Description |
| --- | --- | --- |
| `collect-go-telemetry` | `skills/collect-go-telemetry/` | Choose and operate zero-code Go telemetry tools through the unified zeroins command. Requires explicit confirmation before cluster mutations. |
| `profile-go-with-perfgo` | `skills/profile-go-with-perfgo/` | Profile Go code with perfgo for microarchitectural analysis. Covers test and attach modes, PMU events, and the investigation loop. |

## Repository Structure

```
skills/
  collect-go-telemetry/
    SKILL.md
  profile-go-with-perfgo/
    SKILL.md
```

## Typical workflow

1. **Preflight** — can this cluster run eBPF observers?

   ```bash
   zeroins doctor
   ```

2. **Coverage** — are the workload's libraries covered?

   ```bash
   zeroins obi lookup net/http
   zeroins otelc lookup net/http
   ```

3. **Review** — context, privileges, rendered values, no mutations:

   ```bash
   zeroins obi attach --dry-run \
     --endpoint=https://otel-collector.observability.svc:4318
   ```

4. **Attach** — bounded, self-removing:

   ```bash
   zeroins obi attach --duration=15m \
     --endpoint=https://otel-collector.observability.svc:4318
   ```

5. **Confirm** — telemetry arrives:

   ```bash
   zeroins obi traces checkout --namespace=production --tail=20
   ```

6. **Audit and clean up**:

   ```bash
   zeroins sessions list -A
   zeroins sessions reap -A
   ```

## Requirements

### Catalog (no cluster)

- Go 1.24+
- No Kubernetes, no Linux, runs on any OS

### OBI

- Linux Kubernetes cluster with BTF support
- kubectl, helm v4.x
- Cluster-admin-approved privileges for DaemonSets and ClusterRoles
- Explicit OTLP HTTP(S) base endpoint (no credentials in the URL)

### Profiler

- Linux Kubernetes cluster with BTF support
- kubectl, helm v4.x
- Cluster-admin-approved privileges for DaemonSets
- OTLP/gRPC profiles endpoint in `host:port` form

### otelc

- Go 1.25+ (for building the target with `otelc go build`)
- No Kubernetes required

## Choosing between the three tools

| Question | OBI | otelc | Profiler |
| --- | --- | --- | --- |
| Can you rebuild the service? | Not required | Required | Not required |
| Target OS | Linux only | Any | Linux only |
| Privileges needed | eBPF, host PID | None | eBPF, host PID, tracefs |
| Telemetry type | RED metrics, library spans | Spans, metrics, logs | CPU profiles |
| Deploy mode | DaemonSet or sidecar | Build-time | DaemonSet |

Use OBI when the service is already running on supported Linux and a rebuild
is not available. Use otelc when the team controls the build or needs non-Linux
support. Add the Profiler when whole-node CPU profiles are needed.

## Command reference

```
zeroins
├── doctor                        preflight: tooling, cluster, RBAC, nodes
├── obi
│   ├── lookup <library>          search OBI Go-library support matrix
│   ├── attach [deployment]       --mode --endpoint -n --duration --dry-run -o
│   ├── status                    -A -o
│   ├── traces <deployment>       --tail --follow -o
│   ├── values                    print exactly what attach would apply
│   └── detach [deployment]       --mode -n --dry-run
├── otelc
│   └── lookup <library>          search otelc supported-library catalog
├── profiler
│   ├── attach                    --endpoint --insecure -n --duration --dry-run -o
│   ├── status                    -A -o
│   ├── values
│   └── detach                    -n --dry-run
├── sessions
│   ├── list                      -A -o
│   └── reap                      -A --dry-run
└── version                       -o
```

### Offline catalog lookups

The catalog commands do not contact upstream services. Their output changes only
when zeroins updates its embedded release snapshot.

```bash
zeroins obi lookup gin
zeroins otelc lookup github.com/gin-gonic/gin
```

The default output format is `table`. Use `-o json` for machine-readable output
or `-o markdown` to reproduce the v0.1 markdown bytes.

A valid query that has no match exits successfully and prints guidance. Usage or
internal failures exit with code 1.

### OBI wrapper

OBI requires supported Linux kernels, BTF, process access, and eBPF privileges.
The DaemonSet can observe workloads across a node. The sidecar runs privileged,
enables shared process namespaces, and restarts the selected deployment.

Both modes require an explicit OTLP HTTP(S) base endpoint. OBI derives the
signal paths from it. Endpoint URLs containing userinfo, queries, or fragments
are rejected to keep credentials out of Helm arguments and release values.

```bash
# Node-wide DaemonSet in obi-system.
zeroins obi attach \
  --endpoint=https://otel-collector.observability.svc:4318

# One deployment in the current context namespace.
zeroins obi attach checkout \
  --mode=sidecar \
  --endpoint=https://otel-collector.observability.svc:4318

zeroins obi status
zeroins obi detach
```

Sidecar attach is idempotent. zeroins annotates the pod template, records the
original `shareProcessNamespace` state, and restores that state on detach. It
refuses to remove an `obi` container that it did not create.

`--dry-run` prints the plan (context, namespace, privilege impact, exact
command argv, rendered values) without mutating. `--duration` attaches, blocks
for the given time, and detaches automatically on timer or signal.

`zeroins obi values` prints the exact Helm values or sidecar patch that attach
would apply, with no cluster access required.

Query a Jaeger-compatible HTTP API with `OTEL_BACKEND`:

```bash
OTEL_BACKEND=https://jaeger.example \
  zeroins obi traces checkout --namespace=production --tail=20
```

The namespace becomes a `k8s.namespace.name` Jaeger tag filter.

### Profiler wrapper

`zeroins profiler` requires an OTLP/gRPC profiles endpoint in `host:port` form.
TLS is enabled by default. Use `--insecure` only for an explicitly approved
plaintext destination.

```bash
zeroins profiler attach \
  --endpoint=profiles-collector.observability.svc:4317

zeroins profiler status
zeroins profiler detach
```

The command writes Helm values to a mode-0600 temporary JSON file and removes it
on every exit path. It deploys one dedicated profiling Collector per node. The
profiling preset grants host PID visibility, privileged execution, and a tracefs
mount.

`--dry-run` and `--duration` work the same as for OBI. `zeroins profiler values`
prints the exact Helm values that attach would apply.

zeroins does not install a profiles backend, manage credentials, add custom
headers, or disable certificate verification.

### Sessions

Every successful attach labels the created resource
`zeroins.kakkoyun.dev/managed=true` and annotates it with `session-id`,
`attached-at`, `expires-at` (only under `--duration`), `endpoint`, and `mode`.

```bash
zeroins sessions list -A
zeroins sessions reap -A
```

`sessions reap` detaches every session whose `expires-at` has passed. It never
touches a session without `expires-at`, and never touches a resource missing
the managed label.

### Doctor

```bash
zeroins doctor
zeroins doctor --strict
zeroins doctor -o json
```

Checks tooling (kubectl, helm, go), cluster (context, reachability, server
version), RBAC (can-i create DaemonSets, ClusterRoles, patch Deployments), and
nodes (kernel version, OS, architecture). Exit `0` on pass-or-warn, `1` on any
hard failure. `--strict` promotes warnings to failures.

## Version pins

| Contract | Pin |
| --- | --- |
| OBI and OBI image | v0.10.0 |
| OBI Helm chart | 0.10.0 |
| otelc catalog | v1.0.1 |
| OpenTelemetry Collector Helm chart | 0.166.0 |
| Profiling Collector image | `otel/opentelemetry-collector-ebpf-profiler:0.158.0` |
| Profiler receiver in that image | v0.0.202632 |
| Latest profiler upstream tag at release preparation | v0.0.202633 |
| Helm used by contract verification | v4.2.3 |

The profiler receiver and upstream repository versions differ because Collector
0.158.0 was built before the next calendar-week profiler tag.

## Verify the repository

```bash
make check
make check/helm
```

`make check` formats, vets, tests with the race detector, builds every package,
cross-builds, and asserts the examples fixtures match the integration script
references. `make check/helm` uses Helm v4.2.3 to render both pinned charts and
asserts their image, security, endpoint, and profiles-pipeline contracts.

A separate Linux integration gate runs real eBPF probes against the Kubernetes
cluster selected by `KUBECONFIG`. It installs privileged, host-level DaemonSets
and deletes its fixed-name test resources, so it requires an explicit opt-in:

```bash
ZEROINS_INTEGRATION_ALLOW_CURRENT_CONTEXT=1 make check/integration
```

Run it only on a disposable, directly hosted Kubernetes cluster on a supported
Linux host. Nested-cluster testing could not provide the host PID identity
required by the pinned profiler, so the checked-in workflow starts disposable
K3s directly on the Linux runner. The gate fails rather than reporting success
when the host cannot run the probes.

## Status and scope

v0.2.0 supports the unified `zeroins` command with preflight, dry-run, bounded
attach, session tracking, and structured output. All Go packages remain under
`internal/`; zeroins does not expose a supported Go library API. Krew manifests,
release archives, GoReleaser, managed backends, and authentication-header
management are out of scope.

The command prototypes and research began in the Apache-2.0
`kakkoyun/gopherconuk-26` talk repository. They were imported as a fresh MIT
codebase and relicensed by the original author. See [`docs/research`](docs/research)
for focused evidence and provenance.

## License

MIT. See [`LICENSE`](LICENSE).
