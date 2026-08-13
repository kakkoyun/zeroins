---
name: zeroins-catalog-lookup
description: |
  Search OBI and otelc Go-library support catalogs through zeroins. Covers
  output modes (table, json, markdown) and the deprecated obi-integration and
  otelc-aspect shims. USE WHEN: "obi lookup", "otelc lookup", "catalog
  search", "library coverage", or "is X supported".
license: MIT
compatibility: Requires Go 1.24+. No Kubernetes, no Linux, runs on any OS.
---

# Catalog lookup

The catalog commands do not contact upstream services. Their output changes
only when zeroins updates its embedded release snapshot.

## OBI lookup

```bash
zeroins obi lookup net/http
zeroins obi lookup gin
```

## otelc lookup

```bash
zeroins otelc lookup net/http
zeroins otelc lookup github.com/gin-gonic/gin
```

## Output modes

The default output format is `table`. Use `-o json` for machine-readable output
or `-o markdown` to reproduce the v0.1 markdown bytes.

```bash
zeroins obi lookup net/http -o json | jq
zeroins otelc lookup net/http -o markdown
```

## Exit codes

A valid query that has no match exits successfully and prints guidance. Usage
or internal failures exit with code 1.

## Deprecated shims

The `obi-integration` and `otelc-aspect` binaries still work but print a
deprecation notice on stderr. Use `zeroins obi lookup` and
`zeroins otelc lookup` instead. See [pinned catalogs](references/pinned-catalogs.md)
for the release snapshot each catalog embeds.
