# Changelog

Notable changes to memex, newest first. The version is the git tag (IMPLEMENTATION.md,
"Packaging"); a build that is not a release reports `0.0.0-dev`. A change to the shape of
text output is an interface change and is listed under "Changed output" (SPEC.md §17.3).

## Unreleased

### Added

- `memex update ID --summary=S --body FILE|- [--heading=H] [--status S]` (SPEC.md §11.17)
  appends `## <heading> <today>` and the text to an entry's body and replaces its summary, with
  an optional status, in one commit (`memex: update <id>[ status=<value>]`). A journal entry,
  an unchanged summary and a section under 80 characters of content are refused.
- `W-STALE-SUMMARY` (SPEC.md §7, summary currency): `check`, `check --staged` (and so the
  pre-commit hook) and `doctor`'s check report a task or plan whose summary did not change when
  a section was added to its body or when it was closed or reopened. It reads `HEAD`'s
  first-parent history in one `git log` and one `git cat-file --batch`, plus the working tree
  or the index; a shallow clone's oldest commit counts as the creation.
- A CI workflow (`.gitea/workflows/ci.yml`) runs `nix develop -c scripts/check.sh` on every push.
- A release workflow (`.gitea/workflows/release.yml`): pushing a `v*` tag, or running it by hand
  with an existing tag, creates the release page from the tag's CHANGELOG section and attaches
  `memex-<tag>-linux-amd64` and its `.sha256` file, using only the run's own token.

### Changed

- `memex set` refuses a status change that closes or reopens a task or plan (crosses default
  visibility) unless the same command gives a new, non-empty `summary=` (SPEC.md §11.7), also as
  a `--batch` line. Scripts that close tasks with `status=done` alone must add `summary=`.

### Changed output

- The `exists` refusal of `new` and `capture` (SPEC.md §11.2) says
  `To add to it: memex update <id> --summary=<sentence> --body -.` instead of
  `To add to it: edit the file, then memex save.`; for a journal entry it says
  `A journal body is immutable; to add to it, capture a new entry.`
- `memex check` and `doctor` count `W-STALE-SUMMARY` warnings; a store whose tasks and plans
  were closed with `set status=done` alone shows one per such entry.
- `help --all`, `help topics` and the overview list `update` after `version` (§11 order).
- `set` and `update` check in one defined order (SPEC.md §11.7, refusal order): the command
  line, the store and lock, the id, the entry, the values, the visibility rule, validation.
  `update` now resolves its ID before reading `--body`, and refuses uncommitted edits before
  comparing the summary. `set` reports uncommitted edits before fields it cannot keep.
- `set` and `update` on an entry deleted from the working tree, or whose file cannot be
  parsed, say `<path> has uncommitted changes; run memex save first` (or, for a committed file
  that cannot be parsed, that its fields cannot be kept) instead of `no entry "<id>"`.
- A history `check` cannot read is reported with git's first error line only
  (`memex: check needs the store's git history: <reason>`), and `doctor`'s `check` FAILs with
  the same text; both printed the git command and its whole standard error before.

### Fixed

- `memex update --status=` (an empty status) is a usage error; it committed the update with the
  status unchanged. A CR in `--heading` is a usage error, as an LF is; it failed validation.
- `check` and `doctor` treat a `HEAD` that does not resolve, other than an unborn branch, as a
  history that cannot be read (exit 2; `doctor` FAILs `check`); they treated it as a store
  with no commits and skipped `W-STALE-SUMMARY`.
- `check --staged` checks an entry whose type changed in the index (a symbolic link replaced
  by a file); it skipped it.
- `W-STALE-SUMMARY` leaves out a version whose summary is not valid (not a string, or not one
  line of at most 500 characters), and does not check an entry whose current summary is not
  valid; such summaries were compared as empty.

- `nix develop` now enters a dedicated dev shell (Go 1.27 and git, what `scripts/check.sh`
  needs). It previously fell back to the package's build environment, which sets
  `-mod=vendor` without a vendor directory, so every `go` command failed.

### Documented

- A write refused by validation names its findings before the refusal line, each as
  `memex: <path>: <CODE> <message>` (the `memex check` line), then
  `memex: refused: the result would not validate; nothing was changed`. The lines were printed
  since 0.1.0, but SPEC.md §9.1 and the `new` and `capture` pages did not say so. Conformance
  case 222 pins it: a 501-character summary on `new` and `capture`.
- The README states the status after the 0.1.0 release and names the check command
  (`scripts/check.sh`), the documentation entry points and the licence.

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
