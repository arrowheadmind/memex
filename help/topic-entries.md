# memex help entries

An entry is one UTF-8 Markdown file with LF line endings: a TOML frontmatter between `+++`
lines, a blank line, then the body. memex writes entries in a canonical form; `memex check`
warns (`W-CANONICAL`) when a hand-edited file drifts from it and `memex fmt` restores it.

```
+++
title = "Push the closure to the cache before switching"
status = "active"
created = 2026-01-05
summary = "A host that switches to a closure nobody pushed rebuilds it on the next boot."
tags = ["deploy", "nix"]
keywords = ["cache", "rebuild"]
always = true
+++

**Symptom:** …
```

## Types and statuses

The first status is the default for new entries. Visible statuses are what `search` and `list`
show without `--all`.

| directory | type | statuses | visible by default |
|---|---|---|---|
| `rules/` | rule | `active` `superseded` `retired` | `active` |
| `decisions/` | decision | `accepted` `proposed` `superseded` `rejected` | `accepted` `proposed` |
| `runbooks/` | runbook | `active` `superseded` `retired` | `active` |
| `reference/` | reference | `active` `superseded` `retired` | `active` |
| `tasks/` | task | `open` `in-progress` `blocked` `done` `dropped` | `open` `in-progress` `blocked` |
| `plans/` | plan | `draft` `active` `done` `abandoned` `superseded` | `draft` `active` |
| `journal/` | journal | `raw` `kept` `promoted` `superseded` | `raw` `kept` `promoted` |

Rules cover both constraints and known mistakes; the difference lives in the body. Journal
statuses: `raw` is not yet reviewed, `kept` is reviewed with nothing to promote, `promoted`
means its knowledge moved into another entry. Logs are not a type: a machine-produced log is
an attachment under `files/<id>/`.

## Frontmatter fields

In canonical order. A field not in this table, or on a type it is not allowed on, is an error.

| field | kind | required | allowed on | meaning |
|---|---|---|---|---|
| `title` | string | yes | all | 1–300 characters, no line break |
| `status` | string | yes | all | One of the type's statuses |
| `created` | date | yes | all | Must equal the date in the id |
| `summary` | string | no | all | One line, at most 500 characters; shown in lists and results |
| `tags` | array | no | all | Each matches `^[a-z0-9][a-z0-9-]*$`; for filtering |
| `keywords` | array | no | all | Words a reader would search with; free strings |
| `repos` | array | no | all | Source repository names the entry concerns |
| `hosts` | array | no | all | Machine names the entry concerns |
| `always` | boolean | no | rule | Printed by every `preflight` |
| `kind` | string | no | plan | `prompt` or `report`; absent means an ordinary plan |
| `parent` | string | no | plan, task | Id of an enclosing plan |
| `sources` | array | no | all | Ids this entry was derived from |
| `superseded_by` | string | iff `superseded` | all | Id of the replacing entry |
| `verified` | date | yes | reference | When the facts were last checked against the live system |
| `review_days` | integer 1–36500 | no | reference | Staleness threshold for `verified` |
| `aliases` | array | no | all | Former names or paths that resolve to this entry |
| `body_sha256` | string | yes | journal | SHA-256 of the body; journal bodies are immutable |

There is no `updated` field (git records it) and no reverse-link field (`show` computes them).
`tags`, `keywords`, `repos` and `hosts` are stored sorted and deduplicated. `new` and `capture`
set `created`, `body_sha256` and, unless told otherwise, `repos` from the current repository.

## Bodies

Free Markdown. A body given to `new`, `capture` or `promote` must hold at least 80 characters
of text outside headings (code counts); an existing shorter body is a warning. Journal bodies
are immutable: to correct one, capture the correction and supersede the old entry. These
outlines are suggestions, never enforced:

| type | outline |
|---|---|
| rule | `**Symptom:**` `**Context:**` `**Avoid:**` `**Instead:**` |
| decision | `## Context` `## Decision` `## Consequences` |
| runbook | `## When to use` `## Steps` `## Verify` |
| reference | `## Current state` `## How to re-verify` |
| task | `## Goal` `## Done when` |
| plan | `## Context` `## Steps` `## Verification` |
| journal | `## What was attempted` `## What worked` `## What failed` `## Reusable insights` |

Writing for the next agent: put the reader's likely words in `keywords`; make `summary` the
sentence that answers "should I open this?"; one entry per lesson; link with `[[id]]`; record
evidence (commands, commit pins, measured values) in the body.
