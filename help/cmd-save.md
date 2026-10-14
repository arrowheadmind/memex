# memex save

Commit direct edits to the store.

## Usage

```
memex save [-m MSG] [PATHS…] [--allow-delete] [--fmt]
```

## Description

Editing an entry with any editor is first-class; `save` commits the result through the same
protocol as every other write: the edited entries and their link targets are validated first,
so a broken link or a missing field is refused with the findings and your edit stays in place.
With no paths it takes every added, modified, deleted or untracked file in the store (except
`.memex/` and anything ignored). A deleted entry is refused unless `--allow-delete` is given.
It is also how the first commit of a new store is made.

## Options

| option | value | default | meaning |
|---|---|---|---|
| `-m` | MSG | `memex: save <n> files` | Commit message |
| `--allow-delete` | | | Allow committing deleted entries |
| `--fmt` | | | Rewrite the selected entries into canonical form first |

## Output

Only the outcome line: `pushed <sha7>`, `committed <sha7>, no upstream`, `nothing to commit`,
or `committed <sha7>, NOT pushed: <reason>`.

## Exit codes

| code | when |
|---|---|
| 0 | Committed, or nothing to commit |
| 1 | Validation failed (the findings are printed), or an entry was deleted without `--allow-delete` |
| 2 | Usage error |
| 4 | Another memex held the write lock for 60 s |
| 5 | Committed but not pushed |
| 6 | Committed, rebase conflict |

## Examples

```example
$ memex save
nothing to commit
```

```example
$ memex save -m "note the short key id" runbooks/2026-01-04-rotate-the-backup-key-on-alpha.md
nothing to commit
```

## Mistakes to avoid

- Committing with `git` directly: it works, but the pre-commit hook runs the same validation
  and you lose the outcome line and the push.
- Editing a journal body and saving: refused with `E-HASH`; capture the correction and
  supersede the old entry instead.

## See also

`memex set`, `memex fmt`, `memex help writing`.
