# memex — specification

**Format version 1. Tool version: none released yet.**

memex is a command-line tool for a private, git-backed store of Markdown files. The store holds
what one operator and their agents have learned: rules, mistakes, decisions, runbooks, current
facts, tasks, plans and a journal of investigations. The people and agents do the thinking.
memex is the memory they share.

The name comes from Vannevar Bush's 1945 *memex*, a "memory extender".

This document is the single specification. The code implements it. When they disagree, one of
them is fixed on purpose in the same change; they are never left apart.

**Every section starts as Planned.** A section's status moves to Implemented in the commit
that implements it, and the table below records this. Nothing marked Planned may be described
elsewhere (README, help text) as existing.

| Section | Status |
|---|---|
| 3 Configuration | Planned |
| 4 Store layout | Planned |
| 5 Entries | Planned |
| 6 Links | Planned |
| 7 Search | Planned |
| 8 Writing | Planned |
| 9 Commands | Planned |
| 10 Tool neutrality | Planned |
| 11 Importing a brain v3 store | Planned |

---

## 1. Principles

These come from about 2,300 commits of experience with the predecessor (brain-kit's
`brainctl`, v2 and v3). Each principle states the failure it prevents.

1. **Mechanism over memory.** A rule that depends on being remembered has already failed.
   Every rule memex can check, it checks (`memex check`, hooks, refusals). Documentation
   comes second.
2. **One fact, one place.** Metadata lives only in the entry's frontmatter. An entry's id is
   its filename and its type is its directory, so neither is repeated inside the file. Nothing
   memex generates (indexes, search caches) is ever committed. The predecessor kept status in
   the file, the index row and the backlog, and they drifted apart.
3. **One entry, one file.** Never accumulate knowledge by appending to a shared file. The
   predecessor's shared files reached 1 MB (mistakes) and 19,865 lines (backlog). Every
   session appended to them, so every write collided, and most of its concurrency machinery
   existed only to cope with that.
4. **Current truth is visible; history is out of the way.** A replaced entry changes status and
   leaves default search. It is not annotated in place with strikethrough and "STALE"
   callouts. Git keeps the history.
5. **A miss must look like a miss.** Search ranks results instead of filtering them to nothing,
   and it reports what it left out.
6. **No ambient location.** There is exactly one store, named by configuration. memex never
   infers the store from the current directory. The predecessor's per-directory `BRAIN_ROOT`
   produced four separate "wrong brain, wrong worktree" defects.
7. **The store is tool-neutral.** No knowledge lives in a coding tool's private store (§10).
8. **Small surface.** memex manages knowledge only. Source-code worktrees, host closeout and
   workspace-wide repository sync are out of scope (§12).
9. **Standard library only.** Python 3.11+ standard library. `git` is the only external program.
   SQLite (with FTS5) comes from the standard `sqlite3` module.
10. **Public tool, private store.** This repository is public. It contains no operator-specific
    hostnames, paths or policy. Everything specific to one operator lives in their store.

---

## 2. Terms

- **Store**: a git repository holding entries, with `memex.toml` at its root. Private and
  operator-owned.
- **Entry**: one Markdown file in a type directory of the store.
- **Type**: the kind of entry, given by its directory (§5.2).
- **Id**: the entry's filename without `.md`. It is unique across the whole store and
  never changes.
- **Pointer file**: a file a coding tool reads automatically (`CLAUDE.md`, `AGENTS.md`, …) whose
  only content is a pointer to the store's agent instructions (§10).
- **Workspace**: the directory tree of source repositories the operator works in. It matters only
  for attribution (§5.4) and the pointer check (§10).

---

## 3. Configuration

The store location is the only required setting. It is resolved in this order, and the first
one found wins:

1. `--store PATH` on the command line.
2. The `MEMEX_STORE` environment variable.
3. `store = "PATH"` in `$XDG_CONFIG_HOME/memex/config.toml` (default `~/.config/memex/config.toml`).

If none is set, every command except `memex init` and `memex doctor` exits with status 2 and
explains how to set one. There is no default path and no search of parent directories.

`~` and `$HOME` in the path are expanded. Symlinks are not resolved. The path must contain
`memex.toml`, or memex refuses with status 2.

Other optional settings, all in `config.toml`:

| Key | Default | Meaning |
|---|---|---|
| `host` | short hostname | Value written to `hosts` on capture |
| `workspace` | unset | Root scanned by `check --workspace` and `doctor` for pointer files |
| `sync_on_preflight` | `true` | Whether `preflight` fetches and fast-forwards the store first |

