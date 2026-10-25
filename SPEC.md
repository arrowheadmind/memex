# memex — specification

**Store format version 1. Tool version: 0.1.0 (released 2026-10-17).**

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
| 3 Configuration | Implemented |
| 4 Store layout | Implemented |
| 5 Entries | Implemented |
| 6 Links | Implemented |
| 7 Validation | Implemented |
| 8 Search | Implemented |
| 9 Writing to the store | Implemented |
| 10 Command-line conventions | Implemented |
| 11 Commands | Implemented |
| 12 Tool neutrality | Implemented |
| 13 Importing a brain v3 store | Implemented |
| 15 Conformance suite | Implemented |
| 16 Help | Implemented |
| 17 Agent-first requirements | Implemented |
| 9.2 `update` commit message | Planned |
| 11.7 A status change that crosses visibility needs a new summary | Planned |
| 11.17 `update` | Planned |

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
   and the type is the directory, so neither is repeated in the file. Nothing memex derives
   from entries (lists, indexes, search caches) is committed. The predecessor kept status in
   three places, and they drifted apart.
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
- A leading `~/` and the strings `$HOME` and `${HOME}` are expanded: `~/` only at the start,
  `${HOME}` wherever it occurs, and `$HOME` only where it is followed by `/` or ends the path
  (`$HOMEDIR/x` is left alone). No other expansion takes place, and symlinks are not resolved.
- Expanding `~/`, `$HOME` or `${HOME}` while `HOME` is unset fails with exit 2:
  `memex: cannot expand ~: HOME is not set` (naming `$HOME` or `${HOME}` for those forms).
- A variable of §3.3 set to the empty string counts as unset.
- A relative path is an error.
- The path is then cleaned lexically (repeated `/`, `.` segments, `..` after a name and a
  trailing `/` are removed), so `~/memex-store/` and `~/memex-store` are the same store and
  every path memex prints from it has the clean form.
- The path MUST be a directory containing `memex.toml`, otherwise the command fails with
  exit 2.
- If nothing is set, every command except `init PATH`, `import`, `doctor`, `version` and `help`
  fails with exit 2 and this message on stderr (`init --pointers` needs the store, because the
  pointer text names it):

  ```
  memex: no store configured; use --store, MEMEX_STORE, or store = "..." in <config path>
  ```

There is no default path and no search of parent directories.

### 3.2 Config file

The path is `MEMEX_CONFIG` if set. Otherwise it is `$XDG_CONFIG_HOME/memex/config.toml` when
`XDG_CONFIG_HOME` is set, or else `~/.config/memex/config.toml`.

- A missing file is not an error. When neither `XDG_CONFIG_HOME` nor `HOME` is set there is no
  config file, which is not an error either.
- A file that won't parse, or that has an unknown key, is an error (exit 2).

Keys:

| Key | Type | Default | Meaning |
|---|---|---|---|
| `store` | string | none | Store path (§3.1) |
| `workspace` | string | none | Root for pointer checks and the leak scan (§12) |
| `sync_on_preflight` | boolean | `true` | `preflight` syncs the store first |
| `search_limit` | integer ≥ 1 | `12` | Default `--limit` for `search` and `preflight` |
| `list_limit` | integer ≥ 1 | `50` | Default `--limit` for `list` |
| `network_timeout` | integer ≥ 1 | `20` | Seconds allowed for each `git fetch`, `pull` or `push` (§9.1) |

The config file holds settings only, never knowledge.

### 3.3 Environment variables

| Variable | Meaning |
|---|---|
| `MEMEX_STORE` | Store path (§3.1) |
| `MEMEX_CONFIG` | Config file path (§3.2) |
| `MEMEX_TODAY` | Overrides today's date (`YYYY-MM-DD`). Exists for tests and imports. If it is malformed: exit 2. |

memex reads no other variables of its own.

**Environment given to `git`.** memex runs `git` with the caller's environment, changed in
exactly these ways:
- These are removed, so that memex called from another repository's git hook still addresses
  the store: `GIT_DIR`, `GIT_WORK_TREE`, `GIT_INDEX_FILE`, `GIT_COMMON_DIR`, `GIT_PREFIX`,
  `GIT_OBJECT_DIRECTORY`, `GIT_ALTERNATE_OBJECT_DIRECTORIES`. The one exception is
  `check --staged`, which keeps `GIT_INDEX_FILE`, because `git commit -- <paths>` runs the
  pre-commit hook against a temporary index that only this variable names. The predecessor
  silently validated the wrong repository from a hook for this reason.
- `GIT_TERMINAL_PROMPT=0`, `GIT_EDITOR=true` and `GIT_SSH_COMMAND` extended with
  `-o BatchMode=yes` (set to `ssh -o BatchMode=yes` when it was unset), so that git never
  prompts (Principle 9). A credential that would need a prompt is a failed fetch or push,
  reported as such.
- `LC_ALL=C`, so that git's messages, which memex parses for the outcome reasons, are stable.

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
- `name` is a free string, shown by `doctor`. It is optional.
- Unknown keys are an error.

Type directories and `files/` MAY be absent: git doesn't track empty directories, and an
absent one means empty. memex creates a directory when it first writes into it.

Type directories are flat. Any other file or directory inside a type directory is a validation
error (§7). So is any top-level entry not in the list above, except dot-files and
dot-directories (`.git`, `.gitattributes`, `.editorconfig`, …), `LICENSE`, `flake.nix`,
`flake.lock`, `import-report.json` (§13.6) and `import-report-<YYYY-MM-DD>.json` or
`import-report-<YYYY-MM-DD>-<n>.json` (§13.7), which memex ignores. Dot-files and
dot-directories directly under `files/` are ignored in the same way.

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

**Canonical body.** In canonical form the body is in normalized form: no CR, no trailing
whitespace at the end, exactly one final LF. Journal bodies are the exception: their bytes are
whatever `body_sha256` covers, and `fmt` never touches them.

**Reading.** Any valid TOML in the frontmatter is accepted for reading. A file that isn't in
canonical form is a validation warning (§7), and `memex fmt` fixes it.

**Whitespace**, wherever this specification uses the word (body normalization, the content
measure, fence closing), means the code points with the Unicode `White_Space` property.

**Unicode version.** Every Unicode-dependent rule in this specification (NFKD, general
categories, simple lowercase mapping) is evaluated with the tables of **Unicode 17.0**, the
version the reference toolchain ships. Conformance cases use only code points whose properties
are the same in every Unicode table an implementation might ship, so an implementation whose
runtime ships other tables still passes them.

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

- **Rules cover both constraints and known mistakes.** The difference lives in the body.
- **Machine-produced logs are not a type.** A log belongs to the entry whose work produced it,
  as an attachment under `files/<id>/`. The predecessor's ten dispatch logs were all named
  after the plan they executed.
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
| 10 | `allow_secret` | boolean | no | all | Suppresses `W-SECRET` (§7), also on a journal entry, whose body cannot change. Placed beside `always`, the other boolean |
| 11 | `kind` | string | no | plan | `prompt` or `report`. Absent means an ordinary plan |
| 12 | `parent` | string | no | plan, task | Id of an enclosing plan |
| 13 | `sources` | array | no | all | Ids this entry was derived from |
| 14 | `superseded_by` | string | iff status is `superseded` | all | Id of the replacing entry |
| 15 | `verified` | date | yes | reference | When the facts were last checked against the live system |
| 16 | `review_days` | integer, 1–36500 | no | reference | Staleness threshold for `verified` |
| 17 | `aliases` | array | no | all | Former names or paths that resolve to this entry (§6) |
| 18 | `body_sha256` | string | yes | journal | Lowercase hex SHA-256 of the body bytes |

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
- `repos` is the name of the git repository containing the current directory: the basename of
  the parent of `git rev-parse --git-common-dir` (made absolute), so that a linked worktree such
  as `<repo>/.worktrees/<task>` is attributed to `<repo>`, not to `<task>`. It is empty when the
  current directory is not inside a repository, or when that repository is the store.
- `hosts` is empty, and only `--host` sets it; there is no host setting. The predecessor never
  filtered by host, and a default on every entry is noise.

`--repo NAME` and `--host NAME` (repeatable) set the fields. `--no-repo` empties the default.
This is the only use memex makes of the current directory.

### 5.6 Bodies

- The body is free Markdown. memex never edits a body except by writing one it was given, and
  during import (§13).
- **Content measure:** the number of non-whitespace Unicode code points in body lines that are
  not ATX headings (a line matching `^ {0,3}#{1,6}( |$)`). Lines inside fenced code blocks
  count: a runbook that is mostly commands is a full body. A body given to `new`, `capture` or
  `promote` MUST have a content measure of at least **80**; the command refuses it (exit 1)
  with the rule and a pointer to `memex help entries`. The predecessor accumulated captures
  that were only empty headings. An existing entry below 80 is a warning (`W-SHORT-BODY`),
  because imported units are often short and are still knowledge.
- **Journal bodies are immutable.** `body_sha256` is set when the entry is created.
  To correct one, create a new entry and set the old one's status to `superseded`.
- `memex help entries` prints a suggested body outline per type (Appendix B). It is never
  enforced.

**Fenced code blocks** (used here and in §6): a line matching `^ {0,3}(```+|~~~+)` opens a
block, which is closed by a later line starting (after up to 3 spaces) with at least as many
of the same character and nothing else but whitespace. An unclosed block runs to the end of the
body.

---

## 6. Links

**Syntax:** `[[TARGET]]` or `[[TARGET|TEXT]]`. TARGET is text without `]`, `|` or LF that
either matches the id pattern of §5.4 or equals an alias of some entry; TEXT is any text
without `]` or LF. Anything else between `[[` and `]]` (for example a shell test like
`[[ -f x ]]`) is plain text, not a link. Links inside fenced code blocks and inside inline code
spans are ignored. An inline code span follows CommonMark: a run of N backticks opens it, and
the next run of exactly N backticks on the same line closes it (so `` `a``b` `` is one span); a
run with no such closing run is literal text.

**Resolution of TARGET, in order:**
1. The id of an entry: resolved.
2. Exactly one entry has TARGET in `aliases`: resolved, with a warning `W-ALIAS-LINK` that
   suggests the id.
3. TARGET matches the id pattern but no entry has it: an error (`E-LINK`).

A TARGET that is not an id but is an alias of more than one entry is unresolved: `E-LINK`.

**Field references** (`parent`, `sources`, `superseded_by`) MUST be ids. Aliases are not
accepted there.

**Markdown links** whose destination starts with `files/` are store-root-relative paths and
are checked (`E-FILE-LINK`). A `#fragment` or `?query` is removed and `%XX` escapes are decoded
first. The path must then be plain (no `.`, `..` or empty segment), and must resolve, following
symbolic links, to a regular file or directory whose real path is inside the store root.
Any other URL, path or `repo@sha` pin is free text and is not checked.

---

## 7. Validation

`memex check` validates the whole store. `memex check --staged` validates the entries that are
added or modified in the git index, plus the link targets they reference.

**Output** is one line per finding, sorted by path, then code, then message (all by code
point):

```
<path>: <CODE> <message>
```

The last line is `check: N errors, M warnings`. Exit 0 if there are no errors. With
`--strict`, warnings count as errors.

**Bounding the output.** `--errors-only` prints only errors. `--limit N` (default 200) prints
the first N finding lines and then one line per remaining code, in code-point order of the
code, `<CODE>: K more (raise --limit)`, so that a large imported store does not flood the
caller. The summary line is always last. `--limit 0` means no limit.

The summary counts every finding whatever the options: `--errors-only` only hides warning
lines, and `--strict` changes only the exit code. Messages compare as text, so within one
path and code a finding on line 12 sorts before one on line 7.

**`--staged`** reads the store as the git index holds it: the entries, their contents, the
layout and the existence of `files/` paths all come from the index, not the working tree.
When nothing is added or modified, an empty index included, there are no findings and the
exit code is 0.

A file directly under `files/` (not inside `files/<id>/`) is `E-FILES-OWNER`. Invalid TOML,
in an entry or the manifest, is reported only as `at line <line>`, the line in the file,
never with a parser's own wording.

**Messages.** Every message follows the template in the tables below, exactly. A message
starts with the thing that is wrong, names the value when there is one, and ends with the fix
when the fix is a command. In templates:
- `<id>` is the id of the entry the finding is about, and `<path>` its store-relative path.
- A quoted placeholder (`"<field>"`, `"<value>"`, …) is the value written as a TOML basic
  string (§5.1): in double quotes, with `\`, `"` and control characters escaped.
- `<paths>` is a list of store-relative paths in code-point order, separated by `, `.
- `<line>` is a line number in the file, counting from 1.
- `<new>`, `<word>` and `<sentence>` are written literally, for the reader to fill in.
- ` (did you mean "<nearest>"?)` is present only when the §16.2 rule finds a nearest name
  (Levenshtein distance ≤ 3, ties to the earlier candidate in the list named).
- Where a row lists several templates, the condition decides which one applies.

Errors:

