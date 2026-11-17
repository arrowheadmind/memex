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
3. It reads only the working tree, never git; exit 0 always; `--json` for machines; what it
   cannot read is counted and listed, never dropped silently.
4. `check` reports `W-PAST-DUE` for an open entry past its date, with the three commands that
   clear it: close it, move the date with a reason (`update --due`), park it.
5. `check` reports `W-OPEN-STALE` for an open or in-progress task with neither `due` nor
   `due_when`, unchanged for more than `stale_days` days, read from git: the newest commit
   that changed the file.
6. "Parked" is the existing task status `blocked`; `memex update <id> --status blocked`
   records why, and parked tasks are exempt from both warnings but stay listed.
7. A date written only in a task's text is a hint in `memex due` (a count line;
   `--hints` lists each with the `memex set` line that moves it into `due`), not a finding.
8. `stale_days` is a key of `memex.toml` (0 or absent turns the warning off; `init` and
   `migrate` write 14).
9. The fields and the key are store format 2. A format 1 store reads exactly as before, and
   `memex migrate` moves a store to 2 (and back to 1 while nothing carries a date).
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
come from the `git log` it already ran). Revision after review round 1 (note 9): the benchmark
now runs each command once and checks its exit code and output before timing it, and that
check found that the made-up store had one task in twenty unreadable (a lost newline after
`due`), which the old `|| true` hid; with the store fixed, `due` 214–295 ms and `check`
473–523 ms on 5,000 entries.

**`due` says what it left out, and prints one line per entry.** A hook reads `due`'s output,
so an entry it cannot classify must not vanish (review round 1, finding 4): `due` checks every
field it reads (status, title, summary, `due`, `due_when`) and skips a task or plan where one
is missing or not valid, with a line on standard error, a `skipped, not valid: <k>` count
line and a `skipped` list in the JSON. The exit stays 0: the list printed is correct as far as
it goes, and `check` is the command that fails. A value with a control character could end a
line early or drive a terminal (finding 5): format 2 refuses control characters in `title`,
`summary` and `due_when` at validation, so no write can store one, and `due` escapes them
(`\t`, `\u001B`) in text output for format 1 stores and hand edits; the JSON is the machine
interface and keeps the values. Rejected: a non-zero exit for skipped entries (a hook would
then lose the whole list for one bad file), refusing control characters in format 1 (it would
turn a store that passes `check` today into one that fails).

**"Touched" is read from git, not from a field.** The last change of a task is the committer
date of the newest commit on `HEAD`'s first-parent history that changes the file (the same
`git log` that summary currency already reads, so `check` makes no extra pass), or today when
the working tree differs from `HEAD`. A field (`updated`, `touched`) would have to be written by
every edit, and a hand edit would forget it; §5.3 already says there is no `updated` field
because git records it. The date is the commit's own (its recorded offset), so every reader
gets the same answer. Any commit counts, a one-field `set` or a mass `fmt` included: a session
that looked at a task and set a field has touched it, and a mass reformat is rare. In a
shallow clone the boundary commit is not a change: a task whose newest version is the
boundary's was last changed on the boundary's date or earlier, so it is reported once the
boundary is older than `stale_days`, with "or earlier" in the message (never too soon; at
worst late by the age of the boundary). Rejected: author date (a rebase keeps it old), the
file's mtime (every clone differs), and refusing shallow clones (CI checkouts are shallow).

**Fourteen days, per store, written down.** Fourteen is what the brief proposes; one review
asked for seven. It is a property of the store, so it lives in `memex.toml` (`stale_days`),
never in the caller's config: every machine applies the same threshold. An absent key is off,
not 14: a store must not acquire a policy it never wrote, and a `check --strict` job that
passed yesterday must not fail after an upgrade (review round 1, finding 2). `init` and
`migrate` write `stale_days = 14`, so new and migrated stores have it, visibly. "Every machine
sees the same findings" holds for the same today: today is each machine's local date (§2), so
two machines either side of midnight disagree for those hours (`MEMEX_TODAY` pins it).
Rejected: a per-entry field, a command-line option, a store time zone (a date is a day, and
the hours of disagreement are the price of not having one).

**A task that says when it is owed is not stale.** A long task due months ahead, or one that
waits on an event (`due_when`), is a decision to wait, already recorded; warning on it every
fortnight would push people to make empty commits or to turn the warning off (review round 1,
finding 6). So `W-OPEN-STALE` skips a task with `due` or `due_when`; a past `due` is
`W-PAST-DUE` instead. That is the per-task threshold: a long task sets its own date. No other
exemption was added; `blocked` remains the way to park a task with no date. Rejected: a
per-entry `stale_days`, a `no-stale` tag (a second way to say what `due` says).

