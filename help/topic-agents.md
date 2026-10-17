# memex help agents

Rules of engagement for a language-model agent, as imperatives.
## Before work

- Run `memex preflight "<the task in your own words>"`. It syncs the store, prints the
  `## Always` rules, then a ranked search. Read the `## Always` section every time.
- Read the `matched k/n` label on each result before trusting it: `matched 1/4` means one of
  your four words matched, not that the entry is about your task. Short common words (`the`,
  `on`) count too, so query with content words. `weak results: …` after the results means even
  the top hit matched under half your words: assume nothing relevant exists, or retry with others.
- Open an entry with `memex show <path>`. Paths are relative to the store root; to edit one
  with other tools use `$MEMEX_STORE/<path>` (the root is also the `Store root:` line).
- In a session-start hook with no task text, run `memex preflight --max-chars 4000`.

## Searching

- Words are OR-combined and ranked. Exit 3 means nothing visible matched; the output says why.
- A word of 4 or more letters matches prefixes (`deploy` finds `deployments`); a shorter word
  matches whole words only.
- Quote a phrase so the shell keeps the quotes: `memex search '"dirty tree"'`.
- `N matches hidden by status (use --all)` means retired or done entries matched; add `--all`.
- Filter with `--type`/`--tag`/`--repo`/`--host`; `tags` are for filtering, `keywords` for finding.

## After work

- Capture what you learned in one shell call, body on standard input:

  ```
  memex capture --title="<what you did>" --summary="<one sentence: should the reader open this?>" \
    --tag <topic> --keyword restic --keyword rotation --body - <<'EOF'
  ## What was attempted
  ...
  ## Reusable insights
  ...
  EOF
  ```

- `--summary` is required and `--body` must hold real content (80 characters of text, not just
  headings). One entry per lesson; record evidence in the body (commands, commit pins, measured
  values). Put the reader's likely words in `--keyword`, one per option. Tags are lowercase
  `a-z`, `0-9` and hyphens (`--tag backup-keys`).
- memex commits as you: without git `user.name` and `user.email`, a capture is refused.
- The last line is the outcome: `pushed <sha>`, `committed <sha>, no upstream`,
  `nothing to commit`, or `committed <sha>, NOT pushed: <reason>`. Branch on the exit code:
  0 success, 1 refused and nothing changed, 2 usage error (run the printed `Run:` line), 4 lock
  timeout (retry), 5 committed but not pushed, 6 rebase conflict; the last line says why.
- The same capture twice is a no-op (`exists <path>`, `nothing to commit`); a different body
  under the same title on the same day is refused: read the existing entry or pick a new title.

## Editing and history

- To correct a current entry, edit its file directly, then run `memex save`.
  `memex set <id> field=value` changes frontmatter; for many entries, `memex set --batch FILE`.
- To replace a rule or decision, supersede it: create the new entry, then
  `memex set <old> status=superseded superseded_by=<new>`. Do not rewrite it into another one.
- Journal bodies are immutable: capture the correction, then supersede the old journal likewise.
- Link entries as `[[<id>]]` (the filename without `.md`), not prose paths; `check` flags breaks.
- Never write knowledge into a coding tool's own memory or rules files (`~/.claude/…`,
  `.cursorrules`, …). Knowledge lives in the store only.
