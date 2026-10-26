# memex work ledger

One row per block of work. `scripts/next-block.sh` reads this file: at session start it tells
the agent which block is current and who owns it, and `scripts/next-block.sh done <id>` closes
a block and prints the handoff to the next one. Edit the table only through the script, or keep
its shape exactly: the status column is one of `todo`, `doing`, `done`.

`owner` and `review` name a role, never a model or a tool: `implement` (specified work: a
spec section with its cases), `judge` (design slices and reviews) or `operator`. Which model or
tool fills a role is the operator's choice at the time, from what is available; it is recorded
outside this repository. The reviewer is always an independent session that did not do the
block's work, preferably a different model family than the one that did. Rows B0 to B9 predate
this and name the models that did them (`opus`, `fable`, `sonnet`). The operator decides at
every gate.

| id | block | owner | review | done when | status | done at |
|---|---|---|---|---|---|---|
| B0 | Scaffold: go.mod, cmd/memex, internal/spec table, jsonx writer, conformance runner skeleton, CI checks | opus | fable | `go build`, `go vet` and the runner pass with zero cases; `memex version` and `memex help` work | done | 4023dc0, reviewed fable |
| B1 | Store read path: §3 config, §4 layout, §5 entries, §6 links, §7 check (text and --json) | opus | fable | §3–§7 Implemented with conformance cases; `check` runs on a fixture store | done | 2a1aedb, reviewed fable |
| B2 | Importer §13 against the pinned brain revision, with --leak | fable | opus | `memex check` on the output has zero errors; the report balances; the counts in the plan are reproduced | done | 583c573, reviewed opus+fable |
| B3 | Search and read commands: §8, search, list, show, preflight --no-sync | opus | fable | §8 and §11.3–11.6 Implemented; snippet and rounding cases green | done | d3ad24f, reviewed opus+fable |
| B4 | Replay harness and gate G1: 40 real queries plus known misses, scored against brainctl preflight | fable | operator | Report captured in the brain; operator decides whether retrieval is good enough to continue | done | eb8f9a2, gate G1: operator chose A (2026-10-14) |
| B5 | Write path: §9, new/capture, set, save, sync, promote, fmt, init, hooks, preflight with sync | opus | fable | §9 and the rest of §11 Implemented; dirty-tree, timeout and hook-environment cases green | done | 37ce4f9, reviewed opus+fable |
| B6 | Help: §16 pages, help agents, help --json, executed examples; cold-agent test | fable | opus | A fresh session of a different model captures correctly with only `memex help agents` | done | 9f963ae, reviewed opus+fable; 3 cold rounds |
| B7 | Curation of the imported store: every rule carrying a `marker-*` tag reviewed in batches by a read-only Sonnet curator, operator-approved changes applied; Always tier given summaries | sonnet | operator | Marker backlog reviewed, at most 10 Always rules, preflight under 4,000 characters | done | 5186fbc |
| B8 | Cutover: deployments hook, Nix package, pointer files, auto-memory fold-in, retire app .brain dirs | opus | fable | SessionStart runs `memex preflight`; old brain renamed; brainctl no longer on PATH | done | c395ca3 |
| B9 | Batch form of `set`: one commit for many assignments | opus | fable | `memex set --batch` applies N lines in one §9.1 unit with conformance cases | done | bbd7487 |
| B10 | Summary currency: `memex update`, a status change that crosses visibility needs a new summary in `set`, `W-STALE-SUMMARY` in `check` (SPEC §7, §9.2, §11.7, §11.17) | implement | judge | The four Planned rows of the §0 table are Implemented with conformance cases (each refusal and both finding forms, a section added by hand, a status closed by hand, `--staged`, a shallow clone); help pages for `update`, `set` and `check`, `help agents` teaches `update` for status notes; §11.2's "To add to it" line names `memex update` | done | 8a0af7d |
| B11 | Refusal order for `new`, `capture` and `promote`: inputs read under the lock, the `capture --from` alias no-op before PATH is read, `promote`'s source checks in `set`'s order (SPEC §9.1, §11.2, §11.8) | implement | judge | The three Planned rows of the §0 table are Implemented with conformance cases 234–237 (each checked to fail on the 0.2.0 binary where it pins a change) and unit tests for the §9.1 phases with the lock held; the `new`, `capture` and `promote` pages say when `--body` is read and that a repeat `capture --from` of a removed file is the no-op; CHANGELOG Unreleased lists each change under Changed or Changed output | review | |

