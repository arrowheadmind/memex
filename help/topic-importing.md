# memex help importing

`memex import brain-v3 SRC[@REV]… --out DIR [--leak PATH]…` converts one or more brainctl v3
brains (memex's predecessor) into a new store at `DIR`. It exists for the migration and is
kept separate from everything else memex does.

## Guarantees

- **Read-only on sources.** Each SRC is a git repository, read through git at REV (default
  `HEAD`; a dirty working tree is refused without `@REV`). The first SRC is the primary brain;
  further ones are secondary brains inside application repositories.
- **Fresh output.** `DIR` must not exist. memex creates it as `init` would and writes every
  file, but does not commit: you review, then `memex save -m "import"`.
- **Deterministic.** The same revisions produce byte-identical output, whatever the
  directories; re-run it against later revisions to bring over what was written in between.
- **Lossless, and accounted for.** Every line of every recognized source file is attributed
  to an entry, to the frontmatter, to structure, or to a per-file remainder entry; the import
  exits 1 if any line is unattributed or any unit's text changed.
- **Valid.** `memex check` on `DIR` reports zero errors; only `W-NO-SUMMARY`, `W-NO-TERMS`,
  `W-SHORT-BODY`, `W-ALIAS-LINK`, `W-RAW-OLD`, `W-PROMOTED-UNCITED` and `W-SECRET` may remain.
- **Status comes from structure, never from prose.** A status is set only from an explicit
  carrier (an index row, a `## Status` section, the backlog's sections and heading markers).
  Words such as `STALE` or `RESOLVED` in a rule's text become `marker-*` tags for the curation
  pass; the import never hides an entry.

## What becomes what

| source | entry |
|---|---|
| investigation file | journal; its header values move to the frontmatter |
| decision file | decision |
| runbook file (either directory) | runbook |
| plan file; `prompts/`, `reports/` | plan, with `kind` for prompts and reports |
| known-mistakes and constraints table rows and `##` sections | one rule each; `## Always` bullets get `always = true` |
| backlog `###` items | one task each; status from the section and the heading's markers |
| other docs | reference, `verified` = last commit |
| anything under `plans/<dir>/`, dispatch logs | attachments under `files/<owner>/` |
| `--leak PATH` files (tool memory) | `raw` journal entries, as `capture --from` makes them |

Ids follow the usual rule from the unit's date and title; a file whose name is already a valid
id keeps it. Dates come from the filename, a date in the heading, or the oldest commit that
introduced the unit's own text, never from the last edit. Every unit keeps the paths and
headings it was known by in `aliases`, so old references still resolve. Path references to
imported files are rewritten to `[[id]]` links.

## The report

`DIR/import-report.json` (committed with the import) holds, per source file, the lines and
characters in and out by category, and the lists an operator reviews: index rows that matched
nothing or conflicted with their file, duplicates between brains, dropped aliases, rows of
unexpected shape, short bodies, unresolved references, the markers removed from headings,
units dated by the blame fallback, and the leaks imported. Standard output summarizes the
same counts and ends with `report: DIR/import-report.json`.

## After the import

Curation is a separate, reviewed pass with ordinary commands: `memex list --tag marker-stale`
is the worklist; `memex set <id> status=retired` and `memex set <id> keywords=…` do the work;
`memex check` keeps it honest. The import never deduplicates, rewords, or judges whether a
rule is still true.
