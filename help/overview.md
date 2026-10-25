# memex <version> (store format 1)

If you are an AI agent: run `memex help agents` now (60 lines), or `memex help --all` once for
everything.

memex is a command-line tool for a private, git-backed store of Markdown notes: the rules,
mistakes, decisions, runbooks, facts, tasks, plans and investigations that one operator and
their coding agents accumulate. The people and agents do the thinking; memex is the memory
they share.

## The model

- **Store:** one git repository, named by `--store PATH` (before or after the command),
  `MEMEX_STORE` or the config file; never inferred from the current directory.
- **Entry:** one Markdown file with TOML frontmatter, directly inside a type directory.
- **Type:** the directory: `rules/`, `decisions/`, `runbooks/`, `reference/`, `tasks/`,
  `plans/`, `journal/`.
- **Id:** the filename without `.md`: `2026-01-05-push-the-closure-to-the-cache`. It never
  changes.
- **Status:** one word per type (`active`, `superseded`, `done`, …). Replaced knowledge changes
  status instead of being edited in place.
- **Visibility:** `search` and `list` show the current statuses by default; `--all` shows the
  rest.
- **Frontmatter:** `title`, `status`, `created`, `summary`, `tags`, `keywords` and a few more;
  `memex help entries` lists them all.

## The everyday loop

```
memex preflight "rotate the backup key on alpha"       # before work: sync, Always rules, search
memex capture --title="Rotated the backup key on alpha" \
  --summary="The runbook works; restic wants the short key id." --body notes/body.md
memex update <id> --summary="<where it stands now>" --body notes/progress.md   # task progress
memex save                                              # after editing an entry by hand
```

## Commands

| command | does |
|---|---|
| `init` | Create a store, or write pointer files into a directory |
| `new` | Create an entry of any type and commit it |
| `capture` | Create a journal entry and commit it |
| `search` | Ranked full-text search of the store |
| `preflight` | Start-of-task ritual: sync, Always rules, then search |
| `list` | List entries, newest first |
| `show` | Print entries with their links |
| `set` | Change frontmatter fields of an entry and commit it |
| `promote` | Create an entry derived from another and mark a journal source promoted |
| `fmt` | Rewrite entries into canonical form and commit them |
| `save` | Commit direct edits to the store |
| `sync` | Fetch, fast-forward or rebase, and push the store |
| `check` | Validate the store |
| `doctor` | Check the installation, the store and tool neutrality |
| `import` | Convert brain v3 stores into a new store, or add what is new to an existing one |
| `help` | Print help |
| `version` | Print the version |
| `update` | Append a dated section to an entry and replace its summary, in one commit |

## Topics

| topic | covers |
|---|---|
| `agents` | Rules of engagement for language-model agents |
| `concepts` | The model and the reasoning behind it |
| `entries` | Types, statuses, frontmatter fields and body outlines |
| `ids` | The id and slug rules, with worked examples |
| `links` | Link syntax and resolution |
| `search` | Query syntax, ranking, filters and how to read output |
| `writing` | The write protocol, direct edits, outcomes and conflicts |
| `config` | Store resolution, the config file and environment variables |
| `check-codes` | Every E- and W- code, with its condition and the usual fix |
| `exit-codes` | The exit-code table |
| `neutrality` | Pointer files, the leak scan and tool memory |
| `importing` | The brain v3 importer and its report |

Next: `memex help topics` lists these; `memex help <name>` prints one; `memex <command> --help`
prints a command's page. In its examples, `! ` starts a standard error line and `exit N` ends
an example that exits non-zero.
