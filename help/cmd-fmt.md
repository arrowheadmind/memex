# memex fmt

Rewrite entries into canonical form and commit them.

## Usage

```
memex fmt [PATHS…] [--check]
```

## Description

memex writes entries in one canonical form: fields in a fixed order, one per line, arrays
sorted, a blank line before the body, one final line feed. A hand-edited entry can drift from
it, which `memex check` reports as `W-CANONICAL`. `fmt` rewrites the selected entries (all of
them with no paths) without changing any field value or the body, and commits. Entries with
errors are skipped and reported; fix them by hand first. Journal bodies are never touched.

## Options

| option | value | default | meaning |
|---|---|---|---|
| `--check` | | | Write nothing; list every non-canonical selected entry, one per line, and exit 1 if any |

## Output

Only the outcome line; `nothing to commit` when every entry was canonical. With `--check`, the
non-canonical paths in path order and no outcome line.

## Exit codes

| code | when |
|---|---|
| 0 | Committed, or nothing to do; with `--check`, everything canonical |
| 1 | A path is not an entry; with `--check`, something is not canonical |
| 2 | Usage error |
| 4 | Another memex held the write lock for 60 s |
| 5 | Committed but not pushed |
| 6 | Committed, rebase conflict |

## Examples

```example
$ memex fmt --check
```

```example
$ memex fmt
nothing to commit
```

## Mistakes to avoid

- Expecting `fmt` to fix errors: it only reorders and reformats valid entries. `memex check`
  names what to fix.
- Running `fmt` on a file you have also edited: `memex save --fmt` does both in one commit.

## See also

`memex check`, `memex save`, `memex help entries`.
