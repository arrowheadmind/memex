# memex help exit-codes

An agent can branch on the exit code without parsing text. Each command's page lists the
codes it can return.

| code | meaning |
|---|---|
| 0 | Success |
| 1 | Validation error, check failure, or refused operation |
| 2 | Usage or configuration error |
| 3 | Search found nothing visible |
| 4 | Lock timeout |
| 5 | Committed but not pushed, or sync failed |
| 6 | Rebase conflict |
| 70 | Internal error (a bug) |

Notes:

- 1 covers every refusal that changed nothing: a bad field value, a body too short, a
  duplicate id with a different body, an entry with uncommitted edits, a rebase in progress.
- 2 is a problem with the invocation or the configuration: an unknown command or option, a
  missing value, no store configured, a config file with an unknown key. The message names the
  problem and the help command to run.
- 3 comes from `search` when nothing visible matched; `preflight` exits 0 instead when at least
  one `always` rule was printed, so a session-start hook still delivers the rules.
- 5 means the commit exists locally but is not on the remote; the outcome line says why
  (`timed out after 20s`, `uncommitted changes (run memex save)`, `interrupted`, …). Nothing
  is lost; run `memex sync` once the cause is fixed.
- 6 means the local commit exists and a rebase onto the remote conflicted; the outcome line
  names the paths. Resolve by hand with git, then `memex sync`.
- 70 is a bug in memex, never a problem with your input. Report it with the message.
