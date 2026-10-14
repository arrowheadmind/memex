# memex help concepts

The model, and why it is shaped this way. memex replaces a predecessor that ran for five
months and 2,300 commits; every principle below names the failure it prevents.

## Terms

| term | meaning |
|---|---|
| store | A git repository with `memex.toml` at its root, holding entries. Private, operator-owned. |
| entry | One `.md` file directly inside a type directory. |
| type | The kind of entry, given by its directory. |
| id | The filename without `.md`, `YYYY-MM-DD-slug`. Unique across the store, never changed. |
| frontmatter | The TOML block between `+++` lines at the top of an entry. |
| body | Everything after the frontmatter: free Markdown. |
| pointer file | `CLAUDE.md`, `AGENTS.md` or `GEMINI.md` holding one line that points at the store's `AGENT.md`. |
| today | The current date in the local time zone, or `MEMEX_TODAY`. |

## Principles

0. **The primary user is a language-model agent.** Humans read the same output and must find
   it clear, but when the two pull apart the agent's needs decide: bounded output, fixed
   shapes, paths that paste back, errors that name the fix.
1. **Mechanism over memory.** A rule that depends on being remembered has already failed.
   Everything memex can check, it checks (`check`, the pre-commit hook, refusals).
2. **One fact, one place.** Metadata lives only in the frontmatter; the id is the filename and
   the type is the directory. Nothing derived from entries (lists, indexes) is committed.
3. **One entry, one file.** Never accumulate knowledge by appending to a shared file. The
   predecessor's shared files reached 1 MB and 19,865 lines.
4. **Current truth is visible; history is out of the way.** A replaced entry changes status
   and leaves default search. It is not annotated in place; git keeps the history.
5. **A miss must look like a miss.** Search ranks instead of filtering to nothing, says what
   it hid, and never ends in an empty success.
6. **No ambient location.** One store, named by configuration, never inferred from the
   current directory.
7. **Tool-neutral.** No knowledge lives in a coding tool's private store.
8. **Small surface.** Knowledge only. Source worktrees, host closeout and workspace sync are
   someone else's job.
9. **Never interactive.** memex never prompts and never opens an editor. Agents run it.
10. **Honest outcomes.** Every write ends in exactly one reported state. `pushed` is never
    printed unless the push succeeded.
11. **Public tool, private store.** The tool contains no operator-specific hostnames, paths or
    policy; those live in the store or in configuration.

## How the pieces fit

- `preflight` is the start-of-task ritual: sync, the `always` rules, a search for the task.
- `capture` writes a journal entry; `promote` turns what it taught into a rule, runbook,
  decision or reference, and marks the journal entry `promoted`.
- `check` validates the whole store and runs as the pre-commit hook, so a broken link or a
  missing field never lands.
- `list` replaces the predecessor's hand-maintained index; `show` prints an entry with the
  entries that cite, supersede, link or contain it.