| Code | Condition | Message |
|---|---|---|
| `E-ID` | Filename isn't a valid id (§5.4) | `filename is not a valid id: YYYY-MM-DD-slug, lowercase a-z, 0-9 and single hyphens, at most 120 characters before .md` |
| `E-DUP-ID` | Same id in two type directories | `id "<id>" is also used by <paths>` (the other paths) |
| `E-NOT-ENTRY` | Non-`.md` file or subdirectory in a type directory; unknown top-level item (§4) | Unknown top-level item: `unknown top-level item; it is not part of the store layout`. Subdirectory of a type directory: `subdirectory in a type directory; type directories are flat, attachments go in files/<id>/`. Other non-entry in a type directory: `not a .md file; type directories hold only entries, attachments go in files/<id>/`. A type directory that is a file: `not a directory; <dir>/ holds <type> entries`. `files` as a file: `not a directory; files/ holds attachments as files/<id>/`. `memex.toml`, `AGENT.md` or `README.md` that is not a regular file: `not a regular file` |
| `E-ENCODING` | Not UTF-8, has a BOM, or contains CR | One finding per problem: `not valid UTF-8`, `begins with a byte-order mark`, `contains CR characters; use LF line endings` |
| `E-FRONTMATTER` | Missing delimiters, or invalid TOML | `missing opening +++ line; the file must begin with the line +++`, `missing closing +++ line after the frontmatter`, `invalid TOML in the frontmatter at line <line>` |
| `E-FIELD-UNKNOWN` | Field not in §5.3, or not allowed on this type | `unknown field "<field>" (did you mean "<nearest>"?)` over the §5.3 names in canonical order; `field "<field>" is not allowed on a <type>; it is allowed on: <types>` with the "Allowed on" types of §5.3 separated by `, ` |
| `E-FIELD-MISSING` | Required field absent | `missing required field "<field>"` |
| `E-FIELD-VALUE` | Wrong kind or value outside its constraints | Wrong kind: `<field> must be a string`, `… must be a date written YYYY-MM-DD, unquoted`, `… must be an integer`, `… must be a boolean`, `… must be an array of strings`. Constraints: `title must be 1 to 300 characters on one line`; `summary must be one line of at most 500 characters`; `kind must be "prompt" or "report", not "<value>"`; `body_sha256 must be 64 lowercase hex digits`; `review_days must be between 1 and 36500`; `tags element "<value>" does not match ^[a-z0-9][a-z0-9-]*$`; `<field> element "<value>" contains a line feed` |
| `E-STATUS` | Status not valid for the type | `status "<value>" is not valid for a <type> (did you mean "<nearest>"?); valid: <statuses>`, the type's statuses in §5.2 order separated by `, ` |
| `E-CREATED` | `created` ≠ id date | `created <date> differs from the date in the id, <id date>` |
| `E-SUPERSEDED` | `superseded` without `superseded_by`, or `superseded_by` without `superseded` | `status is superseded but superseded_by is missing; add it: memex set <id> superseded_by=<new>`; `superseded_by is set but status is "<value>"; either memex set <id> status=superseded, or memex set <id> superseded_by=` |
| `E-REF` | `parent`/`sources`/`superseded_by` names a missing id, or `parent` isn't a plan | `<field> "<value>" is not the id of an entry`, followed by `; it is an alias of <path>, write its id "<target id>"` when exactly one entry has the value as an alias; `parent "<value>" is a <type>, not a plan` |
| `E-LINK` | Unresolved `[[...]]` | `[[<target>]] on line <line> is not the id of an entry`; `[[<target>]] on line <line> is an alias of several entries (<paths>); link an id` |
| `E-FILE-LINK` | Markdown link to a missing `files/...` path (§6) | `<destination> on line <line> is not a plain path under files/; write it without . or ..` when the decoded path contains a `.` or `..` segment or an empty segment before its end; `<destination> on line <line> is outside the store` when it resolves outside the store root; otherwise `<destination> on line <line> does not exist`. `<destination>` is written as in the link |
| `E-HASH` | `body_sha256` mismatch | `body does not match body_sha256; journal bodies are immutable, so restore the body, or capture the corrected text and supersede with: memex set <id> status=superseded superseded_by=<new>` |
| `E-ALIAS` | The same alias on two entries, or an alias equal to an id | `alias "<value>" is also on <paths>` (the other entries); `alias "<value>" is the id of <paths>` |
| `E-FILES-OWNER` | `files/<x>/` where `<x>` isn't an id | On `files/<x>`: `"<x>" is not the id of an entry; attachments go in files/<id>/`, written literally as `files/<id>/`; on a file directly under `files/`: `not a directory; attachments go in files/<id>/` |
| `E-MANIFEST` | `memex.toml` invalid | `invalid TOML at line <line>`; `unknown key "<key>" (did you mean "<nearest>"?)` over `format`, `name`; `missing key "format"`; `format must be an integer`; `name must be a string` |

Warnings:

| Code | Condition | Message |
|---|---|---|
| `W-CANONICAL` | Entry valid but not in canonical form (`memex fmt` fixes it) | `not in canonical form; run memex fmt <path>` |
| `W-ALIAS-LINK` | Link resolved through an alias | `[[<target>]] on line <line> resolves through an alias; link [[<target id>]] instead` |
| `W-STALE-REFERENCE` | `reference` with `review_days`, where `verified + review_days < today` | `verified <date> is more than review_days (<n>) ago; check the facts, then: memex set <id> verified=<today>` |
| `W-RAW-OLD` | `raw` journal entry created more than 90 days ago | `raw since <created>, more than 90 days ago; review it, then: memex set <id> status=kept, or memex promote <id>` |
| `W-PROMOTED-UNCITED` | `promoted` journal entry that no entry cites in `sources` | `promoted, but no entry lists "<id>" in sources` |
| `W-NO-TERMS` | Entry with neither `tags` nor `keywords` | `no tags or keywords; add the words a reader would search with: memex set <id> keywords=<word>,<word>` |
| `W-NO-SUMMARY` | Entry without `summary` (imports and hand-written entries may lack one) | `no summary; add one sentence that tells a reader whether to open the entry: memex set <id> summary=<sentence>` |
| `W-SHORT-BODY` | Content measure below 80 (§5.6) | `body content measure <n> is below 80; edit the file, then memex save` |
| `W-STALE-SUMMARY` | A task or plan with a `summary` whose newest significant version is newer than its newest summary-changing version (summary currency, below) | New section: `summary was not updated when the section "<heading>" was added; rewrite it to the current state: memex set <id> summary=<sentence>`. Status change: `summary was not updated when status changed from "<old>" to "<new>"; rewrite it to the current state: memex set <id> summary=<sentence>` |
| `W-SECRET` | A body line, fenced code included, looks like a credential: a PEM private-key header (`-----BEGIN `, then any of `A`–`Z`, `0`–`9` and space, then `PRIVATE KEY-----`); `password=`, `token=` or `secret=`, matched case-sensitively as written, followed by a non-placeholder value; or a run of 40 or more characters from `A`–`Z`, `a`–`z`, `0`–`9`, `+` and `/` that contains at least one uppercase letter, one lowercase letter and one digit (so hex digests such as git SHAs and nix store hashes never match), and that is preceded on the same line by one of the words `token`, `secret`, `password`, `passwd`, `bearer`, `authorization`, `api_key`, `api-key`, `apikey` or `private`, matched case-insensitively (without such a word the run is a public key or a content hash). The value is what follows `=` up to the next whitespace, or a whole `"…"` or `'…'` string, with its quotes removed; it is a placeholder when it is empty, starts with `$`, `<`, `{` or `%`, or consists only of `*`, `x`, `X`, `.` and `…`. Suppressed by a line `<!-- memex: allow-secret -->` anywhere in the body, or by `allow_secret = true` in the frontmatter (§5.3); a journal body cannot change (`E-HASH`), so the field is how a false positive there is cleared | One finding per line: `line <line> looks like a credential (<what>); remove it; if it is not one, add the line <!-- memex: allow-secret -->, or for a journal entry: memex set <id> allow_secret=true`, where `<what>` is `a private key header`, `a password, token or secret assignment` or `a run of 40 or more base64 characters`, the first that applies in that order |

**Summary currency.** Lists and search show an entry's summary, not its body (§8.5,
§11.5), so a task or plan whose body has moved on while its summary has not misleads every
reader who decides from the list. The usual cause is a status section appended by hand (or a
status set) without the summary being rewritten. memex cannot write the summary, because only
the author knows what the new section changes; it checks that the summary was touched.

- **Versions.** The versions of an entry are, newest first: the working-tree file when it
  differs from `HEAD` (with `--staged`, the index file when it differs from `HEAD`); then the
  file as it is in each commit on `HEAD`'s first-parent history that changes it, newest first.
  Each version is compared with the next older one. The oldest version (the entry's creation,
  or the oldest commit a shallow clone holds) is compared with nothing.
  - History is followed by path, without rename detection. A commit that adds the path, or
    that deletes it or makes it something other than a regular file, ends it: a file deleted
    and added again starts over at the add, and a file that `HEAD` does not hold has only its
    working-tree (or index) version.
  - A version whose frontmatter cannot be read (§5.1 delimiters, invalid TOML) is left out, as
    if that commit had not changed the file.
  - A store with no commits, or that is not a git repository, has no committed versions.
- A version **changes the summary** when its `summary` differs from the older version's (an
  absent summary counts as a value), and always when it is the oldest.
- A version is **significant** when, compared with the older version:
  - its body contains some ATX heading line (§5.6), outside fenced code blocks, more times than
    the older body does, lines compared exactly as written: a new section; the finding names
    the first such line in body order, as `<heading>`, without its leading spaces and `#`
    characters, the space after them and trailing whitespace; or
  - its status is visible by default (§5.2) and the older version's is not, or the reverse: the
    entry was closed or reopened; the finding names both statuses.
  When a version is significant for both reasons, the section is named. The oldest version is
  never significant.
- An entry of type task or plan that has a `summary` gets `W-STALE-SUMMARY` when its newest
  significant version is newer than its newest summary-changing version. A version that both
  adds a section and changes the summary is current. Rules, decisions, runbooks and references
  are not checked: their sections are rarely status notes, and references have `verified`.
- Reading history is the only use `check` makes of commits; the current findings of every
  other code come from the files alone. `check`, `check --staged` (and so the pre-commit hook)
  and `doctor`'s `check` read it; the validation of a write (§9.1 steps 3 and 5) uses errors
  only and does not. The history is read in one pass, so that `check` stays fast on a store of
  thousands of entries: one `git log` of `HEAD`'s first-parent history over `tasks/` and
  `plans/`, then one read of the file versions it names. A history that cannot be read in a
  git repository with commits is exit 2, `memex: check needs the store's git history: <reason>`.

`check --workspace` adds the pointer and leak findings of §12 (`E-POINTER`, `W-NO-POINTER`,
`W-LEAK`). Their `<path>` is absolute, `<dir>` is the directory that holds the file, and
`<file>` is the file name relative to it (`.claude/CLAUDE.md` included):

