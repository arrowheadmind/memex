# memex doctor

Check the installation, the store and tool neutrality.

## Usage

```
memex doctor [--json]
```

## Description

Runs a fixed list of checks and prints one line each, so that a person, an agent or a fleet
monitor sees at once whether memex can work here: git, the config file, the store and its
format, upstream, last fetch, uncommitted or unpushed work, the pre-commit hook, `memex
check`, shadowed `memex` executables on `PATH`, the leak scan (knowledge sitting in a coding
tool's own memory) and whether Claude Code's auto memory is off. Read-only, no network, no
lock; it works without a store, reporting that as a failed check. When `store` or `format`
fails, the checks that need the store are left out. A WARN or FAIL message ends with the
command that fixes it, where there is one. `doctor` writes nothing, not even git's index.

## Options

| option | value | default | meaning |
|---|---|---|---|
| `--json` | | | Print one JSON object: `checks` (`id`, `result`, `message`, `details`) and `failed` |

## Output

One line per check, in this order: `git`, `config`, `store`, `format`, `upstream`,
`fetch-age`, `clean`, `hooks`, `check`, `path`, `leaks`, `tool-memory`, each as
`PASS|WARN|FAIL <id> <message>`. After the `leaks` line, one `LEAK <path>` line per file
found, absolute paths in order; `memex capture --from <path>` moves each into the store.
With `--json`, the `LEAK` lines are the `details` of the `leaks` check.

## Exit codes

| code | when |
|---|---|
| 0 | No check failed (warnings allowed) |
| 1 | At least one check failed: git missing or too old, a bad config file, no store, an unsupported format, `check` errors or a git history `check` cannot read |
| 2 | Usage error |

## Examples

```example
$ memex doctor
PASS git version 2.30 or later
PASS config config file parses
PASS store <STORE>, name "example"
PASS format format 1
PASS upstream main tracks origin/main
PASS fetch-age last fetch <TIME>
PASS clean nothing uncommitted or unpushed
WARN hooks core.hooksPath is not set, so the pre-commit check does not run; run memex init <STORE>
PASS check 0 errors, 1 warnings
PASS path 1 memex on PATH
PASS leaks no files in coding tools' own memory
PASS tool-memory ~/.claude does not exist
```

```example
$ memex doctor --store /nonexistent
PASS git version 2.30 or later
PASS config config file parses
FAIL store store /nonexistent: no such file or directory
PASS path 1 memex on PATH
PASS leaks no files in coding tools' own memory
PASS tool-memory ~/.claude does not exist
exit 1
```

## Mistakes to avoid

- Deleting a `LEAK` file before its content is in the store: run `memex capture --from PATH`
  first, review the entry, then delete the file.
- Treating WARN as harmless in a hook: `fetch-age` and `clean` warnings mean other machines
  are not seeing this store's latest knowledge.

## See also

`memex check`, `memex sync`, `memex help neutrality`, `memex help config`.
