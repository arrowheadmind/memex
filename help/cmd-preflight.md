# memex preflight

Start-of-task ritual: sync, Always rules, then search.

## Usage

```
memex preflight [QUERY…] [--no-sync] [--max-chars N] [search options]
```

## Description

Run it before any non-trivial task, with the task in your own words. It syncs the store
(fetch, fast-forward or rebase, push), prints every visible rule marked `always = true` under
`## Always`, then runs `memex search` for the query under `## Results`. Without a query it
stops after the Always rules; that is the form a session-start hook runs:
`memex preflight --max-chars 4000`. The whole output is bounded by `--max-chars`: the sync
line and the Always rules are never cut, results are dropped from the end, and a cut is always
announced.

## Options

| option | value | default | meaning |
|---|---|---|---|
| `--no-sync` | | | Do not sync the store first |
| `--max-chars` | N | 12000 | Bound on the whole output, in characters (minimum 2000) |
| `--type` | T | | Only this type in the results; repeatable |
| `--tag` | X | | Only entries with this tag; repeatable |
| `--repo` | R | | Only entries concerning this repository; repeatable |
| `--host` | H | | Only entries concerning this machine; repeatable |
| `--all` | | | Make every status visible in the results |
| `--limit` | N | `search_limit` (12) | Maximum number of results |

## Output

`store: <state>` (`up to date`, `pushed 1 commits`, `NOT SYNCED: <reason>`, or
`not synced (last fetch <time>)` with `--no-sync`), a blank line, `## Always` with one line per
rule (`- <path>: <title> — <summary>`), then with a query a blank line, `## Results` and the
search output, including its `weak results: …` line when the top hit matched fewer than half
the query items: treat the results as unrelated and search again. When results were dropped
for the budget, the last line is
`output budget reached: <k> results not shown (raise --max-chars or run memex search)`.

## Exit codes

| code | when |
|---|---|
| 0 | No query; or results shown; or nothing matched but an Always rule was printed |
| 2 | Usage error |
| 3 | A query matched nothing visible and there were no Always rules |

A failed sync never changes the exit code.

## Examples

```example
$ memex preflight "rotate backup key" --limit 2
store: up to date

## Always
- rules/2026-01-05-push-the-closure-to-the-cache-before-switching.md: Push the closure to the cache before switching — A host that switches to a closure nobody pushed rebuilds it from source on the next boot.

## Results
1. [runbook, active] runbooks/2026-01-04-rotate-the-backup-key-on-alpha.md  matched 3/3: rotate, backup, key
   Rotate the backup key on alpha — Yearly: add a new restic key, re-check a restore, then remove the old key.
   Once a year, or at once if the **key** may have leaked.
2. [decision, accepted] decisions/2026-01-03-use-restic-for-backups.md  matched 3/3: rotate, backup, key
   Use restic for backups — restic replaces the rsync job: deduplicated, encrypted, and restorable per snapshot.
   Run restic from `alpha` against the NAS repository at 03:00, with a **key** kept in the secrets
7 more visible matches not shown (raise --limit)
1 matches hidden by status (use --all)
Store root: <STORE>
```

```example
$ memex preflight --no-sync
store: not synced (last fetch <TIME>)

## Always
- rules/2026-01-05-push-the-closure-to-the-cache-before-switching.md: Push the closure to the cache before switching — A host that switches to a closure nobody pushed rebuilds it from source on the next boot.
```

## Mistakes to avoid

- Skipping it. The Always rules exist because agents skipped `preflight` or searched with
  words that matched nothing.
- Reading the results without the `matched k/n` label.
- `--no-sync` on a shared store: you may be reading yesterday's knowledge.

## See also

`memex search`, `memex sync`, `memex help agents`.