| Code | Condition | Message |
|---|---|---|
| `E-POINTER` | A pointer file name under the workspace whose content is not the pointer line naming the store (§12 rule 2) | Content differs, `<file>` is `CLAUDE.md`, `AGENTS.md` or `GEMINI.md`: `is not the pointer to this store; move any knowledge with memex capture --from <path>, then memex init --pointers <dir> --force`; for `CLAUDE.local.md` and `.claude/CLAUDE.md` the part after `then ` is `delete the file`. A symbolic link: `is a symbolic link, not the pointer file; delete it`, followed by `, then: memex init --pointers <dir>` for the three files `init` writes. Anything else that is not a regular file: `is not a regular file; delete it` |
| `W-NO-POINTER` | A git top-level directory under the workspace with no `CLAUDE.md` and no `AGENTS.md` | `repository has no CLAUDE.md or AGENTS.md; run memex init --pointers <dir>` (the finding's path is `<dir>`) |
| `W-LEAK` | A file found by the leak scan (§12 rule 3) | `knowledge in a coding tool's own memory; move it into the store: memex capture --from <path>, then delete the file` |

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

The corpus holds every entry whatever its other problems; the fields that are valid are used.
An entry that cannot be parsed (`E-ENCODING`, or an `E-FRONTMATTER` that leaves no fields) is left out, with `memex: skipped <path>: <problem>; run memex check` on standard error.

### 8.2 Tokenization

Applied identically to documents and queries:
1. NFKD, then remove category Mn.
2. Simple lowercase.
3. A token is a maximal run of code points of general category L*, N* or M*. Everything else
   separates tokens. Marks other than Mn survive step 1, so scripts that write vowels as
   spacing marks stay whole: `हिन्दी` is one token (`हिनदी`: its virama, an Mn, is removed; its
   vowel signs, Mc, stay).

There is no stemming and no stop-word list.

### 8.3 Queries and scoring

**Parsing:**
- A `"double-quoted"` span is a **phrase**: its tokens must appear consecutively in one field.
- Every other token is a **term**. A term whose normalized form (§8.2) is 4 or more code points
  long is a **prefix term**, matching any token that begins with it. Shorter terms match whole
  tokens only.
- A phrase matches whole tokens, never prefixes, also when it has one token; such a phrase is
  a different item from the term with the same token. A phrase with no tokens is no item.
- If an unmatched `"` remains, it is treated as a separator.
- Duplicate items (same kind and same normalized tokens) are dropped, keeping the first. Items
  are numbered in query order after this step.
- A query with no terms and no phrases is a usage error (exit 2).

Terms and phrases (together, *query items*) combine with **OR**: a document matches if any item
matches it. Each result reports *which* items matched it (§8.5), because a document that
matched one common word must not look like one that matched the whole query. The predecessor
learned this the hard way: an approximate hit read as an exact one is worse than silence.

**Score** (BM25 with k1 = 1.2 and b = 0.75 and the Lucene idf, which is positive even for an
item that most documents match; the formula below is normative):

```
N        = number of documents
n(q)     = number of documents matched by item q
idf(q)   = ln( 1 + (N − n(q) + 0.5) / (n(q) + 0.5) )
f(q,D)   = Σ over fields c of  w_c × hits(q, D, c)
|D|      = total token count of D across all fields
avgdl    = mean |D| over all documents
bm25(D)  = Σ over items q of  idf(q) × f(q,D) × (k1 + 1) / ( f(q,D) + k1 × (1 − b + b × |D| / avgdl) )
score(D) = bm25(D) × type_weight(D)
```

Where:
- `hits(q, D, c)` is the number of matching tokens (terms) or phrase occurrences (phrases) in
  field `c`. A phrase occurrence is counted at every start position, so occurrences may
  overlap (`"a a"` occurs twice in `a a a`).
- Field weights `w_c`: title 10, keywords 8, tags 5, aliases 3, body 1.
- Type weights: rule 1.5, runbook 1.3, reference 1.3, decision 1.2, task 1.0, plan 1.0,
  journal 0.8.

`N`, `n(q)` and `avgdl` are computed over **all** entries, whatever their status. Visibility
and `--type`/`--tag`/`--repo`/`--host` filters apply after scoring.

**Arithmetic.** All values are IEEE 754 binary64. The sums run in a fixed order: over fields in
the order of §8.1's table, over items in query order. `ln` is the natural logarithm of the
platform's math library; the rounding below absorbs last-bit differences.

**Ranking:** descending rounded score, then type in the order of §5.2's table, then ascending
id.

**Rounding.** The *rounded score* is the score rounded to 9 significant decimal digits, round
half to even on the exact binary value, and parsed back to binary64. Ranking and JSON output
use the rounded score. Two documents whose exact scores straddle a 9-digit boundary may still
order differently across implementations; the conformance suite avoids such cases.

### 8.4 Filters and visibility

- `--type T`, `--tag X`, `--repo R`, `--host H` are each repeatable. Values within one option are
  OR'd, and different options are AND'd. `--type` accepts the type name or its directory name
  (`rule` or `rules`).
- An entry is **visible** if its status is in the type's "visible by default" set (§5.2). `--all`
  makes every status visible.
- A matching document is therefore in exactly one of four groups: **shown** (passes the
  filters, visible, within the limit), **more** (passes the filters, visible, beyond the
  limit), **hidden** (passes the filters, not visible), or **excluded** (fails a filter).
- `--limit N` (default `search_limit`) caps the number of results printed; `--limit 0` means
  no limit.

### 8.5 Output

**Text output:** one block per result, numbered from 1:

```
1. [rule, active] rules/2026-09-12-bare-rebuild-leaves-the-closure-uncached.md  matched 2/3: cache, rebuild
   Bare rebuild leaves the closure uncached — Push the closure before switching, or the next boot rebuilds it.
   …the closure was never pushed to the **cache** so the weekly…
```

- **Line 1:** rank, then `[<type>, <status>]`, then the entry's path relative to the store root,
  then two spaces and `matched <k>/<n>: <items>`, where `n` is the number of query items, `k`
  how many matched this document, and `<items>` the matched items in query order, separated by
  `, `, phrases in double quotes. An item is printed in its normalized form (§8.2): a term as
  its token, a phrase as its tokens joined by single spaces (`Café` prints `cafe`, `"Nix  Build"`
  prints `"nix build"`); the JSON `items` and `matched` arrays use the same strings. The id
  is the filename stem, and an agent can open the path directly with its own file tools.
- **Line 2:** three spaces and the title, then ` — ` and the summary if one is present.
- **Line 3 (snippet):** three spaces, then the snippet, built like this:
  1. Take the first body line, outside fenced code, that contains a match. A line contains a
     match when, tokenized by §8.2, it has a token matching a term, or the consecutive tokens of
     a phrase.
  2. Trim leading and trailing whitespace from that line.
  3. Tokens are located in the *original* line as *runs*: a run is a maximal run of code
     points of category L*, N* or M* in the original text. A run matches when a token of its
     normalized form (§8.2) matches a term, or belongs to an occurrence of a phrase in the
     sequence of the line's run tokens. This keeps positions exact when NFKD changes lengths
     (`Café`, `ﬁ`), and `Les Cafés` written with a combining accent marks `**Cafés**` for the
     query `cafe`. Below, a *matching token* is a matching run.
  4. If the trimmed line is longer than 160 code points, keep a window of 160 code points that
     starts 40 code points before the first matching token's start (or at 0), moved left so
     that it ends at the line's end if it would overrun. If the window starts after 0, prefix
     `…`; if it ends before the line's end, append `…`. The window never splits a matching
     token: a matching token cut by the window edge is dropped from the window, which then
     starts after it or ends before it, and the whitespace this leaves at that edge is removed
     too (`…bbb **cache** ccc…`, not `ccc …`). A token that does not match may be cut.
  5. Wrap each matching token inside the window in `**`. For a phrase, wrap each of its tokens.
     Markers and `…` are not counted in the 160.
  If no body line contains a match, line 3 is omitted.

**After the results, zero or more of these lines, in this order:**

```
weak results: the best match covers <k> of <n> query items; try other words
N more visible matches not shown (raise --limit)
N matches hidden by status (use --all)
N matches excluded by filters (drop --type/--tag/--repo/--host)
Store root: <absolute store path>
```

The `weak results` line is printed when at least one result is shown, the query has `n ≥ 2`
items, and the top shown result matched `k` of them with `2k < n` (fewer than half). When no
entry is relevant, the best results are often matched by one common word (`a`, `for`, `add`),
and twelve such results with no signal read as an answer; this line says they are not.
A count line is printed only when its N is at least 1. The `Store root` line is printed
whenever there is at least one result, so that relative paths can be resolved without another
call.

**When no document matches at all:**

```
No entries matched any term of: <query as given>
```

That case exits 3. If documents matched but none is shown (all hidden or excluded), the count
lines are printed and the command still exits 3. Otherwise it exits 0.

**JSON output** (`--json`): a single object followed by LF, keys in this order:

```json
{"query":"...","items":["cache","rebuild","\"nix build\""],"results":[{"rank":1,"type":"rule","status":"active","id":"...","title":"...","summary":null,"score":12.345679,"path":"rules/....md","matched":["cache","rebuild"],"snippet":"..."}],"more":0,"hidden":0,"excluded":0,"weak":false}
```

- `weak` is `true` exactly when text output would print the `weak results` line.

- `score` is the rounded score, printed in fixed notation with exactly 6 digits after the
  decimal point, round half to even.
- `snippet` is `null` when absent, and has no `**` markers.
- `--json` always prints this object, also when nothing matched (`"results":[]`); the exit code
  is the same as in text mode.
- The encoding rules are those of §10.

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
   needed. Wait up to 60 s (10 s when the caller is `preflight`), then exit 4.
2. **Freshen** (not for `sync`, which does its own): `git fetch` the current branch's upstream,
   bounded by `network_timeout`. If the branch is strictly behind and the working tree has no
   uncommitted changes to tracked files, `git merge --ff-only`. If the fetch fails or times out,
   continue and record the reason for the final report. A dirty tree is never merged or
   rebased by this step. Before the fetch, a rebase, merge or cherry-pick in progress in the
   store refuses the command, exit 1, with `memex: rebase in progress; finish or abort it,
   then retry` (`merge in progress`, `cherry-pick in progress`): HEAD is then not the branch,
   and a commit would land on the operation's detached HEAD.
3. **Validate inputs** and compute the full new contents of every file to write. Nothing has
   been written yet; any error here exits 1 (or 2 for a usage error) with the reason on
   standard error and no outcome line, because no state changed.
   Every mutating command except `save` (and `init`, which is not one) first refuses, exit 1,
   with `memex: store has no commits yet; run memex save` when `memex.toml` is not in `HEAD`:
   the store's pre-commit hook could not validate the commit.
4. **Write** each file via a temporary file in the same directory, then rename it into place.
   A file that cannot be written (permission denied, disk full) restores what was written and
   exits 1 with `memex: cannot write <path>: <reason>; nothing was changed`, where `<path>` is
   store-relative and `<reason>` the operating system's error text without any path. A file
   that changed since the command read it (an edit made while memex worked) is not
   overwritten: `memex: <path> changed while memex was working; retry`, exit 1.
5. **Read back.** Re-read every written file, check its bytes equal what was intended, and run
   §7 validation on the written entries and their link targets. On failure, restore the
   previous contents (or delete new files) and exit 1. For `save`, which wrote nothing, this
   step is validation only: on failure the working tree is left exactly as found. Any error
   counts, also one the entry or a link target already had: the pre-commit hook would refuse
   the commit for the same finding.
   A validation failure, here or in step 3, is reported on standard error as the §7 findings
   it rests on, each as `memex: <path>: <CODE> <message>` in §7's order, then
   `memex: refused: the result would not validate; nothing was changed`: the caller learns the
   field and the rule without running `memex check`, and the last line stays fixed for the
   scripts that match it.
6. **Commit.** `git add -- <paths>`, then `git commit -m <message> -- <paths>`. Only these paths
   are committed. Other changes in the working tree or index are left as they are. After the
   add, each file memex wrote must be staged with exactly the bytes it wrote; otherwise the
   files and the index are restored and the command exits 1 with
   `memex: <path> changed while memex was working; retry`.
7. **Integrate.** If there is no upstream, stop here (outcome `committed, no upstream`).
   If tracked files other than `<paths>` are modified, stop here too: the outcome is
   `committed <sha7>, NOT pushed: uncommitted changes (run memex save)`. memex never rebases or
   autostashes a dirty tree, because an autostash that fails to re-apply leaves conflict
   markers inside entries. Otherwise:
   - `git pull --rebase`, then `git push`, each bounded by `network_timeout`.
   - If the push is rejected as non-fast-forward, repeat this step, at most 5 attempts in total.
   - If the rebase conflicts, run `git rebase --abort`; the local commit is kept and the outcome
     is `conflict` (exit 6). `<paths>` is `<unknown>` when git cannot list them. If the abort
     itself fails, the outcome is `NOT pushed: rebase abort failed: <git's first stderr line>`
     (exit 5). A pull that times out is `NOT pushed: timed out after <N>s`, never a conflict.
   - If the fetch, pull or push fails or times out, the outcome is `NOT pushed` (exit 5) with
     git's first stderr line, or `timed out after <N>s`, as the reason. A fetch that already
     failed or timed out in step 2 is not repeated; the outcome reports that reason.
8. **Unlock and report** (§9.4). `<sha7>` in the outcome line is the commit's hash *after* any
   rebase, which is the hash that exists on the remote.

If git reports the index is locked by another process, retry with delays of 0.2, 0.4, 0.8, 1.6
and 3.2 s, then fail with exit 4. After the commit (in step 7) the commit stays, and the
outcome is `NOT pushed` (exit 5) with that reason.

SIGINT and SIGTERM do not kill memex between steps 1 and 8. The running git process group is
sent SIGTERM and waited for, and the lock is held until the store is consistent. Before the
commit exists, the written files and the index are restored and the command exits 1 with
`memex: interrupted; nothing was changed`; once it exists, the outcome is
`committed <sha7>, NOT pushed: interrupted` (exit 5).

memex adds no trailers to commit messages. Author and committer come from the user's git
configuration.

### 9.2 Commit messages

Exactly:

| Command | Message |
|---|---|
| new / capture | `memex: new <type> <id>` |
| promote | `memex: promote <source-id> to <type> <new-id>` |
| set | `memex: set <id> <field>=<value>[ <field>=<value>…]`, in argument order; a value longer than 40 code points is cut to 39 plus `…` |
| set --batch | `memex: set <n> entries`, where `<n>` counts the entries the batch changed, then an empty line and a body of one line per changed entry, in file order, each exactly the message the single form would give for that line |
| fmt | `memex: fmt <n> entries` |
| save | `memex: save <n> files`, or the `-m` text if given |
| update (Planned) | `memex: update <id>`, followed by ` status=<value>` when `--status` changes the status |

### 9.3 Direct edits

**Editing is first-class.** People and agents MAY edit entries with any editor.
`memex save [-m MSG] [PATHS…] [--allow-delete] [--fmt]` commits them through §9.1, steps 1–2
and 5–8. With no paths it takes every added, modified, deleted or untracked file under the
store except `.memex/` and anything `.gitignore` excludes. `--fmt` first rewrites the selected
entries into canonical form (§11.9) and commits that together with the edits; without it, a
non-canonical entry is committed as is and `check` reports `W-CANONICAL`.

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

`<reason>` is one of `uncommitted changes (run memex save)`, `timed out after <N>s`,
`interrupted`, `rebase abort failed: <git's first stderr line>`, or git's first line of
standard error. Wherever a reason is git's first line of standard error, it is the first
line that starts with `fatal:` or `error:`, or the first non-empty line when none does, with
surrounding whitespace removed. A rebase in progress is refused before anything is written (§9.1 step 2).

A commit is never undone automatically. A command that fails before step 6 of §9.1 changed
nothing and prints no outcome line; its exit code and standard error carry the reason.

### 9.5 Sync

`memex sync` performs §9.1 steps 1 and 7 without a commit:
1. Fetch, bounded by `network_timeout`.
2. If behind and the tree is clean, fast-forward. If ahead and behind, rebase and push. If
   only ahead, push.

It prints exactly two lines:
- `fetched <time>`, where time is now in RFC 3339 UTC. It is always the first line, also when
  there is no upstream or the fetch failed; it records the attempt;
- the **state line**, one of `up to date`, `fast-forwarded N commits`, `pushed N commits`,
  `rebased and pushed N commits`, `no upstream`, or `NOT SYNCED: <reason>`.

Exit 0 for the first five. `NOT SYNCED` exits 5, with `<reason>` one of
`uncommitted changes (run memex save)`; `rebase in progress`, `merge in progress` or
`cherry-pick in progress` (checked first, before the upstream); `timed out after <N>s`;
`interrupted`; `rebase conflict in <paths>` (after `git rebase --abort`); `rebase abort
failed: <git's first stderr line>`; `locked by another memex for 10s` (only within
`preflight`, whose lock wait is 10 s; `sync` itself exits 4); or git's first stderr line. A
dirty tree is never rebased; behind-only with a dirty tree is `NOT SYNCED: uncommitted
changes (run memex save)` as well, because a fast-forward could overwrite an edit in progress.

---

## 10. Command-line conventions

- **Form:** `memex [GLOBAL OPTIONS] COMMAND [OPTIONS] [ARGS]`. Global options: `--store PATH`.
  Global options may also follow the command (`memex list --store ~/notes`).
- **Options:**
  - They may appear before or after positional arguments.
  - `--` ends options.
  - Long options only, except `-m` (`save`) and `-h`.
  - `--opt=value` and `--opt value` are equivalent.
  - An option that is not repeatable, given more than once, is a usage error.
- **Streams:**
  - Standard output carries results only.
  - Diagnostics go to standard error, prefixed `memex: `. A diagnostic of several lines has
    the prefix on its first line only, and its continuation lines are indented two spaces
    (`  candidate: <path>`, `  title: <title>`). A usage error's usage and `Run:` lines
    (§16.8) and the closing instructions of the §11.2 refusal are separate lines, not
    continuations.
  - No color, no progress bars, no paging, no prompts.
- **`--body FILE`** reads the file. `--body -` reads standard input to EOF.
- **Ids as arguments.** Wherever a command takes an entry id (`show`, `set`, `promote`,
  `--parent`, `--source`, `superseded_by=`), it also accepts the entry's store-relative path
  (`rules/2026-….md`) or an alias, because search and list print paths. The value is resolved
  to the id before anything else happens.
- **JSON.** Every `--json` output is one compact JSON text (no spaces outside strings) followed
  by one LF, with keys in the order this specification lists. Strings escape exactly these
  characters: `"` as `\"`, `\` as `\\`, U+000A as `\n`, U+000D as `\r`, U+0009 as `\t`, and every
  other code point below U+0020 as `\u` followed by four lowercase hex digits. Nothing else is
  escaped: `/`, `<`, `>`, `&`, U+007F, U+2028, U+2029 and all non-ASCII characters are written
  literally. A byte sequence that is not valid UTF-8 (possible only in text that came from the
  command line) is written as U+FFFD. Numbers that are integers are written without a
  fraction; the only non-integer is `score` (§8.5).
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
- the seven type directories, each with no files;
- `files/`;
- `.gitignore` containing `.memex/`;
- `.githooks/pre-commit` (Appendix A).

It also sets `git config core.hooksPath .githooks`.

- Existing nonempty files are never overwritten. Missing lines are appended to `.gitignore`.
- Running it again is a no-op.
- It prints one line per created or changed item: `created .git/` when it ran `git init`;
  then one line per item in the order of the list above (`created <file>` for a file,
  `created <dir>/` for a directory, `updated .gitignore` in `.gitignore`'s place when it
  appended the line to an existing file); then `set core.hooksPath .githooks` when that was
  not already its value. The last line is
  `initialized <path>; run: memex save -m "init store"`, with the absolute path, because a
  store with nothing committed refuses every other write (§9.1 step 3).
- It does not commit.

**Pointer form.** `memex init --pointers DIR` writes `CLAUDE.md`, `AGENTS.md` and `GEMINI.md`
into `DIR` with the pointer text of §12. It requires a resolved store (§3.1).
- It prints one line per file, `created <DIR>/<file>`, `unchanged <DIR>/<file>` when the file
  already holds the pointer text, or `replaced <DIR>/<file>`, with `<DIR>` as given.
- The pointer text names the config file's `store` string without trailing `/` (a store
  configured as `~/memex-store/` gives `Read ~/memex-store/AGENT.md …`).
- An existing file whose content is the pointer line naming the same store, compared after
  the §3.1 expansion and cleaning of both paths (as `check --workspace` compares them, §12), is
  `unchanged`.
- A file that already exists with any other content is left alone and reported on standard
  error as `memex: <DIR>/<file> exists with other content; left alone (use --force to replace
  it)`, and the command exits 1.
- `--force` replaces such a file and prints its previous content to standard output first.
- A `DIR` that is not a directory exits 1.
- The pointer form does not commit.

### 11.2 `new` and `capture`

```
memex new TYPE --title=T --summary=S --body FILE|- [--status S] [--tag X]… [--keyword K]…
          [--repo R]… [--no-repo] [--host H]… [--always] [--kind K]
          [--parent ID] [--source ID]… [--verified DATE] [--review-days N] [--alias A]…
memex capture --title=T --summary=S --body FILE|- [same options]
memex capture --from PATH [--title=T] [--summary=S] [same options]
```

`capture` is `new journal`. `capture --from PATH` imports a file found by the leak scan (§12):
the title defaults to the file's first ATX heading text; a file with no ATX heading that
begins with a YAML block takes its `name:` as the title and its `description:`, when present,
as the summary, as §13.3 does; else the title is the filename. The summary otherwise
defaults to `Imported from <PATH>`; the body is the line `Imported from \`<PATH>\` (<today>).`,
a blank line, then the file content, normalized; `aliases` gets PATH in its **alias form**:
expanded as in §3.1, made absolute and cleaned lexically (symbolic links are not resolved),
with the home directory written `~/` when the file is under it, so that `~/x`, `$HOME/x` and
a relative path to the same file are one alias. PATH is read after the §3.1 expansion. It never
deletes PATH, and on success prints `remove <PATH> after review` before the outcome line,
also after `exists <path>` in the repeat of step 3. When an entry has an alias that is PATH as
given, or that is a file path (absolute, or beginning with `~/`, `$HOME` or `${HOME}`) with
the same alias form as PATH, the file is already in the store: `capture --from` prints
`exists <path>` of that entry, `remove <PATH> after review` and `nothing to commit`, and exits
0, before any id is computed.

Behavior:

1. Validate the options against §5.2 and §5.3 (for example, `--always` only on rules).
   `--summary` is required, because search results and lists show it, and it is what lets an
   agent decide whether to open an entry without reading it. `--keyword` SHOULD be given
   whenever the body's vocabulary differs from how someone would ask about it.
   `verified` defaults to today for references.
   An unknown type or status, an option not allowed on the type, and a malformed `--verified`
   or `--review-days` are usage errors (exit 2); a value that fails §5.3 or §7 (a bad tag, a
   `parent` that is not a plan, an unresolved link) exits 1 with the findings, printed as
   §9.1 step 5 describes (for example `memex: <path>: E-FIELD-VALUE summary must be one line
   of at most 500 characters`, then the refusal line).
2. Normalize the body (§5.1) and check its content measure (§5.6).
3. Compute the id (§5.4).
   - If the id already exists with the same type and an identical body: print
     `exists <path>` and `nothing to commit`, and exit 0. Only the body is compared; a repeat
     with different options is still this no-op.
   - If it exists with a different body or type: exit 1, changing nothing, with this on
     standard error, so that the agent does not retry with a tweaked title and create a
     near-duplicate (the predecessor's v2 did exactly that):

     ```
     memex: exists <path>
       title: <existing title>
       summary: <existing summary>
     To read it: memex show <id>. To add to it: edit the file, then memex save.
     To create a different entry: choose a more specific --title.
     ```
4. Write and commit (§9.1). Standard output gets `created <path>`, then the outcome line.

### 11.3 `search`

```
memex search QUERY… [--type T]… [--tag X]… [--repo R]… [--host H]… [--all] [--limit N] [--json]
```

All positional arguments are joined with single spaces into the query. The rest is §8.

### 11.4 `preflight`

```
memex preflight [QUERY…] [--no-sync] [--max-chars N] [search options]
```

This is the start-of-task ritual. With a query it is "sync, always-rules, search"; without
one it is "sync, always-rules", which is the form a session-start hook runs, since a hook has
no task text. It prints:

1. **Sync line.** Unless `--no-sync` is given or `sync_on_preflight = false`, it runs `sync`
   (§9.5) and prints its state line, prefixed `store: `, for example `store: up to date` or
   `store: NOT SYNCED: timed out after 20s`. A failed sync never stops preflight; a write lock
   held by another memex for the whole 10 s wait is the line
   `store: NOT SYNCED: locked by another memex for 10s`, so preflight never exits 4. With sync
   skipped, it prints `store: not synced (last fetch <time or "never">)`, where the time is the
   modification time of the store's `FETCH_HEAD` (`git rev-parse --git-path FETCH_HEAD`) in
   RFC 3339 UTC to the second. It is `never` when that file does not exist, or when the store
   root is not the top of its own repository (`git rev-parse --show-toplevel` run in the store
   names another directory, symbolic links resolved), because a store directory inside another
   repository has never been fetched itself.
2. **Blank line, then `## Always`.** The heading is printed even when no rule follows it.
   For each rule with `always = true` that is visible by default (§5.2; `--all` applies only to
   the results), in id order,
   one line: `- <path>: <title> — <summary>` (the summary is omitted, with the ` — `, when
   absent). The summary is what is printed, not the body, because `summary` is required to be
   the one sentence that tells an agent whether to open the entry, and because the body's first
   paragraph under the rule template is the symptom, not the instruction. An agent reads the
   body with `memex show` when it needs the detail.
3. **With a query: blank line, then `## Results`,** and the §8.5 text output for the query.

**Output budget.** `--max-chars N` (default 12000, minimum 2000) bounds the whole output in
code points.
- The sync line and the `## Always` section are never cut.
- If they alone exceed half the budget, a final line
  `warning: always rules use <n> of <N> characters; review them` is added, where `<n>` is the
  code-point count of the sync line, the blank line and the `## Always` section, line ends
  included.
- Search results are then dropped from the end, whole blocks at a time, until the output fits.
- When any result was dropped, a last line reads:
  `output budget reached: <k> results not shown (raise --max-chars or run memex search)`.
  When both lines are printed, the warning comes first and the budget line is last.
- The count lines of §8.5 are kept, and the `weak results` and `Store root` lines are printed
  only when at least one result block remains.

**Exit code.** 0 when there is no query. With a query, the search's exit code, except that a
search that found nothing exits 0 when at least one Always rule was printed: the output is
then not empty, and a hook that drops the output of a non-zero exit (Claude Code's
`SessionStart` does) must still deliver the rules. The no-match line is in the body either way.
A failed sync never changes the exit code.

**Hook budget.** A session-start hook typically caps output lines and time. The Always section
comes first and is never cut for that reason, and `network_timeout` keeps the sync short.
`help agents` documents the hook form: `memex preflight --max-chars 4000`.

### 11.5 `list`

```
memex list [--type T]… [--status S]… [--tag X]… [--repo R]… [--host H]… [--all] [--limit N]
           [--fields F,…] [--json]
```

- **Order:** descending `created`, then ascending id.
- **Visibility:** by default only visible entries are listed. `--status` selects statuses
  explicitly, and `--all` lists every entry.
- **Limit:** `--limit N` (default `list_limit`, §3.2; `--limit 0` means no limit) prints the
  first N entries, then `N more entries not shown (raise --limit)` if any were cut.
- **Text output:** one line per entry, `<created> <type> <status> <path> <title>`, separated by
  single spaces; the title is last because it contains spaces.
- **Fields:** `--fields` selects and orders the columns, from `created, type, status, id, path,
  title, summary, tags`. Columns are separated by a tab in text output; `tags` is the tags
  joined with `,`, and an absent summary is empty. `--fields` changes only text output; an
  unknown field is a usage error.
- **Statuses:** `--status` selects entries by status whatever their visibility, also with
  `--all`; a status that no type has is a usage error.
- **JSON output:** `{"entries":[{"id":"…","type":"…","status":"…","created":"…","title":"…",
  "summary":null,"tags":[],"path":"…"}],"more":0}`.

The JSON shape is the same with or without `--fields`. An entry that cannot be parsed is left out, with the `memex: skipped` line of §8.1 on standard error.

`list` replaces the predecessor's committed INDEX file.

### 11.6 `show`

```
memex show ID… [--json]
```

For each ID (an id, path or alias, §10), prints the entry file verbatim, then:

```

--- links
cited by: <path>, <path>
supersedes: <path>
children: <path>
links to: <path>
linked from: <path>
```

- **cited by:** entries listing ID in `sources`.
- **supersedes:** entries whose `superseded_by` is ID.
- **children:** entries whose `parent` is ID.
- **links to:** `[[...]]` targets in the body.
- **linked from:** entries whose body links to ID.

Lines with no entries are omitted, but the `--- links` line is always printed. A file that does
not end in LF gets one before the blank line. `links to` lists each resolved target (§6) once,
in body order; unresolved targets are left out. The other lines are in path order, and are
separated by `, `. Several IDs are separated by a line `---`, so that an agent
can read a handful of entries in one call. An unknown id exits 1 after printing the known ones
(§17.4); the JSON output then holds only the known ones. An alias held by more than one entry
names none of them: stderr reads `memex: no entry "<alias>": it is an alias of <N> entries;
give an id`, then its holders as `  candidate:` lines. An entry that cannot be parsed is left out, with the `memex: skipped` line of §8.1 on standard error.

**JSON output:** `{"entries":[{"id":"…","type":"…","path":"…","frontmatter":{…},"body":"…",
"links":{"cited_by":[],"supersedes":[],"children":[],"links_to":[],"linked_from":[]}}]}`,
where `frontmatter` holds the §5.3 fields present, in canonical order, with dates as
`YYYY-MM-DD` strings, and every `links` array holds ids.

### 11.7 `set`

```
memex set ID ASSIGNMENT…
memex set --batch FILE|-
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
- The result must validate; a failure is reported as §9.1 step 5 describes. In particular
  `status=superseded` needs `superseded_by=` in the same command, and leaving `superseded`
  removes `superseded_by` automatically.
- The body bytes are preserved exactly.
- An entry with uncommitted changes (it differs from `HEAD`, or is untracked) is refused,
  exit 1, with `memex: <path> has uncommitted changes; run memex save first`, so that its
  commit never carries someone's unsaved edit.
- An entry with an unknown field or a field whose value is not valid is refused (exit 1),
  because rewriting it would drop that field; fix it by hand, then `memex save`.
- The values of `superseded_by`, `parent` and `sources` are resolved like every id argument
  (§10): an id, a store-relative path or an alias.
- **A status change that crosses visibility needs a new summary** (Planned). On a task or a
  plan, `status=<new>` where exactly one of the old and the new status is visible by default
  (§5.2) — closing it (`done`, `dropped`; `done`, `abandoned`, `superseded`) or reopening it —
  is refused unless the same command also assigns `summary=` a value different from the
  current summary. Exit 1, nothing changed, with
  `memex: <id> changes status from "<old>" to "<new>" but keeps its summary; add summary=<sentence> to the same command, saying where it stands now`.
  In `--batch` the line rules below apply (`memex: --batch line <n>: …`). The summary is what
  lists and search show; a closed task whose summary still describes open work misleads every
  later reader, and `W-STALE-SUMMARY` (§7) would report it.
- Commits through §9.1. Standard output is only the outcome line; an assignment that changes
  nothing gives `nothing to commit`. The commit message shows each value as resolved: ids,
  not the paths or aliases given.

**Batch form.** `memex set --batch FILE` applies many sets as one §9.1 unit, so that a pass
over hundreds of entries is one commit and one push, not hundreds. `FILE` is `-` for standard
input. `--batch` takes no positional arguments: any is a usage error (exit 2), and so is a
`FILE` that cannot be read (`cannot read --batch <FILE>: <reason>`).

- Each line of `FILE` is `ID ASSIGNMENT…`, split into words like a POSIX shell without
  expansions: words are separated by spaces and tabs, single quotes are literal, double quotes
  allow `\"` and `\\`, and a backslash outside quotes escapes the next character. A value with
  spaces is written as on a command line: `'title=A b'` or `"title=A b"`. Lines end at LF; a CR at
  the end of a line is dropped. Lines are numbered from 1, counting every line.
- A line that is empty or blank, or whose first non-blank character is `#`, is skipped. A `#`
  anywhere else is part of a word.
- `ID` resolves like every id argument (§10), and each line obeys every rule of the single
  form above.
- The batch is one §9.1 unit: one lock and one freshen; then step 3 for every line, in file
  order, before anything is written; then one write and read-back of every changed entry, one
  commit (§9.2) and one integrate step.
- A line that is invalid refuses the whole batch, exit 1, and nothing is written. The first
  invalid line in file order is reported as `memex: --batch line <n>: <message>`, where
  `<message>` is what the single form would print after `memex: `, its continuation lines
  included. This covers a line that does not split (`unterminated single quote`,
  `unterminated double quote`), a line with no assignment, a malformed assignment or unknown
  field, an unknown id, a field that cannot be set, and an entry with uncommitted changes.
  What the single form reports as a usage error (exit 2) is a refused line here (exit 1),
  without the usage and `Run:` lines, because the fault is in the file, not the command line.
- An entry named on two lines (by any mix of id, path and alias) is refused at the second
  line: `memex: --batch line <n>: <path> is already set by line <m>; put all its assignments
  on one line`.
- The result is validated once, for all changed entries together. Each error finding is
  printed as in the single form, with `--batch line <n>: ` after `memex: ` when its path is an
  entry the batch changes, then
  `memex: refused: the result would not validate; nothing was changed`.
- A line that changes nothing is not an error and is left out of the commit. When no line
  changes anything, the outcome is `nothing to commit`. Standard output is only the outcome
  line.

### 11.8 `promote`

```
memex promote SOURCE-ID --to TYPE --title=T --summary=S --body FILE|- [new options]
```

1. Creates a new entry, as `new`, with `sources` containing SOURCE-ID plus any `--source` values.
2. If the source is a journal entry whose status is `raw`, `kept` or `promoted`, sets its
   status to `promoted`. A `superseded` source and sources of other types are unchanged.

Both files go in one commit. Read-back (§9.1, step 5) covers both. Promoting a source whose
status is already `promoted` is allowed. A source that is rewritten is refused, as in `set`,
when it has uncommitted changes or fields that cannot be kept. Standard output is `created <path>`, then the outcome
line; the id rules of §11.2 step 3 apply to the new entry.

### 11.9 `fmt`

```
memex fmt [PATHS…] [--check]
```

Rewrites entries into canonical form (§5.1) without changing any field value or the body.
- With no paths it processes all entries.
- Only entries without errors are rewritten (those `check` reports as `W-CANONICAL`). Another
  selected entry that is not canonical, or cannot be read, is reported on standard error as
  `memex: skipped <path>: it has errors; run memex check` (or the reading problem) and left
  alone. A path that is not an entry is `memex: no entry at <path>`, exit 1.
- `--check` writes nothing and takes no lock, lists every non-canonical selected entry one
  per line in path order, those with errors and unreadable ones included, and exits 1 if any.
- Otherwise commits through §9.1. Standard output is only the outcome line.

### 11.10 `save`

§9.3. Standard output is only the outcome line.

### 11.11 `sync`

§9.5.

### 11.12 `check`

```
memex check [--staged] [--workspace] [--strict] [--errors-only] [--limit N] [--json]
```

§7. `--workspace` adds the §12 pointer and leak findings; it combines with every other option,
`--staged` included (the workspace is scanned as it is on disk either way). With `--json`, the output is one object: `{"findings":[{"path":"…","code":"…",
"severity":"error","message":"…"}],"errors":N,"warnings":M,"truncated":K}` in that key order,
where `truncated` is the number of findings not listed because of `--limit`.

### 11.13 `doctor`

```
memex doctor [--json]
```

Read-only. It needs no store (§3.1): a store that does not resolve is a failed check, not an
exit 2. Prints one line per check, `PASS|WARN|FAIL <id> <message>`, in the order of the table.
With `--json`: `{"checks":[{"id":"git","result":"PASS","message":"…","details":[]}],"failed":N}`,
where `details` holds the `LEAK <path>` lines of the `leaks` check and is otherwise empty.

| Id | Check | Result and message |
|---|---|---|
| `git` | `git` is runnable; version ≥ 2.30 | PASS `version 2.30 or later`; FAIL `git is not runnable: <reason>`, `git <x>.<y> is older than 2.30`, or `cannot read the git version from "<output>"` |
| `config` | The config file parses (or is absent), and `MEMEX_TODAY`, when set, is a date | PASS `config file parses` or `no config file`; FAIL with the §3 error message. With the config file failed, the store is resolved from `--store` and `MEMEX_STORE` only; with `MEMEX_TODAY` failed, `check` uses the local date |
| `store` | The store resolves (§3.1) and `memex.toml` is valid | PASS `<root>`, followed by `, name "<name>"` when the manifest has a name; FAIL with the §3.1 message (an unreadable `memex.toml` is `store <root>: <reason>`, the form of an unreadable store directory) or the first `E-MANIFEST` finding as `memex.toml: E-MANIFEST <message>`. When it FAILs, `format` and every check up to `check` are omitted |
| `format` | The store format is supported (§4) | PASS `format 1`; FAIL with the §4 refusal message. When it FAILs, every check up to `check` is omitted |
| `upstream` | The current branch has an upstream | PASS `<branch> tracks <upstream>`; WARN `branch <branch> has no upstream, so nothing is pushed; run git -C <root> push -u origin <branch>`, or `HEAD is not on a branch; run git -C <root> switch main`. When the store root is not the top of its own git repository (`git rev-parse --show-toplevel` fails or, symbolic links resolved, names another directory), WARN `<root> is not a git repository; run memex init <root>`, and `fetch-age`, `clean` and `hooks` are omitted |
| `fetch-age` | Age of `FETCH_HEAD`'s mtime in the store's git directory. A fleet check can watch this through `--json` | PASS `last fetch <time>`; WARN `last fetch <time>, more than 8 hours ago; run memex sync`, or `never fetched; run memex sync` when the file does not exist. `<time>` is RFC 3339 UTC to the second |
| `clean` | The store has no uncommitted changes, no unpushed commits and no rebase in progress | PASS `nothing uncommitted or unpushed`; WARN with one or more of `<n> uncommitted changes; run memex save` (`git status --porcelain --untracked-files=all` lines, `change` when 1), `<n> unpushed commits; run memex sync` (`commit` when 1; only with an upstream), `a rebase is in progress; finish it with git -C <root> rebase --continue, or --abort`, joined by `; ` |
| `hooks` | `core.hooksPath` is `.githooks` and `.githooks/pre-commit` is an executable file | PASS `core.hooksPath is .githooks and the pre-commit hook exists`; WARN `core.hooksPath is not set, so the pre-commit check does not run; run memex init <root>`, `core.hooksPath is "<value>", not .githooks; run git -C <root> config core.hooksPath .githooks`, or `.githooks/pre-commit is missing or not executable; run memex init <root>` |
| `check` | Summary of `memex check` | PASS `<n> errors, <m> warnings` (with `n` = 0); FAIL `<n> errors, <m> warnings; run memex check` |
| `path` | Every `memex` executable on `PATH` resolves to the same file, symbolic links resolved. Relative and empty `PATH` elements are skipped | PASS `1 memex on PATH`, or `<n> memex on PATH, all the same file`; WARN `memex on PATH resolves to different files: <paths>; the first one runs` (the paths as found on `PATH`, separated by `, `), or `no memex on PATH; hooks run memex by name` |
| `leaks` | §12 leak scan, over the home locations and, when `workspace` is configured, the workspace | PASS `no files in coding tools' own memory`; WARN `<n> files in coding tools' own memory; move each into the store with memex capture --from PATH, then delete it` (`file` when 1), followed by one `LEAK <path>` line per finding, absolute paths in code-point order. A configured workspace that does not resolve (§12 rule 2: absent, relative, unreadable, not a directory, or inside the store) appends `; the workspace was not scanned: <message>` and makes the check WARN even with no finding |
| `tool-memory` | §12 rule 5 | PASS `~/.claude does not exist`, `HOME is not set; no ~/.claude to check`, `Claude Code auto memory is disabled by CLAUDE_CODE_DISABLE_AUTO_MEMORY=1`, or `Claude Code auto memory is disabled in <file>` (the first of the rule 5 files that disables it, `~/.claude/settings.json` written so); WARN `Claude Code auto memory is not disabled; set "autoMemoryEnabled": false in ~/.claude/settings.json` |

The `store`-dependent checks (`format` to `check`) run only when `store` PASSes, and `upstream`
to `hooks` only when `git` PASSes too. `path`, `leaks` and `tool-memory` always run. Each git
command `doctor` runs is bounded by 10 seconds; none of them touches the network. `doctor`
takes no lock and runs every git command with `GIT_OPTIONAL_LOCKS=0`, so that `git status`
does not refresh the index: it writes nothing anywhere. `git version` runs in `/`, so that it
does not depend on `HOME` or the current directory.

Exit 1 if any check FAILs, else 0. A usage error is exit 2.

### 11.14 `import`

§13.

### 11.15 `help`

§16.

### 11.16 `version`

```
memex version
```

Prints one line, `memex <version> (store format 1)`, and exits 0. `memex --version` is the
same. It needs no store and accepts no options other than `--help`/`-h`. `<version>` is the
release version, or `0.0.0-dev` for a build that is not a release.

### 11.17 `update`

(Planned.)

```
memex update ID --summary=S --body FILE|- [--heading=H] [--status S]
```

Records progress on an entry in one step: it appends a dated section to the body and replaces
the summary, in one commit. It is the way to add a status note to a task or plan, so that the
summary and the body cannot drift apart (§7, summary currency). Editing the file by hand stays
allowed (§9.3); `W-STALE-SUMMARY` then reports a summary that was not rewritten.

1. `ID` resolves as every id argument (§10). A journal entry is refused, exit 1, with
   `memex: journal bodies are immutable; record the progress with memex capture, then memex set <id> status=superseded superseded_by=<new> if it replaces this one`.
2. `--summary` and `--body` are required. The summary must differ from the entry's current
   summary: otherwise exit 1, `memex: --summary is the current summary of <id>; write the
   summary as it stands after this section`. It must satisfy §5.3 like any summary.
3. The section text is the `--body` input, normalized (§5.1). Its content measure (§5.6) must
   be at least 80, as for `new`; the refusal names the rule and points to `memex help entries`.
4. `--heading` is 1–100 characters on one line; a longer value or an LF is a usage error
   (exit 2). Absent, it is `Status`.
5. `--status`, when given, must be one of the type's statuses (an unknown one is a usage error,
   exit 2) and is set in the same commit. Because the summary changes too, this satisfies the
   rule of §11.7 for a status change that crosses visibility; `status=superseded` still needs
   `superseded_by`, which `update` does not set (use `memex set`).
6. The new body is the current body with trailing whitespace removed, then `\n\n## <H> <today>\n\n`
   (`<H>` the heading, `<today>` as §2), then the section text. When the current body is empty
   the leading `\n\n` is omitted. The result is in normalized form: one final LF.
7. An entry with uncommitted changes, or with a field that cannot be kept, is refused exactly
   as in `set` (§11.7).
8. Commits through §9.1, with the message of §9.2. Standard output is only the outcome line.

Example:

```
memex update 2026-01-10-rotate-the-backup-key --status done \
  --summary "Done 2026-01-12: key rotated on both hosts, old key revoked, restore re-tested." \
  --body - <<'EOF'
Rotated on alpha and beta; the old key is revoked in the vault and a restore from the
newest snapshot succeeded with the new key on both hosts.
EOF
```

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

   `<store>` is the `store` string from the config file if that is where the store came from,
   otherwise the resolved absolute path. The check compares the pointer's path after §3.1
   expansion with the resolved store, so `~/memex-store` and `/home/x/memex-store` are the same
   pointer, and so are `~/memex-store/` and `~/memex-store` (both sides are cleaned as §3.1
   cleans the store path). `check --workspace` reports `E-POINTER <path>` for every `CLAUDE.md`,
   `CLAUDE.local.md`, `.claude/CLAUDE.md`, `AGENTS.md` or `GEMINI.md` under the workspace whose
   content differs, and `W-NO-POINTER <dir>` for every git top-level directory under the
   workspace that has no `CLAUDE.md` and no `AGENTS.md`.
   - The scan reaches depth 4: it visits the workspace itself (depth 0) and every directory
     below it down to depth 4 (`<workspace>/a/b/c/d`), and looks up the names above, and those
     of rule 3, relative to each directory it visits.
   - It does not enter `.git`, `node_modules`, `vendor`, `.worktrees` or the store itself (the
     same directory, compared by device and inode).
   - It does not follow symlinks: a symbolic link to a directory is not visited, and a pointer
     file name that is a symbolic link is `E-POINTER`, because its content is a link and
     writing a pointer through it would change the link's target.
   - A git top-level directory is a visited directory that contains a `.git` entry (a
     directory, or the file of a worktree or submodule).
   - Each file is checked once, under the name of the first directory that reaches it, in
     path order: `a/.claude/CLAUDE.md` is `a`'s `.claude/CLAUDE.md`, not the `CLAUDE.md` of
     the visited directory `a/.claude`.
   - The workspace is the `workspace` key (§3.2), expanded, checked and cleaned like the store
     path (§3.1). For `check --workspace` each of these is exit 2:
     `memex: no workspace configured; set workspace = "..." in <config path>`;
     `memex: workspace path "<path>" is relative; give an absolute path`;
     `memex: workspace <path> is not a directory`;
     `memex: workspace <path>: <reason>` when it does not exist or its entries cannot be listed
     (`no such file or directory`, `permission denied`);
     `memex: workspace <path> is inside the store <root>` when it is the store or below it
     (compared as cleaned paths and, when they resolve, with symbolic links resolved), because
     the scan would read the store's own files as a workspace.
     Directories below the workspace that cannot be listed are skipped.
   - Findings name absolute paths, sort with the store's findings by path (§7), and count in
     the summary like any other finding.
3. **Tool stores are scanned and never read as a source.** The leak scan reports, as
   `W-LEAK <path>`, every regular non-empty file at these locations:
   - `~/.claude/projects/*/memory/**`
   - `~/.claude/CLAUDE.md`
   - `~/.codex/AGENTS.md`
   - `~/.gemini/GEMINI.md`
   - under the workspace (same depth and exclusions as rule 2): `.cursorrules`,
     `.cursor/rules/**`, `.claude/rules/**`, `.github/copilot-instructions.md`,
     `.windsurfrules`

   `**` is every file below the directory, at any depth, without following symbolic links or
   entering the names rule 2 excludes; a symbolic link is not a regular file. The three
   single files under the home directory are each tool's global instructions file: one whose
   content is the pointer line naming the store (compared as in rule 2) is that tool's pointer,
   not a leak. Paths are absolute, each listed once, in code-point order. The home locations
   are skipped when `HOME` is unset, and the workspace ones when no workspace is configured.

   `doctor` prints the same findings as `LEAK <path>` lines (§11.13); the two spellings name
   the same scan.

   This list lives in the specification. Adding a tool means adding a line here and a
   conformance case. `capture --from PATH` (§11.2) moves the content into the store.
4. **Integration is one-way.** Tools' hooks call memex (for example `memex preflight
   --max-chars 4000` at session start, §11.4). memex knows nothing about tools beyond the
   pointer text and the paths in rule 3.
5. **Switching off a tool's own memory is configuration, not memex's job.** `doctor`'s
   `tool-memory` check reports WARN when Claude Code's auto memory isn't disabled. It counts as
   disabled when any of these is true:
   - `CLAUDE_CODE_DISABLE_AUTO_MEMORY=1` is set in memex's environment;
   - `"autoMemoryEnabled": false` appears in `~/.claude/settings.json`, in
     `/etc/claude-code/managed-settings.json`, or in any `*.json` under
     `/etc/claude-code/managed-settings.d/`.

   If `~/.claude` doesn't exist, the check PASSes. "Appears" means the file holds one JSON
   object whose top-level member `autoMemoryEnabled` is `false`; a file that does not parse
   counts as not disabling it. Settings files are read through symbolic links (a system
   configuration manager typically installs them as links). The sources are tried in the
   order listed, `managed-settings.d/` in code-point order of the file names, and the first
   that disables auto memory names the PASS message (§11.13).

---

## 13. Importing a brain v3 store

```
memex import brain-v3 SRC[@REV]… --out DIR [--leak PATH]…
memex import brain-v3 SRC[@REV]… --into DIR [--leak PATH]…
```

Converts one or more brainctl v3 brains into a new store at `DIR` (`--out`), or brings into
the existing store `DIR` only what it does not hold yet (`--into`, the delta form, §13.7). The rules below were
written against the real predecessor data (457 investigations, 487 sections and 113 table rows
of known mistakes, 35 sections, 77 rows and 9 "Always" bullets of constraints, 271 backlog
items, 474 INDEX rows, 116 plans), and the counts in the notes are from it.

### 13.1 Guarantees

- **Read-only on sources.** Each SRC is a git repository, and memex never writes to it. memex
  reads it through git (`git -C SRC show REV:path`, `git log`, `git blame`), never through the
  working tree, because dates come from history. `REV` defaults to `HEAD`; without `@REV`, a
  dirty SRC is refused (exit 1), since the import must describe a commit. With `@REV` the
  working tree is irrelevant.
- **Fresh output.** With `--out`, `DIR` MUST NOT exist. memex creates it as in §11.1, except
  that `AGENT.md` is built as §13.3 says, and writes all files. It does not commit. The
  operator reviews and commits. The delta form (§13.7) adds to an existing store instead and
  never changes what is there.
- **Deterministic.** The same SRC revisions, in the same order, with the same `MEMEX_TODAY`,
  produce byte-identical `DIR` contents. Re-running against later revisions brings over what was
  written in between.
- **Lossless, and accounted for** (§13.6).
- **Valid.** `memex check` on `DIR` reports zero errors. It MAY report `W-NO-SUMMARY`,
  `W-NO-TERMS`, `W-SHORT-BODY`, `W-ALIAS-LINK`, `W-RAW-OLD`, `W-SECRET` and
  `W-PROMOTED-UNCITED` (the predecessor had no `sources` field, so every promoted
  investigation is uncited by construction). Any other finding is an import defect. This is
  what makes the operator's commit possible, since the pre-commit hook runs `check --staged`.
- **Isolated from the machine.** Every read of a source runs git with `GIT_CONFIG_GLOBAL`
  pointing at `/dev/null`, `GIT_CONFIG_NOSYSTEM=1`, and `-c blame.ignoreRevsFile=`,
  `-c log.showSignature=false`, `-c diff.renames=true`, so that two machines read the same
  history the same way. The manifest's `name` is the primary SRC's basename (the only place
  where `DIR`'s own name would otherwise leak into the output).
- **Status comes from structure, never from prose.** A status is set only from an explicit
  status carrier: an INDEX row, a `## Status` section, a `**Status:**` line, the backlog's
  `## In Progress` / `## To Do` / `## Done` sections and the markers in backlog headings. Words
  such as `STALE` or `RESOLVED` inside a rule's title or body become tags (§13.4), not a status,
  because deciding whether a rule is still true is the reviewed curation pass, not the import.

### 13.2 Source layout recognized

The first SRC is the **primary** brain. Each SRC contributes:
- `.brain/investigations/*.md`, `.brain/decisions/*.md`, `.brain/runbooks/*.md`;
- `.brain/plans/**` (top-level files, `prompts/`, `reports/` and any other subdirectory);
- `.brain/dispatch-logs/*`;
- `.brain/known-mistakes.md`, `.brain/constraints.md`, `.brain/INDEX.md`, `.brain/AGENT.md`;
- `.brain/backlog.md`, handled exactly like `docs/backlog.md`.

The primary SRC alone also contributes `docs/**/*.md`, including `docs/runbooks/*.md` and
`docs/backlog.md`. In a secondary SRC (an application repository with a `.brain/`), `docs/` is
the application's documentation, not knowledge, and `.brain/AGENT.md` is read for its
`BRAIN_REPO` line only and listed as `not-knowledge`.

**Not knowledge.** These tracked paths are listed in the report as `not-knowledge` and not
imported: `.brain/templates/**`, `.brain/scripts/**`, `.brain/README.md`, `.brain/plans/README.md`,
every `.gitkeep`, `.githooks/**`, `.envrc`, `.gitattributes`, `.gitignore`, `CLAUDE.md`,
`AGENTS.md`, `GEMINI.md`, and everything outside the paths above. Anything under a recognized
directory that is not a `.md` file is an attachment (§13.3).

### 13.3 Units and mapping

A **unit** is the smallest piece of source that becomes one entry.

**Common rules**, which the table assumes:
- **Titles** are cut to 300 code points after markup stripping (§13.4), with whitespace runs
  collapsed to one space. "First heading" means the first ATX heading of any level. A heading
  that is hard-wrapped over consecutive `## ` lines (no blank line between them) is one
  heading. A trailing `(YYYY-MM-DD)`, `, found YYYY-MM-DD` or ` — YYYY-MM-DD` in a heading is
  removed from the title; the unit's date is the **earliest** `YYYY-MM-DD` written anywhere in
  the heading (`found 2026-09-11, resolved 2026-09-12` is dated the 11th), and text that is
  not one of the three trailing forms is kept. When the cut shortens a title, the heading line
  stays in the body instead of moving to the frontmatter, so no text is lost.
- **Ids never carry two dates.** When the slug of a title begins with a `YYYY-MM-DD` (its own
  date or, for a heading with several, a later one), that prefix and the hyphen after it are
  removed before the id is formed, so a backlog item headed `2026-10-12 — rotate the password`
  becomes `2026-10-12-rotate-the-password`; the heading's date stays in the title.
- **YAML blocks.** When a file (an investigation or a `--leak` file) has no heading at all
  and begins with a YAML block (`---` on the first line, up to the next `---`), the block's
  `name:` value is the title and its `description:` value, when present, is the summary;
  only then does the filename serve as the title. The block stays in the body.
- **Status lines.** The *value* of a `## Status` section or a `**Status:**` line is the first
  word of its first non-blank line, lowercased, with trailing punctuation and symbols removed
  (so `complete — 2026-07-06` is `complete`, and `- [x] Colors changed` is empty, which maps
  to the default). The rest of the section stays in the body.
- **Dates.** A unit's date is, in order: the date in its filename; the date taken from its
  heading; else the `YYYY-MM-DD` part of `%aI` (author time in its recorded offset) of the
  oldest commit in the unit's history. For a file unit that is `git log --follow`. For a line
  unit it is the oldest commit of `git log -S<key> -- <file>`, where the key is taken from
  the unit's first line without its `#` marks, or from a table row's first cell when that
  cell has at least 4 code points (the first cell is the row's identity; the other cells get
  reworded): the longest piece between marker tokens and dates (the tokens that change when
  an item is closed or re-filed), each piece trimmed of spaces, dashes, `:`, `(`, `)`, `,`
  and `.` at both ends (a separator next to a marker arrived with it), cut to its first 60
  code points, with markup kept because `-S` matches the file's bytes literally. That is the commit that first added the text,
  which neither a later edit of the rest of the unit nor a move inside the file changes. When `-S`
  finds nothing (the first line itself was edited), the blame date of that line is used and
  the unit is listed in the report under `dated_by_blame`. `git log -L` is not used: it
  follows a range back through hunks that replaced other text at the same lines.
- **Tables.** Only the table before the first `## ` heading of `known-mistakes.md` and
  `constraints.md` yields row units. A table inside a `## ` section is part of that rule's
  body.
- **Prefix stripping.** A first heading of the form `# <Word>: <text>` where `<Word>` is `Task`,
  `Investigation`, `Plan`, `Decision` or `Runbook` yields `<text>`.
- **`was-<status>` tags.** Whenever a source status maps to a different memex status, the
  entry gets the tag `was-<source status>` (normalized as a tag), so that nothing is flattened
  silently.

| Source unit | Type | Title | Status |
|---|---|---|---|
| Investigation file | journal | Prefix-stripped first heading, else filename stem | The file's status value; if the file has none, the INDEX row's. `raw`, `in-progress` → `raw`; `promoted` → `promoted`; `superseded` → `superseded` when §13.5 resolves a target, else `kept`; anything else → `kept`. Plus `was-<status>` where it changed |
| Decision file | decision | Prefix-stripped first heading | `accepted`; `superseded` when the status value or a superseded callout (§13.5) resolves a target; otherwise `accepted` plus `was-superseded` if the status value contains `supersed` |
| Runbook file (either directory) | runbook | Prefix-stripped first heading | `active` |
| Plan file under `plans/` (top level, `.md`) | plan | First heading | From the status value: `done`, `complete`, `completed`, `executed`, `shipped`, `landed` → `done`; `proposal`, `proposed`, `planned`, `ready`, `draft`, `not` → `draft`; anything else, or no status → `active` |
| File under `plans/prompts/` | plan, `kind = "prompt"` | First heading | `done` |
| File under `plans/reports/` | plan, `kind = "report"` | First heading | `done` |
| Any non-`.md` file under `plans/`, and every file under another `plans/<dir>/` at any depth | attachment | — | Goes to `files/<owner>/<path below plans/>`. The owner is the top-level plan whose source filename stem equals `<dir>` (or the file's stem) or is its longest proper prefix; when none exists, the owner is the SRC's **attachments entry**: one `reference` per SRC titled `Import attachments from <SRC basename>`, whose body lists the files it owns, one `- files/<id>/<path>` line each |
| Dispatch log file | attachment | — | `files/<owner>/dispatch-logs/<filename>`, where the owner is the plan whose stem is a prefix of the filename after its `YYYYMMDDTHHMMSSZ-<host>-` prefix; else the attachments entry |
| `known-mistakes.md` table row | rule | Symptom cell | `active` |
| `known-mistakes.md` `## ` section | rule | Heading text with a leading `Mistake:`, `Mistake / technique:`, `Known Noise:`, `Known Problem:`, `Technique:`, `Pattern:` or `Historical mistake (…):` prefix removed in full, parenthesis included; the prefix becomes a tag (`mistake`, `technique`, `noise`, `problem`, `pattern`, `historical`). A date inside the historical parenthesis is the unit's date only when the rest of the heading has none | `active` |
| `constraints.md` table row | rule | First cell | `active` |
| `constraints.md` `## ` section other than `## Always` | rule | Heading text | `active` |
| `constraints.md` `## Always` top-level bullet | rule, `always = true` | The bullet's leading bold span if it starts with `**…**`; else its first sentence (up to the first `. `, `: ` or ` — `); markup stripped | `active` |
| `docs/backlog.md` `### ` item | task | Heading text with its first marker removed (the symbol and the word after it, when both are present; the word decides, so `✓ MOOT` is `dropped`), then a leading or trailing separator run (`—`, `-`, `:` and spaces) removed, and nothing else | From the enclosing section: `## In Progress` → `in-progress`, `## To Do` → `open`, `## Done` → `done`. Overridden by the **first marker** of the heading, scanning its text outside code spans from left to right, where a marker is: a symbol ✓, ✅ or ⏸ anywhere (✗ counts only for the word after it), together with the marker word right after it if there is one; or one of the words `DONE`, `FIXED`, `CLOSED`, `RESOLVED`, `BLOCKED`, `SUPERSEDED`, `MOOT`, `RETIRED` (case-sensitive, whole word, a trailing `:`, `;`, `,` or `.` allowed) at the start of the heading, after an optional `YYYY-MM-DD — ` prefix, or as its last token. ✓, ✅, `DONE`, `FIXED`, `CLOSED`, `RESOLVED` → `done`; ⏸, `BLOCKED` → `blocked`; `SUPERSEDED`, `MOOT`, `RETIRED` → `dropped`; a fully struck-through heading → `dropped`. Every other word stays, so `(was BLOCKED)` in the middle and `0 \`SUPERSEDED\` markers` are text. Removed markers are listed in the report under `markers_removed`. `####` headings and everything below them belong to the item's body |
| Other `docs/**/*.md` | reference | First heading | `active`, `verified` = date of the last commit touching the file |
| `.brain/AGENT.md` of the primary SRC | `AGENT.md` | — | `DIR/AGENT.md` is Appendix A's text, followed by a blank line and the source file verbatim, so the operator rewrites it with both in view |

`###` headings inside a `##` rule section stay in that rule's body. A `## ` line that is only a
date or a grouping label (it has no content before the next `## ` or `### `) is `structural`.

**Ids** are computed by §5.4 from the date and title, with one exception: file-based units
whose source filename stem is already a valid id (§5.4) keep it, provided no other unit keeps
the same stem. Units are processed in this order, which also decides who keeps a stem and who
gets a suffix: investigations, decisions, runbooks, plans, `plans/prompts`, `plans/reports`,
`docs`, `docs/runbooks`, then `known-mistakes.md`, `constraints.md` and `docs/backlog.md` in
file order, then remainder and attachments entries; within a directory, by filename.
- **Collisions** inside one import get suffixes in that order: `-2`, `-3`, ….
- **Units from a secondary SRC** whose id collides with an identical body are dropped and listed
  as `duplicate-of <id>`. A different body gets the suffix `-<repo>`, where `<repo>` is the
  SRC's directory basename.

**Frontmatter of imported entries:**
- **INDEX rows** are matched to files by the `File` cell: a bare filename is looked up in
  `.brain/investigations/`; a value with a directory is resolved by §13.5, with `runbooks/x.md`
  tried in both runbook directories. When two rows name the same file, the last wins and the
  first is reported as `index-duplicate`. The row provides `summary` and `tags` only when the
  file's own header has none, and `status` only when the file has no status value; where the
  two disagree, the report lists `index-conflict` with both values. A matched row is
  `moved-to-frontmatter`; an orphan or superseded duplicate row stays in the file's remainder
  entry (§13.6), so the INDEX remainder is the list of rows that matched nothing.
- **INDEX placeholders.** A `Summary` of `manual-summary` and a `Tags`/`Context` of `manual`
  are treated as absent (165 rows carry them), and so are brainctl's own context names
  `root`, `dev` and `unknown` wherever they appear as a `Context` value or an investigation's
  `## Context` line. They are counted under `placeholders`.
- **Summary length.** A summary longer than 500 code points is cut at the last space before
  500, and the full text is prepended to the body as a paragraph `**Index summary:** <text>`,
  accounted as a unit body line.
- **Tags** from INDEX `Tags` and `Context` are split on `,`, trimmed, lowercased; every run of
  characters outside `[a-z0-9]` becomes `-`, and leading or trailing `-` are removed. An empty
  result is dropped. The report lists every tag whose normalized form differs from the source.
- **`repos`** is the investigation's `## Source Repo` value (its first word), else the INDEX
  `Repo` cell, else the SRC's `BRAIN_REPO` line in `.brain/AGENT.md`, normalized as a tag. A
  value that equals the basename of any SRC's parent directory (the workspace that held the
  brains), or `unknown`, `root`, `all` or `global`, means empty, because in the predecessor
  such values named the workspace, not a repository.
- **`aliases`:** file-based units get every relative path the file was known by (with and
  without the `.brain/` or `docs/` prefix, and the original filename stem when the id differs
  from it). Line-based units get `<source path>#<slug of the heading or first cell>`, made
  unique with `-2`, `-3`, … in file order. `## Always` bullets get
  `.brain/constraints.md#always-<n>`. Remainder and attachments entries get the source path (or
  SRC basename) itself. Aliases of units from a secondary SRC are prefixed with
  `<SRC basename>/`. An alias that equals the id of any entry is dropped and reported under
  `aliases_dropped`. No alias may repeat across the store (`E-ALIAS`).

**Investigation headers:** the brainctl-written header (the `# Task:` line and the `## Status`,
`## Source Repo`, `## Context` sections, in any order, each at most once, directly after the
title with only blank lines between them) is removed from the body, because its values now
live in the frontmatter. The value of each section is read from its first non-blank line:
the status value (first word), the repository (first word), and the context tags (the line
split on commas, each normalized as a tag). That line moves to the frontmatter only when it
consists of the value alone, a single token with no space or comma; a line that says more
(`COMPLETE — all apps reachable with valid TLS`) stays in the body and counts as body text,
so nothing is lost. The heading lines and the moved value lines are accounted as
`moved-to-frontmatter`. The Status section's lines are the only "superseded by" hints of an
investigation; a decision's hints are its Status section and its blockquotes. A `## Context`
or `## Summary` heading that appears later in the file is content. A leading YAML block
(`---` … `---`, present in a few early files) stays in the body untouched.

### 13.4 Rule text

- **Body:**
  - A table row becomes a body with one paragraph per column: `**<Column header>:** <cell>`.
    Cells are split on `|` not preceded by `\`; when a row has more cells than the header, the
    surplus is joined back into the last cell with ` | ` and the row is reported as
    `row-shape`. `\|` inside a cell becomes `|`.
  - A section becomes its content without the heading line.
- **Stripping markup** for titles removes `**`, `__`, `~~`, backticks, and link syntax (keeping
  link text).
- **Marker tags.** When the title, or the first paragraph of the body, contains one of these
  words (case-insensitive, whole word), the rule gets the tag: `stale` → `marker-stale`;
  `resolved` → `marker-resolved`; `fixed` → `marker-fixed`; `no longer` → `marker-no-longer`;
  `retired` → `marker-retired`; `superseded` → `marker-superseded`; `moot` → `marker-moot`;
  `closed` → `marker-closed`. A fully struck-through title gets `marker-struck`. The status
  stays `active` and the text is kept unchanged. `memex list --tag marker-stale` is the
  curation pass's worklist.

### 13.5 Link rewriting

In every imported body, path references to imported units are replaced by `[[id]]` when they
resolve to exactly one unit. A path reference is a substring matching

```
(PREFIX)?(\.brain/)?(investigations|decisions|plans|plans/prompts|plans/reports|runbooks|docs|docs/runbooks)/[A-Za-z0-9._-]+\.md(:[0-9]+|#L[0-9]+)?
```

not inside fenced code, where `PREFIX` is any of: each SRC path as given on the command line
and as an absolute path, the same with `$HOME` written as `~`, the SRC's basename followed by
`/`, or `./`, for every SRC of the run, longest first. A match that is glued on its left to a
path character (a letter, digit, `.`, `/`, `_`, `-` or `~`, as in `](../.brain/plans/x.md)`)
is part of a longer path and is left alone. A line or anchor suffix (`:12`, `#L12`) is kept
after the link as text. Nothing operator-specific is in the tool: the prefixes come from the
arguments. A reference is resolved first among the units of its own SRC, then among the
primary's; `runbooks/x.md` matches either runbook directory when exactly one has the file.
INDEX `File` values are resolved the same way.

- **Code spans.** A reference wrapped in exactly one pair of backticks (`` `path` ``, the form
  the predecessor used for most references) becomes `[[id]]` without the backticks, because a
  link inside a code span is not a link (§6). A reference inside any other code span stays as
  it is.
- **Existing wikilinks.** A `[[TARGET]]` already in a body is kept when TARGET is an id or an
  alias of the new store. When it matches the id pattern but resolves to nothing, it is wrapped
  in backticks so that it reads as the text it was and is not `E-LINK`; the report lists these
  under `wikilinks_quoted`. Any other `[[…]]` is plain text (§6).
- Unresolved references stay unchanged and are counted in the report, grouped by target.
  Mentions of `docs/backlog.md` as a whole file are expected (over 300 of them).
- `body_sha256` is computed after rewriting.
- **"Superseded by" resolution** for statuses looks, case-insensitively, for `supersed` in the
  unit's status value or in a blockquote (consecutive lines starting with `>`), and takes the
  first path reference in that status section or blockquote. The predecessor wrote the target
  on the line after the marker, so a single-line rule finds nothing.

### 13.6 Accounting

The import writes `DIR/import-report.json` (a top-level file memex ignores, §4, meant to be
committed with the import) and prints a summary.

- **Every line of every recognized source file** is attributed to exactly one of: a unit body,
  `moved-to-frontmatter` (including every INDEX row that matched a file, and every heading line
  that became a title), `structural` (table header and separator rows, top-level file title,
  `## ` lines that only group `### ` items, blank lines between units), or a **remainder**.
- **Remainder** text, meaning text in a source file that belongs to no unit (preambles,
  comments, closing notes), becomes one `reference` entry per source file. It is titled
  `<source path> (import remainder)` (`<SRC basename>/<source path>` for a secondary SRC),
  contains the non-blank lines in source order, so nothing is dropped, and is dated by the
  file's first commit with `verified` set to its last. An attachments entry (§13.3) is dated by
  its SRC's revision.
- **Per source file** the report lists lines and non-whitespace characters in, and the same out
  per category.
- The import exits 1 if any source line is unattributed. It also exits 1 if, for any unit, the
  non-whitespace characters of its source text differ from those of its output text. For this
  comparison:
  - source text is the text of the heading that became the title (without its `#` marks)
    after the importer's removals at their known positions (a `Task:`-style prefix, a
    `Mistake:`-style prefix, a trailing date form, the listed backlog marker tokens), then
    the unit's body lines in order, then an INDEX summary that was prepended to the body;
  - output text is the entry's title, when the title came from a removed heading, followed
    by the written body with every link rewrite mapped back to its original text through the
    `rewritten` record and each generated column label (`**<Column header>:** `) removed
    where it was put; for a table row or an Always bullet the title is not part of the output
    text, because it repeats text that is in the body;
  - both sides are compared after markup stripping (§13.4) and with `|` and `\|` removed;
  - the import's own additions (`was-*` and `marker-*` tags, `**<Column header>:** ` and
    `**Index summary:** ` labels) are not text.
