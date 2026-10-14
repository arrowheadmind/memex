# memex help agents

Rules of engagement for a language-model agent. Written as imperatives; each one exists
because an agent got it wrong before.

## Before work

- Run `memex preflight "<the task in your own words>"`. It syncs the store, prints the
  `## Always` rules, then a ranked search. Read the `## Always` section every time.
- Read the `matched k/n` label on each result before trusting it: `matched 1/4` means one of
  your four words matched, not that the entry is about your task.
- Open an entry with `memex show <path>`; paths printed by memex can be pasted back as given.
- In a session-start hook with no task text, run `memex preflight --max-chars 4000`.

## Searching

- Words are OR-combined and ranked; a miss prints `No entries matched any term of: …`, never an
  empty success. Exit 3 means nothing visible matched.
- A word of 4 or more letters matches prefixes (`deploy` finds `deployments`); a shorter word
  matches whole words only.
- Quote a phrase so the shell keeps the quotes: `memex search '"dirty tree"'`.
- `N matches hidden by status (use --all)` means retired or done entries matched; add `--all`.
- Filter with `--type`, `--tag`, `--repo`, `--host`; `tags` are for filtering, `keywords` are
  for finding.

## After work

- Capture what you learned in one shell call, body on standard input:

  ```
  memex capture --title="<what you did>" --summary="<one sentence: should the reader open this?>" \
    --tag <topic> --keyword <word the next reader will type> --body - <<'EOF'
  ## What was attempted
  ...
  ## Reusable insights
  ...
  EOF
  ```

- `--summary` is required and `--body` must hold real content (80 characters of text, not just
  headings). Put the reader's likely words in `--keyword`.
- The last line is the outcome: `pushed <sha>`, `committed <sha>, no upstream`,
  `nothing to commit`, or `committed <sha>, NOT pushed: <reason>`. Trust it, not the exit code
  alone: 0 is success, 5 means committed but not pushed, 6 a rebase conflict to resolve.
- Running the same capture twice is a no-op (`exists <path>`, `nothing to commit`). A different
  body under the same title on the same day is refused; read the existing entry or pick a more
  specific title.

## Editing and history

- Edit an entry file directly when needed, then run `memex save`. `memex set <id> field=value`
  changes frontmatter without an editor.
- Journal bodies are immutable. To correct one: capture the correction, then
  `memex set <old-id> status=superseded superseded_by=<new-id>`. Never edit history in place;
  supersede it.
- Link other entries as `[[<id>]]`, not as prose paths. `memex check` fails on a broken link.
- Never write knowledge into a coding tool's own memory or rules files (`~/.claude/…`,
  `.cursorrules`, …). Knowledge lives in the store only.