The config file holds settings only, never knowledge.

---

## 4. Store layout

```
memex.toml          store manifest (format version, store name)
AGENT.md            instructions for agents; the target of every pointer file
rules/              one rule or known mistake per file
decisions/          one decision record per file
runbooks/           one procedure per file
reference/          one current-state fact sheet per file
tasks/              one backlog item per file
plans/              one plan, handover prompt or execution report per file
journal/            one investigation per file; the body is immutable
logs/               one machine-produced log per file; the body is immutable
files/<id>/         optional attachments belonging to entry <id> (lists, data, images)
.memex/             local cache; git-ignored, rebuildable, never committed
.githooks/          hooks installed by `memex init` (pre-commit runs `memex check --staged`)
```

`memex.toml`:

```toml
format = 1
name = "example"
```

Type directories are flat: no subdirectories. Anything that is not an entry belongs in
`files/<id>/`. `memex check` rejects a file in a type directory that is not an entry, and an
unknown top-level directory.

`.gitattributes` sets nothing special. With one entry per file there are no shared append
targets, so `merge=union` is not needed.

---

## 5. Entries

### 5.1 File format

An entry is UTF-8 Markdown with TOML frontmatter between `+++` lines:

```markdown
+++
title = "Deploying with a bare rebuild leaves the closure uncached"
status = "active"
created = 2026-09-12
tags = ["nix", "deploy"]
keywords = ["nixos-rebuild", "attic", "cache", "garbage collection"]
+++

**Symptom:** …
```

TOML (not YAML) because the standard library can read it (`tomllib`). memex writes it with
its own minimal writer, which supports only the value kinds listed in §5.3: string, boolean,
date, integer and array of strings.

The **id** is the filename stem: `YYYY-MM-DD-slug`.
- The date is the creation date.
- The slug is lowercase ASCII letters, digits and single hyphens.
- The whole id is at most 120 characters. A longer title is truncated and gets an 8-hex
  SHA-256 suffix.
- An id is never reused, renamed or recycled. A title can change freely; the id doesn't.

### 5.2 Types and their statuses

| Directory | Type | Statuses (default first) | Shown by default in search |
|---|---|---|---|
| `rules/` | rule | `active`, `superseded`, `retired` | `active` |
| `decisions/` | decision | `accepted`, `proposed`, `superseded`, `rejected` | `accepted`, `proposed` |
| `runbooks/` | runbook | `active`, `superseded`, `retired` | `active` |
| `reference/` | reference | `active`, `superseded`, `retired` | `active` |
| `tasks/` | task | `open`, `in-progress`, `blocked`, `done`, `dropped` | `open`, `in-progress`, `blocked` |
| `plans/` | plan | `draft`, `active`, `done`, `abandoned`, `superseded` | `draft`, `active` |
| `journal/` | journal | `raw`, `kept`, `promoted`, `superseded` | `raw`, `kept`, `promoted` |
| `logs/` | log | `kept` | none; reached with `--all` or by id |

- A rule covers both "constraint" and "known mistake". The difference is in the body, not the
  type.
- A journal entry is an investigation write-up: what was tried, what worked, what failed.
  - `raw` means not yet reviewed.
  - `kept` means reviewed, with nothing to promote.
  - `promoted` means some other entry cites it in `sources`.

### 5.3 Frontmatter fields