- **INDEX rows** pointing to missing files are listed as `index-orphan`, with their summary
  preserved in the report; `index-duplicate` and `index-conflict` rows likewise.
- **Short bodies:** every entry below the content measure of 80 is listed under `short-body`,
  so the curation pass can decide what to merge.
- **Every `not-knowledge` file** is listed by path.
- **Leaks:** each `--leak PATH` becomes a `raw` journal entry exactly as `capture --from PATH`
  would make it (§11.2), without committing, dated today, with PATH written in its alias form
  (§11.2) in the summary, body and aliases, and is listed under `leaks` in that form.

**Report shape.** `import-report.json` is one object with these keys, in this order: `format`
(1), `today`, `sources` (basename, rev, commit, primary), `files` (per recognized file: lines
and non-whitespace characters in, and the same out per category), `entries` (id, type, unit
kind, source, path, status, source line range, removed and extra text), `attachments`,
`not_knowledge`, `index` (rows, matched, placeholders, orphan, duplicate, conflict),
`tags_normalized`, `links` (rewritten, wikilinks_quoted, unresolved), `duplicates`,
`aliases_dropped`, `row_shape`, `short_body`, `dated_by_blame`, `markers_removed`, `leaks`,
`warnings` (per code), `problems`. It
is written through the §10 JSON rules and contains no absolute path, so that two runs from the
same revisions are byte-identical whatever the directories.

