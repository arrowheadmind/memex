# memex sync

Fetch, fast-forward or rebase, and push the store.

## Usage

```
memex sync
```

## Description

Brings the store up to date with its upstream and pushes what is local: fetch; fast-forward
when behind and clean; rebase and push when both sides moved; push when only ahead. Every
network step is bounded by `network_timeout` (20 s). A dirty working tree is never rebased or
fast-forwarded: commit it first with `memex save`. `preflight` runs `sync` for you unless
`--no-sync` or `sync_on_preflight = false`.

## Output

Exactly two lines: `fetched <time>` (always, recording the attempt), then the state:
`up to date`, `fast-forwarded N commits`, `pushed N commits`, `rebased and pushed N commits`,
`no upstream`, or `NOT SYNCED: <reason>`.

## Exit codes

| code | when |
|---|---|
| 0 | Synced, or nothing to do, or no upstream |
| 2 | Usage error |
| 4 | Another memex held the write lock for 60 s |
| 5 | `NOT SYNCED`: uncommitted changes, a rebase in progress or conflict, a timeout, a failed fetch or push |

## Examples

```example
$ memex sync
fetched <TIME>
up to date
```

```example
$ memex sync --json
memex: unknown option --json
memex sync
Run: memex help sync
exit 2
```

## Mistakes to avoid

- Running `sync` to "fix" a conflict: it reports the conflict again. Resolve it with git in
  the store, then `sync`.
- Expecting `sync` to commit: it only moves commits. Edits need `memex save` first.

## See also

`memex save`, `memex preflight`, `memex help writing`.
