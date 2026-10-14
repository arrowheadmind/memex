# memex help writing

Every command that changes the store (`new`, `capture`, `promote`, `set`, `fmt`, `save`,
`sync`) runs the same protocol, and ends with exactly one outcome line.

## The write protocol

1. **Lock.** One writer per store at a time (`.memex/write.lock`). Another memex waits up to
   60 s (10 s for `preflight`'s sync), then exits 4.
2. **Freshen.** Fetch the upstream and fast-forward when the tree is clean. A rebase, merge or
   cherry-pick in progress refuses the command before anything is written.
3. **Validate** the inputs and compute every file's full new contents. Nothing is written
   yet; a store with no commits yet is refused here (`run memex save` after `init`).
4. **Write** each file through a temporary file and a rename. A file that changed since memex
   read it is refused: `<path> changed while memex was working; retry`.
5. **Read back** and validate the written entries and their link targets; on failure the
   previous contents are restored and the command exits 1.
6. **Commit** exactly those paths. The pre-commit hook runs `memex check --staged`. Other
   changes in the working tree are left as they are.
7. **Integrate.** No upstream: stop. Other uncommitted changes: stop, the commit is kept but
   not pushed. Otherwise pull with rebase and push, up to five attempts; a conflicting rebase
   is aborted and reported; every network step is bounded by `network_timeout`.
8. **Report** the outcome.

Interrupting memex (Ctrl-C, SIGTERM) leaves the store as it was, or a commit that is reported
as not pushed. It never leaves a half-written file or a lock behind.

## Outcome lines

The last line of standard output is exactly one of these:

| line | exit |
|---|---|
| `pushed <sha7>` | 0 |
| `committed <sha7>, no upstream` | 0 |
| `nothing to commit` | 0 |
| `committed <sha7>, NOT pushed: <reason>` | 5 |
| `committed <sha7>, NOT integrated: rebase conflict in <paths>` | 6 |

Reasons include `uncommitted changes (run memex save)`, `timed out after 20s`, `interrupted`,
`rebase abort failed: …`, or git's own `fatal:` line. A commit is never undone automatically:
fix the cause, then `memex sync`. A command that fails before the commit prints no outcome
line; its exit code and standard error carry the reason, and nothing changed.

## Commit messages

`memex: new <type> <id>`, `memex: promote <source-id> to <type> <new-id>`,
`memex: set <id> field=value …`, `memex: fmt <n> entries`, `memex: save <n> files` (or your
`-m` text). memex adds no trailers; author and committer come from your git configuration.

## Direct edits

Editing an entry with any editor is first-class. Then `memex save [-m MSG] [PATHS…]` commits
it through the same protocol: validation first, so a broken link or a missing field is refused
with the findings and your edit left in place. With no paths it takes every added, modified,
deleted or untracked file in the store. A deleted entry needs `--allow-delete`; `--fmt`
rewrites the selected entries into canonical form first.

`memex set` changes frontmatter without an editor, but refuses an entry that has uncommitted
edits (`run memex save first`), so its commit never carries someone else's unsaved work.

## Sync

`memex sync` fetches, fast-forwards or rebases, and pushes, printing `fetched <time>` and one
state line: `up to date`, `fast-forwarded N commits`, `pushed N commits`,
`rebased and pushed N commits`, `no upstream`, or `NOT SYNCED: <reason>` (exit 5). A dirty
tree is never rebased: `NOT SYNCED: uncommitted changes (run memex save)`.

## Conflicts

Two writers edit the same entry on two hosts: the second push is rejected, memex rebases,
and if the same lines changed the rebase is aborted and reported with exit 6 and the paths.
Resolve with git in the store (`git rebase origin/main`, fix, `git rebase --continue`), then
`memex sync`. With one file per entry this is rare; it is never resolved by guessing.
