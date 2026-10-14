# memex search

Ranked full-text search of the store.

## Usage

```
memex search QUERY… [--type T]… [--tag X]… [--repo R]… [--host H]… [--all] [--limit N] [--json]
```

## Description

Joins the positional arguments into one query, OR-combines its words (prefix matching for
words of 4 or more characters, phrases in `'"double quotes"'`), ranks every entry with BM25
over title, keywords and summary, tags, aliases and body, and prints the best visible ones.
Each result says which query words it matched (`matched k/n`), so a weak hit cannot pass for a
strong one. Entries whose status is not visible by default (retired rules, done tasks,
superseded entries) are counted, not shown; `--all` shows them. A miss is reported, never
silent.

## Options

| option | value | default | meaning |
|---|---|---|---|
| `--type` | T | | Only this type (name or directory); repeatable, OR'd |
| `--tag` | X | | Only entries with this tag; repeatable, OR'd |
| `--repo` | R | | Only entries concerning this repository; repeatable |
| `--host` | H | | Only entries concerning this machine; repeatable |
| `--all` | | | Make every status visible |
| `--limit` | N | `search_limit` (12) | Maximum number of results; 0 means no limit |
| `--json` | | | Print one JSON object instead of text |

## Output

One block per result: `N. [type, status] path  matched k/n: items`, then the title and
summary, then the first matching body line with matches in `**bold**`. After the results:
`weak results: the best match covers k of n query items; try other words` when the top hit
matched fewer than half the query items (`"weak":true` in `--json`; search again with other
words), `N more visible matches not shown (raise --limit)`, `N matches hidden by status (use
--all)`, `N matches excluded by filters (…)` when they apply, and `Store root: <path>` so that
the paths resolve. When nothing matched: `No entries matched any term of: <query>`.

## Exit codes

| code | when |
|---|---|
| 0 | At least one result shown |
| 2 | Empty query, unknown option or filter value |
| 3 | Nothing shown (nothing matched, or all matches hidden or excluded) |

## Examples

```example
$ memex search deploy dirty --limit 2
1. [rule, active] rules/2026-01-06-never-deploy-from-a-dirty-tree.md  matched 2/2: deploy, dirty
   Never deploy from a dirty tree — A deploy from uncommitted changes cannot be reproduced or rolled back; commit first.
   **Avoid:** running the **deploy** script with uncommitted changes in the **deployments** checkout.
2. [rule, active] rules/2026-01-05-push-the-closure-to-the-cache-before-switching.md  matched 1/2: deploy
   Push the closure to the cache before switching — A host that switches to a closure nobody pushed rebuilds it from source on the next boot.
   **Symptom:** a **deploy** finishes, and the next reboot of the host spends twenty minutes compiling.
2 more visible matches not shown (raise --limit)
Store root: <STORE>
```

```example
$ memex search midnight
1 matches hidden by status (use --all)
exit 3
```

## Mistakes to avoid

- `memex search "dirty tree"` without the inner quotes is two OR'd words; write
  `'"dirty tree"'` for a phrase.
- `matched 1/4` means one of four words matched; read the label before trusting the rank.
- Exit 3 with a hidden count means the knowledge exists but is retired, done or superseded;
  add `--all` when history is what you need.

## See also

`memex preflight`, `memex show`, `memex help search`.
