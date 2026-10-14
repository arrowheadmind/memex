# memex help config

memex has one store, named by configuration. It never looks for a store in the current
directory or its parents.

## Store resolution

The store path is the first of these that is set:

1. the `--store PATH` option (accepted anywhere on the command line);
2. the `MEMEX_STORE` environment variable, if non-empty;
3. the `store` key in the config file.

Rules:

- A leading `~/`, `$HOME` followed by `/` or the end of the path, and `${HOME}` anywhere are
  expanded. Nothing else is, and symbolic links are not resolved. With `HOME` unset, a path
  that needs it is an error.
- A relative path is an error.
- The path must be a directory containing `memex.toml`; otherwise exit 2.
- With nothing set, every command except `init PATH`, `import`, `version` and `help` fails with
  exit 2 and
  `memex: no store configured; use --store, MEMEX_STORE, or store = "..." in <config path>`.

## Config file

`MEMEX_CONFIG` if set; otherwise `$XDG_CONFIG_HOME/memex/config.toml`, or
`~/.config/memex/config.toml`. A missing file is fine. A file that does not parse, or that has
an unknown key, is exit 2.

| key | type | default | meaning |
|---|---|---|---|
| `store` | string | none | Store path |
| `workspace` | string | none | Root for pointer checks and the leak scan |
| `sync_on_preflight` | boolean | `true` | `preflight` syncs the store first |
| `search_limit` | integer ≥ 1 | `12` | Default `--limit` for `search` and `preflight` |
| `list_limit` | integer ≥ 1 | `50` | Default `--limit` for `list` |
| `network_timeout` | integer ≥ 1 | `20` | Seconds allowed for each `git fetch`, `pull` or `push` |

The config file holds settings only, never knowledge.

Example:

```
store = "~/memex-store"
search_limit = 12
network_timeout = 20
```

## Environment variables

| variable | meaning |
|---|---|
| `MEMEX_STORE` | Store path |
| `MEMEX_CONFIG` | Config file path |
| `MEMEX_TODAY` | Overrides today's date (`YYYY-MM-DD`); for tests and imports. Malformed: exit 2 |

memex reads no other variables of its own.

## What memex passes to git

git runs with your environment, changed in these ways: `GIT_DIR`, `GIT_WORK_TREE`,
`GIT_INDEX_FILE` and the other repository-selecting variables are removed (so memex called
from another repository's hook still addresses the store; `check --staged` keeps
`GIT_INDEX_FILE` for the pre-commit hook); `GIT_TERMINAL_PROMPT=0`, `GIT_EDITOR=true` and ssh
`BatchMode=yes` are set, so git never prompts; `LC_ALL=C`, so git's messages are stable. Every
fetch, pull and push is bounded by `network_timeout`.
