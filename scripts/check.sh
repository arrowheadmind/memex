#!/usr/bin/env bash
# check.sh: every check CI runs (IMPLEMENTATION.md, "Checks in CI"), in order:
# gofmt, go vet, go test, go build, then the conformance runner against the built binary.
# Needs go on PATH (on NixOS: nix shell nixpkgs#go_1_27 -c scripts/check.sh).
set -eu
cd "$(dirname "$0")/.."

unformatted=$(gofmt -l .)
if [ -n "$unformatted" ]; then
  printf 'gofmt: these files need formatting:\n%s\n' "$unformatted" >&2
  exit 1
fi
go vet ./...
go test ./...
go build -o memex ./cmd/memex
go run ./conformance/runner --memex ./memex

# Every conformance fixture file must be tracked, ignored ones included: a case's own
# store/.gitignore can hide files from `git add`, and a clean clone would then run a different
# case. Skipped with a note when this is not a git checkout (an archive copy).
if git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  untracked=$(git ls-files --others --exclude-standard conformance/cases)
  ignored=$(git ls-files --others --ignored --exclude-standard conformance/cases)
  if [ -n "$untracked$ignored" ]; then
    printf 'conformance: untracked fixture files (use git add -f):\n%s\n%s\n' "$untracked" "$ignored" >&2
    exit 1
  fi
else
  echo "conformance: not a git checkout, fixture tracking not checked"
fi
