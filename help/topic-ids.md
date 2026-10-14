# memex help ids

An id is the entry's filename without `.md`: `DATE-SLUG`, where `DATE` is `YYYY-MM-DD` and the
whole id matches `^[0-9]{4}-[0-9]{2}-[0-9]{2}-[a-z0-9]+(-[a-z0-9]+)*$`, at most 120 characters.
The date is the day the entry was created; the slug comes from the title. An id is never
renamed or reused, and changing the title does not change it.

## From title to slug

1. Normalize the title to Unicode NFKD and remove every combining mark.
2. Lowercase.
3. Replace every run of characters outside `a-z0-9` with one `-`.
4. Strip leading and trailing `-`.
5. If nothing is left, use `entry`.

Length cap: if `DATE-SLUG` is longer than 120 characters, keep the first 111, strip a trailing
`-`, and append `-` plus the first 8 hex digits of the SHA-256 of the title.

## Worked examples

| title (created 2026-01-15) | id |
|---|---|
| `Push the closure to the cache` | `2026-01-15-push-the-closure-to-the-cache` |
| `Café: déjà vu on the NAS` | `2026-01-15-cafe-deja-vu-on-the-nas` |
| `--help` | `2026-01-15-help` (but see below: a title starting with `-` must be written `--title=--help`) |
| `nixos-rebuild switch --flake .#alpha` | `2026-01-15-nixos-rebuild-switch-flake-alpha` |
| `日本語` | `2026-01-15-entry` (no ASCII letters survive, so the fallback applies) |
| a 130-character English title | the first 111 characters of the id, then `-` and 8 hex digits |

## Rules that follow

- Two entries cannot share an id, even in different types; the same title on the same day is
  refused with the existing entry's title and summary.
- The frontmatter's `created` must equal the id's date (`check` reports `E-CREATED`).
- Wherever a command takes an id (`show`, `set`, `promote`, `--parent`, `--source`,
  `superseded_by=`), it also takes the entry's store-relative path or one of its aliases.
- A misspelled id gets up to three `candidate:` lines: ids within edit distance 3 first, then
  the best search matches for the text.
