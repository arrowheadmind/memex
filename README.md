# memex

A command-line tool for a private, git-backed store of Markdown notes: the rules, mistakes,
decisions, runbooks, current facts, tasks, plans and investigations that one operator and
their coding agents accumulate. The people and agents do the thinking. memex is the memory
they share.

The name comes from Vannevar Bush's 1945 *memex*, a "memory extender".

**Status: specification only.** Nothing is implemented yet.

- [SPEC.md](SPEC.md) is the single specification. It is language-neutral and precise enough that
  two implementations produce byte-identical results. Each section is marked Planned or
  Implemented.
- [IMPLEMENTATION.md](IMPLEMENTATION.md) records the choices for the Go implementation.
- `conformance/` (to come) is the language-neutral test suite that any implementation must pass.

Design in brief:

- **The tool is public and the store is private.** The store is a separate git repository at a
  path you configure.
- **One entry per file,** with TOML frontmatter. The id is the filename and the type is the
  directory.
- **Ranked full-text search** (BM25, OR-combined terms, prefix matching). A search that hides
  matches says so, and one that finds nothing never looks like success.
- **Tool-neutral.** Coding tools get a one-line pointer file. Knowledge never lives in a tool's
  private memory.
- **Written in Go,** with a few widely used dependencies. `git` is the only external program.

License: MIT.
