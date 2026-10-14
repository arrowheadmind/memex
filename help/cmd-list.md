# memex list

List entries, newest first.

## Usage

```
memex list [--type T]… [--status S]… [--tag X]… [--repo R]… [--host H]… [--all] [--limit N]
           [--fields F,…] [--json]
```

## Description

Lists entries ordered by `created` descending, then id. By default only entries with a
visible status are listed and at most `list_limit` (50) of them; `--status` selects statuses
explicitly whatever their visibility, `--all` lists every entry, and `--limit 0` lifts the
cap. `list` replaces a hand-maintained index: it is always current because it is computed.

## Options

| option | value | default | meaning |
|---|---|---|---|
| `--type` | T | | Only this type (name or directory); repeatable |
| `--status` | S | | Only this status; repeatable; a status no type has is a usage error |
| `--tag` | X | | Only entries with this tag; repeatable |
| `--repo` | R | | Only entries concerning this repository; repeatable |
| `--host` | H | | Only entries concerning this machine; repeatable |
| `--all` | | | List every entry, whatever its status |
| `--limit` | N | `list_limit` (50) | Maximum number of entries; 0 means no limit |
| `--fields` | F,… | | Columns to print, in order, from `created, type, status, id, path, title, summary, tags` |
| `--json` | | | Print one JSON object instead of text |

## Output

One line per entry, `<created> <type> <status> <path> <title>`; with `--fields`, the chosen
columns separated by tabs. When entries were cut: `N more entries not shown (raise --limit)`.
`--json` prints `{"entries":[{"id","type","status","created","title","summary","tags","path"}
…],"more":N}`.

## Exit codes

| code | when |
|---|---|
| 0 | Listed (an empty list is still 0) |
| 2 | Unknown option, field or status |

## Examples

```example
$ memex list --type task
2026-01-11 task in-progress tasks/2026-01-11-replace-rsync-on-beta.md Replace rsync on beta
2026-01-07 task open tasks/2026-01-07-document-the-restore-test.md Document the restore test
```

```example
$ memex list --tag backup --fields id,status --limit 3
2026-01-11-replace-rsync-on-beta	in-progress
2026-01-10-first-key-rotation	kept
2026-01-09-restore-test-on-alpha	raw
4 more entries not shown (raise --limit)
```

## Mistakes to avoid

- `memex list` on a large store prints 50 entries and a `more` line; filter with `--type`,
  `--tag` or `--status` rather than raising the limit.
- Done tasks and retired rules are not listed by default; use `--status done` or `--all`.

## See also

`memex search`, `memex show`, `memex help entries`.
