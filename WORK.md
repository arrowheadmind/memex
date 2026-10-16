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
| B0 | Scaffold: go.mod, cmd/memex, internal/spec table, jsonx writer, conformance runner skeleton, CI checks | opus | fable | `go build`, `go vet` and the runner pass with zero cases; `memex version` and `memex help` work | done | 4023dc0, reviewed fable |
| B1 | Store read path: §3 config, §4 layout, §5 entries, §6 links, §7 check (text and --json) | opus | fable | §3–§7 Implemented with conformance cases; `check` runs on a fixture store | done | 6dd3da5, reviewed fable |
| B2 | Importer §13 against the pinned brain revision, with --leak | fable | opus | `memex check` on the output has zero errors; the report balances; the counts in the plan are reproduced | done | 3fc8827, reviewed opus+fable |
| B3 | Search and read commands: §8, search, list, show, preflight --no-sync | opus | fable | §8 and §11.3–11.6 Implemented; snippet and rounding cases green | done | af1a646, reviewed opus+fable |
| B4 | Replay harness and gate G1: 40 real queries plus known misses, scored against brainctl preflight | fable | operator | Report captured in the brain; operator decides whether retrieval is good enough to continue | done | eb8f9a2, gate G1: operator chose A (2026-10-14) |
| B5 | Write path: §9, new/capture, set, save, sync, promote, fmt, init, hooks, preflight with sync | opus | fable | §9 and the rest of §11 Implemented; dirty-tree, timeout and hook-environment cases green | done | fc60251, reviewed opus+fable |
| B6 | Help: §16 pages, help agents, help --json, executed examples; cold-agent test | fable | opus | A fresh session of a different model captures correctly with only `memex help agents` | done | e327057, reviewed opus+fable; 3 cold rounds |
| B7 | Curation of the imported store: every rule carrying a `marker-*` tag reviewed in batches by a read-only Sonnet curator, operator-approved changes applied; Always tier given summaries | sonnet | operator | Marker backlog reviewed, at most 10 Always rules, preflight under 4,000 characters | done | 82a4eed |
| B8 | Cutover: deployments hook, Nix package, pointer files, auto-memory fold-in, retire app .brain dirs | opus | fable | SessionStart runs `memex preflight`; old brain renamed; brainctl no longer on PATH | done | 198eed4 |

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
- B4 replay (2026-10-13): `scripts/replay/` holds the harness; since B8 its data (the 62-query
  set, normalized results, blind grades, `REPORT.md`, the gate G1 evidence, and the B7 reruns)
  lives in the store as the attachments of the reference entry
  `2026-10-14-memex-retrieval-replay-queries-blind-grades-and-results-gate-g1-evidence`, and
  every script takes that directory as `--data DIR` (default `$REPLAY_DATA`);
  `scripts/replay/README.md` says how. Re-run it after any change to §8: build memex, set
  `MEMEX_STORE` and `REPLAY_DATA`, run `rerun.py` (memex only; `run.py` also needs brainctl and
  `--brain`). New entries in the top-10 start ungraded; `packets.py` builds blind packets for
  them. Spec candidates recorded in the data's `REPORT.md`, to try with the harness before
  adopting: coverage bonus on k/n, b = 0.5, reverse prefix matching.
- B7 (2026-10-14): the original "under 200 active rules" target was dropped with the operator's
  agreement. Four batches reviewed 144 marker-tagged rules and closed 27, about one in five:
  a closure word in a body is usually the lesson's subject, and the real closures carry a date
  or name a tool that no longer exists. `scripts/replay/rerun.py` (memex-only, B4 grades)
  showed no retrieval loss after batches 1–3; every top-10 item lost was a closed entry graded
  "related". Summaries and keywords for governing rules continue through B8, measured the same
  way. Each batch is recorded as a journal entry in the store.
- B7 close (2026-10-14): six batches, 284 rules reviewed, 46 closed (37 retired, 9 superseded),
  5 rules extracted, 8 Always summaries written. Store at the close: 649 active rules, 8 Always,
  preflight 2,485 characters, 0 check errors. `rerun-2026-10-14-final.md` (replay data, now in the store): the
  only grade-2 result lost is the retired brainctl INDEX.md defect for two old-brain queries.
  Left for later: 121 kept rules still carry `marker-*` tags (strip in bulk when a batch form of
  `set` exists or history noise is acceptable); 636 active rules have no summary; the Always
  rules "Sub-task preflight" and "The brain is the memory" name brainctl and `.brain/` and
  need rewording at cutover; the `roaming` Always line restates the fleet-alerting rule.
  Lesson for the curators: Sonnet read accurately but proposed retire+extract far too often
  (18 of 35 rejected in the last batch); every closure needs a manager check against the entry
  and earlier decisions.
- History rewritten 2026-10-14 (operator decision, before the GitHub mirror): the runner binary
  committed by accident, the replay data files (now attachments of a reference entry in the
  store) and the stray `.brain/` directory were removed from every commit, and two host names
  in an old test string replaced. Every sha changed; the ledger's "done at" column was remapped
  from the filter-repo commit map. Shas cited in brain captures and the plan are the old ones:
  B0 00ccd9d→d8793e5; B1 7822a9a→6dd3da5; B2 81b86b3→3fc8827; B3 2210083→af1a646; B4 gate f3ebc8e→1d2da16; B5 70999f2→fc60251; B6 f5be017→e327057; B7 377a2b5→82a4eed; B8 fix round 58cb357→648d99e.
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

