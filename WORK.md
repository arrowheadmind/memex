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
| B1 | Store read path: §3 config, §4 layout, §5 entries, §6 links, §7 check (text and --json) | opus | fable | §3–§7 Implemented with conformance cases; `check` runs on a fixture store | doing |  |
| B2 | Importer §13 against the pinned brain revision, with --leak | fable | opus | `memex check` on the output has zero errors; the report balances; the counts in the plan are reproduced | todo | |
| B3 | Search and read commands: §8, search, list, show, preflight --no-sync | opus | fable | §8 and §11.3–11.6 Implemented; snippet and rounding cases green | todo | |
| B4 | Replay harness and gate G1: 40 real queries plus known misses, scored against brainctl preflight | fable | operator | Report captured in the brain; operator decides whether retrieval is good enough to continue | todo | |
| B5 | Write path: §9, new/capture, set, save, sync, promote, fmt, init, hooks, preflight with sync | opus | fable | §9 and the rest of §11 Implemented; dirty-tree, timeout and hook-environment cases green | todo | |
| B6 | Help: §16 pages, help agents, help --json, executed examples; cold-agent test | fable | opus | A fresh session of a different model captures correctly with only `memex help agents` | todo | |
| B7 | Curation of the imported store in batches of 30 from `list --tag marker-stale`; Always tier last | opus | operator | Under 200 active rules, at most 10 Always rules, preflight under 4,000 characters | todo | |
| B8 | Cutover: deployments hook, Nix package, pointer files, auto-memory fold-in, retire app .brain dirs | opus | fable | SessionStart runs `memex preflight`; old brain renamed; brainctl no longer on PATH | todo | |

Rules of the ledger:

- One block per session where possible. A block is closed only when its "done when" holds and
  the reviewer has signed off in the closing commit message.
- Closing a block: `scripts/next-block.sh done <id>`, then `brainctl capture --global` (until
  cutover) with what was learned.
- The next session starts with the model the next row names. The session-start hook says so;
  an agent that is not that model stops and asks the operator to switch.

Open items for later blocks:

- B5: the slug algorithm and length cap (§5.4) and body normalization (§5.1) have unit tests
  only; their conformance cases arrive with `new`, the first command that exposes them.

- B6: capture's `--title`, `--summary` and `--body` are marked optional in `internal/spec`,
  because `capture --from PATH` makes them so. Decide how `help --json` expresses
  "required unless --from".
- B6: preflight's "search options" are taken to exclude `--json`, because SPEC defines no
  preflight JSON shape although §17.3 says every command that prints data accepts `--json`.
- B6: option summaries are Go code in `internal/spec`, so another implementation would have to
  copy them. Operator leans towards a language-neutral `help/interface.json` that
  `internal/spec` loads and the conformance suite compares with `help --json`.