**Command output.** On standard output: `imported <N> entries from <k> sources into <DIR>`,
then indented lines with the entry counts per type directory, the attachment, not-knowledge
and leak counts, the INDEX counts, the link and duplicate counts, the warning counts per code,
and last `report: <DIR>/import-report.json`. Exit 0. With problems, every problem is printed
on standard error as `memex: <problem>` after the summary, followed by a last line
`memex: import has <N> problems; see <DIR>/import-report.json`, and the exit code is 1; the
output is written anyway so that the report can be read. An existing `DIR` is exit 1; a first
argument other than the literal `brain-v3`, a missing SRC, neither `--out` nor `--into`
(`missing --out or --into`), or both (`--out and --into cannot be combined; --out makes a new
store, --into adds to an existing one`) is a usage error (exit 2); a SRC that is not a git
repository, a dirty SRC without `@REV`, or an unknown REV is exit 1.

**Never automatic:** deduplication, rewording, or judging whether a rule is still true. Those
are a separate, reviewed curation pass, done with ordinary memex commands.

### 13.7 Delta import

`memex import brain-v3 SRC[@REV]… --into DIR [--leak PATH]…` is for a store that was made by
an earlier import and has been curated since, while the brains went on being written: it
brings over only the units that are not in the store yet, and never rewrites, re-dates or
removes anything that is.

