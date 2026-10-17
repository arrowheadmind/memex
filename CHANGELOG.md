# Changelog

Notable changes to memex, newest first. The version is the git tag (IMPLEMENTATION.md,
"Packaging"); a build that is not a release reports `0.0.0-dev`. A change to the shape of
text output is an interface change and is listed under "Changed output" (SPEC.md §17.3).

## 0.1.0 (2026-10-17)

First release: the tool, its specification (SPEC.md) and the conformance suite (137 cases)
as they stood when memex replaced brainctl v3 as the shared memory of one operator and
their coding agents.

### Added

- Store format 1: one git repository of Markdown entries with TOML frontmatter; seven
  types (`rules`, `decisions`, `runbooks`, `reference`, `tasks`, `plans`, `journal`), one
  status vocabulary per type, journal bodies immutable once committed, and every write a
  commit that is pushed.
- Commands: `init`, `new`, `capture`, `search`, `preflight`, `list`, `show`, `set`
  (single entry or `--batch`), `promote`, `fmt`, `save`, `sync`, `check`, `doctor`,
  `import` (brain v3), `help`, `version`. Every command that prints data takes `--json`;
  mutating commands end with one outcome line; exit codes carry meaning.
- Search: BM25 over title, keywords and summary, tags, aliases and body with field and type
  weights, prefix terms, current statuses by default and `--all` for the rest.
- `check`: error and warning codes for frontmatter, links, hashes, pointers, secrets and
  curation debt; `--workspace` checks the pointer files of every repository under the
  workspace; the `allow_secret` field and the `<!-- memex: allow-secret -->` body line
  clear a secret false positive.
- `doctor`: installation, configuration, store, upstream, hooks, the conformance of the
  store, and a leak scan of coding tools' own memory directories.
- Agent integration: `preflight` as the session-start hook, `memex help agents` as the
  rules of engagement, pointer files written by `init --pointers`.
- Packaging: a Nix flake (`packages.default`, Go 1.27, `CGO_ENABLED=0`); `go install
  github.com/arrowheadmind/memex/cmd/memex@v0.1.0`.
- `scripts/replay/`: a retrieval replay harness with blind grading, for measuring search
  changes against a real store before adopting them.
