# Constraints — Work

A preamble line that belongs to no rule.

| Constraint | Notes |
|------------|-------|
| Primary checkouts are integration-only | Start a task worktree instead; enforced by the pre-commit hook since 2026-01-01 |

## Always

- **The brain is the memory. Never write to a tool's own store.** Anything written
  elsewhere is lost. Added 2026-01-02 after a session recorded a preference in a
  vendor directory.
  - nested detail line

- Plain bullet with a first sentence. And a second sentence that is not the title.

## Nix

Never run a bare rebuild without pushing the closure to the cache first.

### Why

Because the weekly job rebuilds from source otherwise, which takes hours.
