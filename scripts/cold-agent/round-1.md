# Cold-agent round 1

- Date: 2026-10-14. Model: Claude Sonnet, fresh session. Pages at memex 3d3759e.
- Task: "rotate the backup key on alpha" (a runbook for it exists in the fixture).
- Prompt: `PROMPT.md` with the paths filled; plus one harness note that the agent's shell does
  not persist state between commands, so it prefixes each command with `source env.sh &&`.

## Commands run, in order

1. `cat $MEMEX_STORE/AGENT.md`
2. `memex help agents`
3. `memex preflight "rotate the backup key on alpha"` → `store: up to date`, one Always rule,
   the runbook at rank 1 with `matched 6/6`, the restic decision, a journal entry, two reference
   entries, a plan, a task, one hidden match.
4. `memex show runbooks/2026-01-04-rotate-the-backup-key-on-alpha.md`
5. `memex show journal/2026-01-10-first-key-rotation.md`
6. Wrote the no-op note to `~/rotate-note.txt`.
7. `memex capture --title="Rotate the backup key on alpha (dry run)" --summary=… --tag backup
   --keyword rotation --keyword restic --body - <<EOF … EOF`
   - First attempt: exit 1, `memex: git commit failed; nothing was changed: Author identity
     unknown … fatal: empty ident name`.
   - Retried with GIT_AUTHOR_* and GIT_COMMITTER_* set: `created journal/2026-10-14-rotate-the-
     backup-key-on-alpha-dry-run.md`, `pushed 742a905`, exit 0.

## Outcome

Preflight, reading, task, capture with a real title, summary, tags and keywords, `pushed`.
One hard stumble, three soft ones.

## Stumbles and what changed

| stumble | cause | change |
|---|---|---|
| git identity error on the first capture | the round's HOME had no `.gitconfig` (harness) | `setup.sh` writes one; `help agents` now says memex commits as you and git must know `user.name`/`user.email` |
| `matched 6/6` counted `the` and `on`, which made the label look generous | by design (no stop-word list) | `help agents` says short common words count too, so query with content words |
| did not know it could link the runbook and journal entry from the capture body | the `[[id]]` rule sat under "Editing and history", after the capture section | the capture bullet now says to link the entries used as `[[<id>]]` |
| `1 matches hidden by status` did not name the entry | by design (`--all` shows it) | none |

The agent also suggested the runbook should carry the journal entry's short-id insight; that is
curation of the fixture, not a page defect.