1. **Refusals, before anything is written.** `--into ""` is the usage error
   `empty value for --into` (and `--out ""` is `empty value for --out`). In the messages below
   `<DIR>` is the absolute path. `DIR` without a regular `memex.toml` is exit 1
   (`memex: <DIR> is not a memex store; --into must name an existing store`); `DIR` that is not
   the top of its own git repository is exit 1; a store format memex does not read is the §4
   refusal (exit 2); uncommitted changes in `DIR` (any line of `git status --porcelain
   --untracked-files=all`) are exit 1 with `memex: <DIR> has uncommitted changes; commit them
   first, so that the import is the only change to review`. A clean tree makes the import's
   whole effect one reviewable diff, and `git checkout . && git clean -fd` undoes it.
2. **A fresh import first.** The sources are imported exactly as by `--out`, with every §13.1
   to §13.6 rule, into a temporary directory outside `DIR`. If that import has problems, nothing
   is merged: only the report (step 6) is written, with an empty `delta`, and the command exits 1.
3. **Under the write lock** (§9.1 step 1, 60 s, exit 4 when it stays held), the tree is checked
   clean again and every fresh entry is classified, in the fresh import's unit order (§13.3):
   - **Present, by alias.** A store entry holds one of the fresh entry's aliases (§13.3: every
     path and heading a unit was known by; a store alias that is an absolute path also counts
     in its §11.2 alias form). Identity is the alias set, not the id, because a fresh import
     of later revisions can shift `-2` suffixes and re-date an attachments entry.
     Each store entry is claimed by at most one fresh entry; among several candidates, the one
     with the fresh entry's id is preferred, else the first in alias order.
   - **Present, by id.** Otherwise, an unclaimed store entry with the same id, type and body.
   - **New**, otherwise.

   Every fresh id is then mapped: a present entry to the id of the store entry that matched
   it; a new one to its own id, or, when the store already has that id or alias, to the first of
   `<id>-2`, `<id>-3`, … that is free (§5.4's length cap applies, as in §13.3).
4. **Present entries are never written.** Their bodies are compared, after mapping (step 5),
   with the store's: equal is `unchanged`, different is `changed`. A changed entry is a unit
   that was edited in the brain after the first import, or an entry curated in the store;
   either way it is left alone and listed, for the operator to reconcile by hand. Frontmatter
   is not compared, because curation owns it.
5. **New entries are written** into their type directory as the fresh import made them, with
   the fresh ids mapped (step 3) in `[[…]]` link targets outside code (§6), in `files/<id>/`
   prefixes, and in `superseded_by`, `parent` and `sources`; an alias that the store or an
   earlier new entry already holds, or that equals an id, is dropped and listed; a journal
   entry's `body_sha256` is recomputed. Each attachment under `files/<owner>/` is copied to
   `files/<mapped owner>/` when no file exists at that path; a different file there is left
   alone and listed. `AGENT.md`, `memex.toml` and every other existing file are untouched.
6. **The report** is written next to the existing ones, never over them:
   `DIR/import-report-<today>.json`, or the first free `import-report-<today>-<n>.json`,
   `n` = 2, 3, …. It is the §13.6 object of the fresh import, followed by one more key,
   `delta`: `{"added":[{"id":"…","fresh_id":"…","type":"…","path":"…"}],"unchanged":N,
   "changed":[{"id":"…","fresh_id":"…"}],"moved":[{"id":"…","fresh_id":"…"}],
   "attachments_added":["files/…"],"attachments_changed":["files/…"],
   "aliases_dropped":[{"id":"…","alias":"…"}]}`, where `id` is the store's id, `moved` lists
   the present entries whose fresh id differs, and the attachment lists are in code-point order.
7. **Valid.** `memex check` on `DIR` after the merge reports zero errors, as §13.1 requires of
   any import. Every error is a problem: it is printed on standard error after the summary as
   in §13.6, the files stay written for review, and the exit code is 1.

**Command output.** `imported <N> new entries from <k> sources into <DIR>`; then, when `N` > 0,
`  new entries: <dir> <n>, …` in §5.2 order; `  already in the store: <u> unchanged, <c> with
a different body (left alone), <m> under another id`; `  attachments added <a>, different and
left alone <d>; aliases dropped <x>`; and last `report: <DIR>/<report name>`. It does not
commit. Running it again with the same revisions adds nothing (`imported 0 new entries`) and
writes one more report.

An entry deleted from the store after the first import comes back with the next delta,
because nothing in the store remembers it; retire entries (§5.2) instead of deleting them.

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
| `store/` | Initial store contents (optional). The runner copies it to a temporary directory, runs `git init`, commits everything as `test <test@example.invalid>` with the message `conformance: seed`, and adds a bare clone as `origin` with upstream tracking. Symbolic links in `store/` are copied as links. Without `store/`, the store path is an empty temporary directory with no git repository and no origin, and when the case also has `src/`, that empty directory is removed so that a command may create the store. |
| `src/<name>/` | Optional, one per directory. Source repositories for `import` (§13): each is copied to a temporary directory and committed as the seed with the message `conformance: seed`, author `test <test@example.invalid>` and author and committer date `2026-01-10T12:00:00+00:00`, so that commit hashes, blame dates and first-commit dates are the same on every run. `<SRC:name>` in `cmd`, `env` and `expect/stdout` is that directory's absolute path. |
| `src2/<name>/` | Optional. A second commit of `src/<name>/`, dated `2026-01-11T12:00:00+00:00`: the paths listed in its `.remove` file (one per line) are deleted, its files are copied over the first commit's, and the result is committed as `conformance: seed 2`. This is how a case exercises history: a rename, an edit, a moved item. |
| `home/` | Optional. Files copied into the temporary `HOME` before the command runs, for commands that read files outside the store (`import --leak`, the leak scan). |
| `env` | `KEY=VALUE` lines added to the environment, after the fixed variables below, so a case can override them. `<STORE>` and `<HOME>` in a value are replaced by the paths they name in `expect/stdout`. |
| `cmd` | The argument vector, one argument per line, without the program name. `<STORE>` and `<HOME>` are replaced as in `env`. |
| `stdin` | Standard input (optional; empty if absent) |
| `git` | Optional. git arguments, one per line, run as `git -C <store> <args>` with the runner's environment (the case's `env` included) after the previous step and before this step's `cmd`, so that a case can stage changes for `check --staged`, commit by hand or move `origin`. An empty line separates commands, which run in order; a command that starts with `-C <path>` runs elsewhere, as git applies successive `-C` options in turn. `<STORE>` and `<HOME>` are replaced as in `env`. A git command that fails fails the case, except the last one when `expect/git-exit` exists. |
| `expect/git-exit` | Optional. The expected exit code of the step's last git command, for a git step that must fail (a commit the pre-commit hook refuses), or `0` for one that must succeed and whose standard error the case checks (a warning the hook prints). |
| `expect/git-stderr-contains` | Optional. Lines that must each appear in that last git command's standard error; read only when `expect/git-exit` exists. |
| `expect/stdout` | Exact expected standard output, after placeholder substitution: `<SHA7>` matches any 7 hex digits; `<TIME>` matches any RFC 3339 timestamp; `<STORE>` is the temporary store's absolute path; `<HOME>` is the temporary home; `<VERSION>` is the version token of `memex version`'s output (`0.0.0-dev` in `memex 0.0.0-dev (store format 1)`) |
| `expect/exit` | Expected exit code |
| `expect/store/` | Optional. The exact expected store working tree after the command, excluding `.git` and `.memex`. Regular files are compared by relative path, bytes and executable bit. Directories are not compared: an empty directory is ignored on both sides, because a case directory kept in git cannot hold one |
| `expect/commits` | Optional. Expected `git log --format=%s` of the store, newest first, without the `conformance: seed` commit |
| `expect/stderr-contains` | Optional. Lines that must each appear in standard error |

