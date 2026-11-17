# Design note: dates that somebody reads, and open tasks that went quiet (B18)

The normative text is in SPEC.md (§4 `stale_days`, §5.3 `due` and `due_when`, §7 `W-PAST-DUE`
and `W-OPEN-STALE`, §11.2 and §11.17 `--due` and `--due-when`, §11.18 `memex due`). This note
says why it has that shape and what was considered and rejected.

## The problem

In a store that has been used for a few months, a date written in a task's title or body
("2026-11-26: review the dumps", "re-check on 11-09", "by 2026-11-21") is read by nobody: no
command knows it is a date. A pass over two real stores found five such dates or conditions
that had passed in silence, while the two dates someone kept on a hand-written list were all
met; about one open task in five was in fact done and never closed; and the promises that got
lost most were tied to an event, not a date ("after the upgrade lands", "later").

So the store needs (1) a date that memex understands, (2) a way to write down "when X
happens" without a date, (3) one command a hook or a timer can run that prints what is late
and what is near, and (4) a finding for an open task that nobody has touched.

## The design in ten lines

1. `due` (a date, never a time) and `due_when` (one line of text) are optional fields on tasks
   and plans; `new`, `update` and `set` write them.
2. `memex due` lists open tasks and plans: past due first, then due within `--days` (7), then
   the `due_when` ones, then blocked (parked) tasks with their summary as the reason.
3. It reads only the working tree, never git; exit 0 always; `--json` for machines.
4. `check` reports `W-PAST-DUE` for an open entry past its date, with the three commands that
   clear it: close it, move the date with a reason (`update --due`), park it.
5. `check` reports `W-OPEN-STALE` for an open or in-progress task unchanged for more than
   `stale_days` (14) days, read from git: the newest commit that changed the file.
6. "Parked" is the existing task status `blocked`; `memex update <id> --status blocked`
   records why, and parked tasks are exempt from both warnings but stay listed.
7. A date written only in a task's text is a hint in `memex due` (a count line;
   `--hints` lists each with the `memex set` line that moves it into `due`), not a finding.
8. `stale_days` is a key of `memex.toml` (0 turns the warning off).
9. The store format stays 1: every new field and key is optional, and stores without them
   read exactly as before.
10. Closing an entry (`done`, `dropped`, `abandoned`) takes it off every list and clears both
    warnings; nothing else is needed.

## Decisions and the alternatives rejected

**A date, not a time.** memex's today is a date (§2), `created` and `verified` are dates, and
`W-STALE-REFERENCE` and `W-RAW-OLD` count days. A time would need a time zone in the store, and
a promise that must be kept to the hour belongs in a calendar. Rejected: a TOML date-time, or a
`due` string with an optional time.

**On tasks and plans only.** The brief asked for any entry. A due date needs a status that
says "still owed", and only tasks and plans have one: a rule, a runbook or a reference is
`active` for years, and a reference already has `verified` and `review_days`; a journal is a
record. Allowing it there would give `W-PAST-DUE` an entry that cannot be closed. Rejected:
all types, and decisions (a `proposed` decision that needs deciding is better written as a
task).

**`due_when` as free text that never fires.** An event cannot be read by memex, and pretending
to (matching commit messages, watching files) would make it fire wrongly. Listing it on every
`memex due` is what keeps it from being lost; the reader decides whether it happened.
Rejected: a structured condition, or reading git landings (left for later, outside memex).

**A command, `memex due`, not `memex list --due`.** `list` prints one line shape for every
entry, sorted by `created`, and callers parse it; `due` groups by what to do (late, soon,
waiting, parked), sorts by date and has its own empty answer (`nothing due`). Putting that
behind a flag of `list` would give the same command two output shapes. Rejected:
`list --due [--before DATE]`.

**Fast by reading files only.** `memex due` loads the entries as `list` does and never runs
git, so it costs what `list` costs. Measured with `scripts/due-bench.sh` on a made-up store of
5,000 entries: 235–290 ms per run (text or `--json`), against 455–490 ms for `check` on the
same store. On a copy of a real store of about 2,800 entries, `due` and `list --limit 1` both
took 220–255 ms; `check` took the same time before and after this change (the last-change dates
come from the `git log` it already ran).

