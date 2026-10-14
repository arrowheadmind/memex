# memex import

Convert brain v3 stores into a new store.

## Usage

```
memex import brain-v3 SRC[@REV]… --out DIR [--leak PATH]…
```

## Description

The one-time migration from memex's predecessor. Reads one or more brain v3 repositories
through git at the given revision (never their working trees; a dirty tree is refused without
`@REV`), and writes a complete new store at `DIR`, which must not exist. The result passes
`memex check` with zero errors, every source line is accounted for, and the same revisions
always produce the same bytes. It does not commit: review `DIR` and its
`import-report.json`, then `memex save -m "import"`. `memex help importing` describes the
mapping and the report.

## Options

| option | value | default | meaning |
|---|---|---|---|
| `--out` | DIR | | New store directory; must not exist (required) |
| `--leak` | PATH | | Tool-memory file to import as a raw journal entry; repeatable |

## Output

`imported <N> entries from <k> sources into <DIR>`, indented count lines per type and per
report list, and last `report: <DIR>/import-report.json`. Problems go to standard error as
`memex: <problem>` after the summary, then a final `memex: import has <N> problems; see
<DIR>/import-report.json`, and the exit code is 1.

## Exit codes

| code | when |
|---|---|
| 0 | Imported |
| 1 | `DIR` exists; a SRC is not a repository, is dirty without `@REV`, or REV is unknown; the accounting does not balance |
| 2 | A first argument other than `brain-v3`, a missing SRC or a missing `--out` |

## Examples

```example
$ memex import brain-v3 --out newstore
! memex: missing SRC
! memex import brain-v3 SRC[@REV]… --out DIR [--leak PATH]…
! Run: memex help import
exit 2
```

```example
$ memex import brain-v3 repo --out newstore
! memex: import: repo is not a git repository
exit 1
```

## Mistakes to avoid

- Importing into an existing store: `--out` must be new. Re-run against later revisions to
  pick up what was written in between, then merge by hand if needed.
- Treating the import as curation: it keeps everything and judges nothing. Retire and merge
  afterwards with `memex set` and `memex save`.

## See also

`memex help importing`, `memex check`, `memex save`.
