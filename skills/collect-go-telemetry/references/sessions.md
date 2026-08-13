# Sessions

Every successful attach labels the created resource
`zeroins.kakkoyun.dev/managed=true` and annotates it with `session-id`,
`attached-at`, `expires-at` (only under `--duration`), `endpoint`, and `mode`.

## List

```bash
zeroins sessions list -A
zeroins sessions list -A -o json
```

## Reap

```bash
zeroins sessions reap -A
```

`sessions reap` detaches every session whose `expires-at` has passed. It never
touches a session without `expires-at`, and never touches a resource missing
the managed label.

## Audit before detach

Before proposing any detach, list active sessions. This is step 7 of the
mandatory workflow. An unbounded attach (no `--duration`) needs a manual
detach and should appear in the sessions list until removed.
