# memex — specification

**Store format version 1. Tool version: none released yet.**

memex is a command-line tool for a private, git-backed store of Markdown files. The store holds
what one operator and their agents have learned: rules, mistakes, decisions, runbooks, current
facts, tasks, plans and a journal of investigations. The people and agents do the thinking.
memex is the memory they share.

The name comes from Vannevar Bush's 1945 *memex*, a "memory extender".

## 0. About this document

**It is the single specification.** The code implements it. When they disagree, one of them
is fixed on purpose in the same change; they are never left apart.

**It is independent of implementation language.** It defines observable behavior: files
read and written, command-line interface, standard output, standard error, exit codes and git
effects. Two conforming implementations, in any languages, given the same store, arguments,
environment and date, produce byte-identical standard output, exit codes and store contents.
Choices that don't affect observable behavior belong in [IMPLEMENTATION.md](IMPLEMENTATION.md).

**Conformance is tested, not claimed.** The language-neutral conformance suite (§15) is the
executable form of this document. A behavior defined here without a conformance case is a gap
to fill.

**The key words** MUST, MUST NOT, SHOULD and MAY are used as in RFC 2119.

**Sections start Planned.** A section's status changes to Implemented in the commit that
implements it and passes its conformance cases. Nothing Planned may be described elsewhere
(README, help text) as existing.

| Section | Status |
|---|---|
| 3 Configuration | Planned |
| 4 Store layout | Planned |
| 5 Entries | Planned |
| 6 Links | Planned |
| 7 Validation | Planned |
| 8 Search | Planned |
| 9 Writing to the store | Planned |
| 10 Command-line conventions | Planned |
| 11 Commands | Planned |
| 12 Tool neutrality | Planned |
| 13 Importing a brain v3 store | Planned |
| 15 Conformance suite | Planned |
| 16 Help | Planned |
| 17 Agent-first requirements | Planned |

---

## 1. Principles