Rules of the ledger:

- One block per session where possible. A block is closed only when its "done when" holds and
  the reviewer has signed off in the closing commit message.
- Closing a block: `scripts/next-block.sh done <id>`, then `memex capture` with what was
  learned. The closing commit message names the review: who reviewed (role and the model or
  tool that filled it) and the verdict.
- The session-start hook names the current block's roles. A session working on a block says
  which role it fills; the review is never done by the session that did the work.

Open items for later blocks:

- B6: capture's `--title`, `--summary` and `--body` are marked optional in `internal/spec`,
  because `capture --from PATH` makes them so. Decide how `help --json` expresses
  "required unless --from".
- B6: preflight's "search options" are taken to exclude `--json`, because SPEC defines no
  preflight JSON shape although §17.3 says every command that prints data accepts `--json`.
- B6: option summaries are Go code in `internal/spec`, so another implementation would have to
  copy them. Operator leans towards a language-neutral `help/interface.json` that
  `internal/spec` loads and the conformance suite compares with `help --json`.
- B2 pin (2026-10-13): the importer source was pinned to one commit of the primary predecessor
  brain and one of each secondary repository (the exact pins are operator narrative, kept in the
  operator's store). Every importer run in B2 uses these.
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
  B0 00ccd9d→987a24e; B1 7822a9a→2a1aedb; B2 81b86b3→583c573; B3 2210083→d3ad24f; B4 gate f3ebc8e→07b030f; B5 70999f2→37ce4f9; B6 f5be017→9f963ae; B7 377a2b5→5186fbc; B8 fix round 58cb357→35ba766.
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
  already present, files deleted by the operator); on the first workstation steps 0, 3 (session
  form), 5, 7, 8, 9a done; the fleet change (hook, config file, package, brainctl removal,
  auto memory off) is deployments branch `memex-cutover` at 7f7f66e, evaluated for
  two workstations, not merged or deployed. Open for the B8 close: step 10 (GitHub
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
- B8 close (2026-10-16, manager): the photo/book migration session finished, so the cutover
  completed. Second delta import from brain@b6a6f01: 6 new (5 handover plans, 1 rule), 14
  changed; 9 changed only in the brain (plans, the alerting reference, the to-do index) took
  the brain's body plus 3 attachments, 5 changed only in the store (the reworded Always rules,
  the network reference) kept the store's. Brain renamed to brain-v3 with memex pointer files.
  Deployments `memex-cutover` rebased onto a moved main (clean), evaluated for all four
  workstations, merged by the operator and deployed to the first workstation: system memex only
  (user-profile copy removed), brainctl off PATH, managed drop-in `50-memex.json` (hook +
  auto memory off), `~/.config/memex/config.toml` written by activation, hook command exits 0
  with the Always rules, `doctor` all PASS, no failed units. Pending outside the block: the
  other hosts converge on their daily auto-upgrade or the next deploy; the deployments
  pre-commit hook still names `brainctl work start` in its message (it runs nothing from
  brainctl); brain-kit's `.brain/` is not imported; `capture --from` alias form. Found at the
  close: the store's Always rule forbids Co-Authored-By trailers and this repository's commits
  carried them; no further commit adds one (history left as is, it is already mirrored).
- post-B8 fix (2026-10-16): `capture --from` alias form, at e647f3f. A file path is recorded
  as an alias in one form (§11.2): expanded as in §3.1, made absolute and cleaned lexically,
  the home directory written `~/`; outside HOME it stays absolute. `capture --from` and the
  importer's `--leak` both write it, and `capture --from` compares PATH as given or, for an
  alias that is itself a file path, in that form, so `~/x`, `$HOME/x`, an absolute path and a
  relative path to one file are the same alias; delta import also matches an older absolute
  alias in that form (§13.7). Cases 215 (absolute, then `~/` and `$HOME`), 216 (`~/`, then
  absolute and an uncleaned path), 217 (outside HOME) and 218 (relative) added; 157 now
  expects `~/` aliases. All four fail on the previous binary.
