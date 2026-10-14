# memex import

Convert brain v3 stores into a new store, or add what is new to an existing one.

## Usage

```
memex import brain-v3 SRC[@REV]… --out DIR [--leak PATH]…
memex import brain-v3 SRC[@REV]… --into DIR [--leak PATH]…
```

## Description

The migration from memex's predecessor. Reads one or more brain v3 repositories through git at
the given revision (never their working trees; a dirty tree is refused without `@REV`). With
`--out`, writes a complete new store at `DIR`, which must not exist: it passes `memex check`
with zero errors, every source line is accounted for, and the same revisions always produce
the same bytes. With `--into`, `DIR` is an existing store with a clean working tree, made by
an earlier import and curated since: only the units it does not hold yet (matched by their
aliases, then by id) are added, with links mapped to the store's ids; nothing present is
rewritten or re-dated, and the report goes to `import-report-<today>.json` next to the old
one. Neither form commits: review, then `memex save -m "import"`. `memex help importing`
describes the mapping and the report.

## Options

| option | value | default | meaning |
|---|---|---|---|
| `--out` | DIR | | New store directory; must not exist (this or `--into`) |
| `--into` | DIR | | Existing store to add only the entries it does not hold yet |
| `--leak` | PATH | | Tool-memory file to import as a raw journal entry; repeatable |

## Output

`imported <N> entries from <k> sources into <DIR>`, indented count lines per type and per
report list, and last `report: <DIR>/import-report.json`. With `--into`: `imported <N> new
entries …`, the new entries per type, `already in the store: <u> unchanged, <c> with a
different body (left alone), <m> under another id`, the attachment and alias counts, and
`report: <DIR>/import-report-<today>.json`. Problems go to standard error as
`memex: <problem>` after the summary, then a final `memex: import has <N> problems; see
<DIR>/import-report.json`, and the exit code is 1.

## Exit codes

| code | when |
|---|---|
| 0 | Imported |
| 1 | `--out DIR` exists, or `--into DIR` is not a clean store; a SRC is not a repository, is dirty without `@REV`, or REV is unknown; the accounting does not balance |
| 2 | A first argument other than `brain-v3`, a missing SRC, neither or both of `--out` and `--into` |
| 4 | `--into`: another memex held the store's write lock for 60 s |

## Examples

```example
$ memex import brain-v3 --out newstore
! memex: missing SRC
! memex import brain-v3 SRC[@REV]… --out DIR [--leak PATH]…
! memex import brain-v3 SRC[@REV]… --into DIR [--leak PATH]…
! Run: memex help import
exit 2
```

```example
$ memex import brain-v3 repo --out newstore
! memex: repo is not a git repository
exit 1
```

```example
$ memex import brain-v3 repo --into notes
! memex: notes is not a memex store; --into must name an existing store
exit 1
```

## Mistakes to avoid

- Importing later revisions with `--out` and merging by hand: `--into` does that merge, and
  lists in the report's `delta` every entry whose body changed on either side, for you to
  reconcile.
- Deleting an imported entry instead of retiring it: the next `--into` brings it back.
- Treating the import as curation: it keeps everything and judges nothing. Retire and merge
  afterwards with `memex set` and `memex save`.

## See also

`memex help importing`, `memex check`, `memex save`.