- B8 (2026-10-14): `doctor` (§11.13) and all of §12 are in; §11 and §12 are Implemented.
  Decisions written into SPEC.md, for review: every doctor message and result (PASS/WARN/FAIL)
  is now in the §11.13 table, `hooks` is WARN, and PASS messages carry no host-specific text
  except paths that §15 placeholders cover (the git version and the config path are left out
  so that cases and help examples stay exact); the store-dependent checks are omitted, not
  printed, when `store` or `format` fails, and the git ones also when `git` fails. §12: depth
  counts visited directories (workspace = 0, down to 4); a pointer-file name that is a symlink
  is `E-POINTER` (and `init --pointers --force` now replaces a symlink instead of writing
  through it); `~/.claude/CLAUDE.md`, `~/.codex/AGENTS.md` and `~/.gemini/GEMINI.md` holding
  exactly the pointer are that tool's global pointer, not a leak; finding paths are absolute;
  `check --workspace` combines with `--staged`; the E-POINTER, W-NO-POINTER and W-LEAK
  message templates are in §7. Settings files of rule 5 are read through symlinks (NixOS
  installs `/etc/claude-code/managed-settings.d/*.json` as links). The tool-memory WARN has a
  unit test rather than a case, because the runner cannot hide the host's `/etc/claude-code`.
- B8: delta import is `memex import brain-v3 SRC… --into DIR` (§13.7). Decisions: identity is
  the alias set (an id-shift-proof key; the id alone is not, because later revisions shift
  `-2` suffixes and re-date the attachments entry), then id+type+body; a new entry whose id the
  store holds gets the next free `-N`; present entries are compared by body only (curation
  owns frontmatter) and never written, `changed` ones are listed; links, `superseded_by`,
  `parent`, `sources` and `files/<id>/` prefixes of new entries are mapped to store ids; new
  attachment files are copied only where no file exists; the store must be clean (one diff to
  review); the fresh import runs outside the lock, the merge under it; the report is
  `import-report-<today>[-n].json` = the §13.6 object plus `delta`; §4 ignores those names. An
  entry deleted from the store comes back on the next delta (nothing remembers it).
- B8 real-data proof (2026-10-14), on a scratch clone of the store with its origin removed and
  scratch clones of the sources (HOME pointed at the scratch tree so the `~/…` prefixes match):
  delta at the pinned revisions adds 0 and finds 1664 unchanged, 0 changed (B7 curation
  touched only frontmatter); delta at brain@13b4b199 (apps unchanged) adds 21 (plans 12,
  journal 8, tasks 1), 1655 unchanged, 9 changed (left alone: 7 plans updated in the brain on
  10-12/10-13, the operator to-do index and one backlog item), 1 moved (the attachments entry,
  re-dated), 0 attachments, 0 aliases dropped; `check` on the result: 0 errors, 2467 warnings.
  The cutover's step 1 reruns this on the real store.
- B8: `flake.nix` builds memex with `go_1_27`, `vendorHash` pinned, nixpkgs pinned in
  `flake.lock`; the built binary passes all conformance cases. A flake cannot read git tags, so
  the version is a line in `flake.nix` (`0.0.0-dev`) that the release commit sets.
- B8: `docs/cutover.md` is the operator's runbook (not run). It names no machine or path; the
  operator fills shell variables. It drops the suggested `--store "$(git rev-parse
  --show-toplevel)"` hook form: that would make the session's repository the store, which
  Principle 6 forbids; the hook is `memex preflight --max-chars 4000` with the store from the
  config file or `MEMEX_STORE`.
- B8 cutover progress (2026-10-14, manager): store-side steps 1, 2, 6 done (delta import from
  brain@b627487: 21 new, 9 updated; Always rules and AGENT.md reworded; 18 leaks captured or
  already present, files deleted by the operator); on thinkcentre-02 steps 0, 3 (session
  form), 5, 7, 8, 9a done; the fleet change (hook, config file, package, brainctl removal,
  auto memory off) is deployments branch `memex-cutover` at 7f7f66e, evaluated for
  thinkcentre-02 and thinkpad, not merged or deployed. Open for the B8 close: step 10 (GitHub
  mirror) and the deploy; `capture --from` should store the alias in the `~/` form when the
  file is under HOME (the importer did; today `show` resolves only the form stored).
- B8 findings for the manager: (1) a 3.7 MB conformance-runner binary was committed at the repo
  root in fea9ec4; removed from the tree in B8, still in history. (2) The repository is to be
  mirrored to GitHub, but WORK.md, `scripts/replay/` (queries, grades and results built from
  the real store) and `scripts/cold-agent/` carry operator names, hosts and paths; Principle 11
  covers the tool, spec, help and cases, and the mirror would publish the rest. The runbook's
  step 10 says to audit and rewrite history before the mirror exists. (3) The real
  `doctor` run finds 18 leak files; 14 were imported by the first import's `--leak`.
- B8 fix round (2026-10-14), after the review: doctor sets `GIT_OPTIONAL_LOCKS=0` and writes
  nothing (checked: the index mtime is unchanged where a plain `git status` refreshes it);
  `git version` runs in `/`; a store root that is not the top of its own repository is one
  `upstream` WARN, `<root> is not a git repository; run memex init <root>`, with fetch-age,
  clean and hooks omitted (decided: a store nested in another repository's tree counts too,
  since its git checks would describe the other repository); an unreadable `memex.toml` is
  `store <root>: <reason>`. The workspace must be listable and not inside the store. The store
  path is cleaned lexically in §3.1 (so `~/../store` now equals the store: case 151 steps 03–04
  became `unchanged`, case 013 names the cleaned path), pointers compare cleaned paths and the
  pointer text drops a trailing `/`. `--into ""`/`--out ""` are usage errors. `capture --from`
  of a path that is already an alias (as given, absolute, or `~/…`) is the no-op `exists` +
  `nothing to commit`. Cases 211–214 added. The replay data moved to the store (manager); the
  scripts take `--data`.
