# memex work ledger

One row per block of work. `scripts/next-block.sh` reads this file: at session start it tells
the agent which block is current and who owns it, and `scripts/next-block.sh done <id>` closes
a block and prints the handoff to the next one. Edit the table only through the script, or keep
its shape exactly: the status column is one of `todo`, `doing`, `done`.

Owners name the model that does the work: `fable` (Claude Fable, judgment-heavy slices and
reviews) or `opus` (Claude Opus, specified implementation). `review` names who checks the block
against SPEC.md before it is closed. The operator decides at every gate.

| id | block | owner | review | done when | status | done at |
|---|---|---|---|---|---|---|
| B0 | Scaffold: go.mod, cmd/memex, internal/spec table, jsonx writer, conformance runner skeleton, CI checks | opus | fable | `go build`, `go vet` and the runner pass with zero cases; `memex version` and `memex help` work | done | 2b37c54, reviewed fable |
| B1 | Store read path: §3 config, §4 layout, §5 entries, §6 links, §7 check (text and --json) | opus | fable | §3–§7 Implemented with conformance cases; `check` runs on a fixture store | done | 7822a9a, reviewed fable |
| B2 | Importer §13 against the pinned brain revision, with --leak | fable | opus | `memex check` on the output has zero errors; the report balances; the counts in the plan are reproduced | done | 81b86b3, reviewed opus+fable |
| B3 | Search and read commands: §8, search, list, show, preflight --no-sync | opus | fable | §8 and §11.3–11.6 Implemented; snippet and rounding cases green | done | 2210083, reviewed opus+fable |
| B4 | Replay harness and gate G1: 40 real queries plus known misses, scored against brainctl preflight | fable | operator | Report captured in the brain; operator decides whether retrieval is good enough to continue | done | e9b4bca, gate G1: operator chose A (2026-10-14) |
| B5 | Write path: §9, new/capture, set, save, sync, promote, fmt, init, hooks, preflight with sync | opus | fable | §9 and the rest of §11 Implemented; dirty-tree, timeout and hook-environment cases green | done | 70999f2, reviewed opus+fable |
| B6 | Help: §16 pages, help agents, help --json, executed examples; cold-agent test | fable | opus | A fresh session of a different model captures correctly with only `memex help agents` | done | f5be017, reviewed opus+fable; 3 cold rounds |
| B7 | Curation of the imported store: every rule carrying a `marker-*` tag reviewed in batches by a read-only Sonnet curator, operator-approved changes applied; Always tier given summaries | sonnet | operator | Marker backlog reviewed, at most 10 Always rules, preflight under 4,000 characters | done | 377a2b5 |
| B8 | Cutover: deployments hook, Nix package, pointer files, auto-memory fold-in, retire app .brain dirs | opus | fable | SessionStart runs `memex preflight`; old brain renamed; brainctl no longer on PATH | todo | |

Rules of the ledger:

- One block per session where possible. A block is closed only when its "done when" holds and
  the reviewer has signed off in the closing commit message.
- Closing a block: `scripts/next-block.sh done <id>`, then `brainctl capture --global` (until
  cutover) with what was learned.
- The next session starts with the model the next row names. The session-start hook says so;
  an agent that is not that model stops and asks the operator to switch.

Open items for later blocks:

- B6: capture's `--title`, `--summary` and `--body` are marked optional in `internal/spec`,
  because `capture --from PATH` makes them so. Decide how `help --json` expresses
  "required unless --from".
- B6: preflight's "search options" are taken to exclude `--json`, because SPEC defines no
  preflight JSON shape although §17.3 says every command that prints data accepts `--json`.
- B6: option summaries are Go code in `internal/spec`, so another implementation would have to
  copy them. Operator leans towards a language-neutral `help/interface.json` that
  `internal/spec` loads and the conformance suite compares with `help --json`.
- B2 pin (2026-10-13): primary brain ~/Programming/brain@1e3e921629461a714f415b0c1eb5db8466a21f65;
  secondaries ~/Programming/apps/mesorobots-page@8ccd0c59b94c4179b1e2b61497e99e4af8800f03 and
  ~/Programming/apps/arrowheadmind-page@0859f9057d58905ed1dab493b8fe6bc5f34de62b. Every importer run in B2 uses these.