- Second history rewrite (2026-10-16, operator decision): every Co-Authored-By trailer removed
  from the commit messages (168 lines over 182 commits; every tree unchanged), because the
  store's Always rule forbids them in any repository. Shas above are remapped to the new
  history; the previous map's left-hand sides are the original shas. Block closes before and
  after: B0 d8793e5→987a24e; B1 6dd3da5→2a1aedb; B2 3fc8827→583c573; B3 af1a646→d3ad24f; B4
  gate 1d2da16→07b030f; B5 fc60251→37ce4f9; B6 e327057→9f963ae; B7 82a4eed→5186fbc; B8 fix
  round 648d99e→35ba766; B8 close 198eed4→c395ca3. Backup mirror of the pre-strip history kept
  by the manager outside the repository.
- Trailer strip in the two other repositories (2026-10-16, operator decision, same rule): the
  deployments and brain histories lost every Co-Authored-By and Claude-Session line (157 and
  294 lines; every tree on every branch unchanged; branch protection lifted by the operator for
  the push). Shas this ledger names move with them: deployments 896b9ef→9dd792c,
  5cd405c→0aa8771; brain b627487→5c78630, b6a6f01→7c8bc33, 4544db4→e24b495, 59790e6→48d02e3,
  15e5f87→c4cc3c9. The store's journal entries quote the old shas and stay as written.
- Night curation and replay (2026-10-16/17): the operator ran four scripts against the store
  (121 `marker-*` tag strips, 18 captured entries triaged and 8 retitled, 4 rules extracted,
  637 rule summaries; every active rule now has one). Summaries sit in the keywords field at
  weight 8, so the replay reshuffled 59 of 62 top-10 lists and put 38 ungraded items in graded
  top 10s. Three Sonnet judges graded them blind (71% agreement with the implementer grades on
  252 controls); `rerun.py` now overlays `DATA/grades-additions-*.json` on both grade sets.
  Graded result: mean nDCG@10 0.764 before and after under the implementer grades, 0.803→0.791
  under Sonnet's; grade-2 top result 27→25; a grade-2 in the top 10 44→44; expected-id hits
  unchanged. The queries that fell are old-brain vocabulary (brain plan commits, the Pi rebuild
  script); `git commit` and `git commit backlog` rose. Reports and grades are attachments of
  the replay reference entry (`rerun-2026-10-17.md`, `rerun-2026-10-17-graded.md`).
- B9 batch `set` (2026-10-17): the night curation needed 637 separate `memex set` calls, each
  its own commit and push, 45 minutes. `memex set --batch FILE|-` reads one `ID ASSIGNMENT…`
  line per entry (shell-style quoting, blank and `#` lines skipped) and runs the whole batch as
  one §9.1 unit: every line resolved and computed first, the changed entries validated together,
  one commit `memex: set <n> entries` whose body is the single form's message per changed entry,
  one push. The first bad line refuses the batch as `memex: --batch line <n>: <single-form
  message>`, exit 1, also for what the single form calls a usage error; naming one entry twice
  is refused. SPEC §9.2 and §11.7, help (set page with an executed example, agents and writing
  topics), cases 219 and 220; 219 checks the commit body with `git rev-parse :/<regex>`, since
  `expect/commits` holds subjects only. Left out: `help --json` still marks `ID` and
  `ASSIGNMENT` required (§16.7 has no condition string for positionals); the subject says
  `1 entries` for one entry, as `fmt` does. Timing: 300 lines against a local copy of the
  store (no upstream) took 5.5 s; each line runs one `git status`, not profiled further.
- Post-cutover housekeeping (2026-10-17): the ten `W-SECRET` findings were all false positives
  (placeholders such as `?token=VALUE`, field names, a Python `password=None`, an SSH key
  fingerprint); eight entries carry `<!-- memex: allow-secret -->` now. The two left are in one
  journal entry, whose body cannot take the marker without breaking `body_sha256`: a spec gap,
  since W-SECRET's only suppression is a body line. Candidates: honour the marker in a
  front-matter field, or treat an all-caps word such as `VALUE` as a placeholder. All 58
  `W-RAW-OLD` journals reviewed by a Sonnet curator (read-only) and set `kept`; its one
  promotion (an empty-2xx-body JSON lesson) was kept as a journal, not fleet knowledge.
  brain-kit's `.brain/constraints.md` captured, its `.brain/` removed (brain-kit e7c6b34).
  Keywords added to three entries for replay misses; the "ssh access review" expected entry is
  an 18,000-word plan that length normalization keeps at rank 30, and the SSH access runbook at
  rank 1 is the better answer. Search candidates measured on branch `search-exp` (e42f23c):
  none adopted, §8.3 unchanged; report and grades are replay attachments.
