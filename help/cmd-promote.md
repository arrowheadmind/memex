# memex promote

Create an entry derived from another and mark a journal source promoted.

## Usage

```
memex promote SOURCE-ID --to TYPE --title=T --summary=S --body FILE|- [new options]
```

## Description

Turns what a journal entry taught into durable knowledge: creates a new entry of `TYPE` as
`memex new` would, with `sources` set to `SOURCE-ID` (plus any `--source`), and, when the
source is a journal entry in `raw` or `kept`, sets its status to `promoted`. Both files go in
one commit. An already promoted or superseded journal, or a source of another type, is cited
but not changed. The source may be given as an id, a path or an alias. memex resolves and
checks the store, takes the write lock, freshens, reads its entries, resolves and checks the
source, and only then reads the `--body` file or standard input.

## Options

| option | value | default | meaning |
|---|---|---|---|
| `--to` | TYPE | | Type of the new entry (required) |
| `--title` | T | | Title of the new entry (required) |
| `--summary` | S | | One sentence that tells a reader whether to open the entry (required) |
| `--body` | FILE | | Body file, or `-` for standard input (required) |
| `--status` | S | first status of the type | Initial status |
| `--tag` | X | | Tag for filtering; repeatable |
| `--keyword` | K | | Word a reader would search with; repeatable |
| `--repo` | R | the current repository | Repository the entry concerns; repeatable |
| `--no-repo` | | | Do not attribute the entry to the current repository |
| `--host` | H | | Machine the entry concerns; repeatable |
| `--always` | | | Rule printed by every preflight (rules only) |
| `--kind` | K | | `prompt` or `report` (plans only) |
| `--parent` | ID | | Enclosing plan (plans and tasks only) |
| `--source` | ID | | Further entry this one was derived from; repeatable |
| `--verified` | DATE | today | When the facts were checked (references only) |
| `--review-days` | N | | Staleness threshold for `verified` (references only) |
| `--alias` | A | | Former name or path that resolves to this entry; repeatable |

## Output

`created <path>`, then the outcome line. As in `capture`, a repeat whose new entry would have
the same id and an identical body prints `exists <path>` and `nothing to commit` (exit 0); the
same id with a different body is refused (exit 1) with `memex: exists <path>`, its title and
summary, and the three correct moves.

## Exit codes

| code | when |
|---|---|
| 0 | Created |
| 1 | Unknown source; the source has uncommitted edits or fields that promote cannot keep; the new entry refused as in `new`, or its id taken by a different body |
| 2 | Usage error |
| 4 | Another memex held the write lock for 60 s |
| 5 | Committed but not pushed |
| 6 | Committed, rebase conflict |

## Examples

```example
$ memex promote 2026-01-09-restore-test-on-alpha --to rule --title="Create the restore target before running restic" --summary="restic restore does not create its target directory; mkdir it first." --tag backup --keyword restic --body notes/body.md
created rules/2026-01-15-create-the-restore-target-before-running-restic.md
pushed <SHA7>
```

```example
$ memex show journal/2026-01-09-restore-test-on-alpha.md --json
{"entries":[{"id":"2026-01-09-restore-test-on-alpha","type":"journal","path":"journal/2026-01-09-restore-test-on-alpha.md","frontmatter":{"title":"Restore test on alpha","status":"promoted","created":"2026-01-09","summary":"First restore from restic on alpha worked; the mount point must exist before restic runs.","tags":["backup"],"repos":["deployments"],"hosts":["alpha"],"body_sha256":"ff0e2a694b3faa17f0e5419899629f2cdd9061888856d2054f93768dd51e5e7a"},"body":"## What was attempted\n\nRestore the latest snapshot's `/srv/photos/2025` into `/tmp/restore-test` on alpha.\n\n## What worked\n\n`restic restore latest --target /tmp/restore-test --include /srv/photos/2025` finished in\n40 s and the files compared equal.\n\n## What failed\n\nThe first attempt failed because `/tmp/restore-test` did not exist; restic does not create the\ntarget directory.\n\n## Reusable insights\n\nCreate the target directory first. Worth a line in the rotation runbook.\n","links":{"cited_by":["2026-01-15-create-the-restore-target-before-running-restic"],"supersedes":[],"children":[],"links_to":[],"linked_from":[]}}]}
```

## Mistakes to avoid

- Promoting the whole journal entry. A rule is one lesson in the reader's words; the journal
  entry stays as the evidence, cited in `sources`.
- Forgetting `--keyword`: the rule will be searched with the reader's words, not the
  incident's.

## See also

`memex capture`, `memex new`, `memex help entries`.
