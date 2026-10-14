# memex init

Create a store, or write pointer files into a directory.

## Usage

```
memex init PATH [--name NAME]
memex init --pointers DIR [--force]
```

## Description

The store form creates `PATH` if needed, runs `git init` if it is not a repository, and writes
`memex.toml`, `AGENT.md`, the seven type directories, `files/`, `.gitignore` and the pre-commit
hook, then sets `core.hooksPath`. Existing files are never overwritten; running it again is a
no-op. It does not commit: run `memex save -m "init store"` next, because a store with no
commits refuses every other write. Add an upstream with `git remote add origin …` and
`git push -u origin main` so that writes are pushed.

The pointer form writes `CLAUDE.md`, `AGENTS.md` and `GEMINI.md` into `DIR`, each holding the
single line `Read <store>/AGENT.md before starting work, and follow it.` It needs a resolved
store, because the line names it.

## Options

| option | value | default | meaning |
|---|---|---|---|
| `--name` | NAME | the directory basename | Store name, written to `memex.toml` |
| `--pointers` | DIR | | Write the three pointer files into DIR instead of creating a store |
| `--force` | | | Replace a pointer file that has other content, printing its old content first |

## Output

Store form: one line per created or changed item (`created memex.toml`, `created rules/`,
`set core.hooksPath .githooks`, …), then `initialized <absolute path>; run: memex save -m
"init store"`. Pointer form: `created`, `unchanged` or `replaced <DIR>/<file>` per file; a file
with other content is reported on standard error and left alone.

## Exit codes

| code | when |
|---|---|
| 0 | Done |
| 1 | A pointer file has other content (without `--force`), or DIR is not a directory |
| 2 | Usage error, or no store configured for `--pointers` |

## Examples

```example
$ memex init newstore
created .git/
created memex.toml
created AGENT.md
created rules/
created decisions/
created runbooks/
created reference/
created tasks/
created plans/
created journal/
created files/
created .gitignore
created .githooks/pre-commit
set core.hooksPath .githooks
initialized <HOME>/newstore; run: memex save -m "init store"
```

```example
$ memex init --pointers repo
created repo/CLAUDE.md
created repo/AGENTS.md
created repo/GEMINI.md
```

## Mistakes to avoid

- Writing an entry right after `init` fails with `store has no commits yet; run memex save`:
  the pre-commit hook needs `memex.toml` committed.
- `init` never infers the store from the current directory; `--pointers` uses the configured
  store.

## See also

`memex save`, `memex help config`, `memex help neutrality`.
