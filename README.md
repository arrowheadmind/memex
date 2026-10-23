# memex

A command-line tool for a private, git-backed store of Markdown notes: the rules, mistakes,
decisions, runbooks, current facts, tasks, plans and investigations that one operator and
their coding agents accumulate. The people and agents do the thinking. memex is the memory
they share.

The name comes from Vannevar Bush's 1945 *memex*, a "memory extender".

**Status:** 0.1.0 released 2026-10-17; the specification and the Go implementation are
complete for that release, and later changes are listed under "Unreleased" in
[CHANGELOG.md](CHANGELOG.md). Releases are git tags (`vMAJOR.MINOR.PATCH`).
Pushing a `v*` tag publishes the release page (`.gitea/workflows/release.yml`: the CHANGELOG
section as the body, the Linux binary and its sha256 file attached); for an existing tag, Actions,
Release, Run workflow with the tag.

**Check:** `scripts/check.sh` (gofmt, go vet, go test, build, then the conformance suite against
the built binary). With Nix: `nix develop --command scripts/check.sh`.

**Docs:** [SPEC.md](SPEC.md) (the specification), [IMPLEMENTATION.md](IMPLEMENTATION.md) and
`memex help`.

- [SPEC.md](SPEC.md) is the single specification. It is language-neutral and precise enough that
  two implementations produce byte-identical results. Each section is marked Planned or
  Implemented.
- [IMPLEMENTATION.md](IMPLEMENTATION.md) records the choices for the Go implementation.
- `conformance/` is the language-neutral test suite that any implementation must pass.

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

## Install

With Nix (flakes), from a checkout or straight from the repository:

```
nix profile install .#memex          # in a checkout
nix build .#memex && ./result/bin/memex version
```

A NixOS or home-manager configuration takes the flake as an input and adds
`inputs.memex.packages.${system}.default` to its packages. With Go 1.27 or later:

```
go install github.com/arrowheadmind/memex/cmd/memex@latest
```

CI runs `scripts/check.sh` (`nix develop -c scripts/check.sh`) on every push.

Then `memex help` for the overview, or `memex help agents` for an agent's rules of engagement.

**License:** MIT (see [LICENSE](LICENSE)).
