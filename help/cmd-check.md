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

## Options

| option | value | default | meaning |
|---|---|---|---|
| `--staged` | | | Validate only entries added or modified in the git index, plus their link targets |
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
| 2 | Usage error; with `--workspace`, no usable `workspace` in the config file |

## Examples

```example
$ memex check
reference/2026-01-12-nas-shares.md: W-NO-TERMS no tags or keywords; add the words a reader would search with: memex set 2026-01-12-nas-shares keywords=<word>,<word>
check: 0 errors, 1 warnings
```

```example
$ memex check --errors-only
check: 0 errors, 1 warnings
```

## Mistakes to avoid

- Fixing a warning by deleting the entry. Warnings name the command that fixes them.
- Running `check` with `--strict` in the hook before the imported store is curated: thousands
  of warnings would block every commit.

## See also

`memex fmt`, `memex save`, `memex doctor`, `memex help check-codes`, `memex help neutrality`.
