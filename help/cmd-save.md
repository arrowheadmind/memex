# memex save

Commit direct edits to the store.

## Usage

```
memex save [-m MSG] [PATHS…] [--all] [--allow-delete] [--fmt]
```

## Description

Editing an entry with any editor is first-class; `save` commits the result through the same
protocol as every other write: the edited entries and their link targets are validated first,
so a link to a missing id (`E-LINK`) or a missing field is refused with the findings and your
edit stays in place.
Name the files you edited: `memex save PATH…` commits only those (a directory names every
changed file under it), so another session's half-done edit in the same checkout stays out of
your commit. With no paths it takes the one changed file, added, modified, deleted or
untracked (except `.memex/` and anything ignored); when more than one file has changes it
lists them on standard error and refuses (exit 1) unless `--all` is given, because they may not
all be yours. A deleted entry is refused unless `--allow-delete` is given.
It is also how the first commit of a new store is made: a store with no commits yet takes every
file without `--all`.

## Options

| option | value | default | meaning |
|---|---|---|---|
| `-m` | MSG | `memex: save <n> files` | Commit message |
| `--all` | | | With no PATHS, save every changed file, even when more than one has changes |
| `--allow-delete` | | | Allow committing deleted entries |
| `--fmt` | | | Rewrite the selected entries into canonical form first |

## Output

Only the outcome line: `pushed <sha7>`, `committed <sha7>, no upstream`, `nothing to commit`,
or `committed <sha7>, NOT pushed: <reason>`.

## Exit codes

| code | when |
|---|---|
| 0 | Committed, or nothing to commit |
| 1 | Validation failed (the findings are printed), an entry was deleted without `--allow-delete`, or more than one file has changes and neither PATHS nor `--all` was given |
| 2 | Usage error |
| 4 | Another memex held the write lock for 60 s |
| 5 | Committed but not pushed |
| 6 | Committed, rebase conflict |

## Examples

```example
$ memex save -m "note the short key id" runbooks/2026-01-04-rotate-the-backup-key-on-alpha.md
pushed <SHA7>
```

```example
$ memex save
nothing to commit
```

## Mistakes to avoid

- A bare `memex save` in a checkout another session also edits: name your paths. Give `--all`
  only when every listed change is yours.
- Committing with `git` directly: it works, but the pre-commit hook runs the same validation
  and you lose the outcome line and the push.
- Editing a journal body and saving: refused with `E-HASH`; capture the correction and
  supersede the old entry instead.

## See also

`memex set`, `memex fmt`, `memex help writing`.
