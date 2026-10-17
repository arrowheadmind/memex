# memex set

Change frontmatter fields of an entry and commit it.

## Usage

```
memex set ID ASSIGNMENT…
memex set --batch FILE|-
```

## Description

Changes one or more frontmatter fields of an entry without an editor, keeps the body byte for
byte, and commits. Assignments: `field=value` sets, `field=` removes an optional field,
`field+=value` appends to an array, `field-=value` removes from one. Arrays in `field=` form
are comma-separated; booleans are `true` or `false`; dates are `YYYY-MM-DD`. `created` and
`body_sha256` cannot be set. `status=superseded` needs `superseded_by=<id>` in the same
command; leaving `superseded` removes `superseded_by`. The result must validate.
`--batch FILE` reads one `ID ASSIGNMENT…` line per entry, quoted as on a command line
(`'title=A b'`); blank lines and `#` lines are skipped. Every line is checked before anything is
written, and the whole batch is one commit: one bad line (`--batch line <n>: …`) writes nothing.

## Options

| option | value | default | meaning |
|---|---|---|---|
| `--batch` | FILE | | Apply one ID ASSIGNMENT… line of FILE (- for standard input) per entry, in one commit |

## Output

Only the outcome line (`pushed <sha7>`, …), or `nothing to commit` when nothing changed. The
commit message lists the assignments with resolved ids. With `--batch`, the subject is
`memex: set <n> entries` and the body has the single form's message for each changed entry.

## Exit codes

| code | when |
|---|---|
| 0 | Committed, or nothing to do |
| 1 | Unknown ID; the entry has uncommitted edits (`run memex save first`); the result would not validate; a field that cannot be set; any invalid `--batch` line |
| 2 | Usage error: a malformed assignment or an unknown field; arguments given with `--batch` |
| 4 | Another memex held the write lock for 60 s |
| 5 | Committed but not pushed |
| 6 | Committed, rebase conflict |

## Examples

```example
$ memex set 2026-01-07-document-the-restore-test status=in-progress keywords+=restore
pushed <SHA7>
```

```example
$ memex set 2025-12-20-backups-run-at-midnight status=superseded
! memex: rules/2025-12-20-backups-run-at-midnight.md: E-SUPERSEDED status is superseded but superseded_by is missing; add it: memex set 2025-12-20-backups-run-at-midnight superseded_by=<new>
! memex: refused: the result would not validate; nothing was changed
exit 1
```

```example
$ memex set --batch notes/batch.txt
pushed <SHA7>
```

## Mistakes to avoid

- Editing a journal body with `set`: only frontmatter changes; to correct a journal entry,
  capture the correction and set the old one `status=superseded superseded_by=<new-id>`.
- `set` on a file you have edited by hand: run `memex save` first, so the two changes do not
  share one commit.
- Hundreds of `memex set` calls in a loop: each is a commit and a push. Write the lines to a
  file and run `memex set --batch FILE` once; naming one entry on two lines is refused.

## See also

`memex save`, `memex show`, `memex help entries`.