Principle 0 is the design stance. The rest come from about 2,300 commits of experience with
the predecessor (brain-kit's `brainctl`, v2 and v3), and each names the failure it prevents.

0. **The primary user is a language-model agent.** Humans are the second user: they read the
   same output and must find it clear, but when the two pull apart, the agent's needs decide.
   §17 states what this requires, and every command is designed against it.
1. **Mechanism over memory.** A rule that depends on being remembered has already failed.
   Everything memex can check, it checks (`memex check`, hooks, refusals). Documentation comes
   second.
2. **One fact, one place.** Metadata lives only in an entry's frontmatter. The id is the filename
   and the type is the directory, so neither is repeated in the file. Nothing memex generates
   (lists, search caches) is committed. The predecessor kept status in three places, and they
   drifted apart.
3. **One entry, one file.** Never accumulate knowledge by appending to a shared file. The
   predecessor's shared files reached 1 MB and 19,865 lines. Every session appended to them,
   and its concurrency machinery existed mostly to cope with that.
4. **Current truth is visible; history is out of the way.** A replaced entry changes status and
   leaves default search. It is not annotated in place. Git keeps the history.
5. **A miss must look like a miss.** Search ranks instead of filtering to nothing, says what it
   hid, and never ends in an empty success.
6. **No ambient location.** One store, named by configuration, never inferred from the current
   directory. The predecessor's per-directory root produced four "wrong place" defects.
7. **Tool-neutral.** No knowledge lives in a coding tool's private store (§12).
8. **Small surface.** Knowledge only. Source-code worktrees, host closeout and workspace-wide
   repository sync are out of scope (§14).
9. **Never interactive.** memex never prompts and never opens an editor. Agents run it.
10. **Honest outcomes.** Every write ends in exactly one reported state (§9.4). "Pushed" is never
    printed unless the push succeeded.
11. **Public tool, private store.** The tool contains no operator-specific hostnames, paths or
    policy. Everything operator-specific lives in the store or in configuration.

---

## 2. Terms

| Term | Meaning |
|---|---|
| **Store** | A git repository with `memex.toml` at its root, holding entries. Private and operator-owned. |
| **Entry** | One `.md` file directly inside a type directory of the store. |
| **Type** | The kind of entry, given by its directory (§5.2). |
| **Id** | The entry's filename without `.md`. It is unique across the store and never changes. |
| **Frontmatter** | The TOML block at the top of an entry (§5.1). |
| **Body** | Everything in an entry after the frontmatter (§5.1). |
| **Pointer file** | A file a coding tool loads automatically (`CLAUDE.md`, `AGENTS.md`, `GEMINI.md`) whose entire content is a pointer to the store's `AGENT.md` (§12). |
| **Workspace** | The configured directory tree of source repositories. It is used only by the pointer check and the leak scan (§12). |
| **Today** | The current date in the local time zone, or `MEMEX_TODAY` if set (§3.3). |

---

## 3. Configuration

### 3.1 Store resolution

The store path is resolved from the first of these that is set:

1. The `--store PATH` option.
2. The `MEMEX_STORE` environment variable, if non-empty.
3. The `store` key in the config file (§3.2).

Rules:
- A leading `~/` and the strings `$HOME` and `${HOME}` are expanded. No other expansion takes
  place, and symlinks are not resolved.
- A relative path is an error.
- The path MUST be a directory containing `memex.toml`, otherwise the command fails with
  exit 2.
- If nothing is set, every command except `init`, `doctor`, `version` and `help` fails with
  exit 2 and this message on stderr:

  ```
  memex: no store configured; use --store, MEMEX_STORE, or store = "..." in <config path>
  ```

There is no default path and no search of parent directories.

### 3.2 Config file

The path is `MEMEX_CONFIG` if set. Otherwise it is `$XDG_CONFIG_HOME/memex/config.toml` when
`XDG_CONFIG_HOME` is set, or else `~/.config/memex/config.toml`.

- A missing file is not an error.
- A file that won't parse, or that has an unknown key, is an error (exit 2).

Keys:

| Key | Type | Default | Meaning |
|---|---|---|---|
| `store` | string | none | Store path (§3.1) |
| `host` | string | short hostname (text before the first `.`, lowercased) | Default `hosts` value on new entries |
| `workspace` | string | none | Root for pointer checks and the leak scan (§12) |
| `sync_on_preflight` | boolean | `true` | `preflight` syncs the store first |
| `search_limit` | integer ≥ 1 | `12` | Default `--limit` |

The config file holds settings only, never knowledge.

### 3.3 Environment variables

| Variable | Meaning |
|---|---|
| `MEMEX_STORE` | Store path (§3.1) |
| `MEMEX_CONFIG` | Config file path (§3.2) |
| `MEMEX_TODAY` | Overrides today's date (`YYYY-MM-DD`). Exists for tests and imports. If it is malformed: exit 2. |
| `MEMEX_HOST` | Overrides the `host` setting |

memex reads no other variables of its own. It passes the environment through to `git`
unchanged.

---

## 4. Store layout

```
memex.toml          store manifest
AGENT.md            agent instructions; the target of every pointer file
rules/              one rule or known mistake per file
decisions/          one decision record per file
runbooks/           one procedure per file
reference/          one current-state fact sheet per file
tasks/              one backlog item per file
plans/              one plan, handover prompt or execution report per file
journal/            one investigation per file; body immutable
logs/               one machine-produced log per file; body immutable
files/<id>/         attachments belonging to entry <id>; any files, any depth
.memex/             local cache and lock; listed in .gitignore; never committed
.githooks/          hooks written by `memex init`
.gitignore          contains the line `.memex/`
README.md           optional, free text, ignored by memex
```

`memex.toml`:

```toml
format = 1
name = "example"
```

- `format` is an integer. memex refuses (exit 2) a store whose `format` is greater than the one it
  implements. If it is lower, memex tells the operator to run the migration that ships with the
  version that raised it.
- `name` is a free string, shown by `doctor`.
- Unknown keys are an error.

Type directories and `files/` MAY be absent: git doesn't track empty directories, and an
absent one means empty. memex creates a directory when it first writes into it.

Type directories are flat. Any other file or directory inside a type directory is a validation
error (§7). So is any top-level entry not in the list above, except dot-files and
dot-directories (`.git`, `.gitattributes`, `.editorconfig`, …), which memex ignores.

---

## 5. Entries

### 5.1 File format

An entry is UTF-8 text with LF line endings. No byte-order mark, no CR characters.

```
+++
title = "Bare rebuild leaves the closure uncached"
status = "active"
created = 2026-09-12
tags = ["deploy", "nix"]
+++

**Symptom:** …
```

**Structure, exactly:**
- The file MUST begin with the line `+++`.
- The frontmatter is every line after it, up to the next line that is exactly `+++`. It is a
  TOML 1.0 document.
- The **body region** is everything after that closing line's LF.
- The **body** is the body region with one leading LF removed, if it begins with one.

**Canonical form.** memex writes entries only in canonical form, which is the following
concatenation:

1. `+++\n`
2. One line per present field, in the order of §5.3, as `key = value\n`.
3. `+++\n`
4. `\n`
5. The body.

Fields are rendered like this:

| Kind | Rendering |
|---|---|
| string | A TOML basic string in double quotes. `\` → `\\`, `"` → `\"`; U+0000–U+001F and U+007F as `\b \t \n \f \r` where defined, else `\uXXXX` with uppercase hex. All other characters are written literally. |
| date | `YYYY-MM-DD`, unquoted |
| integer | Decimal, no sign for positive, no leading zeros |
| boolean | `true` |
| array of strings | `["a", "b"]` on one line, elements rendered as strings, separated by `, ` |

Omitted from canonical output:
- absent optional fields;
- empty arrays;
- booleans that are `false`.

Arrays of strings are written in their stored order. `tags`, `keywords`, `repos` and `hosts`
are stored sorted (ascending by code point) and deduplicated.

**Body normalization.** When memex receives a body from the user (`new`, `capture`, `promote`):
1. CRLF becomes LF, and any remaining CR becomes LF.
2. Trailing whitespace at the end of the body is removed.
3. Exactly one LF is appended.

The body is otherwise written byte for byte.

**Reading.** Any valid TOML in the frontmatter is accepted for reading. A file that isn't in
canonical form is a validation warning (§7), and `memex fmt` fixes it.

### 5.2 Types and statuses

| Directory | Type | Statuses (first is the default for new entries) | Visible by default |
|---|---|---|---|
| `rules/` | rule | `active` `superseded` `retired` | `active` |
| `decisions/` | decision | `accepted` `proposed` `superseded` `rejected` | `accepted` `proposed` |
| `runbooks/` | runbook | `active` `superseded` `retired` | `active` |
| `reference/` | reference | `active` `superseded` `retired` | `active` |
| `tasks/` | task | `open` `in-progress` `blocked` `done` `dropped` | `open` `in-progress` `blocked` |
| `plans/` | plan | `draft` `active` `done` `abandoned` `superseded` | `draft` `active` |
| `journal/` | journal | `raw` `kept` `promoted` `superseded` | `raw` `kept` `promoted` |
| `logs/` | log | `kept` | none |

- **Rules cover both constraints and known mistakes.** The difference lives in the body.
- **Journal statuses:**
  - `raw`: not yet reviewed.
  - `kept`: reviewed, nothing to promote.
  - `promoted`: knowledge was moved into another entry.
- **Visible by default** controls `search` and `list` without `--all` (§8.4, §11).

### 5.3 Frontmatter fields

Listed in canonical order.

| # | Field | Kind | Required | Allowed on | Meaning |
|---|---|---|---|---|---|
| 1 | `title` | string | yes | all | 1–300 characters, no LF |
| 2 | `status` | string | yes | all | One of the type's statuses |
| 3 | `created` | date | yes | all | Must equal the date in the id |
| 4 | `summary` | string | no | all | One line, at most 500 characters. Shown in lists and search results |
| 5 | `tags` | array | no | all | Each one matches `^[a-z0-9][a-z0-9-]*$` |
| 6 | `keywords` | array | no | all | Words a reader would search with; free strings, no LF |
| 7 | `repos` | array | no | all | Source repository names the entry concerns |
| 8 | `hosts` | array | no | all | Machine names the entry concerns |
| 9 | `always` | boolean | no | rule | Printed by every `preflight` (§11.4) |
| 10 | `kind` | string | no | plan | `prompt` or `report`. Absent means an ordinary plan |
| 11 | `parent` | string | no | plan, task | Id of an enclosing plan |
| 12 | `sources` | array | no | all | Ids this entry was derived from |
| 13 | `superseded_by` | string | iff status is `superseded` | all | Id of the replacing entry |
| 14 | `verified` | date | yes | reference | When the facts were last checked against the live system |
| 15 | `review_days` | integer ≥ 1 | no | reference | Staleness threshold for `verified` |
| 16 | `aliases` | array | no | all | Former names or paths that resolve to this entry (§6) |
| 17 | `body_sha256` | string | yes | journal, log | Lowercase hex SHA-256 of the body bytes |

- A field not in this table is an error.
- A field on a type not listed under "Allowed on" is an error.
- There is no `updated` field (git records it), and no reverse-link field. Reverse links are
  computed (§11.6).

### 5.4 Ids

`id = DATE "-" SLUG`, where `DATE` is `YYYY-MM-DD` and the whole id matches
`^[0-9]{4}-[0-9]{2}-[0-9]{2}-[a-z0-9]+(-[a-z0-9]+)*$` with length ≤ 120.

**Slug algorithm** (title → slug):

1. Normalize the title to Unicode NFKD and remove every code point of general category Mn.
2. Lowercase with Unicode simple lowercase mapping.
3. Replace every maximal run of characters outside `[a-z0-9]` with one `-`.
4. Strip leading and trailing `-`.
5. If the result is empty, use `entry`.

**Length cap:** let `id = DATE + "-" + slug`. If `len(id) > 120`, take the first 111
characters, strip trailing `-`, and append `-` plus the first 8 lowercase hex digits of
SHA-256 of the UTF-8 title.

- `DATE` is today (§2) when the entry is created.
- An id is never renamed or reused. Changing the title doesn't change the id.
- Two types MUST NOT share an id.

### 5.5 Attribution

On creation, unless options override it:
- `repos` is the basename of the top-level directory of the git repository containing the
  current directory, as reported by `git rev-parse --show-toplevel`, but only if that repository
  is not the store. Otherwise it is empty.
- `hosts` is `[host]` (§3.2).

`--repo NAME` and `--host NAME` (repeatable) replace the defaults. `--no-repo` and `--no-host`
empty them. This is the only use memex makes of the current directory.

### 5.6 Bodies

- The body is free Markdown. memex never edits a body except by writing one it was given, and
  during import (§13).
- **Content measure:** the number of non-whitespace Unicode code points in body lines that are
  not ATX headings (a line matching `^ {0,3}#{1,6}( |$)`) and not inside fenced code blocks.
  The body MUST have a content measure of at least **80**. The predecessor accumulated captures
  that were only empty headings.
- **Journal and log bodies are immutable.** `body_sha256` is set when the entry is created.
  To correct one, create a new entry and set the old one's status to `superseded`.
- `memex template TYPE` prints a suggested body outline (Appendix B). It is never enforced.

**Fenced code blocks** (used here and in §6): a line matching `^ {0,3}(```+|~~~+)` opens a
block, which is closed by a later line starting (after up to 3 spaces) with at least as many
of the same character and nothing else but whitespace. An unclosed block runs to the end of the
body.

---

## 6. Links

**Syntax:** `[[TARGET]]` or `[[TARGET|TEXT]]`. TARGET is any text without `]`, `|` or LF.
Links inside fenced code blocks and inside inline code spans (backtick-delimited on one line)
are ignored.

**Resolution of TARGET, in order:**
1. The id of an entry: resolved.
2. Exactly one entry has TARGET in `aliases`: resolved, with a warning `W-ALIAS-LINK` that
   suggests the id.
3. Otherwise unresolved, which is an error (`E-LINK`).

**Field references** (`parent`, `sources`, `superseded_by`) MUST be ids. Aliases are not
accepted there.

**Markdown links** to `files/...` are checked for existence (`E-FILE-LINK`). Any other URL,
path or `repo@sha` pin is free text and is not checked.

---

## 7. Validation

`memex check` validates the whole store. `memex check --staged` validates the entries that are
added or modified in the git index, plus the link targets they reference.

**Output** is one line per finding, sorted by path and then code:

```
<path>: <CODE> <message>
```

The last line is `check: N errors, M warnings`. Exit 0 if there are no errors. With
`--strict`, warnings count as errors.

Errors:

| Code | Condition |
|---|---|
| `E-ID` | Filename isn't a valid id (§5.4) |
| `E-DUP-ID` | Same id in two type directories |
| `E-NOT-ENTRY` | Non-`.md` file or subdirectory in a type directory; unknown top-level item (§4) |
| `E-ENCODING` | Not UTF-8, has a BOM, or contains CR |
| `E-FRONTMATTER` | Missing delimiters, or invalid TOML |
| `E-FIELD-UNKNOWN` | Field not in §5.3, or not allowed on this type |
| `E-FIELD-MISSING` | Required field absent |
| `E-FIELD-VALUE` | Wrong kind or value outside its constraints |
| `E-STATUS` | Status not valid for the type |
| `E-CREATED` | `created` ≠ id date |
| `E-SUPERSEDED` | `superseded` without `superseded_by`, or `superseded_by` without `superseded` |
| `E-REF` | `parent`/`sources`/`superseded_by` names a missing id, or `parent` isn't a plan |
| `E-LINK` | Unresolved `[[...]]` |
| `E-FILE-LINK` | Markdown link to a missing `files/...` path |
| `E-HASH` | `body_sha256` mismatch |
| `E-BODY` | Content measure below 80 |
| `E-ALIAS` | The same alias on two entries, or an alias equal to an id |
| `E-FILES-OWNER` | `files/<x>/` where `<x>` isn't an id |
| `E-MANIFEST` | `memex.toml` invalid |

Warnings:

| Code | Condition |
|---|---|
| `W-CANONICAL` | Entry valid but not in canonical form (`memex fmt` fixes it) |
| `W-ALIAS-LINK` | Link resolved through an alias |
| `W-STALE-REFERENCE` | `reference` with `review_days`, where `verified + review_days < today` |
| `W-RAW-OLD` | `raw` journal entry created more than 90 days ago |
| `W-PROMOTED-UNCITED` | `promoted` journal entry that no entry cites in `sources` |
| `W-NO-TERMS` | Entry with neither `tags` nor `keywords` |
| `W-NO-SUMMARY` | Entry without `summary` (imports and hand-written entries may lack one) |

`check --workspace` adds the pointer and leak findings of §12 (`E-POINTER`, `W-LEAK`).

---

## 8. Search

### 8.1 Corpus

Every entry is a search document with five fields:

| Field | Content |
|---|---|
| title | `title` |
| keywords | `keywords` joined with spaces, then `summary` |
| tags | `tags` joined with spaces |
| aliases | `aliases` joined with spaces |
| body | The body |

### 8.2 Tokenization

Applied identically to documents and queries:
1. NFKD, then remove category Mn.
2. Simple lowercase.
3. A token is a maximal run of code points of general category L* or N*. Everything else
   separates tokens.

There is no stemming and no stop-word list.

### 8.3 Queries and scoring

**Parsing:**
- A `"double-quoted"` span is a **phrase**: its tokens must appear consecutively in one field.
- Every other token is a **term**. A term of 4 or more characters is a **prefix term**,
  matching any token that begins with it. Shorter terms match whole tokens only.
- If an unmatched `"` remains, it is treated as a separator.
- Duplicate terms are dropped.
- A query with no terms and no phrases is a usage error (exit 2).

