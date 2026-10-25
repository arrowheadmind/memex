# memex check

Validate the store.

## Usage

```
memex check [--staged] [--workspace] [--strict] [--errors-only] [--limit N] [--json]
```

## Description

Checks every entry's name, encoding, frontmatter, fields, status, links, aliases and hash, and
the store layout, and prints one line per finding. Errors are what must be fixed; warnings are
hygiene (no summary, no terms, stale reference, old raw journal entry, a line that looks like
a credential). The pre-commit hook runs `check --staged` on what is about to be committed, so
an error never lands. `--workspace` also scans the configured `workspace` (§12): pointer files
that are not the pointer (`E-POINTER`), repositories without one (`W-NO-POINTER`) and files in
coding tools' own memory (`W-LEAK`), with absolute paths. Read-only; `check` takes no lock.
`W-STALE-SUMMARY` is the one finding that reads git history (first-parent, plus the working
tree or, with `--staged`, the index): a task or plan whose summary was not rewritten when a
section was added to its body or when it was closed or reopened. A history git cannot read is
exit 2, `memex: check needs the store's git history: <git's first error line>`.

## Options

| option | value | default | meaning |
|---|---|---|---|
| `--staged` | | | Validate only entries added, modified or changed in type in the git index, plus their link targets |
| `--workspace` | | | Also check pointer files and scan for leaks in tool memory (`workspace` key) |
| `--strict` | | | Count warnings as errors for the exit code |
| `--errors-only` | | | Print only error lines (the summary still counts warnings) |
| `--limit` | N | 200 | Maximum number of finding lines; 0 means no limit |
| `--json` | | | Print one JSON object: `findings`, `errors`, `warnings`, `truncated` |

## Output

`<path>: <CODE> <message>` per finding, sorted by path, code and message; when cut,
`<CODE>: K more (raise --limit)` per remaining code; last, `check: N errors, M warnings`.

## Exit codes

| code | when |
|---|---|
| 0 | No errors (no warnings either, with `--strict`) |
| 1 | Errors found |
| 2 | Usage error; with `--workspace`, no usable `workspace` in the config file; a git history that cannot be read |

## Examples

The task `2026-01-11-replace-rsync-on-beta` was closed by hand in the working tree, its
summary left as it was:

```example
$ memex check
reference/2026-01-12-nas-shares.md: W-NO-TERMS no tags or keywords; add the words a reader would search with: memex set 2026-01-12-nas-shares keywords=<word>,<word>
tasks/2026-01-11-replace-rsync-on-beta.md: W-STALE-SUMMARY summary was not updated when status changed from "in-progress" to "done"; rewrite it to the current state: memex set 2026-01-11-replace-rsync-on-beta summary=<sentence>
check: 0 errors, 2 warnings
```

```example
$ memex check --errors-only
check: 0 errors, 2 warnings
```

## Mistakes to avoid

- Fixing a warning by deleting the entry. Warnings name the command that fixes them.
- Clearing `W-STALE-SUMMARY` with a reworded copy of the old summary: write where the entry
  stands now. Next time, record progress with `memex update`.
- Running `check` with `--strict` in the hook before the imported store is curated: thousands
  of warnings would block every commit.

## See also

`memex fmt`, `memex save`, `memex doctor`, `memex help check-codes`, `memex help neutrality`.