| Field | Kind | Required | Applies to | Meaning |
|---|---|---|---|---|
| `title` | string | yes | all | Human title |
| `status` | string | yes | all | One of the type's statuses |
| `created` | date | yes | all | Must equal the id's date |
| `tags` | array | no | all | Topic labels, lowercase |
| `keywords` | array | no | all | Words a *reader* would search with, when the body uses other words |
| `repos` | array | no | all | Source repositories the entry concerns |
| `hosts` | array | no | all | Machines the entry concerns |
| `superseded_by` | string | when status is `superseded` | all | Id of the replacing entry |
| `sources` | array | no | all | Ids this entry was derived from |
| `aliases` | array | no | all | Former paths or names that should resolve to this entry (§6) |
| `always` | boolean | no | rule | Printed for every `preflight`, whatever the query |
| `verified` | date | yes | reference | When the facts were last checked against the live system |
| `review_days` | integer | no | reference | `check` warns when `verified` is older than this |
| `kind` | string | no | plan | `plan` (default), `prompt` (a handover to execute) or `report` (an executor's result) |
| `parent` | string | no | plan, task | Id of the enclosing plan |
| `body_sha256` | string | yes | journal, log | SHA-256 of the body bytes after the closing `+++` line |

Unknown fields are an error. Field order on write is the order of this table.

There is no `updated` field, because git records that. There is no `promoted_to` field, because
it is the reverse of `sources` and is computed.

### 5.4 Attribution

When memex creates an entry:
- `repos` defaults to the basename of the git top-level of the current directory, if there is
  one.
- `hosts` defaults to the configured `host`.

Both can be overridden with `--repo`/`--host`, or emptied with `--no-repo`/`--no-host`.
This is the only use memex makes of the current directory.

### 5.5 Bodies

- The body is free Markdown. memex never rewrites it, except that `memex new` writes the body
  it is given.
- **Journal and log bodies are immutable.** `body_sha256` is written at creation, and `check`
  fails if the body no longer matches. To correct one, write a new entry that supersedes it.
- A body must contain at least 80 non-blank characters that are not part of a Markdown heading.
  `new` refuses an emptier body and `check` reports one. The predecessor accumulated captures
  that were only empty section headings.
- A body template per type is shipped as guidance (`memex new --template TYPE` prints it). It
  is never enforced.

### 5.6 Checks

`memex check` validates the whole store and exits 1 if anything is an error. Errors:

- A filename that isn't a valid id.
- A duplicate id across directories.
- A `created` that doesn't match the id date.
- Frontmatter that won't parse, has unknown fields or is missing required ones.
- A status not valid for the type.
- `superseded` without a resolvable `superseded_by`.
- An unresolvable `[[id]]` link, `sources`, `parent` or `superseded_by`.
- A journal entry marked `promoted` that no entry cites.
- A `body_sha256` mismatch.
- A body below the minimum.
- `always` on a non-rule.
- A file that isn't an entry inside a type directory.
- An `aliases` value claimed by two entries.

Warnings (exit 0 unless `--strict`):
- A `reference` entry past its `review_days`.
- A `raw` journal entry older than 90 days.
- An entry with no tags and no keywords.

`memex check --staged` checks only the entries staged in the git index, plus link targets. It
is what the pre-commit hook runs.

---

## 6. Links

- In bodies, an entry is linked as `[[id]]` or `[[id|text]]`.
- `memex check` resolves every link. A link that matches an `aliases` value of exactly one entry
  is a warning that suggests the id. A link that resolves to nothing is an error.
- Plain Markdown links to `files/<id>/…` are allowed and checked for existence.
- Links to anything outside the store (URLs, `repo@sha` commit pins) are free text and not
  checked.

---

## 7. Search

### 7.1 Engine

- Search uses SQLite FTS5 with BM25 ranking, held in `.memex/index.sqlite`.
- The index is a cache. Before every search it is brought up to date by comparing each entry's
  path, size and mtime with what was indexed, so deleting it is always safe.
- Indexed columns, with their BM25 weights:

  | Column | Weight |
  |---|---|
  | title | 10 |
  | keywords | 8 |
  | tags | 5 |
  | aliases | 3 |
  | body | 1 |

- Tokenizer: `unicode61 remove_diacritics 2`, with `-`, `_`, `.` and `/` as separators.
- `memex doctor` fails if the Python `sqlite3` module lacks FTS5.

### 7.2 Queries

- The query is free text. Each word is a term, and terms are combined with **OR**. Ranking, not
  filtering, decides what comes first, so a query with one unmatched word still finds the
  entries that match the others.
- `"quoted phrases"` match as phrases.
- Punctuation is a separator, never syntax. Text is never interpreted as a regex or as FTS5
  operators.
- A query with no letters or digits is an error.
- The final score is `bm25 × type_weight`:

  | Type | Weight |
  |---|---|
  | rule | 1.5 |
  | runbook | 1.3 |
  | reference | 1.3 |
  | decision | 1.2 |
  | task | 1.0 |
  | plan | 1.0 |
  | journal | 0.8 |
  | log | 0.5 |

### 7.3 Output

`memex search QUERY` prints, in order:

1. Up to `--limit` (default 12) results. Each shows score rank, type, status, id, title, and
   a one-line snippet with the matched terms marked.
2. One line per hidden category: matches cut off by the limit, and matches hidden by status
   (for example `7 more matches; 3 superseded/retired hidden — use --all`).
3. If nothing matched: `No entries matched any term of "<query>".` and exit 3.

These are the only ways a search ends. It never prints an empty success.

`--type`, `--tag`, `--repo` and `--host` filter the results. `--all` includes every status.
`--json` prints machine-readable results.

---

## 8. Writing

Every memex command that changes the store does it as one unit:

1. Take an exclusive lock on `.memex/write.lock`. Wait at most 60 s, then fail with exit 4.
2. Fetch the upstream. If the store branch is strictly behind, fast-forward it.
3. Write the files.
4. Re-read every written file, parse it, and compare it with what was intended. Abort on any
   difference.
5. `git commit --only -- <exact paths>` with a generated message (`memex: new rule <id>`,
   `memex: set <id> status=superseded`). Other changes in the working tree, made by other
   sessions or by hand, are never staged or committed.
6. `git pull --rebase --autostash`, then push. Retry up to 5 times on a rejected push.
7. Report the outcome honestly:
   - `pushed <sha>`;
   - or `committed <sha>, NOT pushed: <reason>`, with exit 5.

A commit is never undone automatically.

**Direct edits are first-class.** Agents and people may edit entries with any editor.
`memex save [PATHS…]` commits the given paths, or every changed entry if none are given,
through the same steps 1–7, after `memex check` passes on them.

**Rejected alternative:** the predecessor ran every write in a throwaway git worktree, to protect
shared append-only files. With one file per entry, two writers touch the same file only when
they edit the same entry. That is a real conflict, and the rebase in step 6 reports it rather
than hiding it.

The store has no hook that forbids direct commits. The pre-commit hook only runs
`memex check --staged`.

---

## 9. Commands

Global options: `--store PATH`, `--json` where it applies, `--help`, `--version`.

Exit codes:

| Code | Meaning |
|---|---|
| 0 | Success |
| 1 | Check failed or validation error |
| 2 | Usage or configuration error |
| 3 | Search found nothing |
| 4 | Lock timeout |
| 5 | Committed but not pushed |

| Command | Does |
|---|---|
| `memex init PATH [--name NAME]` | Creates a store: `memex.toml`, `AGENT.md`, type directories, `.gitignore` (`.memex/`), `.githooks/pre-commit`, and `core.hooksPath`. It is idempotent and never overwrites a nonempty file. |
| `memex new TYPE --title T --body FILE\|- [--tag …] [--keyword …] [--repo …] [--host …] [--status S] [--parent ID] [--kind K]` | Creates an entry (§8). If the id already exists: the same body exits 0 and prints the path; a different body exits 1 and changes nothing. A title starting with `-` is refused. |
| `memex capture --title T --body FILE\|- [...]` | Shorthand for `memex new journal`. |
| `memex preflight [--no-sync] QUERY` | Runs the session-start ritual: sync the store (unless `sync_on_preflight` is false or `--no-sync` is given), print every `always` rule, then run `search QUERY`. If the sync fails, it says so on the first line and still searches. |
| `memex search QUERY [filters]` | §7. |
| `memex show ID` | Prints the entry, plus the computed reverse links: entries citing it in `sources`, and entries it supersedes. |
| `memex list [--type] [--status] [--tag] [--repo] [--host] [--json]` | Prints a table of entries, newest first. This replaces a committed INDEX file. |
| `memex set ID FIELD=VALUE…` | Changes frontmatter fields without touching the body bytes. Setting `status=superseded` requires `superseded_by=`. |
| `memex promote ID --to TYPE --title T --body FILE\|- [...]` | Creates the new entry with `sources = [ID]` and sets ID to `promoted`, in one commit. Read-back verification (§8 step 4) covers both files. |
| `memex save [PATHS…]` | Commits direct edits (§8). |
| `memex sync` | Fetches the store and fast-forwards it if it is clean or only has unrelated untracked files. Reports when the last fetch happened and whether the store is ahead, behind or diverged. |
| `memex check [--staged] [--strict] [--workspace]` | §5.6. `--workspace` adds the pointer-file check (§10). |
| `memex doctor` | Read-only environment report, one PASS, WARN or FAIL per line: Python version, git, FTS5, store resolution, upstream, fetch age, hooks path, executable shadowing, and the tool-store leak scan (§10). |
| `memex import-leak PATH` | Turns a file found in a tool store into a `raw` journal entry, with `sources` empty and the original path in `aliases`. It prints the path to delete afterwards and never deletes it itself. |
| `memex import brain-v3 PATH --out DIR` | §11. |

---

## 10. Tool neutrality

Coding tools come and go, and each has its own private memory: Claude Code's
`~/.claude/projects/*/memory/` and `~/.claude/CLAUDE.md`, Codex's `~/.codex/`, Cursor's
`.cursorrules`, Copilot's `.github/copilot-instructions.md`, and their successors. Knowledge
written there is invisible to every other tool and is lost when the tool changes.

The invariants:

1. **Knowledge lives only in the store.**
2. **Pointer files contain only a pointer.** The content of each one is exactly:

   ```
   Read <store>/AGENT.md before starting work, and follow it.
   ```

   `<store>` is the configured store path, written with `~`. `memex init --pointers DIR` writes
   `CLAUDE.md` and `AGENTS.md` into a repository. `memex check --workspace` reports any pointer
   file under the workspace whose content differs.
3. **Tool stores are scanned and never read as a source.** `memex doctor` lists every file
   holding content in the known tool stores (the list lives in code and is extended as tools
   appear) as `LEAK <path>`. `memex import-leak` moves that content into the store for review.
4. **Integration is one-way.** A tool's hooks call `memex` (for example `memex preflight` at
   session start). memex knows nothing about any tool beyond the pointer text and the leak
   scan paths.
