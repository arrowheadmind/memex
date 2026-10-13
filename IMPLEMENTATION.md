# memex — implementation notes (Go)

[SPEC.md](SPEC.md) defines what memex does, independent of language. This file records how the
Go implementation does it: choices that don't change observable behavior.

The intent is that memex can later be re-implemented in another language as a learning exercise.
The spec and the conformance suite (`conformance/`) are the contract, and nothing here is.

## Toolchain

- **Go:** the current stable release at the time of the first code commit, pinned by the `go`
  and `toolchain` lines in `go.mod`.
- **Module path:** *to decide before the first tag*. It follows where the canonical public home
  will be. Changing a module path after release breaks `go install` for everyone, so it is
  chosen once.
- **Checks in CI:** `gofmt -l` must produce no output, plus `go vet ./...` and
  `go test ./...`.

## Dependencies

Dependencies are allowed when they are widely used and actively maintained. Each one is justified
here, pinned by `go.sum`, and reviewed when bumped.

| Need | Choice | Why not the standard library |
|---|---|---|
| Reading TOML (§3.2, §4, §5.1) | `github.com/BurntSushi/toml` | Go has no TOML in the stdlib. memex *writes* canonical TOML with its own small writer (§5.1), so this library is only used for reading. |
| Unicode NFKD (§5.4, §8.2) | `golang.org/x/text/unicode/norm` | The Go stdlib lacks normalization. `x/text` is maintained by the Go team. Category Mn and simple lowercase come from the stdlib `unicode` package. |
| Command-line parsing (§10) | `github.com/spf13/pflag` | The stdlib `flag` stops at the first positional argument. §10 requires options anywhere, and `--opt=value` / `--opt value` both. Subcommand dispatch is written by hand; no framework. |

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
conformance/runner/   §15 runner (Go, but it only drives the binary)
conformance/cases/    §15 cases (data, language-neutral)
```

## Behavior-preserving details to get right

- **JSON (§8.5, §11):**
  - Use `json.Encoder` with `SetEscapeHTML(false)`, so `<`, `>` and `&` are written literally.
  - Use structs to fix key order, and pointer fields for `null`.
  - Format `score` with `strconv.FormatFloat(x, 'f', 6, 64)` via a custom marshaler.
- **Rounding (§8.3):** compare scores after `strconv.ParseFloat(strconv.FormatFloat(s, 'g', 9, 64), 64)`.
- **Lowercase (§5.4, §8.2):** `unicode.ToLower` per rune is the simple mapping. Don't use
  `strings.ToLower` together with special-casing tables.
- **Lock (§9.1):** `syscall.Flock(fd, LOCK_EX|LOCK_NB)`, polled every 100 ms, up to 60 s.
  It is POSIX only, matching §14.
- **Atomic write (§9.1 step 4):** use `os.CreateTemp` in the same directory, then `Write`,
  `Sync`, `Close` and `os.Rename`.
- **Current-directory attribution (§5.5):** the only place that calls `os.Getwd`. Any other use
  is a review finding.
- **Exit codes:** `main` maps typed errors to §10 codes. A panic is recovered at the top level,
  printed as `memex: internal error: …`, and exits 70.

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
- **Other users:** `go install <module>/cmd/memex@vX.Y.Z` and release binaries.
- **Version:** set at build time with `-ldflags "-X main.version=…"` from the git tag, and
  `0.0.0-dev` otherwise. The tag is the single source. Every release gets a `CHANGELOG.md` entry.
