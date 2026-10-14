# Cold-agent round 3

- Date: 2026-10-14. Model: Claude Sonnet, fresh session. Pages at memex 1483546 (after the
  review batch: exit codes, correct-vs-supersede, tag syntax, evidence, weak-results line).
- Task: "the restore test on alpha now takes 40 minutes instead of 10; find out why and record
  what you learn" (told to assume the snapshot grew by a new 30 GB directory). Designed to make
  the agent decide whether an existing entry should be corrected or superseded.

## Commands run, in order

1. `cat $MEMEX_STORE/AGENT.md`; `memex help agents`.
2. `memex preflight "restore test alpha slow backup snapshot duration"` (content words, as the
   page says) → one Always rule, 9 results, the journal and task about the restore test at
   `matched 5/7`, no weak line.
3. `memex show` on the journal, the task and the runbook.
4. `memex search "10 minutes restore duration"` → `matched 1/4` and the `weak results` line; the
   agent concluded correctly that no entry records the baseline.
5. Scratch note with the checks it would run.
6. `sed -i … runbooks/…md && memex save` from the wrong directory: sed failed, `nothing to
   commit`. Repeated from the store root: `pushed a2ed08e`. The runbook's step 2 now says to
   restore a small path with `--include`, with the reason.
7. `memex capture --title="Why the restore test on alpha went from 10 to 40 minutes" … --body -`
   with links to the journal and the runbook → `pushed 6fbffdb`.

## Outcome

The whole loop, plus a correct judgment: the current runbook was corrected in place and saved,
the immutable journal was left alone, nothing was superseded, and the new capture links both.
The weak-results line did its job on the baseline search.

## Stumbles and what changed

| stumble | cause | change |
|---|---|---|
| editing a printed path with sed from outside the store | paths are store-relative; the agent had `MEMEX_STORE` but did not think to use it | `help agents`: paths are relative to the store root; edit with `$MEMEX_STORE/<path>` |
| unsure whether editing the runbook was right | it was right; the page now distinguishes correcting a current entry from replacing one | none further |
| no baseline for "10 minutes" in the store | fixture content | none |

Three rounds completed the loop; the third needed no page change beyond one line. B6 closes.
