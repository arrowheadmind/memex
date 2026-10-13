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

# Every conformance fixture file must be tracked: a case's own store/.gitignore can hide
# files from `git add`, and a clean clone would then run a different case.
untracked=$(git ls-files --others --exclude-standard conformance/cases)
if [ -n "$untracked" ]; then
  printf 'conformance: untracked fixture files (use git add -f):\n%s\n' "$untracked" >&2
  exit 1
fi