5. **Turning off a tool's own memory is the operator's job, not memex's.** Where a tool offers
   a switch (Claude Code: `autoMemoryEnabled = false`), set it in the machine configuration.
   `doctor` warns if that switch is detectably on.

---

## 11. Importing a brain v3 store

`memex import brain-v3 SRC --out DIR` converts a brainctl v3 brain into a new memex store. It
writes only under `DIR` and never modifies `SRC`, which should be a clean clone at a known
commit. It is deterministic: the same `SRC` commit always gives a byte-identical `DIR`. Run it
again against a later commit to bring over everything written since.

Mapping:

| Source | Becomes |
|---|---|
| `.brain/investigations/*.md` | `journal/`. Keeps the dated filename stem as the id; undated files take their first-commit date. Status comes from the INDEX row, else the file's `## Status`. Tags, repo and context come from the INDEX row; context becomes a tag. |
| `known-mistakes.md` table rows | One `rule` each. The body is the four columns as labelled paragraphs. |
| `known-mistakes.md` `## Mistake:` sections and dated `##` sections | One `rule` each. |
| `constraints.md` table rows and `##` sections | One `rule` each. Items under `## Always` get `always = true`. |
| `decisions/`, `runbooks/` (both locations), `plans/` (including `prompts/` → `kind = "prompt"`, `reports/` → `kind = "report"`) | Same type. Topic subdirectories go to `files/<id>/`. |
| `docs/backlog.md` `###` items | One `task` each. Status is inferred from section and markers (✓, DONE, SUPERSEDED, strikethrough). |
| Other `docs/*.md` | `reference`, with `verified` set to the last commit date. |
| `dispatch-logs/*.log` | `log`. |
| `AGENT.md` | `AGENT.md`, unchanged, for manual rewriting. |

