# AGENTS.md — zeroins skill governance

Instructions for AI coding agents working on the `skills/` directory and the
`tools/skillcheck` validation gate.

---

## Skill layout

Each skill is a directory under `skills/` containing:

- `SKILL.md` — the lean entry point. Target under 120 lines, hard cap under 500
  logical lines (checked by `make check/skills`).
- `references/*.md` — progressive disclosure files loaded on demand. Move detail
  here when SKILL.md grows past the target.

A skill directory name must equal the `name:` field in its frontmatter.

## Frontmatter

SKILL.md begins with YAML frontmatter delimited by `---` lines. Allowed keys:

| Key | Required | Constraint |
| --- | --- | --- |
| `name` | yes | equals directory name, matches `^[a-z0-9]+(-[a-z0-9]+)*$`, ≤ 64 chars |
| `description` | yes | non-empty, ≤ 1024 chars |
| `license` | no | — |
| `compatibility` | no | ≤ 500 chars |
| `metadata` | no | — |

No other keys are permitted. Frontmatter is parsed with a real YAML parser
(`gopkg.in/yaml.v3`), not line matching. An unquoted `description` containing a
bare `: ` breaks the document while a regex still reports a plausible character
count for frontmatter no spec client can load.

## Registration triple

A skill counts as registered only when it appears in all three:

1. The directory `skills/<name>/`
2. The `plugins` array in `.claude-plugin/marketplace.json` (with
   `source` equal to `./skills/<name>`)
3. Both README indexes: the Available Skills table and the Repository Structure
   tree

`make check/skills` diffs all three in both directions. A skill with no entry
and an entry with no skill both fail.

The Available Skills table rows must match this shape:

```
| `skill-name` | `skills/skill-name/` | description |
```

The Repository Structure tree must use a bare `skills/` line with 2-space
indentation for skill directories:

```
skills/
  skill-name/
    SKILL.md
```

## Gate-bearing skills

Four skills can mutate a cluster and carry a confirmation gate:

- `collect-go-telemetry` — the spine; its `references/confirmation-gate.md` is
  the canonical copy
- `zeroins-obi-attach`
- `zeroins-ebpf-profiler-attach`
- `profile-go-with-perfgo`

`references/confirmation-gate.md` ships byte-identical in all four. The
canonical copy is `skills/collect-go-telemetry/references/confirmation-gate.md`.
`make check/skills` verifies byte-identity and that each gate-bearing SKILL.md
links to it before the first fenced block containing `attach` or `detach`.

## Version pins

Version pins quoted anywhere under `skills/**` must agree with the README
"Version pins" table. `make check/skills` verifies this.

## Links

Skills cite maintained upstream URLs rather than repo-relative paths. An
installed skill cannot see the repo tree. `examples/` directories are linked by
absolute GitHub URL.

Relative Markdown links under `skills/**` must resolve to existing files.
`make check/skills` checks this offline. External-URL link checking is a
separate follow-up.

## Evals

Gate-bearing skills ship `evals/evals.json` with hostile fixtures under
`evals/files/`. The schema mirrors the upstream
`ollygarden/opentelemetry-agent-skills` format: a top-level `skill` name and a
`cases` array, each case with a unique non-empty `id`, a non-empty `prompt`,
optional `files[]`, and at least one `expectation`. `make check/skills`
validates schema only; the eval harness is run manually and reported in the PR.

## Commits

Use conventional commits scoped by skill:

```
feat(skills): add skillcheck gate and registration surfaces
refactor(skills/obi-attach): split DaemonSet and sidecar sections
test(skills/perfgo): add confirmation-gate evals
```

## Running the gate

```bash
make check/skills    # validate skills only
make check           # full gate including skills
```

The gate is offline, no network, no cluster. It runs on both ubuntu and macOS
in CI via the existing `verify.yml` `check` job.
