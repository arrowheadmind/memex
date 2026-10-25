# memex update

Append a dated section to an entry and replace its summary, in one commit.

## Usage

```
memex update ID --summary=S --body FILE|- [--heading=H] [--status S]
```

## Description

Records progress on an entry in one step, so that its summary and its body cannot drift apart:
lists and search show the summary, not the body. Use it for every status note on a task or a
plan, and to close one: `--status done` sets the status in the same commit. The body gets
`## <heading> <today>`, a blank line and the `--body` text (at least 80 characters outside
headings), after the current body; `--heading` defaults to `Status`. `--summary` must differ
from the current summary: write the summary as it stands after this section. Journal entries
are refused (their bodies are immutable), and so is an entry with uncommitted edits.
`status=superseded` also needs `superseded_by`, which only `memex set` sets.

```
memex update 2026-01-10-rotate-the-backup-key --status done \
  --summary "Done 2026-01-12: key rotated on both hosts, old key revoked." --body - <<'EOF'
Rotated on alpha and beta; the old key is revoked and a restore with the new key succeeded.
EOF
```

## Options

| option | value | default | meaning |
|---|---|---|---|
| `--summary` | S | | The summary as it stands after this section; must differ from the current one (required) |
| `--body` | FILE | | Text of the new section: a file, or `-` for standard input (required) |
| `--heading` | H | Status | Heading of the new section, 1 to 100 characters; the date is appended |
| `--status` | S | | New status, set in the same commit |

## Output

Only the outcome line (`pushed <sha7>`, …). The commit message is `memex: update <id>`, with
` status=<value>` when `--status` changed the status.

## Exit codes

| code | when |
|---|---|
| 0 | Committed |
| 1 | Unknown ID; a journal entry; the same summary; a short section; uncommitted edits; the result would not validate |
| 2 | Usage error: a missing `--summary` or `--body`, a bad `--heading`, an unknown `--status` |
| 4 | Another memex held the write lock for 60 s |
| 5 | Committed but not pushed |
| 6 | Committed, rebase conflict |

## Examples

```example
$ memex update 2026-01-11-replace-rsync-on-beta --status done --summary="Done 2026-01-15: beta backs up with restic like alpha; the rsync timer is gone." --body notes/progress.md
pushed <SHA7>
```

```example
$ memex update 2026-01-07-document-the-restore-test --summary="Write down the restore check so that the key rotation runbook can point at it." --body notes/progress.md
! memex: --summary is the current summary of 2026-01-07-document-the-restore-test; write the summary as it stands after this section
exit 1
```

```example
$ memex update 2026-01-09-restore-test-on-alpha --summary="Restore tested again." --body notes/progress.md
! memex: journal bodies are immutable; record the progress with memex capture, then memex set 2026-01-09-restore-test-on-alpha status=superseded superseded_by=<new> if it replaces this one
exit 1
```

## Mistakes to avoid

- Appending a status section by hand and leaving the summary: lists then show stale work as
  current, and `memex check` reports `W-STALE-SUMMARY`. `update` is the one-step form.
- Repeating the old summary: refused. Say where the entry stands now, not what it set out to do.
- `memex set <id> status=done` alone on a task or plan: refused, because it keeps the summary;
  add `summary=` to the same command, or use `update` when there is something to record.

## See also

`memex set`, `memex save`, `memex help writing`, `memex help check-codes`.
