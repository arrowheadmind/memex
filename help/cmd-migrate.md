# memex migrate

Move the store to another store format, in one commit.

## Usage

```
memex migrate [--to N]
```

## Description

Rewrites `memex.toml` to store format N (default 2, the newest) and commits it, after checking
that every entry is valid at that format. Format 2 adds `due` and `due_when` on tasks and
plans, `stale_days` in `memex.toml`, and refuses control characters in `title`, `summary` and
`due_when`. Moving to 2 writes `format = 2` and `stale_days = 14` (when absent); moving back to
1 writes `format = 1` and removes `stale_days`, and is refused while an entry carries `due` or
`due_when` (each one is named, with the `memex set` line that removes it). Every other line of
`memex.toml` is kept as it is. A store already at N is left alone. The store must be clean
(`memex save` first): only `memex.toml` is committed.

A memex older than format 2 refuses a format 2 store as a whole (`store format 2 is newer than
this memex reads (1); upgrade memex`), so upgrade memex on every machine that uses the store
before migrating it. A memex that reads format 2 reads and writes format 1 stores as before.

## Options

| option | value | default | meaning |
|---|---|---|---|
| `--to` | N | 2 | The store format to move to: 1 or 2 |

## Output

`store format <old> -> <new>`, a line saying `stale_days = 14` was written when it was (so
`check` now reports `W-OPEN-STALE`), then the write outcome (`pushed <sha>`, …). A store already at
N: `store format <n> already; nothing to migrate`, then `nothing to commit`. A refusal lists
each entry that would not validate on standard error.

## Exit codes

| code | when |
|---|---|
| 0 | Migrated, or already at the format |
| 1 | Refused: an entry would not validate at the new format, `memex.toml` is not valid, or the store has uncommitted changes |
| 2 | Usage error, or a store format this memex cannot read |
| 4 | Another memex held the write lock for 60 s |
| 5 | Committed but not pushed |
| 6 | Rebase conflict |

## Examples

```example
$ memex migrate
store format 2 already; nothing to migrate
nothing to commit
```

```example
$ memex migrate --to 3
! memex: value for --to must be a store format from 1 to 2, not "3"
! memex migrate [--to N]
! Run: memex help migrate
exit 2
```

## Mistakes to avoid

- Migrating before every machine runs a memex that reads format 2: the older ones then refuse
  the whole store until they are upgraded.
- Editing `format` in `memex.toml` by hand: nothing checks the entries first, and `stale_days`
  is not written. Use `memex migrate`.

## See also

`memex help config` (the manifest keys), `memex help entries` (`due`, `due_when`),
`memex due`, `memex check`.
