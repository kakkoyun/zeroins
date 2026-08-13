# Pinned components

The eBPF Profiler deploys a dedicated profiling Collector per node. The
versions are pinned in the README "Version pins" table.

## Chart

The Collector Helm chart version is listed in the README "Version pins" table.

## Image

The profiling Collector uses a dedicated image:
`otel/opentelemetry-collector-ebpf-profiler`. The image version is listed in
the README "Version pins" table.

## Receiver

The profiler receiver version inside that image is listed in the README
"Version pins" table. The receiver and upstream repository versions differ
because the Collector image was built before the next calendar-week
profiler tag.

## Pipeline

The command enables only a profiles pipeline. It does not configure traces,
metrics, or logs pipelines in the profiling Collector.

## Values file handling

The command writes Helm values to a mode-0600 temporary JSON file and removes
it on every exit path.