Terms and phrases (together, *query items*) combine with **OR**: a document matches if any item
matches it.

**Score** (BM25, as computed by SQLite FTS5's `bm25()`, with k1 = 1.2 and b = 0.75):

```
N        = number of documents
n(q)     = number of documents matched by item q
idf(q)   = max( ln((N − n(q) + 0.5) / (n(q) + 0.5)), 1e-6 )
f(q,D)   = Σ over fields c of  w_c × hits(q, D, c)
|D|      = total token count of D across all fields
avgdl    = mean |D| over all documents
bm25(D)  = Σ over items q of  idf(q) × f(q,D) × (k1 + 1) / ( f(q,D) + k1 × (1 − b + b × |D| / avgdl) )
score(D) = bm25(D) × type_weight(D)
```

Where:
- `hits(q, D, c)` is the number of matching tokens (terms) or phrase occurrences (phrases) in
  field `c`.
- Field weights `w_c`: title 10, keywords 8, tags 5, aliases 3, body 1.
- Type weights: rule 1.5, runbook 1.3, reference 1.3, decision 1.2, task 1.0, plan 1.0,
  journal 0.8, log 0.5.

`N`, `n(q)` and `avgdl` are computed over **all** entries, whatever their status. Visibility
filters apply after scoring.

**Ranking:** descending score, then type in the order of §5.2's table, then ascending id.

Scores are compared after rounding to 9 significant digits, so that implementations whose
floating-point arithmetic differs in the last bits still rank identically.

### 8.4 Filters and visibility

- `--type T`, `--tag X`, `--repo R`, `--host H` are each repeatable. Values within one option are
  OR'd, and different options are AND'd.
- An entry is **visible** if its status is in the type's "visible by default" set (§5.2). `--all`
  makes every status visible.
- `--limit N` caps the number of visible results printed.

### 8.5 Output

**Text output:** one block per result, numbered from 1:

```
1. [rule, active] rules/2026-09-12-bare-rebuild-leaves-the-closure-uncached.md
   Bare rebuild leaves the closure uncached
   …the closure was never pushed to the **cache** so the weekly…
```

- **Line 1:** rank, then `[<type>, <status>]`, then the entry's path relative to the store root.
  The id is the filename stem, and an agent can open the path directly with its own file tools.
- **Line 2:** three spaces and the title, then ` — ` and the summary if one is present.
- **Line 3 (snippet):** three spaces, then the first body line (outside fenced code) that
  contains a matching token. If no body line matches, line 3 is omitted.
  - In the snippet, each matching token is wrapped in `**`.
  - If the line is longer than 160 code points, a window of 160 is taken, starting 40 code
    points before the first match (or at the start). `…` is added at each cut end.
  - Leading and trailing whitespace is trimmed.

**After the results, zero or more of these lines, in this order:**

```
N more visible matches not shown (raise --limit)
N matches hidden by status (use --all)
Store root: <absolute store path>
```

The `Store root` line is printed whenever there is at least one result, so that relative paths
can be resolved without another call.

**When nothing matches at all,** including hidden entries:

```
No entries matched any term of: <query as given>
```

That case exits 3. If the only matches are hidden, the hidden-count line is printed and the
command still exits 3. Otherwise it exits 0.

**JSON output** (`--json`): a single object followed by LF, keys in this order:

```json
{"query":"...","results":[{"rank":1,"type":"rule","status":"active","id":"...","title":"...","summary":null,"score":12.345679,"path":"rules/....md","snippet":"..."}],"more":0,"hidden":0}
```

- `score` is the rounded score, printed in fixed notation with exactly 6 digits after the
  decimal point.
- `snippet` is `null` when absent, and has no `**` markers.
- JSON output is compact (no spaces), with keys in the documented order and non-ASCII characters
  written literally.

### 8.6 Cache

Implementations MAY keep a search index under `.memex/`. It MUST be invisible: results MUST be
identical with the index deleted, stale or present. An implementation that uses one MUST check
each entry's path, size and modification time before every query and reindex what changed.

---

## 9. Writing to the store

### 9.1 Write protocol

Every mutating command (`new`, `capture`, `promote`, `set`, `fmt`, `save`, `sync`) runs as one
unit:

1. **Lock.** Take an exclusive advisory lock on `.memex/write.lock`, creating `.memex/` if
   needed. Wait up to 60 s, then exit 4.
2. **Freshen** (not for `sync`, which does its own): `git fetch` the current branch's upstream.
   If the branch is strictly behind, `git merge --ff-only`. If the fetch fails, continue and
   record the reason for the final report.
3. **Validate inputs** and compute the full new contents of every file to write. Nothing has
   been written yet; any error here exits without changes.
4. **Write** each file via a temporary file in the same directory, then rename it into place.
5. **Read back.** Re-read every written file, check its bytes equal what was intended, and run
   §7 validation on the written entries and their link targets. On failure, restore the
   previous contents (or delete new files) and exit 1.
6. **Commit.** `git add -- <paths>`, then `git commit -m <message> -- <paths>`. Only these paths
   are committed. Other changes in the working tree or index are left as they are.
7. **Integrate.** If there is no upstream, stop here (outcome `committed, no upstream`).
   Otherwise:
   - `git pull --rebase --autostash`, then `git push`.
   - If the push is rejected as non-fast-forward, repeat this step, at most 5 attempts in total.
   - If the rebase conflicts, run `git rebase --abort`; the local commit is kept and the outcome
     is `conflict` (exit 6).
8. **Unlock and report** (§9.4).

If git reports the index is locked by another process, retry with delays of 0.2, 0.4, 0.8, 1.6
and 3.2 s, then fail with exit 4.

memex adds no trailers to commit messages. Author and committer come from the user's git
configuration.

### 9.2 Commit messages

Exactly:

| Command | Message |
|---|---|
| new / capture | `memex: new <type> <id>` |
| promote | `memex: promote <source-id> to <type> <new-id>` |
| set | `memex: set <id> <field>=<value>[ <field>=<value>…]`, in argument order |
| fmt | `memex: fmt <n> entries` |
| save | `memex: save <n> files`, or the `-m` text if given |

### 9.3 Direct edits

**Editing is first-class.** People and agents MAY edit entries with any editor.
`memex save [-m MSG] [PATHS…]` commits them through §9.1, steps 1–2 and 5–8. With no paths it
takes every added, modified or untracked file under the type directories, `files/`,
`AGENT.md` and `memex.toml`.

**Deletions:**
- A deleted entry is refused (exit 1), unless `--allow-delete` is given.
- A deleted file under `files/` is allowed.

**Rejected alternative:** the predecessor ran each write in a throwaway git worktree, to protect
shared append-only files. With one file per entry, two writers touch the same file only when
they edit the same entry. That is a real conflict, and step 7 reports it.

### 9.4 Outcomes

The last line of every mutating command's standard output is exactly one of these:

| Line | Exit |
|---|---|
| `pushed <sha7>` | 0 |
| `committed <sha7>, no upstream` | 0 |
| `nothing to commit` | 0 |
| `committed <sha7>, NOT pushed: <reason>` | 5 |
| `committed <sha7>, NOT integrated: rebase conflict in <paths>` | 6 |

A commit is never undone automatically.

### 9.5 Sync

`memex sync` performs §9.1 steps 1 and 7 without a commit:
1. Fetch.
2. If ahead or behind, pull with rebase and autostash, then push if ahead.

It prints:
- `fetched <time>`, where time is now in RFC 3339 UTC,
- then one of `up to date`, `fast-forwarded N commits`, `pushed N commits`, or
  `rebased and pushed N commits`,

followed by the outcome line where one applies. If the fetch fails, it prints
`NOT SYNCED: <reason>` and exits 5.

---

## 10. Command-line conventions

- **Form:** `memex [GLOBAL OPTIONS] COMMAND [OPTIONS] [ARGS]`. Global options: `--store PATH`.
- **Options:**
  - They may appear before or after positional arguments.
  - `--` ends options.
  - Long options only, except `-m` (`save`) and `-h`.
  - `--opt=value` and `--opt value` are equivalent.
- **Streams:**
  - Standard output carries results only.
  - Diagnostics go to standard error, prefixed `memex: `.
  - No color, no progress bars, no paging, no prompts.
- **`--body FILE`** reads the file. `--body -` reads standard input to EOF.
- **Titles and other values** that begin with `-` MUST be passed as `--title=-x`. A positional
  value or a bare `--title -x` beginning with `-` is a usage error, never a silently accepted
  title. The predecessor created junk entries from `capture --help`.
- **Help:** `memex help …`, `memex COMMAND --help` and `-h` print help to standard output and exit
  0. They never perform the command, and they work without a store. See §16.
- **Version:** `memex version` and `memex --version` print `memex <version> (store format 1)`.
- **Exit codes:**

  | Code | Meaning |
  |---|---|
  | 0 | Success |
  | 1 | Validation error, check failure, or refused operation |
  | 2 | Usage or configuration error |
  | 3 | Search found nothing visible |
  | 4 | Lock timeout |
  | 5 | Committed but not pushed, or sync failed |
  | 6 | Rebase conflict |
  | 70 | Internal error (a bug) |

---

## 11. Commands

### 11.1 `init`

```
memex init PATH [--name NAME]
memex init --pointers DIR [--force]
```

**Store form.** `memex init PATH` creates the directory if needed, runs `git init` if it isn't a
repository, and writes:
- `memex.toml` (`name` defaults to the directory basename);
- `AGENT.md` (Appendix A);
- the eight type directories, each with no files;
- `files/`;
- `.gitignore` containing `.memex/`;
- `.githooks/pre-commit` (Appendix A).

It also sets `git config core.hooksPath .githooks`.

- Existing nonempty files are never overwritten. Missing lines are appended to `.gitignore`.
- Running it again is a no-op.
- It prints one line per created item, then `initialized <path>`.
- It does not commit.

**Pointer form.** `memex init --pointers DIR` writes `CLAUDE.md`, `AGENTS.md` and `GEMINI.md`
into `DIR` with the pointer text of §12.
- A file that already exists with any other content is left alone and reported, and the
  command exits 1.
- `--force` replaces such a file and prints its previous content to standard output first.
- The pointer form does not commit.

### 11.2 `new` and `capture`

```
memex new TYPE --title=T --summary=S --body FILE|- [--status S] [--tag X]… [--keyword K]…
          [--repo R]… [--no-repo] [--host H]… [--no-host] [--always] [--kind K]
          [--parent ID] [--source ID]… [--verified DATE] [--review-days N] [--alias A]…
memex capture --title=T --summary=S --body FILE|- [same options]
```

`capture` is `new journal`. Behavior:

1. Validate the options against §5.2 and §5.3 (for example, `--always` only on rules).
   `--summary` is required, because search results and lists show it, and it is what lets an
   agent decide whether to open an entry without reading it. `--keyword` SHOULD be given
   whenever the body's vocabulary differs from how someone would ask about it.
   `verified` defaults to today for references.
2. Normalize the body (§5.1) and check its content measure (§5.6).
3. Compute the id (§5.4).
   - If the id already exists with the same type and an identical body: print
     `exists <path>` and `nothing to commit`, and exit 0.
   - If it exists with a different body or type: print `exists <path>` to standard error and
     exit 1, changing nothing.
4. Write and commit (§9.1). Standard output gets `created <path>`, then the outcome line.

### 11.3 `search`

```
memex search QUERY… [--type T]… [--tag X]… [--repo R]… [--host H]… [--all] [--limit N] [--json]
```

All positional arguments are joined with single spaces into the query. The rest is §8.

### 11.4 `preflight`

```
memex preflight QUERY… [--no-sync] [search options]
```

This is the start-of-task ritual. It prints:

1. **Sync line.** Unless `--no-sync` is given or `sync_on_preflight = false`, it runs `sync` and
   prints its first status line, prefixed `store: `. On failure it prints
   `store: NOT SYNCED: <reason>` and continues. With sync skipped, it prints
   `store: not synced (last fetch <time or "never">)`.
2. **Blank line, then `## Always`.** For each visible rule with `always = true`, in id order:
   - `- [[<id>]] <title>`;
   - then the body's first paragraph (lines up to the first blank line, outside fenced code),
     each line indented two spaces, cut to 600 code points total, with `…` appended if cut.
3. **Blank line, then `## Results`,** and the §8.5 text output for the query.

**Output budget.** `--max-chars N` (default 12000, minimum 2000) bounds the whole output in
code points.
- The sync line and the `## Always` section are never cut.
- If they alone exceed half the budget, a final line
  `warning: always rules use <n> of <N> characters; review them` is added.
- Search results are then dropped from the end, whole blocks at a time, until the output fits.
- When any result was dropped, a last line reads:
  `output budget reached: <k> results not shown (raise --max-chars or run memex search)`.

The exit code is the search's. A failed sync never changes it.

### 11.5 `list`

```
memex list [--type T]… [--status S]… [--tag X]… [--repo R]… [--host H]… [--all] [--limit N]
           [--fields F,…] [--json]
```

- **Order:** descending `created`, then ascending id.
- **Visibility:** by default only visible entries are listed. `--status` selects statuses
  explicitly, and `--all` lists every entry.
- **Limit:** `--limit N` prints the first N entries, then
  `N more entries not shown (raise --limit)` if any were cut.
- **Text output:** one line per entry, `<created> <type> <status> <id> <title>`, separated by
  single spaces.
- **Fields:** `--fields` selects and orders the columns, from `created, type, status, id, path,
  title, summary, tags`. Columns are separated by a tab in text output.
- **JSON output:** an array of objects with keys `id, type, status, created, title, summary,
  tags, path`.

`list` replaces the predecessor's committed INDEX file.

### 11.6 `show`

```
memex show ID [--json]
```

Prints the entry file verbatim, then:

```

--- links
cited by: <id>, <id>
supersedes: <id>
children: <id>
links to: <id>
linked from: <id>
```

- **cited by:** entries listing ID in `sources`.
- **supersedes:** entries whose `superseded_by` is ID.
- **children:** entries whose `parent` is ID.
- **links to:** `[[...]]` targets in the body.
- **linked from:** entries whose body links to ID.

Lines with no ids are omitted. ID may also be an alias. An unknown id exits 1.

### 11.7 `set`

```
memex set ID ASSIGNMENT…
```

| Assignment | Effect |
|---|---|
| `field=value` | Sets a field |
| `field=` | Removes an optional field |
| `field+=value` | Appends to an array field |
| `field-=value` | Removes from an array field |

- Arrays given in `field=` form are comma-separated.
- Booleans are `true` or `false`. Dates are `YYYY-MM-DD`.
- `title`, `status`, `summary` and every other §5.3 field may be set, except `created` and
  `body_sha256` (exit 1).
- The result must validate. In particular `status=superseded` needs `superseded_by=` in the
  same command, and leaving `superseded` removes `superseded_by` automatically.
- The body bytes are preserved exactly.
- Commits through §9.1.

### 11.8 `promote`

```
memex promote SOURCE-ID --to TYPE --title=T --summary=S --body FILE|- [new options]
```

1. Creates a new entry, as `new`, with `sources` containing SOURCE-ID plus any `--source` values.
2. If the source is a journal entry, sets its status to `promoted`. Sources of other types are
   unchanged.

Both files go in one commit. Read-back (§9.1, step 5) covers both. Promoting a source whose
status is already `promoted` is allowed.

### 11.9 `fmt`

```
memex fmt [PATHS…] [--check]
```

Rewrites entries into canonical form (§5.1) without changing any field value or the body.
- With no paths it processes all entries.
- `--check` writes nothing, lists the non-canonical paths, and exits 1 if any.
- Otherwise commits through §9.1.

### 11.10 `save`

§9.3.

### 11.11 `sync`

§9.5.

### 11.12 `check`

```
memex check [--staged] [--strict] [--workspace]
```

§7.

### 11.13 `doctor`

```
memex doctor
```

Read-only. Prints one line per check, `PASS|WARN|FAIL <id> <message>`, in this order:

| Id | Check |
|---|---|
| `git` | `git` is runnable; version ≥ 2.30 |
| `config` | The config file parses (or is absent) |
| `store` | The store resolves and `memex.toml` is valid (FAIL if not; later store checks are skipped) |
| `format` | The store format is supported |
| `upstream` | The current branch has an upstream (WARN if not) |
| `fetch-age` | Age of `.git/FETCH_HEAD`'s mtime: WARN over 8 h, or if it doesn't exist |
| `clean` | WARN if the store has uncommitted changes, unpushed commits, or a rebase in progress |
| `hooks` | `core.hooksPath` is `.githooks` and the pre-commit hook exists |
| `path` | Every `memex` executable on `PATH` resolves to the same file (WARN when shadowed) |
| `check` | Summary of `memex check` (FAIL if it has errors) |
| `leaks` | §12 leak scan. WARN with one `LEAK <path>` line per finding, printed after this line |
| `tool-memory` | §12 rule 5 |

Exit 1 if any check FAILs, else 0.

### 11.14 `import-leak`

```
memex import-leak PATH [--title=T]
```

Creates a `raw` journal entry from a file found by the leak scan.
- The title defaults to the file's first ATX heading text, else its filename.
- The body is the line `Imported from \`<PATH>\` (<today>).`, then a blank line, then the file
  content, normalized.
- It never deletes PATH. On success it prints `remove <PATH> after review` before the outcome
  line.

### 11.15 `import`

§13.

### 11.16 `help`

§16.

### 11.17 `template`

```
memex template TYPE
```

Prints Appendix B's outline for TYPE.

---

## 12. Tool neutrality

Coding tools each have private memory: Claude Code's `~/.claude/projects/*/memory/` and
`~/.claude/CLAUDE.md`, Codex's `~/.codex/`, Cursor's rules files, Copilot's instructions file,
and their successors. Knowledge written there is invisible to every other tool and is lost when
the tool changes.

1. **Knowledge lives only in the store.**
2. **Pointer files contain only the pointer.** Their exact content is the following single line,
   followed by LF:

   ```
   Read <store>/AGENT.md before starting work, and follow it.
   ```

   `<store>` is the store path as configured (with `~` kept if it was written that way).
   `check --workspace` reports `E-POINTER <path>` for every `CLAUDE.md`, `AGENTS.md` or
   `GEMINI.md` under the workspace whose content differs.
   - The scan reaches depth 4.
   - It does not enter `.git`, `node_modules`, `vendor`, `.worktrees` or the store itself.
   - It does not follow symlinks.
3. **Tool stores are scanned and never read as a source.** The leak scan reports, as
   `W-LEAK <path>`, every regular non-empty file at these locations:
   - `~/.claude/projects/*/memory/**`
   - `~/.claude/CLAUDE.md`
   - `~/.codex/AGENTS.md`
   - `~/.gemini/GEMINI.md`
   - under the workspace (same depth and exclusions as rule 2): `.cursorrules`,
     `.cursor/rules/**`, `.github/copilot-instructions.md`, `.windsurfrules`

   This list lives in the specification. Adding a tool means adding a line here and a
   conformance case. `import-leak` moves the content into the store.
4. **Integration is one-way.** Tools' hooks call memex (for example `memex preflight` at session
   start). memex knows nothing about tools beyond the pointer text and the paths in rule 3.
