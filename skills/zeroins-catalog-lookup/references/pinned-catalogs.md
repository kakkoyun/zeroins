# Pinned catalogs

The OBI and otelc catalogs are embedded in the zeroins binary at build time.
They do not contact upstream services.

## OBI catalog

The OBI catalog is pinned to the release listed in the README "Version pins"
table. It maps Go import paths to supported library versions and the telemetry
OBI can infer for each.

## otelc catalog

The otelc catalog is pinned to the release listed in the README "Version pins"
table. It maps Go import paths to the libraries otelc can instrument at
compile time.

## When the catalog has no match

A valid query with no match exits 0 and prints guidance. This means the
library is not in the supported set. Add manual instrumentation or choose a
different tool.