**"Touched" is read from git, not from a field.** The last change of a task is the committer
date of the newest commit on `HEAD`'s first-parent history that changes the file (the same
`git log` that summary currency already reads, so `check` makes no extra pass), or today when
the working tree differs from `HEAD`. A field (`updated`, `touched`) would have to be written by
every edit, and a hand edit would forget it; §5.3 already says there is no `updated` field
because git records it. The date is the commit's own (its recorded offset), so every reader
gets the same answer. Any commit counts, a one-field `set` or a mass `fmt` included: a session
that looked at a task and set a field has touched it, and a mass reformat is rare. In a
shallow clone the boundary commit is not a change, so a shallow CI checkout reports only tasks
changed after its boundary (fewer findings, never false ones). Rejected:
author date (a rebase keeps it old), the file's mtime (every clone differs).

**Fourteen days, per store.** Fourteen is what the brief proposes; one review asked for seven.
It is a property of the store, so it lives in `memex.toml` (`stale_days`), never in the
caller's config: every session must see the same findings. Rejected: a per-entry field, a
command-line option.

**Parked is `blocked`.** The task statuses are fixed by §5.2 (`open`, `in-progress`,
`blocked`, `done`, `dropped`), not per store. A new `parked` status would make every older
memex report `E-STATUS` on those entries and refuse to `set` them. The same is true of `due`
itself (`E-FIELD-UNKNOWN`), which cannot be avoided; a status is avoidable, so it was. `blocked` already means
"not moving, for a reason", is visible by default, and `memex update --status blocked` forces
a new summary, which is the reason `memex due` prints. Parked tasks are exempt from
`W-PAST-DUE` and `W-OPEN-STALE` but always listed under `blocked`, dated or not, so a parked
task is seen at every `memex due` and never silently dropped. Plans have no such status; a
plan that waits drops its date for a `due_when`. Rejected: a `parked` status, a `parked` tag.

**The date in the text is a hint in `memex due`, not a check note.** `check` has two
severities and the immutable count; a third ("note") would change the summary line and the
JSON of every caller for one migration aid. The people who act on it are reading `memex due`
anyway. The pattern is narrow on purpose: a title that begins with a date, or a deadline word
(`due`, `by`, `deadline`, `no later than`, `on or after`) right before a full date. Words such
as `before`, `until` and `on` were measured and dropped: they mostly date past facts
("deleted before 2026-10-28", "a limit until 2026-11-06"). Dates without a year are not
matched. Measured on copies of two real stores (about 80 open tasks): 19 tasks hinted, all 19
naming a real deadline (17 in the title, 2 in the body); one of the two body hints is the first
of several dates, which its body later replaces, so the suggested `memex set` line must be read
before it is run. No task was hinted for a date that is not a deadline. The pattern misses
"re-check on 11-09" (no year, and `on` alone is too broad).

**Messages carry the commands.** Each warning ends with the exact commands that clear it,
like every other `check` message. To move a date with its reason in one commit, `update`
gains `--due` and `--due-when`: the new section is the reason, and the summary is rewritten
with it. Rejected: `memex set <id> due=<new>` alone, which moves the date and records no reason.

## Compatibility

No store format change. A store without `due`, `due_when` or `stale_days` reads exactly as
before and needs no migration. Two things change for existing stores:

- `check` now reports `W-OPEN-STALE` for every open task unchanged for more than 14 days, so a
  store that used to pass `check --strict` may fail it. That is the point of the warning; set
  `stale_days = 0` in `memex.toml` to turn it off.
- An older memex does not know the new fields: on an entry with `due` or `due_when` it reports
  `E-FIELD-UNKNOWN` and its `set` and `update` refuse the entry, and a `memex.toml` with
  `stale_days` is `E-MANIFEST` for it. Every machine that writes a store should run the new
  version before the first `due` or `stale_days` is written.

## Not in this piece

Closing tasks from git landings, any hook or timer (they belong to whoever deploys memex), and
syncing dates to a calendar.