5. **Switching off a tool's own memory is configuration, not memex's job.** `doctor`'s
   `tool-memory` check reports WARN when Claude Code's auto memory isn't disabled. It counts as
   disabled when any of these is true:
   - `CLAUDE_CODE_DISABLE_AUTO_MEMORY=1` is set in memex's environment;
   - `"autoMemoryEnabled": false` appears in `~/.claude/settings.json`, in
     `/etc/claude-code/managed-settings.json`, or in any `*.json` under
     `/etc/claude-code/managed-settings.d/`.

   If `~/.claude` doesn't exist, the check PASSes.

---

## 13. Importing a brain v3 store

```
memex import brain-v3 SRC… --out DIR
```

Converts one or more brainctl v3 brains into a new store at `DIR`.

### 13.1 Guarantees

- **Read-only on sources.** Each SRC is a git working tree, and memex never writes to it. A
  dirty SRC is refused (exit 1), since the import must describe a commit.
- **Fresh output.** `DIR` MUST NOT exist. memex creates it as in §11.1 and writes all files. It
  does not commit. The operator reviews and commits.
- **Deterministic.** The same SRC commits, in the same order, with the same `MEMEX_TODAY`,
  produce byte-identical `DIR` contents. Re-running against later commits brings over what was
  written in between.
