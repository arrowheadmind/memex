# memex new

Create an entry of any type and commit it.

## Usage

```
memex new TYPE --title=T --summary=S --body FILE|- [--status S] [--tag X]… [--keyword K]…
          [--repo R]… [--no-repo] [--host H]… [--always] [--kind K]
          [--parent ID] [--source ID]… [--verified DATE] [--review-days N] [--alias A]…
```

## Description

Creates one entry of `TYPE` (`rule`, `decision`, `runbook`, `reference`, `task`, `plan` or
`journal`; `capture` is the shortcut for a journal entry), writes it in canonical form, and
commits and pushes it. The id is today's date plus a slug of the title. `--summary` is
required because lists and search results show it: it is what lets the next reader decide
whether to open the entry. Give `--keyword` whenever the body's vocabulary differs from how
someone would ask about it. The body must hold at least 80 characters of text outside
headings. Unless `--repo` or `--no-repo` says otherwise, the entry is attributed to the git
repository of the current directory. memex resolves and checks the store, takes the write
lock, freshens, and reads its entries before it reads the `--body` file or standard input.

## Options

| option | value | default | meaning |
|---|---|---|---|
| `--title` | T | | Title (required); write `--title=-x` for a value starting with `-` |
| `--summary` | S | | One sentence that tells a reader whether to open the entry (required) |
| `--body` | FILE | | Body file, or `-` for standard input (required) |
| `--status` | S | first status of the type | Initial status |
| `--tag` | X | | Tag for filtering; repeatable |
| `--keyword` | K | | Word a reader would search with; repeatable |
| `--repo` | R | the current repository | Repository the entry concerns; repeatable |
| `--no-repo` | | | Do not attribute the entry to the current repository |
| `--host` | H | | Machine the entry concerns; repeatable |
| `--always` | | | Rule printed by every preflight (rules only) |
| `--kind` | K | | `prompt` or `report` (plans only) |
| `--parent` | ID | | Enclosing plan (plans and tasks only) |
| `--source` | ID | | Entry this one was derived from; repeatable |
| `--verified` | DATE | today | When the facts were checked (references only) |
| `--review-days` | N | | Staleness threshold for `verified` (references only) |
| `--alias` | A | | Former name or path that resolves to this entry; repeatable |

## Output

`created <path>`, then the outcome line: `pushed <sha7>`, `committed <sha7>, no upstream`,
or `committed <sha7>, NOT pushed: <reason>`. Running the same command again prints
`exists <path>` and `nothing to commit`.

## Exit codes

| code | when |
|---|---|
| 0 | Created, or already existed with the same body |
| 1 | Refused: body too short, a field or link invalid, the id taken by a different body |
| 2 | Usage error: unknown type or status, an option the type does not allow, missing option |
| 4 | Another memex held the write lock for 60 s |
| 5 | Committed but not pushed (the last line says why) |
| 6 | Committed, rebase conflict |

## Examples

```example
$ memex new rule --title="Create the restore target before running restic" --summary="restic restore does not create its target directory; mkdir it first." --tag backup --keyword restic --keyword restore --body notes/body.md
created rules/2026-01-15-create-the-restore-target-before-running-restic.md
pushed <SHA7>
```

```example
$ memex new task --title="Short" --summary="A body that is only a heading is refused." --body -
! memex: body content measure 0 is below 80: a body needs at least 80 non-whitespace characters outside headings, the knowledge itself and not only its outline; see memex help entries
exit 1
```

## Mistakes to avoid

- A title starting with `-` must be written `--title=-x`; a bare `--title -x` is refused.
- Headings alone are not a body; the check counts text, including code.
- A summary is one line of at most 500 characters. A refused write says what failed before
  the refusal line, as `memex check` would: `memex: <path>: E-FIELD-VALUE summary must be one
  line of at most 500 characters`, then `memex: refused: the result would not validate;
  nothing was changed`. Fix the named field and retry; nothing was created.
- The same title twice on one day is refused when the body differs: read the existing entry
  (`memex show <path>`) or choose a more specific title.

## See also

`memex capture`, `memex promote`, `memex help entries`, `memex help ids`.
