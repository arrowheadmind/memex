# memex — implementation notes (Go)

[SPEC.md](SPEC.md) defines what memex does, independent of language. This file records how the
Go implementation does it: choices that don't change observable behavior.

The intent is that memex can later be re-implemented in another language as a learning exercise.
The spec and the conformance suite (`conformance/`) are the contract, and nothing here is.

## Toolchain

- **Go:** the current stable release at the time of the first code commit, pinned by the `go`
  and `toolchain` lines in `go.mod`.
- **Module path:** `github.com/arrowheadmind/memex`. It names the canonical public home, which
  is the GitHub organization `arrowheadmind`, even though development happens on the operator's
  Gitea (`eduardo/memex`) and GitHub receives the repository later through a Gitea push mirror.
  The path is baked into every import and into `go install`, and changing it after a release
  breaks installs, so it is fixed here before the first line of code. Hosting is not checked
  until someone runs `go install`, by which time the mirror exists.
- **Checks in CI:** `gofmt -l` must produce no output, plus `go vet ./...` and
  `go test ./...`.

## Dependencies

Dependencies are allowed when they are widely used and actively maintained. Each one is justified
here, pinned by `go.sum`, and reviewed when bumped.

| Need | Choice | Why not the standard library |
|---|---|---|
| Reading TOML (§3.2, §4, §5.1) | `github.com/BurntSushi/toml` | Go has no TOML in the stdlib. memex *writes* canonical TOML with its own small writer (§5.1), so this library is only used for reading. |
| Unicode NFKD (§5.4, §8.2) | `golang.org/x/text/unicode/norm` | The Go stdlib lacks normalization. `x/text` is maintained by the Go team. Category Mn and simple lowercase come from the stdlib `unicode` package. |
| Command-line parsing (§10) | `github.com/spf13/pflag` | The stdlib `flag` stops at the first positional argument. §10 requires options anywhere, and `--opt=value` / `--opt value` both. Subcommand dispatch is written by hand; no framework. pflag does **not** refuse `--title -x` (it takes `-x` as the value) and prints its own usage on error, so argv is pre-scanned for the §10 rule, and the flag set runs with `ContinueOnError` and a discarded output. |
| Flock (§9.1) | `golang.org/x/sys/unix` | `unix.Flock` with `LOCK_EX|LOCK_NB`; the `syscall` package is frozen. |

Deliberately not used:
- **SQLite.** §8 defines the BM25 math exactly, so search is implemented directly in Go over
  the parsed entries. That's well within budget for a store of a few thousand entries. The §8.6
  cache is added only if a measured `preflight` exceeds 300 ms on the real store. If that
  happens, the candidate is `modernc.org/sqlite`, which is pure Go and includes FTS5, used purely
  as a cache. Results must stay identical (§8.6).
- **A CLI framework (cobra).** Help text and errors must stay under our control (§10).
- **A git library.** memex runs the `git` executable (§9). Behavior stays identical to what a
  person would type, and git's own configuration (hooks, credentials, signing) applies.

## Package layout

```
cmd/memex/            main: argument dispatch, exit codes
internal/config/      §3 store resolution, config file, environment
internal/store/       §4 layout, manifest, entry enumeration
internal/entry/       §5 parse, canonical write, body normalization, content measure
internal/slug/        §5.4 ids
internal/links/       §6 link extraction (fenced-code and inline-code aware)
internal/check/       §7 findings and codes
internal/search/      §8 tokenizer, query parser, BM25, ranking, snippets, output
internal/gitx/        thin wrapper around running git, with captured stderr
internal/write/       §9 lock, write protocol, outcomes
internal/commands/    one file per §11 command
internal/neutral/     §12 pointer check, leak scan, tool-memory check
internal/importv3/    §13 brain v3 importer and accounting
internal/jsonx/       the §10 JSON writer (encoding/json cannot produce its escaping)
internal/help/        §16 help: prints the embedded pages verbatim
internal/spec/        single source of commands, options, types, statuses, fields and exit
                      codes; the parser, validator and `help --json` are all driven from it
help/                 §16 help pages (data, language-neutral, shared by any implementation),
                      plus help/embed.go, a tiny package whose only job is
                      `//go:embed *.md` (embed patterns cannot leave their package directory,
                      so `../../help` from internal/help is not an option)
