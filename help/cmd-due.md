# memex due

List what open tasks and plans owe: past due, due soon, waiting, blocked.

## Usage

```
memex due [--days N] [--hints] [--json]
```

## Description

Prints the `due` dates and `due_when` conditions of open entries (tasks `open` or
`in-progress`, plans `draft` or `active`), grouped by what to do: past due, due by the
horizon (today plus `--days`), waiting on a condition, and blocked tasks with their summary as
the reason. Closed entries are never listed, so closing one takes it off. It reads only the
store's files, never git or the network, and always exits 0 once it has printed, so a
session-start hook, a timer or a nightly check can run it. Tasks that name a date only in
their title or body (`2026-02-01: …`, `by 2026-01-30`, `due: 2026-03-01`) are counted as
hints; `--hints` lists them with the `memex set` line that moves the date into `due`.
`memex check` reports the past ones as `W-PAST-DUE`.

## Options

| option | value | default | meaning |
|---|---|---|---|
| `--days` | N | 7 | Horizon in days from today, 0 to 36500; 0 lists only today |
| `--hints` | | | List tasks that name a date only in their text |
| `--json` | | | Print one JSON object instead of text |

## Output

Sections in this order, each omitted when empty: `past due: <n>`, `due by <horizon>: <n>`,
`when: <n>`, `blocked: <n>`. Each entry is one line, `<due> <status> <path> <title>`, with
`-` for no date, then ` (when: <due_when>)` when it has a condition, and in `blocked`
` (why: <summary>)`. Then `due after <horizon>: <k> (raise --days)` when later dates exist,
and `dates in text, no due: <h> (memex due --hints lists them)`; with `--hints` that is a
section of lines ending `(set it: memex set <id> due=<date>)`. With nothing to print:
`nothing due`. `--json` prints `{"today","horizon","past":[…],"upcoming":[…],"when":[…],
"blocked":[…],"later":K,"hints":[…]}`; each entry is `{"id","type","status","path","title",
"summary","due","due_when"}` (null when absent), each hint `{"id","status","path","title",
"date"}`, and the hints are listed with or without `--hints`.

## Exit codes

| code | when |
|---|---|
| 0 | Listed, also when something is past due |
| 2 | Usage or configuration error |

## Examples

```example
$ memex due
due by 2026-01-22: 1
2026-01-20 in-progress tasks/2026-01-11-replace-rsync-on-beta.md Replace rsync on beta
when: 1
- active plans/2026-01-08-backup-overhaul.md Backup overhaul (when: after the restore test is documented)
```

```example
$ memex due --days 2
when: 1
- active plans/2026-01-08-backup-overhaul.md Backup overhaul (when: after the restore test is documented)
due after 2026-01-17: 1 (raise --days)
```

```example
$ memex due --days=-1
! memex: value for --days must be an integer from 0 to 36500, not "-1"
! memex due [--days N] [--hints] [--json]
! Run: memex help due
exit 2
```

## Mistakes to avoid

- Writing a date only in a title or a body: nothing reads it. Set `due`
  (`memex set <id> due=2026-02-01`), or `due_when` for an event with no date.
- Moving a date with `memex set` alone: the reason is lost. Use
  `memex update <id> --due=<new> --summary=<sentence> --body -`, whose section says why.
- Leaving a task open that waits on a decision: park it with
  `memex update <id> --status blocked …`; it stays listed under `blocked` with its summary.
- Reading exit 0 as "nothing is late": `due` always exits 0; `memex check --strict` fails on
  `W-PAST-DUE`.

## See also

`memex check` (`W-PAST-DUE`, `W-OPEN-STALE`), `memex update`, `memex set`,
`memex help entries`.
