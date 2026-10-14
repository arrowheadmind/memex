# memex help neutrality

Coding tools each have private memory: Claude Code's `~/.claude/projects/*/memory/` and
`~/.claude/CLAUDE.md`, Codex's `~/.codex/`, Cursor's rules files, Copilot's instructions file,
and their successors. Knowledge written there is invisible to every other tool and is lost
when the tool changes. memex keeps knowledge in one place and makes the rule mechanical.

## The rules

1. **Knowledge lives only in the store.** Nothing durable goes into a tool's own memory or
   rules files.
2. **Pointer files contain only the pointer.** A repository's `CLAUDE.md`, `AGENTS.md` and
   `GEMINI.md` hold this single line, followed by a line feed:

   ```
   Read <store>/AGENT.md before starting work, and follow it.
   ```

   `<store>` is the store path as configured. `memex init --pointers DIR` writes the three
   files; a file that already holds the pointer is left as `unchanged`, one with other content
   is left alone and reported unless `--force` replaces it (printing the old content first).
3. **Tool stores are never read as a source.** When knowledge has leaked into one, bring it
   into the store with `memex capture --from PATH`: the file becomes a `raw` journal entry
   titled from its first heading (or its YAML `name:`), with the path as an alias and a first
   line `Imported from PATH (date)`. memex never deletes the original; it prints
   `remove PATH after review` for you to do once the entry is committed.
4. **Integration is one-way.** Tools' hooks call memex (`memex preflight --max-chars 4000` at
   session start). memex knows nothing about tools beyond the pointer text and the paths
   below.
5. **Switching off a tool's own memory is configuration, not memex's job.** For Claude Code,
   that is `CLAUDE_CODE_DISABLE_AUTO_MEMORY=1` or `"autoMemoryEnabled": false` in its settings.

## The pointer check

`memex check --workspace` walks the `workspace` of the config file (`workspace = "~/src"`),
down to depth 4 (`~/src/a/b/c/d`), without entering `.git`, `node_modules`, `vendor`,
`.worktrees`, the store, or any symbolic link to a directory. In each directory it visits:

- `CLAUDE.md`, `CLAUDE.local.md`, `.claude/CLAUDE.md`, `AGENTS.md` and `GEMINI.md` must be the
  pointer line naming this store (`~/memex-store` and its absolute path are the same pointer);
  anything else, a symbolic link included, is `E-POINTER`.
- A git repository (a directory with a `.git` entry) with neither `CLAUDE.md` nor `AGENTS.md`
  is `W-NO-POINTER`; `memex init --pointers <dir>` fixes it.

Workspace findings carry absolute paths and sort with the store's own findings.

## The leak scan

Every regular, non-empty file at these places is a leak, `W-LEAK` in `check --workspace` and a
`LEAK <path>` line under `memex doctor`'s `leaks` check (the same scan):

- `~/.claude/projects/*/memory/**` (every file, at any depth)
- `~/.claude/CLAUDE.md`, `~/.codex/AGENTS.md`, `~/.gemini/GEMINI.md`, unless the file is
  exactly the pointer line, which makes it that tool's global pointer
- in each workspace directory the pointer check visits: `.cursorrules`, `.cursor/rules/**`,
  `.claude/rules/**`, `.github/copilot-instructions.md`, `.windsurfrules`

Symbolic links are never followed and are not files. The fix for each one is
`memex capture --from <path>`, a review of the new entry, then deleting the file.

## The tool-memory check

`memex doctor`'s `tool-memory` check WARNs while Claude Code's auto memory is on. It PASSes
when `~/.claude` does not exist, when `CLAUDE_CODE_DISABLE_AUTO_MEMORY=1` is set, or when
`"autoMemoryEnabled": false` is a top-level member of `~/.claude/settings.json`,
`/etc/claude-code/managed-settings.json` or a `*.json` in `/etc/claude-code/managed-settings.d/`
(checked in that order; the message names the first that disables it).

## AGENT.md

`AGENT.md` at the store root is what every pointer file points at. `memex init` writes a
starting version: how to learn memex, run `preflight` before work, `capture` after it, never
write to a tool's memory, edit then `save`. Operator-specific instructions follow it. Edit it
like any file and commit with `memex save`.