- Fleet (2026-10-17): deployments 362512e pins memex 2b9e62a (B9's `set --batch`) and renames
  the sync check to `workspace-sync-check` (discovery skips linked worktrees); deployed to the
  three push targets, laptops on their daily converge. Open elsewhere: `check-brain-pointers.sh`
  reports 37 bare pointers on deployments main; two app repos still mention `brainctl work`.
- `allow_secret` (2026-10-17): closes the gap the housekeeping note found. A boolean frontmatter
  field, allowed on every type and placed beside `always`, suppresses `W-SECRET` like the body
  line does; it is how a journal entry, whose body cannot change, clears a false positive. The
  message names `memex set <id> allow_secret=true`. SPEC §5.3 and §7, help (entries and check
  codes topics), case 076's expected messages, new case 221.
- Fixes landed (2026-10-17, later): `allow_secret` applied to the one journal with false
  positives, the store has 0 `W-SECRET`. deployments 28c67d3 qualifies the 37 bare store
  pointers as `memex show <path>` (every target checked to resolve) and 6b86498 pins memex
  1c711be; deployed to the three push targets. Both page repos' `release.sh` describe the
  worktree and `fleet/images.nix` flow instead of `brainctl work`. Nothing is left open from
  the cutover.
- Release v0.1.0 (2026-10-17): the first tag. 77b801f sets `flake.nix` to 0.1.0 and adds
  `CHANGELOG.md`; the Nix build prints `memex 0.1.0 (store format 1)` and passes the 137
  conformance cases; the tag is on Gitea and the GitHub mirror. This commit sets the flake
  version back to `0.0.0-dev` (IMPLEMENTATION.md, "Packaging"). The fleet pins the release
  commit, not this one.
- Refusal reason (2026-10-21, branch `refusal-reason`, not merged): the store task of 2026-10-20
  asked that `refused: the result would not validate` name the failing rule and field. It
  already does: `validationFailed` (internal/commands/write.go, since B5) prints every error
  finding as `memex: <path>: <CODE> <message>` before the refusal line, for `new`, `capture`,
  `promote`, `set` (both forms), `save` and `fmt`. Reproduced on HEAD and on the installed
  0.1.0, against a scratch store and a clone of the real store, with a 501-character summary,
  a bad tag and both together: the finding line is there every time. The 2026-10-20 reports
  (two long summaries, one unsupported flag) could not be reproduced; the store holds no
  transcript, and an unsupported flag is a usage error (exit 2) that names the option. Done
  instead: case 222 (`new` and `capture` with a 501-character summary: exit 1, the
  E-FIELD-VALUE line, the refusal line, store unchanged; checked to fail on a binary without
  the finding loop and on a wrong expectation), SPEC §9.1 step 5 states the format and §11.2
  and §11.7 point to it, the `new`, `capture` and writing pages say it, CHANGELOG Unreleased.
  Decision: every error finding stays printed (§11.7 already says so for `--batch`; one line
  per entry would be a "Changed output" that says less). If it recurs: record the exact
  command, `memex version` and the full standard error before filing.
- B10 summary currency (spec written 2026-10-25, operator request): in the operator's store,
  14 of 27 open tasks and active plans had a summary older than their newest status section,
  and a backlog list built from summaries passed finished work off as open twice in one day.
  82 earlier closes were `set status=done` without a summary change. Design: memex cannot write
  a summary, so it refuses or reports when one was not touched. `update` appends a dated
  section and replaces the summary in one commit; `set` refuses a close or reopen of a task or
  plan without a new summary; `check` reads first-parent history (and the working tree or the
  index) to find a section added, or a visibility change, after the summary's last change.
  Expect the first full `check` after B10 to warn on most of those 82 closed entries: they are
  genuinely stale; clean them up with one `memex set --batch` pass, reviewed by the operator.
  For the implementer: history reading must stay fast on a store of about 1,000 entries (one
  `git log --name-only` pass, not one `git log` per entry; record the time in this note); a
  shallow clone treats its oldest commit as the creation.
- B10 implemented (2026-10-25): history is one `git log --first-parent --raw -z` over `tasks/`
  and `plans/` plus one `git cat-file --batch` for the blobs it names. Measured on a scratch
  copy of the operator's store (2,104 entries, 536 tasks and plans, 1,840 commits, 1,778 file
  versions read): the history read alone takes 0.14 s, and a full `memex check` 1.57 s against
  1.35 s before B10 (+0.22 s, history plus parsing every version). The write path's validation
  reads no history. That copy gets 182 `W-STALE-SUMMARY` findings, all checked to be genuinely
  stale: 105 plans and 77 tasks; 163 for a close (`active`→`done` 97, `open`→`done` 48,
  `in-progress`→`done` 9, others 9) and 19 for a section added by hand (11 of them on entries
  still open, blocked, in progress or draft). The cleanup is one reviewed `memex set --batch`
  pass. Spec decisions made while implementing: history follows a path without renames and
  ends at an add or a delete; an unreadable version is skipped; heading lines compare exactly;
  removing the summary does not satisfy the close rule; an empty `--summary` is a usage error;
  the `exists` refusal is type-aware; `expect/git-exit` may be 0 so that a case can check what
  the pre-commit hook prints.