**Runner environment.** Every command runs with exactly this environment, then the case's `env`:

- `PATH` is one temporary directory holding only two entries: `memex`, a symbolic link to the
  executable under test, and `git`, a symbolic link to the system `git`. Nothing else from the
  host `PATH` is reachable, so a case cannot depend on a host tool. The link named `memex` is
  what the pre-commit hook of Appendix A runs.
- `HOME` is a temporary directory. It holds `.gitconfig` with `user.name = test`,
  `user.email = test@example.invalid` and `init.defaultBranch = main`, and nothing else.
- `GIT_CONFIG_GLOBAL` names that `.gitconfig`, so the identity holds even when a case's `env`
  overrides `HOME`, and `GIT_CONFIG_NOSYSTEM=1`.
- `MEMEX_STORE` is the temporary store, `MEMEX_CONFIG` an empty file,
  `MEMEX_TODAY=2026-01-15`, `TZ=UTC` and `LC_ALL=C`.

No other variable is inherited from the runner's environment. The working directory is the
temporary `HOME`, which is not inside a git repository, so §5.5 attribution is empty unless a
case says otherwise.

A case with several steps uses `steps/01/`, `steps/02/`, …, each holding its own `cmd`, `stdin`,
`git` and `expect/`, sharing one store. A step may also hold `store/`: before its git step, the
paths listed in its `.remove` file (one per line) are deleted from the store's working tree and
its other files are copied over it, without committing, which is how a case makes a direct edit
or a dirty tree. The case's own `store/` is the seed, never an overlay.