- **Date when the source has none.** Ids need a date. The date of the commit that first
  introduced the text is used (`git log --follow` for files, `git blame` for rows and
  sections).
- **Statuses.** Status markers in rule text ("STALE", "RESOLVED", "NO LONGER", strikethrough
  titles) map to `retired`. The original text is kept unchanged.
- **Links.** Path references to other source entries are rewritten to `[[id]]` when they
  resolve unambiguously. Every original path is recorded in the target entry's `aliases`.
- **Accounting.** The import writes `DIR/.memex/import-report.json` and a readable summary.
  - Every source unit (file, table row, section, backlog item) maps to exactly one entry or is
    listed under `unmapped`, with a reason.
  - The summary gives per-source line and byte totals for input against output.
  - The import exits 1 if any non-whitespace source text is unaccounted for.
- **Never applied automatically:** deduplication, rewording, or judging whether a rule is still
  true. Those are a separate, reviewed curation pass.

---

## 12. Out of scope

These are deliberately not part of memex:

- **Source-code task worktrees, checkpoints, tested-SHA integration and host closeout.** This
  was brainctl's `work`, `finish` and `session`. It is a separate concern with its own tool.
- **Workspace-wide repository status and sync.** memex syncs only the store.
- **Multiple stores, and per-repository local stores.**
- **Generated summaries from a language model, embeddings, and any network service other than
  the store's git remote.**
- **Publishing or exporting parts of a store.**

---

## 13. Releases

- The version is stated once, in `src/memex/__init__.py`. Everything else reads it from there.
- Every release has a git tag `vX.Y.Z` and a `CHANGELOG.md` entry.
- A change to the store format increments `format` in `memex.toml` and ships a migration
  command. memex refuses a store whose `format` is newer than the one it knows.