- **Lossless, and accounted for** (§13.6).

### 13.2 Source layout recognized

The first SRC is the primary (global) brain. Each SRC contributes:
- `.brain/investigations/*.md`, `.brain/decisions/*.md`, `.brain/runbooks/*.md`;
- `.brain/plans/**` (top-level files, plus `prompts/`, `reports/` and other subdirectories);
- `.brain/dispatch-logs/*`;
- `.brain/known-mistakes.md`, `.brain/constraints.md`, `.brain/INDEX.md`, `.brain/AGENT.md`;
- `docs/**/*.md`, including `docs/runbooks/*.md` and `docs/backlog.md`.

Everything else tracked in SRC is listed in the report as `not-knowledge` (scripts, hooks,
templates, pointer files, configuration). It is accounted for, and not imported.

### 13.3 Units and mapping

A **unit** is the smallest piece of source that becomes one entry.

| Source unit | Type | Title | Date | Status |
|---|---|---|---|---|
| Investigation file | journal | `# Task:` text, else first ATX heading, else filename | Filename date, else first-commit date | INDEX row status, else the `## Status` section's first line. Mapped as `raw`/`in-progress` → `raw`; `promoted` → `promoted`; `superseded` → `superseded` if resolvable (§13.5), else `kept` plus tag `was-superseded`; anything else → `kept` |
| Decision file | decision | `# Decision:` text, else first heading | Filename date, else first-commit date | `accepted`; `superseded` if the `## Status` section's first line contains "supersed" and a target resolves |
| Runbook file (either directory) | runbook | `# Runbook:` text, else first heading | First-commit date | `active` |
| Plan file under `plans/` (top level) | plan | First heading | Filename date, else first-commit date | `active`; `done` if the first 20 lines contain `DONE` or `COMPLETE` in bold (`**…**`) |
| File under `plans/prompts/` | plan, `kind = "prompt"` | First heading | as plans | `done` |
| File under `plans/reports/` | plan, `kind = "report"` | First heading | as plans | `done` |
| Other `plans/<dir>/**` | attachment | — | — | Goes to `files/<id>/`, where `<id>` is the plan whose source filename stem is the longest prefix of `<dir>`; if there is none, a `reference` entry titled `<dir>` is created to own it |
| Dispatch log file | log | Filename | Timestamp in the filename | `kept` |
| `known-mistakes.md` table row | rule | Symptom cell, markup stripped (§13.4), cut to 300 characters | Commit that introduced the row's line (`git blame`) | `active`, or `retired` per §13.4 |
| `known-mistakes.md` `## ` section | rule | Heading text without a `Mistake:` prefix | blame of the heading line | as above |
| `constraints.md` table row | rule | First cell, markup stripped, cut to 300 | blame | as above |
| `constraints.md` `## ` section other than `## Always` | rule | Heading text | blame | as above |
| `constraints.md` `## Always` top-level bullet | rule, `always = true` | First sentence of the bullet (up to the first `. ` or `—`), markup stripped, cut to 300 | blame | as above |
| `docs/backlog.md` `### ` item | task | Heading text | blame of the heading | From the enclosing `## In Progress` → `in-progress`, `## To Do` → `open`, `## Done` → `done`. Overridden by markers in the heading: ✓, `DONE` or `FIXED` → `done`; `SUPERSEDED`, `MOOT`, `RETIRED` or a fully struck-through heading → `dropped` |
| Other `docs/*.md` | reference | First heading | First-commit date | `active`, `verified` = last-commit date |
| `.brain/AGENT.md` of the primary SRC | `AGENT.md` | — | — | Copied verbatim into `DIR/AGENT.md`, for manual rewriting |

