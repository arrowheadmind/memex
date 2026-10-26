# memex capture

Create a journal entry and commit it.

## Usage

```
memex capture --title=T --summary=S --body FILE|- [same options]
memex capture --from PATH [--title=T] [--summary=S] [same options]
```

## Description

`capture` is `memex new journal`: the end-of-task record of what was attempted, worked, failed
and is reusable; its body is immutable.

`--from PATH` imports a file that leaked into a coding tool's memory: title from its first
heading (else YAML `name:`, else the filename), summary from YAML `description:` or `Imported
from PATH`, body after `Imported from PATH (date)`; alias PATH, absolute, home as `~/`. PATH is
never deleted (`remove PATH after review`); a file already an alias in any form is a no-op.
A repeat is still that no-op after PATH has been removed; then the remove line is omitted.
memex resolves and checks the store, takes the write lock, freshens, and reads its entries
before it reads the `--body` file, standard input or the `--from` file.

## Options

| option | value | default | meaning |
|---|---|---|---|
| `--title` | T | | Title (required unless `--from`); write `--title=-x` for a value starting with `-` |
| `--summary` | S | | One sentence that tells a reader whether to open the entry (required unless `--from`) |
| `--body` | FILE | | Body file, or `-` for standard input (required unless `--from`) |
| `--from` | PATH | | Import a file found in a tool's memory as a `raw` journal entry |
| `--status` | S | `raw` | Initial status |
| `--tag` | X | | Tag for filtering; repeatable |
| `--keyword` | K | | Word a reader would search with; repeatable |
| `--repo` | R | the current repository | Repository the entry concerns; repeatable |
| `--no-repo` | | | Do not attribute the entry to the current repository |
| `--host` | H | | Machine the entry concerns; repeatable |
| `--always` | | | Rule printed by every preflight (rules only) |
| `--kind` | K | | Not allowed on journal entries |
| `--parent` | ID | | Not allowed on journal entries |
| `--source` | ID | | Entry this one was derived from; repeatable |
| `--verified` | DATE | | Not allowed on journal entries |
| `--review-days` | N | | Not allowed on journal entries |
| `--alias` | A | | Former name or path that resolves to this entry; repeatable |

## Output

`created <path>`, then the outcome line (`pushed <sha7>`, …). A repeat with the same body
prints `exists <path>` and `nothing to commit`. With `--from`, `remove <PATH> after review`
follows the `created` line, or the `exists` line when PATH still exists.

## Exit codes

| code | when |
|---|---|
| 0 | Created, or already existed with the same body |
| 1 | Refused: body too short, the id taken by a different body |
| 2 | Usage error: missing `--title`, `--summary` or `--body`, an option journal entries do not allow |
| 4 | Another memex held the write lock for 60 s |
| 5 | Committed but not pushed |
| 6 | Committed, rebase conflict |

## Examples

```example
$ memex capture --title="Deployed the restic timer to beta" --summary="The timer runs; the dirty-tree check caught an uncommitted edit first." --tag backup --keyword restic --keyword timer --host beta --body notes/body.md
created journal/2026-01-15-deployed-the-restic-timer-to-beta.md
pushed <SHA7>
```

```example
$ memex capture --title="Deployed the restic timer to beta" --summary="The timer runs; the dirty-tree check caught an uncommitted edit first." --tag backup --keyword restic --keyword timer --host beta --body notes/body.md
exists journal/2026-01-15-deployed-the-restic-timer-to-beta.md
nothing to commit
```

## Mistakes to avoid

- A title starting with `-` (`capture --help` once created a junk entry in the predecessor)
  must be written `--title=-x`; `--help` prints this page instead.
- An empty or heading-only body is refused; write what the next reader needs.
- A summary is one line of at most 500 characters; a longer one is refused with
  `memex: <path>: E-FIELD-VALUE summary must be one line of at most 500 characters` before the
  refusal line. Shorten it and retry; nothing was created.
- Do not write the lesson into a tool's own memory file instead; capture it here, where every
  tool and the next session find it.

## See also

`memex new`, `memex promote`, `memex help agents`, `memex help neutrality`.