- B10 fix round (2026-10-25, after an independent review returned "request changes"): code
  fixes for `update --status=` (now a usage error), `update` reading `--body` before resolving
  its ID, deleted or unparseable dirty entries resolving to `no entry` in `set` and `update`,
  and `check --staged` skipping type changes (`T`). Spec decisions: one refusal order for `set`
  and `update` (§11.7); the visibility rule looks at final values; a version with an invalid
  summary is left out; the named heading is the first extra occurrence; a history read failure
  is `check` exit 2 and `doctor` FAIL, with git's first error line as the reason; a CR in
  `--heading` is a usage error. The runner gained `expect/stderr` (exact) and `<SHA>`. Cases
  229-232. The store copy still gets 182 `W-STALE-SUMMARY` findings, the same set as before.
  Left as is: `new`, `capture` and `promote` still read `--body` before the lock (§11.2 does
  not order it), unlike `update` now.
- B10 second fix round (2026-10-25, re-review "approve with fixes"): an unborn `HEAD` is "no
  commits", any other `HEAD` that does not resolve is the §7 history failure (case 233); case
  229 pins `update`'s value order (a short `--body` before an unknown `--status`) and targets
  turned into a symbolic link or a directory. Every `stderr-contains` in the suite (119 files,
  48 cases) is now an exact `expect/stderr`; none stays a substring check. Two of them hold
  text that is not memex's own: 170 git's index-lock message, 171 the pre-commit hook's
  output, both deterministic in the runner's sandbox. The `stderr-contains` form stays in §15
  for a case whose standard error carries text that varies.