**Ids** are computed by §5.4 from the date and title, with one exception: file-based units
whose source filename stem is already a valid id keep it.
- **Collisions** inside one import get suffixes in source order: `-2`, `-3`, ….
- **Units from a secondary SRC** whose id collides with an identical body are dropped and listed
  as `duplicate-of <id>`. A different body gets the suffix `-<repo>`, where `<repo>` is the
  SRC's directory basename.

**Frontmatter of imported entries:**
- `summary`, `tags` and `repos` come from the INDEX row when one exists. INDEX `Context` values
  become tags.
- `aliases` gets every source path the unit was known by: the relative path, and the path with
  `.brain/` or `docs/` prefixes.
- Every non-INDEX `repos` value is the SRC's `BRAIN_REPO` from `.brain/AGENT.md`, if stated.

**Investigation headers:** the brainctl-written header (`# Task:` line and `## Status`,
`## Source Repo`, `## Context` sections, each holding one value line) is removed from the body,
because its values now live in the frontmatter. The removed lines are accounted as
`moved-to-frontmatter`.

### 13.4 Rule text

- **Body:**
  - A table row becomes a body with one paragraph per column: `**<Column header>:** <cell>`.
  - A section becomes its content without the heading line.
- **Stripping markup** for titles removes `**`, `__`, `~~`, backticks, and link syntax (keeping
  link text).
- **Retired:** a rule becomes `retired` when its title or the first 200 characters of its body
  contain `STALE`, `RESOLVED`, `NO LONGER`, `RETIRED` or `SUPERSEDED` (case-sensitive), or its
  title is fully struck through. The text is kept unchanged.

### 13.5 Link rewriting

In every imported body, path references to imported units are replaced by `[[id]]` when they
resolve to exactly one unit. A path reference is a substring matching
`(~/Programming/brain/|\./)?(\.brain/)?(investigations|decisions|plans|plans/prompts|plans/reports|runbooks|docs|docs/runbooks)/[A-Za-z0-9._-]+\.md`,
not inside fenced code. INDEX `File` values are resolved the same way.

- Unresolved references stay unchanged and are counted in the report.
- `body_sha256` is computed after rewriting.
- "Superseded by" resolution for statuses uses the first such reference found in a superseded
  callout (a line starting with `>` containing "Superseded").