The same placeholders apply to the ` ```example ` blocks of the help pages (§16.9), so that a
help example can show `pushed <SHA7>` and `Store root: <STORE>` and still be executed.

**Network cases.** A case directory may contain `origin-unreachable`, which makes the runner
point `origin` at a local TCP listener that accepts connections and never answers, instead of
the bare clone (as a `git://` URL, so that no transport helper is involved), so that `network_timeout` behavior (§9.1) is tested with a deterministic hang
and no assumption about the host's network; the runner sets `MEMEX_CONFIG` to a file with
`network_timeout = 2` for such cases.

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
| `memex help --all` | The overview, every topic page, then every command page, in the order of §16.6 and §11 (§11.1 `init` through §11.16 `version`). Pages are separated by a line `---`. |
| `memex help --json` | The machine-readable interface (§16.7) |

- An unknown name exits 2 and prints, on stderr:
  `memex: no help for "<name>"; did you mean "<nearest>"? Run: memex help topics`.
  - The nearest name is the one with the smallest Levenshtein distance, and only if that
    distance is ≤ 3; otherwise the `; did you mean "<nearest>"?` clause is replaced by `.`,
    giving `memex: no help for "<name>". Run: memex help topics`.
  - Ties are broken by the order of the candidate list (commands in §11 order, then topics
    in §16.6 order).
- An unknown command exits 2 and prints, on stderr, with the same suggestion rule over the
  command names:
  `memex: unknown command "<name>"; did you mean "<nearest>"? Run: memex help topics`.
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
  that implements it (§0), and leaves `internal/spec`'s table until then, so that `help --json`,
  `help topics` and the pages always agree.

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
   command accepts and no others. The value and default cells are those of `help --json`
   (empty for none; backticks are formatting), and the meaning says `repeatable` exactly when
   the option is repeatable and `(required)` or `(required <condition>)` exactly when it is
   required.
5. **Output:** the exact shape of standard output, including the outcome line (§9.4) for
   mutating commands.
6. **Exit codes:** only the codes this command can return.
7. **Examples:** two to five. Each is a fenced block marked ` ```example `, holding one command
   line prefixed `$ ` (split like a POSIX shell: single quotes literal, double quotes with
   `\"`, no expansions) followed by its exact output: the standard output lines, then the
   standard error lines, each prefixed `! `, then a line `exit N` when the exit code is not 0.
   The runner compares the two streams and the exit code separately. The examples of one page run in order
   against one store seeded from `conformance/help-fixture/store/`, with
   `conformance/help-fixture/home/` copied into the temporary home and the working directory
   set to it, so a mutating example changes what later examples see. When
   `conformance/help-fixture/edits/cmd-<command>/` exists, its files are copied over the
   store's working tree, without committing, before that page's examples run, so that an
   example can show a direct edit (`save`) or a non-canonical file (`fmt --check`). The §15
   placeholders apply.
8. **Mistakes to avoid:** the misuse the command refuses, and why. Example for `capture`:
   titles starting with `-`, empty bodies, and writing to a tool's memory instead.
9. **See also:** related commands and topics.

Each page is at most 120 lines.

### 16.6 Topic pages

In this order (the `--all` order):

| Topic | Covers |
|---|---|
| `agents` | Rules of engagement for language-model agents, at most 60 lines: run `preflight` before work; capture after it, with the heredoc form `--body - <<'EOF'` so one shell call suffices; never write a tool's own memory; edit then `save`; respect outcome lines and exit codes; read `matched k/n` and any `weak results` line before trusting a result; quote a phrase as `'"two words"'` so the shell keeps the quotes; `tags` are for filtering and `keywords` for finding; journal bodies are immutable; how to supersede instead of editing history. Written as imperatives. |
| `concepts` | The model and the reasoning behind it (§1, §2) |
| `entries` | Types, statuses, visibility and every frontmatter field, as tables (§5.2, §5.3), then the body outline of each type (Appendix B) |
| `ids` | The id and slug rules, with worked examples (§5.4) |
| `links` | Link syntax and resolution (§6) |
| `search` | Query syntax, prefix terms, phrases, ranking, filters, and how to read output, including hidden-count lines (§8) |
| `writing` | The write protocol, direct edits, outcomes and conflicts (§9) |
| `config` | Store resolution, the config file and environment variables (§3) |
| `check-codes` | Every `E-` and `W-` code, with its condition and the usual fix (§7) |
| `exit-codes` | The §10 table |
| `neutrality` | Pointer files, the leak scan and tool memory (§12) |
| `importing` | The brain v3 importer and its report (§13). Named so that it doesn't collide with the `import` command in `memex help import` |

Each topic is at most 200 lines. `memex help --all` as a whole MUST stay under 3,000 lines. The
conformance suite measures this, so that reading all of memex stays one affordable call.

### 16.7 Machine-readable interface

`memex help --json` prints one compact JSON object, keys in this order:

```json
{"version":"…","format":1,
 "global_options":[{"name":"--store","value":"PATH","repeatable":false,"required":false,"default":null,"summary":"…"}],
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
  `"when status is superseded"`. On options it is `true`, `false`, or a condition string:
  `capture`'s `--title`, `--summary` and `--body` are `"unless --from"`.
- A positional that is a literal keyword is listed under its literal (`brain-v3` for
  `import`).
- `global_options` lists the options accepted before or after any command (§10), with the
  same keys as a command's options. They are not repeated in each command's `options`, and a
  page's Options table need not list them.

### 16.8 Usage errors

On a usage error (exit 2), stderr carries:
1. `memex: <problem>`;
2. the command's usage line;
3. `Run: memex help <command>`.

Examples of `<problem>`: `unknown option --tittle (did you mean --title?)`, `missing --body`,
`value for --title starts with "-"; write --title=-…`.

`memex` with no command is a usage error with the same three lines, using the general form
of §10 as the usage line:

```
memex: missing command
memex [GLOBAL OPTIONS] COMMAND [OPTIONS] [ARGS]
Run: memex help
```

### 16.9 Guarantees enforced by the conformance suite

- Every command and topic has a page. `memex help topics` lists exactly them.
- Every option and positional accepted by a command appears in its page's Options table and in
  `help --json`. Conversely, every documented option is accepted.
  - The runner checks this by comparing `help --json` with the Options tables: names, values,
    defaults, repeatable and required (§16.5).
  - It also tries each documented option. An undocumented `--x` must fail as unknown.
- Every ` ```example ` block is executed as §16.5 says, in a store seeded from
  `conformance/help-fixture/`, and its output must match exactly. The fixture is a small
  realistic store (a dozen entries across every type, one `always` rule, a hidden entry, an
  alias) and a `home/` with a body file and an empty repository directory for the examples.
- The runner performs these checks as one more case, named `help`, so that
  `scripts/check.sh` fails when a page drifts from the implementation.
- Each command page has the §16.5 sections in order (Usage, Description, Output, Exit codes
  and Examples present), a Description of at most 15 lines, two to five examples, and an
  Exit codes table listing exactly the command's `exit_codes` in `help --json`, in order.
  `memex COMMAND --help` prints the page.
- The overview has the §16.4 parts in order: the version and agent lines, then the sections
  `The model`, `The everyday loop`, `Commands` (every command with its summary, in §11
  order) and `Topics`, ending with a pointer to `memex help topics`.
- Headings inside fenced code blocks are not section headings for any of these checks.
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

- **Bounded output.** Every command that can print an unbounded amount has a bound by
  default: `search` has `--limit` (`search_limit`), `preflight` has `--max-chars`, `list` has
  `--limit` (`list_limit`) and `--fields`, and `check` has `--limit` and `--errors-only`.
  `show` prints whole entries, which the caller chose.
- **No filler.** No banners, no decorative output, no repeated headers, and no "success!"
  lines beyond the outcome line.
- **Triage before reading.** Results show title, summary and a one-line snippet, enough to
  decide whether to open the entry. That is why `summary` is required on new entries.
- **Truncation is announced.** Whenever output is cut, a line at the end of the output, before
  the summary or outcome line when one exists, says so and names the option that lifts the
  cut. Silent truncation is a defect.

### 17.3 Unambiguous and stable output

- **Stable shapes.** Text output has fixed line shapes (§8.5, §9.4, §11), and every command
  that prints data accepts `--json`, except `preflight`, whose output is for reading in
  context; use `search --json` for the same results. Changing a text shape counts as an interface change, and
  goes in `CHANGELOG.md` under "Changed output".
- **The outcome is always last.** For mutating commands, the last line is exactly one outcome
  line (§9.4).
- **Exit codes carry meaning** (§10). An agent can branch on the code without parsing text.
- **Paths are relative to the store root,** and the root is printed once, so an agent can use
  its own file tools. Every place that takes an id also takes that path (§10), so what memex
  prints can be pasted back without transformation.
- **Partial matches are labeled.** Every search result says which query items it matched
  (§8.5).

### 17.4 Forgiving of model mistakes, never silently

Language models predictably make certain mistakes. memex catches each one and says how to fix it.

| Mistake | memex's response |
|---|---|
| A misspelled command, option, type, status, field or topic | Exit 2 with `did you mean "<nearest>"?` (Levenshtein distance ≤ 3), plus the help command |
| An unknown or invented id (`show`, `set`, `promote`, `--parent`, `--source`) | Exit 1 with `no entry "<id>"`, plus up to 3 candidates, one per line, as `  candidate: <path>`: first every entry whose id is within Levenshtein distance 3 of the given text (the smaller of the distances to the whole id and to the id without its `DATE-` prefix), nearest first, then by id; then the entries that rank highest for the given text as a search query (§8.3), skipping those already listed |
| The same capture run twice | Idempotent no-op (§11.2) |
| A second entry with the same title on the same day | Refused with the existing entry's title and summary and the three correct moves (§11.2) |
| A phrase typed as `"two words"` that the shell unquoted | Not detectable; `help search` and `help agents` show the `'"…"'` form |
| An id typed as the path search printed | Accepted everywhere (§10) |
| A title starting with `-`, or an option swallowed as a value | Refused (§10) |
| An empty or heading-only body | Refused, with the content-measure rule and `memex help entries` |
| Editing a journal body | `check` fails with `E-HASH`, and the message names `set <id> status=superseded` plus `capture` as the correct path |
| Writing knowledge into a tool's memory | Detected by the leak scan (§12), with `capture --from PATH` as the fix: `W-LEAK` in `check --workspace`, `LEAK <path>` under `doctor` |
| Writing knowledge into a repository's pointer file (`CLAUDE.md`, `AGENTS.md`, …) | `check --workspace` reports `E-POINTER`, naming `capture --from <path>` and `init --pointers <dir> --force` (§12 rule 2) |
| Starting work in a repository that has no pointer file | `check --workspace` reports `W-NO-POINTER` with `init --pointers <dir>` |
| A search phrased differently from the stored text | OR-combined ranked terms, prefix matching, and `keywords` (§8). The miss is reported, never silent |

### 17.5 Safe to run unattended

- Never interactive (Principle 9).
- Reads are side-effect free, apart from the search cache (§8.6) and `preflight`'s sync of the
  store.
- Writes are atomic and verified (§9.1). A write that fails part-way leaves the store as it was,
  or reports exactly what was left.
- An interrupted write (SIGINT, SIGTERM) leaves the store as it was, or a commit that is
  reported as not pushed (§9.1); it never leaves files half-written or a lock behind.
- Concurrent agents on one host are serialized by the write lock. Agents on different hosts are
  reconciled by rebase, and a true conflict is reported with exit 6, never resolved by guessing.
- **Every network operation is bounded** by `network_timeout` (§3.2), so an offline host or a
  stuck remote costs seconds, not a hung session. The predecessor had both: a fetch with no
  bound, and a stale DNS entry that failed every sync for days before anyone noticed.

### 17.6 Writing for the next agent

memex's own guidance, in `help agents` and `help entries`, tells agents how to write entries
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
- Before any non-trivial task: `memex preflight "<the task, in content words>"` (skip a, the, for).
  Read the `matched k/n` label and any `weak results` line before trusting a result.
- After any non-trivial task:
  `memex capture --title="<task>" --summary="<one sentence>" --body - <<'EOF' … EOF`.
- Never write knowledge into a coding tool's own memory or rules files.
  Knowledge lives here only.
- Edit entries directly when needed, then run `memex save`.

Operator-specific instructions follow.
```

`.githooks/pre-commit`:

```sh
#!/bin/sh
# Written by memex init. Add your own checks as executables in .githooks/pre-commit.d/.
store=$(git rev-parse --show-toplevel) || exit 1
memex --store "$store" check --staged || exit 1
if [ -d "$store/.githooks/pre-commit.d" ]; then
  for hook in "$store"/.githooks/pre-commit.d/*; do
    [ -x "$hook" ] && { "$hook" || exit 1; }
  done
fi
exit 0
```

The file mode is `0755`. The hook passes `--store` explicitly because memex never infers the
store from the current directory (Principle 6), and a commit made by hand, by another user or
by automation may run without `MEMEX_STORE` or a config file. The `pre-commit.d/` loop exists
so that an operator's own checks (for example a secret scanner) chain after memex's without
editing a file that `init` owns.

## Appendix B. Body templates

Printed by `memex help entries`, one per type. Suggestions only; nothing checks them.

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
