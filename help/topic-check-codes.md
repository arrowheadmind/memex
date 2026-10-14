# memex help check-codes

`memex check` prints one line per finding, `<path>: <CODE> <message>`, sorted by path, code
and message, then `check: N errors, M warnings`. Errors exit 1; `--strict` makes warnings count.
Every message starts with what is wrong and ends with the fix when the fix is a command.

## Errors

| code | condition | usual fix |
|---|---|---|
| `E-ID` | Filename is not a valid id | Rename to `YYYY-MM-DD-slug.md`, lowercase, single hyphens |
| `E-DUP-ID` | Same id in two type directories | Rename one; ids are unique across the store |
| `E-NOT-ENTRY` | Non-`.md` file or subdirectory in a type directory; unknown top-level item | Move attachments to `files/<id>/`; remove stray files |
| `E-ENCODING` | Not UTF-8, has a BOM, or contains CR | Re-save as UTF-8 with LF line endings |
| `E-FRONTMATTER` | Missing `+++` delimiters, or invalid TOML | Fix the frontmatter; the message gives the line |
| `E-FIELD-UNKNOWN` | Field not in the table, or not allowed on this type | Remove or rename it (the message suggests the nearest name) |
| `E-FIELD-MISSING` | Required field absent | Add it |
| `E-FIELD-VALUE` | Wrong kind or value outside its constraints | Fix the value; the message says the kind or the limit |
| `E-STATUS` | Status not valid for the type | Use one of the listed statuses |
| `E-CREATED` | `created` differs from the id's date | Set `created` to the id's date |
| `E-SUPERSEDED` | `superseded` without `superseded_by`, or the reverse | `memex set <id> superseded_by=<new>`, or clear one side |
| `E-REF` | `parent`, `sources` or `superseded_by` names a missing id, or `parent` is not a plan | Use the id the message gives, or fix the reference |
| `E-LINK` | `[[…]]` that resolves to no entry, or to several through a shared alias | Link an id |
| `E-FILE-LINK` | Markdown link to a missing, escaping or non-plain `files/…` path | Fix the path or add the attachment |
| `E-HASH` | A journal body changed | Restore the body, or capture the correction and `memex set <id> status=superseded superseded_by=<new>` |
| `E-ALIAS` | The same alias on two entries, or an alias equal to an id | Remove the duplicate alias |
| `E-FILES-OWNER` | `files/<x>/` where `<x>` is not an id, or a loose file under `files/` | Move it under `files/<id>/` |
| `E-MANIFEST` | `memex.toml` invalid | Fix `format` (integer) and `name` (string); no other keys |
| `E-POINTER` | (`--workspace`) A `CLAUDE.md`, `CLAUDE.local.md`, `.claude/CLAUDE.md`, `AGENTS.md` or `GEMINI.md` under the workspace that is not the pointer line, or is a symbolic link | `memex capture --from <path>` for any knowledge in it, then `memex init --pointers <dir> --force` (or delete the file) |

## Warnings

| code | condition | usual fix |
|---|---|---|
| `W-CANONICAL` | Valid but not in canonical form | `memex fmt <path>` |
| `W-ALIAS-LINK` | A link resolved through an alias | Link the id the message gives |
| `W-STALE-REFERENCE` | `verified + review_days` is in the past | Re-check the facts, then `memex set <id> verified=<today>` |
| `W-RAW-OLD` | A `raw` journal entry older than 90 days | `memex set <id> status=kept`, or `memex promote <id>` |
| `W-PROMOTED-UNCITED` | A `promoted` journal entry no entry cites in `sources` | Add it to the derived entry's `sources`, or set `kept` |
| `W-NO-TERMS` | Neither `tags` nor `keywords` | `memex set <id> keywords=<word>,<word>` |
| `W-NO-SUMMARY` | No `summary` | `memex set <id> summary=<sentence>` |
| `W-SHORT-BODY` | Body content measure below 80 | Edit the file, then `memex save` |
| `W-NO-POINTER` | (`--workspace`) A git repository under the workspace with no `CLAUDE.md` and no `AGENTS.md` | `memex init --pointers <dir>` |
| `W-LEAK` | (`--workspace`) A file in a coding tool's own memory (§12 leak scan) | `memex capture --from <path>`, review, then delete the file |
| `W-SECRET` | A line looks like a credential (private-key header, `password=`/`token=`/`secret=` with a real value, or a long base64 run after a credential word) | Remove it, or add the line `<!-- memex: allow-secret -->` to the body if it is not one |

## Options

- `--staged` validates the entries added or modified in the git index, plus their link
  targets; the pre-commit hook runs it.
- `--workspace` adds `E-POINTER`, `W-NO-POINTER` and `W-LEAK` for the configured `workspace`
  and the home locations of the leak scan; their paths are absolute (`memex help neutrality`).
- `--errors-only` hides warning lines; `--limit N` (default 200, 0 for no limit) caps the
  finding lines and prints `<CODE>: K more (raise --limit)` for the rest; `--json` prints one
  object with `findings`, `errors`, `warnings` and `truncated`.
