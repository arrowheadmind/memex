# Agent instructions

This store is the shared memory for this operator's work. It is managed with `memex`.

- If you are new to memex: `memex help agents` (short), or `memex help --all` (everything).
- Before any non-trivial task: `memex preflight "<task in your own words>"`. Read the
  `matched k/n` label before trusting a result.
- After any non-trivial task:
  `memex capture --title="<task>" --summary="<one sentence>" --body - <<'EOF' … EOF`.
- Never write knowledge into a coding tool's own memory or rules files.
  Knowledge lives here only.
- Edit entries directly when needed, then run `memex save`.

Operator-specific instructions follow.

# Agent instructions (brainctl v3)

- **BRAIN_CONTEXT**: root
- **BRAIN_REPO**: src

Run `brainctl preflight` before work.