**Parked is `blocked`.** The task statuses are fixed by §5.2 (`open`, `in-progress`,
`blocked`, `done`, `dropped`), not per store. A new `parked` status would be one more thing
format 2 adds, for no gain. `blocked` already means
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
"re-check on 11-09" (no year, and `on` alone is too broad). It stays a guess: `2026-01-15
incident follow-up` and "was completed by 2026-01-15" are hinted too (review round 1, note 8),
so the output says "guessed from the wording" and the `memex set` line reads "if it is a
deadline"; both cases are in the tests.

**Messages carry the commands.** Each warning ends with the exact commands that clear it,
like every other `check` message. To move a date with its reason in one commit, `update`
gains `--due` and `--due-when`: the new section is the reason, and the summary is rewritten
with it. Rejected: `memex set <id> due=<new>` alone, which moves the date and records no reason.

## Compatibility: store format 2 (review round 1, the blocking finding)

The first version kept format 1 and added the fields as optional. Codex's review (round 1,
finding 1) blocked it: an older memex rejects `due`, `due_when` and `stale_days`, so on a
machine that has not upgraded, `check` and every write refuse the entries that carry them
(and `stale_days` refuses every write), while reads go on and silently drop what they cannot
read. Format negotiation exists so that an old reader says "upgrade" once, for the whole
store, instead of failing entry by entry. Two answers were possible: (a) store format 2, with
a migration, or (b) no new fields at all, the dates kept in a place older clients already
accept.

**Chosen: (a), store format 2.** (b) needs a place that does not exist in the format: every
frontmatter field not in §5.3 is an error, every `memex.toml` key not in §4 is an error, and
the only things an older memex ignores are dot-files at the top level. A sidecar
(`.memex-due.toml`, id to date) would work mechanically, but it is a second database beside
the entries: a date away from its task, a hand edit of the task that cannot see or move it, an
id rename or a promotion that leaves it behind, its own validation, merge conflicts in one
shared file, and `show` and `search` that cannot read it without learning a second file
format. It avoids a format bump by making the format worse for every user, for good, to spare
a one-time step. (a) is the mechanism the spec already has (§4: an older memex refuses a newer
format with "upgrade memex", exit 2), and it costs one command once. (a) is also the smaller
change in what it leaves behind: one manifest line and one command, against a second store
format inside the first.

What (a) is:

- `StoreFormat` is 2. This memex reads and writes formats 1 and 2. In a format 1 store, `due`
  and `due_when` are `E-FIELD-UNKNOWN` with the command that allows them
  (`field "due" needs store format 2; run memex migrate`), `stale_days` is `E-MANIFEST` the
  same way, and the stale finding is off: a format 1 store means exactly what it meant before.
- `memex init` and the importer write `format = 2` and `stale_days = 14`.
- `memex migrate` (default `--to 2`) rewrites `memex.toml` in one commit: `format = 2`, and
  `stale_days = 14` if absent; every other line kept. It first validates the store as it would
  be and refuses if that adds an error (format 2 also refuses control characters in `title`,
  `summary` and `due_when`, finding 5), naming each entry. `memex migrate --to 1` is the undo:
  refused while any task or plan carries `due` or `due_when` (each named with the
  `memex set <id> due=` line that clears it), then `format = 1` and `stale_days` removed.
- An older memex (format 1 only) on a format 2 store: every command exits 2 with
  `store format 2 is newer than this memex reads (1); upgrade memex`. It fails fast and whole,
  never half-reads.

**What a format bump costs a public user.** One command, once, per store: `memex migrate`,
after upgrading memex on every machine that uses the store. Nothing changes until they run it:
the new memex reads their format 1 store as before (no new warnings, no `stale_days`), so an
upgrade alone is safe. Until they migrate they cannot write `due`; the refusal names the
command. A user with one machine upgrades and migrates in a minute. A user with several, or a
CI job pinned to an older memex, must upgrade those first, or the old ones stop at "upgrade
memex" (loud, not lossy). Migrating back is possible as long as no entry carries a date.

**Machines on mixed versions** (for example machines that take a new memex at a nightly pull,
so for some hours old and new run side by side): (1) release; machines pick up the new memex
at their next pull. During that window the stores stay format 1, so old and new memex read
and write them alike, and nothing can write a due date yet (the new memex refuses it in a
format 1 store, the old one does not know it). (2) Once every machine reports the new version
(`memex version` prints `store format 2`), run `memex migrate` once per store from one machine
and let the others pull. (3) A machine that was missed fails with "upgrade memex" on that store
until it upgrades; no entry is misread. To undo: clear the dates, `memex migrate --to 1`. The
order is the only coordination needed: upgrade everywhere, then migrate.

## Not in this piece

Closing tasks from git landings, any hook or timer (they belong to whoever deploys memex), and
syncing dates to a calendar.