- B2 facts for the real run: the pinned brain has 463 investigations and 480 INDEX rows (the
  plan's 457 and 474 were a week older), and no `.brain/dispatch-logs/` directory at all, so
  the dispatch-log attachment rule has no real input yet. Most path references in bodies are
  wrapped in single backticks; §13.5 now unwraps them when it rewrites.
- B4 replay (2026-10-13): `scripts/replay/` holds the harness, the 62-query set, the normalized
  results and the blind grades; `REPORT.md` there is the gate G1 evidence. Re-run it after B7
  (curation adds keywords and summaries) and after any change to §8: build memex, set
  `REPLAY_SCRATCH`, `MEMEX_STORE` (an import of the pinned sources) and `MEMEX_BIN`, run
  `run.py`, then `metrics.py` with `grades.json`. New entries in the top-10 start ungraded;
  `packets.py` builds blind packets for them. Spec candidates recorded there, to try with the
  harness before adopting: coverage bonus on k/n, b = 0.5, reverse prefix matching.
- B7 (2026-10-14): the original "under 200 active rules" target was dropped with the operator's
  agreement. Four batches reviewed 144 marker-tagged rules and closed 27, about one in five:
  a closure word in a body is usually the lesson's subject, and the real closures carry a date
  or name a tool that no longer exists. `scripts/replay/rerun.py` (memex-only, B4 grades)
  showed no retrieval loss after batches 1–3; every top-10 item lost was a closed entry graded
  "related". Summaries and keywords for governing rules continue through B8, measured the same
  way. Each batch is recorded as a journal entry in the store.
- B7 close (2026-10-14): six batches, 284 rules reviewed, 46 closed (37 retired, 9 superseded),
  5 rules extracted, 8 Always summaries written. Store at the close: 649 active rules, 8 Always,
  preflight 2,485 characters, 0 check errors. `scripts/replay/rerun-2026-10-14-final.md`: the
  only grade-2 result lost is the retired brainctl INDEX.md defect for two old-brain queries.
  Left for later: 121 kept rules still carry `marker-*` tags (strip in bulk when a batch form of
  `set` exists or history noise is acceptable); 636 active rules have no summary; the Always
  rules "Sub-task preflight" and "The brain is the memory" name brainctl and `.brain/` and
  need rewording at cutover; the `roaming` Always line restates the fleet-alerting rule.
  Lesson for the curators: Sonnet read accurately but proposed retire+extract far too often
  (18 of 35 rejected in the last batch); every closure needs a manager check against the entry
  and earlier decisions.
- B8: `doctor` (§11.13) and all of §12 (`check --workspace`, the leak scan, the tool-memory
  check) moved here from B5, because they need the fleet configuration to test against. §11
  is marked Implemented except 11.13; `doctor` still answers "not implemented yet". §12 stays
  Planned until then.
- B6: §10's help bullet (`memex COMMAND --help` prints the page and exits 0) holds only once
  the §16 pages exist; until then `--help` on a command without a page is the §16.2 usage
  error. §10 was marked Implemented in B5 on the strength of every other bullet.
- B6: the output lines of init, init --pointers, set, promote, fmt and save are now in
  §11.1 and §11.7–11.10 (decided after B5's questions); the help pages show them from there.

- B6 (2026-10-14): help pages, `help topics/--all/--json`, the §16.9 runner checks and the help
  fixture are in; §15, §16 and §17 are Implemented. The cold-agent rounds (§17.1) are prepared
  in `scripts/cold-agent/` but not run: the implementing fork could not spawn fresh agents. The
  manager runs two rounds and records them before B6 closes.
- B8: `doctor` and `check --workspace` re-enter `internal/spec`, get pages, and the `neutrality`
  topic grows the leak scan and the tool-memory check; the §17.4 rows that depend on §12 get
  their cases then.
- Future, not scheduled: a language-neutral `help/interface.json` generated from
  `internal/spec` for a second implementation; today `help --json` is that artifact at run time.

