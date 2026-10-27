# Agent instructions

This store is the shared memory for this operator's work. It is managed with `memex`.

- If you are new to memex: `memex help agents` (short), or `memex help --all` (everything).
- Before any non-trivial task: `memex preflight "<the task, in content words>"` (skip a, the, for).
  Read the `matched k/n` label and any `weak results` line before trusting a result.
- After any non-trivial task:
  `memex capture --title="<task>" --summary="<one sentence>" --body - <<'EOF' … EOF`.
- Never write knowledge into a coding tool's own memory or rules files.
  Knowledge lives here only.
- Edit entries directly when needed, then run `memex save`.

Operator-specific instructions follow.