### 13.6 Accounting

The import writes `DIR/.memex/import-report.json` and prints a summary.

- **Every line of every recognized source file** is attributed to exactly one of: a unit body,
  `moved-to-frontmatter`, `structural` (table header and separator rows, top-level file title,
  `## ` lines that only group `### ` items), or a **remainder**.
- **Remainder** text, meaning text in a source file that belongs to no unit (preambles,
  comments, closing notes), becomes one `reference` entry per source file. It is titled
  `<source path> (import remainder)` and contains the lines in source order, so nothing is
  dropped.
- **Per source file** the report lists lines and non-whitespace characters in, and the same out
  per category.
- The import exits 1 if any source line is unattributed. It also exits 1 if, for any unit, the
  non-whitespace characters of its source text differ from those of its output text. For this
  comparison:
  - table-cell delimiters (`|`) are excluded from the source text;
  - generated labels (`**<Column header>:** `) are excluded from the output text;
  - link rewrites are compared through the `rewritten` record, which holds each original path
    and its replacement.
- **INDEX rows** pointing to missing files are listed as `index-orphan`, with their summary
  preserved in the report.
- **Every `not-knowledge` file** is listed by path.

**Never automatic:** deduplication, rewording, or judging whether a rule is still true. Those
are a separate, reviewed curation pass, done with ordinary memex commands.

---

## 14. Out of scope

