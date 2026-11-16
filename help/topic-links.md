# memex help links

Entries link to each other by id, never by prose path, so that a rename or a move cannot
silently break a reference. `memex check` fails on a link that looks like an id and resolves to
nothing, and the pre-commit hook runs `check`, so such a link never lands. A link written
another way that names no entry (`[[tasks/<id>]]`, `[[<id>.md]]`) is a `W-BROKEN-LINK`
warning.

## Syntax

- `[[TARGET]]` or `[[TARGET|TEXT]]`. TARGET is an id, or an alias of some entry; TEXT is any
  text without `]` or a line break.
- Anything else between `[[` and `]]` (a shell test like `[[ -f x ]]`) is plain text, not a
  link.
- Links inside fenced code blocks and inline code spans are ignored. A code span follows
  CommonMark: a run of N backticks is closed by the next run of exactly N backticks on the same
  line.

## Resolution

1. TARGET is the id of an entry: resolved.
2. Exactly one entry has TARGET in its `aliases`: resolved, with a `W-ALIAS-LINK` warning
   that suggests the id.
3. TARGET looks like an id but no entry has it, or it is an alias of several entries:
   `E-LINK`.
4. Anything else is plain text. When it is shaped like a link (only letters, digits, `-`, `_`,
   `.` and `/`, with at least one `-`, `_` or `/`), `check` reports `W-BROKEN-LINK`, naming
   the entry meant when it finds exactly one: the id left after removing a type directory
   (`tasks/`) and `.md`, or the capped id of an id written out beyond 120 characters. Link
   that id, or put the text in backticks if it is not a link. `[[ -f x ]]`, `[[:space:]]`
   and `[[links]]` are not reported.

Field references (`parent`, `sources`, `superseded_by`) must be ids; aliases are not accepted
there, though `memex set` resolves an alias or a path to the id for you.

## Attachments

A Markdown link whose destination starts with `files/` names an attachment under
`files/<id>/` (store-root-relative). It must be a plain path (no `.`, `..` or empty
segment), and must resolve, following symbolic links, to a file or directory inside the store;
otherwise `E-FILE-LINK`. A `#fragment` or `?query` is dropped and `%XX` escapes are decoded
first.

Any other URL, path or `repo@sha` pin is free text and is not checked.

## Aliases

`aliases` holds former names or paths that resolve to the entry: an imported file's original
path, a title the entry used to have. An alias may not equal any entry's id, and the same
alias may not appear on two entries (`E-ALIAS`). `memex show <alias>` works like
`memex show <id>`.

## What show prints

`memex show <id>` ends with the entry's relations: `cited by` (entries listing it in
`sources`), `supersedes` (entries whose `superseded_by` is it), `children` (entries whose
`parent` is it), `links to` (its own `[[…]]` targets, in body order) and `linked from`
(entries whose body links to it). Paths, so you can open them directly.