conformance/runner/   §15 runner (Go, but it only drives the binary)
conformance/cases/    §15 cases (data, language-neutral)
```

## Behavior-preserving details to get right

- **JSON (§10):** `encoding/json` always escapes U+2028 and U+2029, escapes `<`, `>` and `&`
  unless told not to, and (since Go 1.22) writes U+0008/U+000C as `\b`/`\f` where §10 requires
  `\u0008`/`\u000c`. Checked against Go 1.27. `internal/jsonx` is a small writer: ordered objects,
  arrays, strings escaped exactly as §10 says, integers, `null`, and `score` through
  `strconv.FormatFloat(x, 'f', 6, 64)` (Go rounds half to even on the exact binary value, as
  required). No reflection, no struct tags.
- **Rounding (§8.3):** `strconv.ParseFloat(strconv.FormatFloat(s, 'g', 9, 64), 64)` is exactly
  the rounded score. Sum in the order §8.3 fixes; don't reorder for speed.
- **Unicode (§5.1):** `unicode` (stdlib) and `golang.org/x/text/unicode/norm` each ship their
  own tables, which can be a Unicode version apart. Pin `x/text` to a release whose tables
  match the toolchain's `unicode.Version`, and add a test that asserts `unicode.Version` and
  `norm.Version` are equal (both `17.0.0` with Go 1.27). If no `x/text` release has 17.0 tables
  when B1 pins it, stop and report; do not pin an older Go to make them agree.
- **Lowercase (§5.4, §8.2):** `unicode.ToLower` per rune is the simple mapping. Don't use
  `strings.ToLower` together with special-casing tables.
- **TOML (§5.1, §5.3):** `BurntSushi/toml` decodes `2026-09-12` and `2026-09-12T00:00:00Z` both
  into `time.Time`. Decode into `map[string]any` with `toml.MetaData`, and use
  `md.Type(key) == "Local Date"` to enforce the date kind; `md.Undecoded()` is not needed with a
  map, but every key is checked against §5.3 by name.
- **Git (§3.3, §9):** `internal/gitx` builds the child environment explicitly (scrub list,
  `GIT_TERMINAL_PROMPT=0`, `GIT_EDITOR=true`, `GIT_SSH_COMMAND`, `LC_ALL=C`) and runs every
  network command under `context.WithTimeout(network_timeout)`. On timeout the process group is
  killed, so an ssh child doesn't linger.
- **Lock (§9.1):** `unix.Flock(fd, LOCK_EX|LOCK_NB)`, polled every 100 ms, up to 60 s (10 s
  from `preflight`). It is POSIX only, matching §14.
- **Atomic write (§9.1 step 4):** use `os.CreateTemp` in the same directory, then `Write`,
  `Sync`, `Close`, `os.Rename`, and `Sync` on the directory.
- **Current-directory attribution (§5.5):** the only place that calls `os.Getwd`, and it runs
  `git rev-parse --git-common-dir` there, not `--show-toplevel`. Any other use of `os.Getwd` is
  a review finding.
- **Snippets (§8.5):** tokenize the original line into `[]struct{start, end int; norm string}`
  by rune index, and do the window arithmetic in runes, never bytes.
- **Exit codes:** `main` maps typed errors to §10 codes. A panic is recovered at the top level,
  printed as `memex: internal error: …`, and exits 70.

## Help and the agent-first stance

- **Design for agents first** (SPEC Principle 0, §17). Agents are memex's main users. When a
  choice of wording or shape comes up, pick what an agent parses unambiguously and cheaply.
- **One source for the interface.** `internal/spec` holds a table of commands and options with
  summaries. The parser registers options from that table, so an option can't exist without a
  summary, and `help --json` serializes the same table. The prose pages in `help/` are checked
  against it by the conformance runner (§16.9).
- **Suggestions.** One shared Levenshtein helper, used everywhere a name is unknown (§16.2,
  §17.4).

## Testing

- **Unit tests** live next to each package (`*_test.go`) and cover the pure functions: slug,
  canonical TOML writer, tokenizer, BM25, link extraction and content measure.
- **Conformance:** run `go run ./conformance/runner --memex ./memex` after `go build`. Every
  implemented §-section has cases (§15).
- **Never point a mutating test at a real store.** The runner and the tests create temporary
  stores with a bare `origin`.

## Packaging

- **Nix:** a flake in this repo exposes `packages.default` via `buildGoModule`, with `vendorHash`
  pinned. It is the only durable installation route on the operator's machines.
- **Other users:** `go install github.com/arrowheadmind/memex/cmd/memex@vX.Y.Z` and release
  binaries.
- **Version:** set at build time with `-ldflags "-X main.version=…"` from the git tag, and
  `0.0.0-dev` otherwise. The tag is the single source. Every release gets a `CHANGELOG.md` entry.
