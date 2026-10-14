# memex show

Print entries with their links.

## Usage

```
memex show ID… [--json]
```

## Description

Prints each entry file verbatim, then its relations: `cited by` (entries listing it in
`sources`), `supersedes`, `children` (entries whose `parent` it is), `links to` (its own
`[[…]]` targets) and `linked from`. Each ID may be an id, the store-relative path that
`search` and `list` print, or an alias. Several IDs are printed in one call, separated by a
line `---`, so that reading a handful of entries costs one command.

## Options

| option | value | default | meaning |
|---|---|---|---|
| `--json` | | | Print one JSON object: `entries` with `id`, `type`, `path`, `frontmatter`, `body`, `links` |

## Output

The file, a blank line, `--- links`, then the relation lines that have entries, as paths. An
unknown ID exits 1 after the known ones are printed, with `memex: no entry "<id>"` and up to
three `  candidate: <path>` lines.

## Exit codes

| code | when |
|---|---|
| 0 | Every ID was found |
| 1 | Some ID was not found |
| 2 | Usage error |

## Examples

```example
$ memex show 2026-01-07-document-the-restore-test
+++
title = "Document the restore test"
status = "open"
created = 2026-01-07
summary = "Write down the restore check so that the key rotation runbook can point at it."
tags = ["backup"]
parent = "2026-01-08-backup-overhaul"
+++

## Goal

A short runbook for restoring one snapshot into a scratch directory and comparing it.

## Done when

The runbook exists and the rotation runbook links to it.

--- links
linked from: plans/2026-01-08-backup-overhaul.md
```

```example
$ memex show 2026-01-07-document-the-restore-tset
! memex: no entry "2026-01-07-document-the-restore-tset"
!   candidate: tasks/2026-01-07-document-the-restore-test.md
!   candidate: plans/2026-01-08-backup-overhaul.md
!   candidate: decisions/2026-01-03-use-restic-for-backups.md
exit 1
```

## Mistakes to avoid

- Guessing an id. Paste the path a search printed, or use the `candidate:` line.
- Reading a whole entry when the summary already answered the question.

## See also

`memex search`, `memex list`, `memex help links`.
