# memex help search

Search is ranked full-text search over every entry. A query that matches little still returns
what it matched and says what it hid. When no entry matches any query item at all, the whole
output is `No entries matched any term of: <query>` and the exit code is 3, never an empty
success.

## Query syntax

- Words are split on anything that is not a letter, digit or mark, lowercased, and stripped of
  accents (`Café` and `cafe` are the same word).
- Every word is a **term**. A term of 4 or more characters is a **prefix term**: `deploy`
  matches `deploy`, `deploying` and `deployments`. A shorter term (`nix`, `ssh`) matches whole
  words only.
- A `"double-quoted"` span is a **phrase**: its words must appear consecutively. Quote it so
  the shell keeps the quotes: `memex search '"dirty tree"'`.
- Terms and phrases combine with **OR**: an entry matches if any of them matches. A duplicate
  is dropped. A query with nothing left is a usage error.

## Ranking

BM25 over five fields with weights: title 10, keywords and summary 8, tags 5, aliases 3,
body 1; then a type weight: rule 1.5, runbook and reference 1.3, decision 1.2, task and plan
1.0, journal 0.8. Rare words count more than common ones. Ties are broken by type (rules
first) and then by id. The exact formula is in the specification; two implementations rank
identically.

## Reading the output

```
1. [rule, active] rules/2026-01-06-never-deploy-from-a-dirty-tree.md  matched 2/2: deploy, dirty
   Never deploy from a dirty tree — A deploy from uncommitted changes cannot be reproduced …
   **Avoid:** running the **deploy** script with uncommitted changes in the deployments checkout.
```

- Line 1: rank, type and status, the path (paste it into `memex show`), and `matched k/n`:
  how many of your n query items this entry matched, and which. `matched 1/4` is a weak hit.
- Line 2: title, then the summary. That is enough to decide whether to open the entry.
- Line 3: the first body line that matched, matches in `**bold**`, cut to 160 characters.

After the results, zero or more of these lines, then the store root:

```
weak results: the best match covers k of n query items; try other words
N more visible matches not shown (raise --limit)
N matches hidden by status (use --all)
N matches excluded by filters (drop --type/--tag/--repo/--host)
Store root: /path/to/store
```

`weak results` means even the top hit matched fewer than half your words: likely nothing
relevant exists, so search again with other content words before trusting any result.
Exit 3 when nothing is shown; the count lines say whether something matched but was hidden or
excluded.

## Filters and options

- `--type T`, `--tag X`, `--repo R`, `--host H`: repeatable; values of one option are OR'd,
  different options are AND'd. `--type` takes the type name or its directory (`rule` or
  `rules`).
- `--all` makes every status visible (retired rules, done tasks, superseded entries).
- `--limit N` caps the results (default `search_limit`, 12; `0` means no limit).
- `--json` prints one object: `query`, `items`, `results` (with `rank`, `type`, `status`,
  `id`, `title`, `summary`, `score`, `path`, `matched`, `snippet`), `more`, `hidden`,
  `excluded`, `weak`. It is printed even when nothing matched.

## Writing entries that get found

Put the reader's likely words in `keywords`: the body says `nixos-rebuild switch`, the reader
will type `deploy`. The summary is searched with the keywords' weight, so write it in the
reader's words too. `tags` are for `--tag` filtering and carry less weight in ranking.
