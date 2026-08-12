# zeroins

`zeroins` is a small toolkit for observing Go services without changing their
source. It supports release-pinned catalog lookups, OBI deployment helpers, and
the OpenTelemetry eBPF Profiler.

The two Kubernetes commands are experimental. They deploy privileged eBPF
observers and are not production installers. Review the rendered resources,
cluster context, telemetry destination, and security impact before using them.

## Install

Go 1.24 or newer is required to build zeroins.

```bash
go install github.com/kakkoyun/zeroins/cmd/...@latest
```

This installs four commands:

| Command | Purpose |
| --- | --- |
| `obi-integration` | Search the embedded OBI v0.10.0 Go-library support matrix |
| `otelc-aspect` | Search the embedded otelc v1.0.1 supported-library catalog |
| `kubectl-obi` | Attach, inspect, query, or detach OBI in Kubernetes |
| `kubectl-profiler` | Deploy, inspect, or remove the eBPF profiling Collector |

Kubectl discovers executables named `kubectl-<name>` as plugins. After
installation, use `kubectl obi ...` and `kubectl profiler ...`.

Run any command without installing it:

```bash
go run github.com/kakkoyun/zeroins/cmd/obi-integration@latest net/http
go run github.com/kakkoyun/zeroins/cmd/otelc-aspect@latest google.golang.org/grpc
go run github.com/kakkoyun/zeroins/cmd/kubectl-obi@latest version
go run github.com/kakkoyun/zeroins/cmd/kubectl-profiler@latest version
```

## Install the Agent Skill

The repository follows the Agent Skills directory convention:

```bash
npx skills add kakkoyun/zeroins --all
```

The `collect-go-telemetry` skill helps an agent choose between OBI, otelc, and
the eBPF Profiler. It requires explicit confirmation before cluster mutations.

## Offline integration lookups

The catalog commands do not contact upstream services. Their output changes only
when zeroins updates its embedded release snapshot.

```bash
obi-integration gin
otelc-aspect github.com/gin-gonic/gin
```

A valid query that has no match exits successfully and prints guidance. Usage or
internal failures exit with code 1.

## OBI wrapper

OBI requires supported Linux kernels, BTF, process access, and eBPF privileges.
The DaemonSet can observe workloads across a node. The sidecar runs privileged,
enables shared process namespaces, and restarts the selected deployment.

Both modes require an explicit OTLP HTTP(S) endpoint. Endpoint URLs containing
userinfo, queries, or fragments are rejected to keep credentials out of Helm
arguments and release values.

```bash
# Node-wide DaemonSet in obi-system.
kubectl obi attach \
  --endpoint=https://otel-collector.observability.svc:4318

# One deployment in the current context namespace.
kubectl obi attach checkout \
  --mode=sidecar \
  --endpoint=https://otel-collector.observability.svc:4318

kubectl obi status
kubectl obi detach
```

Sidecar attach is idempotent. zeroins annotates the pod template, records the
original `shareProcessNamespace` state, and restores that state on detach. It
refuses to remove an `obi` container that it did not create.

Query a Jaeger-compatible HTTP API with `OTEL_BACKEND`:

```bash
OTEL_BACKEND=https://jaeger.example \
  kubectl obi traces checkout --namespace=production --tail=20
```

The namespace becomes a `k8s.namespace.name` Jaeger tag filter.

## Profiler wrapper

`kubectl-profiler` requires an OTLP/gRPC profiles endpoint in `host:port` form.
TLS is enabled by default. Use `--insecure` only for an explicitly approved
plaintext destination.

```bash
kubectl profiler attach \
  --endpoint=profiles-collector.observability.svc:4317

kubectl profiler status
kubectl profiler detach
```

The command writes Helm values to a mode-0600 temporary JSON file and removes it
on every exit path. It deploys one dedicated profiling Collector per node. The
profiling preset grants host PID visibility, privileged execution, and a tracefs
mount.

zeroins does not install a profiles backend, manage credentials, add custom
headers, or disable certificate verification.

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

`make check` formats, vets, tests with the race detector, and builds every
package. `make check/helm` uses Helm v4.2.3 to render both pinned charts and
asserts their image, security, endpoint, and profiles-pipeline contracts.

A separate Linux integration gate uses a disposable, directly hosted Kubernetes
cluster and runs real eBPF probes:

```bash
make check/integration
```

The integration gate must run on a supported Linux host. Nested clusters such as
kind and minikube are unsupported by the pinned profiler because their node
containers use a different PID namespace. The checked-in workflow therefore
starts K3s directly on the Linux runner. The gate fails rather than reporting
success when the host cannot run the probes.

## Status and scope

v0.1.0 supports command interfaces only. All Go packages remain under
`internal/`; zeroins does not expose a supported Go library API. Krew manifests,
release archives, GoReleaser, managed backends, and authentication-header
management are out of scope.

The command prototypes and research began in the Apache-2.0
`kakkoyun/gopherconuk-26` talk repository. They were imported as a fresh MIT
codebase and relicensed by the original author. See [`docs/research`](docs/research)
for focused evidence and provenance.

## License

MIT. See [`LICENSE`](LICENSE).
