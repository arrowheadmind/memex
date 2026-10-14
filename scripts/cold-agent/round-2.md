# Cold-agent round 2

- Date: 2026-10-14. Model: Claude Sonnet, fresh session. Pages at memex 03a059e (after the
  round-1 changes).
- Task: "add a disk-usage alert for beta" (nothing in the fixture is about disk usage,
  monitoring or alerts; the hosts reference and a beta task are the only related entries).

## Commands run, in order

1. `cat $MEMEX_STORE/AGENT.md`
2. `memex help agents`
3. `memex preflight "add a disk-usage alert for beta"` → `store: up to date`, one Always rule,
   12 results, the best `matched 3/7` on `a`, `for`, `beta`; nothing about disk or alerts.
4. `memex show reference/2026-01-02-hosts.md`, `memex show tasks/2026-01-11-replace-rsync-on-beta.md`
5. Wrote the no-op note to `~/disk-alert-note.txt`.
6. `memex capture --title="Plan disk-usage alert for beta" --summary=… --tag monitoring
   --keyword disk --keyword alert --keyword beta --body - <<EOF … EOF` with three `[[id]]`
   links in the body → `created journal/2026-10-14-plan-disk-usage-alert-for-beta.md`,
   `pushed 5f3b352`, exit 0.

## Outcome

Preflight, two reads, task, capture with title, summary, tag, three keywords and three
resolving links, `pushed`. No hard stumble. The git identity, the link format and the
repeated flags all worked without guessing beyond what the pages say.

## Stumbles and what changed

| stumble | cause | change |
|---|---|---|
| 12 weak results with no signal that none was relevant | OR ranking fills the list; nothing announces a probable miss | spec §8.5: a `weak results: the best match covers k of n query items; try other words` trailer line (and `"weak"` in JSON) when the best result covers fewer than half the items; pages updated |
| saw the "content words" advice only after wording the query | it was in `help agents`, not in AGENT.md's preflight line | Appendix A / AGENT.md preflight bullet now says "in content words" and to read the k/n label and the weak line |
| unsure whether `--keyword` may repeat | the example showed it once | the capture example now shows it twice |

Both rounds completed the whole loop. A third round is run only if the review of the pages
changes them substantially.