- Source-code task worktrees, checkpoints, tested-SHA integration, and host closeout
  (brainctl's `work`, `finish`, `session`).
- Workspace-wide repository status and sync. memex syncs only the store.
- Multiple stores, and per-repository local stores.
- Language-model summaries, embeddings, and any network service other than the store's git
  remote.
- Publishing or exporting parts of a store.
- Windows. Behavior is specified for POSIX systems; other platforms are untested.

---

## 15. Conformance suite

The suite lives in `conformance/` and is independent of the implementation's language. Any
implementation runs it through a runner that takes the path of the `memex` executable.

**Case layout:** each case is a directory `conformance/cases/<NNN-name>/` containing:

| File | Meaning |
|---|---|
| `store/` | Initial store contents (optional). The runner copies it to a temporary directory, runs `git init`, commits everything as `test <test@example.invalid>`, and adds a bare clone as `origin` with upstream tracking. |
| `env` | `KEY=VALUE` lines added to the environment. The runner always sets `MEMEX_STORE` (to the temporary store), `MEMEX_CONFIG` (to an empty file), `MEMEX_TODAY=2026-01-15`, `MEMEX_HOST=testhost`, `HOME` (a temporary directory), `TZ=UTC` and `LC_ALL=C`. |
| `cmd` | The argument vector, one argument per line, without the program name |
| `stdin` | Standard input (optional; empty if absent) |
| `expect/stdout` | Exact expected standard output. `<SHA7>` matches any 7 hex digits; `<TIME>` matches any RFC 3339 timestamp |
| `expect/exit` | Expected exit code |
| `expect/store/` | Optional. The exact expected store working tree after the command, excluding `.git` and `.memex` |
| `expect/commits` | Optional. Expected `git log --format=%s` of the store, newest first |
| `expect/stderr-contains` | Optional. Lines that must each appear in standard error |

A case with several steps uses `steps/01/`, `steps/02/`, …, each holding its own `cmd`, `stdin`
and `expect/`, sharing one store.

Every section of this specification marked Implemented MUST have at least one case, and each
`E-` and `W-` code MUST have a case that triggers it.

---

## 16. Help

Help is the way a person or a language model learns memex. It is part of the specified
interface, held to the same standard as any other output: exact, tested, and unable to drift
from the behavior it describes.

### 16.1 Goals

1. **Learnable in one call.** `memex help --all` prints everything needed to use memex
   correctly, in one output, with no need to read the source or this specification.
2. **Cheap to consult.** Every piece is reachable on its own and sized to be read quickly.
   An agent fetches only what it needs.
3. **Correct by construction.** Every example is executed by the conformance suite. Every
   option the parser accepts is documented, and nothing undocumented is accepted.
4. **Errors teach.** A usage error names the problem and the exact help command that explains
   it.

### 16.2 Forms

| Invocation | Prints |
|---|---|
| `memex help` or `memex --help` | The overview (§16.4) |
| `memex help COMMAND`, `memex COMMAND --help`, `memex COMMAND -h` | The command page (§16.5) |
| `memex help TOPIC` | The topic page (§16.6) |
| `memex help topics` | One line per command and topic: name, two spaces, one-line description |
| `memex help --all` | The overview, every topic page, then every command page, in the order of §16.6 and §11. Pages are separated by a line `---`. |
| `memex help --json` | The machine-readable interface (§16.7) |

- An unknown name exits 2 and prints, on stderr:
  `memex: no help for "<name>"; did you mean "<nearest>"? Run: memex help topics`.
  - The nearest name is the one with the smallest Levenshtein distance, and only if that
    distance is ≤ 3; otherwise the `did you mean` clause is omitted.
  - The same suggestion rule applies to an unknown command.
- Help never needs a store, configuration or network, and never writes anything.

### 16.3 Format and sources

- **Format.** All help output is Markdown, plain enough to read raw in a terminal:
  - ATX headings, lists, tables, and fenced code blocks;
  - lines at most 100 characters, except table rows and code;
  - no ANSI escapes.
- **Help text is data, not code.** It lives in this repository as `help/*.md`: one file per
  command (`help/cmd-<command>.md`), one per topic (`help/topic-<name>.md`) and
  `help/overview.md`. An implementation embeds these files and prints them verbatim. The one
  exception is the first line of the overview, which carries the version (§16.4). This keeps
  help identical across implementations in different languages.
- **Only implemented behavior is documented.** A command or option enters help in the commit
  that implements it (§0).

### 16.4 Overview

`help/overview.md`, at most 80 lines, in this order:

1. **Version line:** `# memex <version> (store format 1)`.
2. **The agent line:** "If you are an AI agent: run `memex help agents` now (60 lines), or
   `memex help --all` once for everything."
3. **What memex is:** two or three sentences.
4. **The model:** store, entry, type, id, status, visibility, and frontmatter, in one short list.
5. **The everyday loop,** as a runnable example:
   - `memex preflight "…"` before work;
   - `memex capture --title=… --summary=… --body FILE` after it;
   - edit an entry, then `memex save`.
6. **The command table:** each command with its one-line description.
7. **The topic table.**
8. **Where to go next:** `memex help topics`.

### 16.5 Command pages

Each `help/cmd-<command>.md` has these sections, in this order. Sections that don't apply are
omitted.

1. `# memex <command>`, followed by its one-line description.
2. **Usage:** the synopsis exactly as in §11.
3. **Description:** what it does and when to use it, at most 15 lines.
4. **Options:** a table with columns option, value, default, meaning, covering every option the
   command accepts and no others.
5. **Output:** the exact shape of standard output, including the outcome line (§9.4) for
   mutating commands.
6. **Exit codes:** only the codes this command can return.
7. **Examples:** two to five. Each is a fenced block marked ` ```example `, holding the command
   line prefixed `$ ` followed by its exact output.
8. **Mistakes to avoid:** the misuse the command refuses, and why. Example for `capture`:
   titles starting with `-`, empty bodies, and writing to a tool's memory instead.
9. **See also:** related commands and topics.

Each page is at most 120 lines.

### 16.6 Topic pages

In this order (the `--all` order):

| Topic | Covers |
|---|---|
| `agents` | Rules of engagement for language-model agents, at most 60 lines: run `preflight` before work; capture after it; never write a tool's own memory; edit then `save`; respect outcome lines and exit codes; journal bodies are immutable; how to supersede instead of editing history. Written as imperatives. |
| `concepts` | The model and the reasoning behind it (§1, §2) |
| `entries` | Types, statuses, visibility and every frontmatter field, as tables (§5.2, §5.3) |
| `ids` | The id and slug rules, with worked examples (§5.4) |
| `links` | Link syntax and resolution (§6) |
| `search` | Query syntax, prefix terms, phrases, ranking, filters, and how to read output, including hidden-count lines (§8) |
| `writing` | The write protocol, direct edits, outcomes and conflicts (§9) |
| `config` | Store resolution, the config file and environment variables (§3) |
| `check-codes` | Every `E-` and `W-` code, with its condition and the usual fix (§7) |
| `exit-codes` | The §10 table |
| `neutrality` | Pointer files, the leak scan and tool memory (§12) |
| `import` | The brain v3 importer and its report (§13) |

Each topic is at most 200 lines. `memex help --all` as a whole MUST stay under 3,000 lines. The
conformance suite measures this, so that reading all of memex stays one affordable call.

### 16.7 Machine-readable interface

`memex help --json` prints one compact JSON object, keys in this order:

```json
{"version":"…","format":1,
 "commands":[{"name":"…","summary":"…","usage":"…","mutating":true,
   "options":[{"name":"--title","value":"T","repeatable":false,"required":true,"default":null,"summary":"…"}],
   "positionals":[{"name":"QUERY","repeatable":true,"required":true}],
   "exit_codes":[0,1,2]}],
 "types":[{"name":"rule","directory":"rules","statuses":["active","superseded","retired"],"visible":["active"]}],
 "fields":[{"name":"title","kind":"string","required":"yes","allowed_on":["*"]}],
 "exit_codes":[{"code":0,"meaning":"Success"}],
 "topics":[{"name":"agents","summary":"…"}]}
```

- The JSON is generated from the same definitions the parser and validator use, never written
  by hand.
- `required` on fields is `"yes"`, `"no"`, or a condition string such as
  `"when status is superseded"`.

### 16.8 Usage errors

On a usage error (exit 2), stderr carries:
1. `memex: <problem>`;
2. the command's usage line;
3. `Run: memex help <command>`.

Examples of `<problem>`: `unknown option --tittle (did you mean --title?)`, `missing --body`,
`value for --title starts with "-"; write --title=-…`.

### 16.9 Guarantees enforced by the conformance suite

- Every command and topic has a page. `memex help topics` lists exactly them.
- Every option and positional accepted by a command appears in its page's Options table and in
  `help --json`. Conversely, every documented option is accepted.
  - The runner checks this by comparing `help --json` with the Options tables.
  - It also tries each documented option. An undocumented `--x` must fail as unknown.
- Every ` ```example ` block is executed in a fresh store seeded from
  `conformance/help-fixture/`, and its output must match exactly.
- Size limits (§16.4–16.6) hold.
- `help --all` is byte-identical to the concatenation rule of §16.2.

## 17. Agent-first requirements

These are the concrete consequences of Principle 0. Each is testable, and each has conformance
cases.

### 17.1 Learnable without the source

- An agent that has never seen memex can use it correctly after `memex help agents`, and fully
  after `memex help --all` (§16).
- `AGENT.md` (Appendix A) tells it so in its first lines.

### 17.2 Economical with context

- **Bounded output.** Every command that can print an unbounded amount has a bound: `search`
  has `--limit`, `preflight` has `--max-chars`, and `list` has `--limit` (default unlimited)
  and `--fields`.
- **No filler.** No banners, no decorative output, no repeated headers, and no "success!"
  lines beyond the outcome line.
- **Triage before reading.** Results show title, summary and a one-line snippet, enough to
  decide whether to open the entry. That is why `summary` is required on new entries.
- **Truncation is announced.** Whenever output is cut, a final line says so and names the
  option that lifts the cut. Silent truncation is a defect.

### 17.3 Unambiguous and stable output

- **Stable shapes.** Text output has fixed line shapes (§8.5, §9.4, §11), and every command
  that prints data accepts `--json`. Changing a text shape counts as an interface change, and
  goes in `CHANGELOG.md` under "Changed output".
- **The outcome is always last.** For mutating commands, the last line is exactly one outcome
  line (§9.4).
- **Exit codes carry meaning** (§10). An agent can branch on the code without parsing text.
- **Paths are relative to the store root,** and the root is printed once, so an agent can use
  its own file tools.

### 17.4 Forgiving of model mistakes, never silently

Language models predictably make certain mistakes. memex catches each one and says how to fix it.

| Mistake | memex's response |
|---|---|
| A misspelled command, option, type, status, field or topic | Exit 2 with `did you mean "<nearest>"?` (Levenshtein distance ≤ 3), plus the help command |
| An unknown or invented id (`show`, `set`, `promote`, `--parent`, `--source`) | Exit 1 with `no entry "<id>"`, plus up to 3 nearest ids by search score of the given text, one per line, as `  candidate: <path>` |
| The same capture run twice | Idempotent no-op (§11.2) |
| A title starting with `-`, or an option swallowed as a value | Refused (§10) |
| An empty or heading-only body | Refused, with the content-measure rule and the template command |
| Editing a journal body | `check` fails with `E-HASH`, and the message names `set <id> status=superseded` plus `capture` as the correct path |
| Writing knowledge into a tool's memory | Detected by the leak scan (§12), with `import-leak` as the fix |
| A search phrased differently from the stored text | OR-combined ranked terms, prefix matching, and `keywords` (§8). The miss is reported, never silent |

### 17.5 Safe to run unattended

- Never interactive (Principle 9).
- Reads are side-effect free, apart from the search cache (§8.6) and `preflight`'s sync of the
  store.
- Writes are atomic and verified (§9.1). A write that fails part-way leaves the store as it was,
  or reports exactly what was left.
- Concurrent agents on one host are serialized by the write lock. Agents on different hosts are
  reconciled by rebase, and a true conflict is reported with exit 6, never resolved by guessing.

### 17.6 Writing for the next agent

memex's own guidance, in `help agents` and the templates, tells agents how to write entries
that the next agent will find and trust:
- Put the reader's likely words in `keywords`.
- Make `summary` a sentence that answers "should I open this?".
- Use one entry per lesson.
- Link with `[[id]]` rather than prose paths.
- Supersede instead of rewriting history.
- Record evidence (commands, commit pins, measured values) in the body.

## Appendix A. Files written by `init`

`AGENT.md`:

```markdown
# Agent instructions

This store is the shared memory for this operator's work. It is managed with `memex`.

- If you are new to memex: `memex help agents` (short), or `memex help --all` (everything).
- Before any non-trivial task: `memex preflight "<task in your own words>"`.
- After any non-trivial task: write what you learned to a file and run
  `memex capture --title="<task>" --summary="<one sentence>" --body FILE`.
- Never write knowledge into a coding tool's own memory or rules files.
  Knowledge lives here only.
- Edit entries directly when needed, then run `memex save`.

Operator-specific instructions follow.
```

`.githooks/pre-commit`:

```sh
#!/bin/sh
exec memex check --staged
```

The file mode is `0755`.

## Appendix B. Body templates

**rule**

```markdown
**Symptom:**

**Context:**

**Avoid:**

**Instead:**
```

**decision**

```markdown
## Context

## Decision

## Consequences
```

**runbook**

```markdown
## When to use

## Steps

## Verify
```

**reference**

```markdown
## Current state

## How to re-verify
```

**task**

```markdown
## Goal

## Done when
```

**plan**

```markdown
## Context

## Steps

## Verification
```

**journal**

```markdown
## What was attempted

## What worked

## What failed

## Reusable insights
```

**log**

```markdown
(raw log text)
```