- B11 refusal order (spec drafted 2026-10-25, judged in three rounds; the operator's go
  2026-10-26, with the four open questions decided as drafted: a repeat `capture --from` of a
  removed file omits `remove <PATH> after review`; `promote` refuses a deleted, unparseable or
  replaced source as `set` does; holding the lock while standard input is read is accepted;
  the unfixable warnings on hash-locked journals are a separate block, B12, not part of
  B11): closes the gap the B10
  fix round left open. `new`, `capture` and `promote` read `--body` (and `capture --from` its
  file) and check the content measure before §9.1 step 1, while §9.1 orders the lock first
  and `update` reads `--body` only after resolving its id. Seen on the 0.2.0 binary: a short
  body against a store that does not exist exits 1 with the content-measure message, where
  `update` exits 2 with the store message; a short body against a store with no commits gives
  the body message, not `store has no commits yet`; a store with an unparseable entry prints
  no `skipped` line before the body refusal. Two smaller faults found while reading: a repeat
  `capture --from PATH` after the file was removed, as the first run's `remove <PATH> after
  review` told, is the usage error `cannot read --from` instead of the §11.2 no-op, because
  the file is read before the alias check; `promote` checks a rewritten source's fields before
  its uncommitted changes (the reverse of §11.7 step 4), its `promote cannot keep` message was
  in no case or spec text, and a deleted or unparseable source is `no entry` where `set`
  resolves it since B10. Decisions written into SPEC.md, for review: §9.1's four phases, the
  command line, then the store checks that need no lock (format, `memex.toml` in `HEAD`, as
  §11.7 step 2 already orders them), then the lock and freshen, then every other input
  (`--body` FILE or `-`, `set --batch` FILE or `-`, the `--from` file), so `set --batch` reads
  its FILE under the lock too (§11.7 step 1 and the batch paragraph say so, Planned); the
  lock is held while standard input is read; step 1 of §11.2 and §11.8 has a total order (the
  arguments left to right, `TYPE` or `SOURCE-ID` and `--to` or the `--from`/`--body`
  conflict, the missing `--title`/`--summary`/`--body`, the values that need no store), which
  is the order the 0.2.0 parser already uses: `cmd/memex/main.go` takes the global options
  before the command first, `parseArgs` (`cmd/memex/args.go`) then parses the command's
  arguments left to right and checks the positional count last, and `parseCommand`
  (`main.go`) detects a `--store` given before and after the command only after that parse;
  `set --batch` reads the store's entries (their `skipped` lines) before FILE; a journal
  already `promoted` is only cited, never rewritten or checked, as `promote.go` does (the
  §11.8 behavior list said otherwise and is corrected); `<problems>` is defined in §11.7 step 4 as the
  code builds it (the §8.1 problem, or the `E-FIELD-UNKNOWN`/`E-FIELD-VALUE` messages in byte
  order of field name joined by `; `); the alias no-op is decided before PATH is read and
  drops the `remove` line when PATH no longer exists; `promote`'s source is checked between
  the store and the values, only when it is rewritten or its file cannot be read as an
  entry. The judges' reviews (2026-10-26, two rounds, request changes) found the lock boundary
  contradictory, step 1 unordered, `<problems>` unspecified and the cases short; this note
  and the cases below are the fixed draft.
  Cases to add (numbers are the next free ones; "fails on 0.2.0" means it must be shown to):
  - 234-new-refusal-order. 01 `--store <HOME>/nostore` with a short body on stdin: exit 2,
    the §3.1 store message, not the content-measure refusal (fails on 0.2.0). 02 git step
    removes `memex.toml` from `HEAD` (`rm --cached`, `commit --no-verify`), short body: exit 1,
    `store has no commits yet; run memex save` (fails on 0.2.0); git step restores it. 03
    overlay adds an unparseable plan, short body: the `memex: skipped …` line, then the
    content-measure refusal (fails on 0.2.0). 04 unreadable `--body` file and an unknown
    `--parent`: usage error with usage and `Run:` lines (body before parent). 05 short body
    and unknown `--parent`: the content measure. 06 good body, unknown `--parent`, title of an
    existing entry with another body: `no entry "<value>"` with candidates (values before the
    id). 07 the existing title with a 501-character summary: the `exists` refusal, not the
    E-FIELD-VALUE finding (id before validation). 08 `--body <HOME>/missing.md` (no such file)
    against `<HOME>/nostore`: the store message, not `cannot read --body` (fails on 0.2.0;
    this, not standard input, is what shows that no input is read before the store). 09 a
    command-line failure with a bad store: `new rule --title=T --body - --store
    <HOME>/nostore` (no `--summary`): `missing --summary` with usage and `Run:` lines, not the
    store message (command line before store). 10 `new rule --status=bogus --body -` with
    neither `--title` nor `--summary`: `missing --title` (missing options before values). 11
    `new` with no `TYPE` and no `--title`: `missing TYPE`. 12 `new rule
    --verified=2026-13-01` with a good title, summary and body: the option not allowed on a
    rule, not the malformed date (not-allowed before form). 13 `new bogus --body -` (no
    `--title`): the unknown TYPE, not `missing --title`. 14 argv `--store <STORE> new rule
    --store <STORE> --bogus` (the runner supplies the executable): the unknown option `--bogus`, not `option --store given more
    than once` (the cross-position `--store` check follows the command's parse).
    `expect/store` unchanged after every step, `expect/stderr` exact.
  - 235-capture-from-refusal-order. 01 missing `--from` file against `<HOME>/nostore`: exit 2,
    the store message, not `cannot read --from` (fails on 0.2.0). 02 seeded entry whose alias
    is `~/old/notes.md`, no such file in `home/`: `capture --from <HOME>/old/notes.md` prints
    `exists <path>` and `nothing to commit`, no `remove` line, exit 0 (fails on 0.2.0). 03 the
    same with the file present in `home/`: `exists`, `remove … after review`, `nothing to
    commit` (unchanged behavior, pins the line's condition). 04 unreadable `--from` with a
    store: usage error with usage and `Run:` lines. 05 `capture --from <HOME>/x.md --body -`
    against `<HOME>/nostore`: the `--from and --body cannot be combined` usage error, not the
    store message.
  - 236-promote-refusal-order. 01 nostore and a short body: exit 2, store message (fails on
    0.2.0). 02 unknown SOURCE-ID and a short body: `no entry` (fails on 0.2.0). 03 raw journal
    source with an uncommitted frontmatter edit that also adds an unknown field: `has
    uncommitted changes; run memex save first` (fails on 0.2.0: fields first). 04 the same
    edit committed by a git step, with two unknown fields, `zeta` written above
    `aardvark` (so byte order differs from file order): `has fields that promote cannot keep
    (<problem of aardvark>; <problem of zeta>); edit the file, then memex save`, exact. 05
    source deleted from the working tree (`git rm`): uncommitted changes, not `no entry`
    (fails on 0.2.0). 06 a rule source with an uncommitted edit and a good body: created, the
    rule untouched (step 4 only for a rewritten source). 07 repeat of a successful promote:
    `exists <path>`, `nothing to commit`, exit 0. 08 the same repeat with a 501-character
    `--summary`: still the `exists` no-op, not the E-FIELD-VALUE finding (step 6 before step
    7). 09 the same repeat with an unknown `--source`: `no entry` (step 5 before step 6). 10 a
    raw journal source made unparseable by an overlay (no opening `+++`): the `skipped` line,
    then uncommitted changes (fails on 0.2.0: `no entry`). 11 that file committed by a git step
    (`commit --no-verify`): the `skipped` line, then `promote cannot keep (<§8.1 problem>)`
    (fails on 0.2.0). 12 a raw journal source replaced by a symbolic link (an overlay that
    lists it in `.remove` and carries the link, as 229 step 12 does): uncommitted changes
    (fails on 0.2.0). 13 one replaced by a directory, as 229 step 13: uncommitted changes
    (fails on 0.2.0). 14 the deleted source of 05 named by its
    store-relative path `journal/<id>.md`: uncommitted changes (fails on 0.2.0). 15 the same
    source named by an alias it had: `no entry`, because an alias is read from the file. 16
    `promote` with no `SOURCE-ID` and no `--to`: `missing SOURCE-ID`. 17 a SOURCE-ID and
    neither `--to` nor `--title`: `missing --to`. 18 a SOURCE-ID, `--to bogus` and no
    `--title`: the unknown `--to` type, not `missing --title`. 19 a journal source already
    `promoted`, with an uncommitted edit (overlay) that adds an unknown field and keeps
    `status = "promoted"`: created, one commit holding only the new entry, the source untouched
    and still uncommitted (it is only cited, so neither checked nor validated).
  - Unit tests beside `preflight_lock_test.go` (the runner cannot hold a lock): with the lock
    held by another process, `new`, `capture` and `promote` with an unreadable `--body`, and
    `capture --from` with a missing file, wait and exit 4 (a short wait in the test), not the
    usage error; `new --body -` and `set --batch -` with the lock held leave standard input
    unread (a pipe whose writer is still open, so a read would block); `set --batch` with a
    missing FILE and the lock held exits 4.
  - 237-set-batch-file-order. 01 an overlay adds an unrelated unparseable plan; `set --batch
    <HOME>/missing.txt`: the `memex: skipped …` line, then the usage error `cannot read
    --batch <HOME>/missing.txt: no such file or directory` with usage and `Run:` lines, exit
    2 (fails on 0.2.0, which reads FILE first and prints no `skipped` line).
