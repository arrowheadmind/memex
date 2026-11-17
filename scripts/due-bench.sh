#!/usr/bin/env bash
# due-bench.sh: time `memex due` and `memex check` on a made-up store (design note
# docs/due-and-stale.md). Usage: scripts/due-bench.sh MEMEX [N]
#
# Builds a store of N entries (default 5000) in a temporary directory: a quarter tasks, some
# with due dates, conditions and dates written in their text, the rest rules, references and
# journals with bodies of a few hundred characters; commits it in one commit, then runs each
# command five times and prints the wall time of each run. Nothing outside the temporary
# directory is read or written.
set -eu
memex=$(cd "$(dirname "$1")" && pwd)/$(basename "$1")
n=${2:-5000}
dir=$(mktemp -d)
trap 'rm -rf "$dir"' EXIT
store=$dir/store
mkdir -p "$store"/{tasks,rules,reference,journal}
printf 'format = 1\nname = "bench"\n' >"$store/memex.toml"
printf '.memex/\n' >"$store/.gitignore"
body='The procedure was run on both machines and the result compared with the previous run; the
numbers match within a percent, and the log is kept with the entry for the next reader.'
statuses=(open in-progress blocked done done done)
for i in $(seq 1 "$n"); do
  id=$(printf '2025-%02d-%02d-entry-%05d' $((i % 12 + 1)) $((i % 28 + 1)) "$i")
  case $((i % 4)) in
  0)
    st=${statuses[$((i % 6))]}
    extra=''
    case $((i % 5)) in
    0) extra=$(printf 'due = 2026-%02d-%02d\n' $((i % 12 + 1)) $((i % 28 + 1))) ;;
    1) extra='due_when = "after the upgrade lands"\n' ;;
    esac
    text=$body
    [ $((i % 7)) -eq 0 ] && text="$body Renew it by 2026-03-01 at the latest."
    printf '+++\ntitle = "Task %d"\nstatus = "%s"\ncreated = %s\nsummary = "A task."\nkeywords = ["bench"]\n'"$extra"'+++\n\n%s\n' \
      "$i" "$st" "${id:0:10}" "$text" >"$store/tasks/$id.md"
    ;;
  1) printf '+++\ntitle = "Rule %d"\nstatus = "active"\ncreated = %s\nsummary = "A rule."\nkeywords = ["bench"]\n+++\n\n%s\n' \
    "$i" "${id:0:10}" "$body" >"$store/rules/$id.md" ;;
  2) printf '+++\ntitle = "Reference %d"\nstatus = "active"\ncreated = %s\nsummary = "A fact."\nkeywords = ["bench"]\nverified = %s\n+++\n\n%s\n' \
    "$i" "${id:0:10}" "${id:0:10}" "$body" >"$store/reference/$id.md" ;;
  3) printf '+++\ntitle = "Rule b%d"\nstatus = "active"\ncreated = %s\nsummary = "A rule."\nkeywords = ["bench"]\n+++\n\n%s\n' \
    "$i" "${id:0:10}" "$body" >"$store/rules/$id-b.md" ;;
  esac
done
git -C "$store" init -q
git -C "$store" add -A
git -C "$store" -c user.name=bench -c user.email=bench@example.invalid commit -q -m seed
echo "store: $n entries"
for cmd in "due" "due --json" "check"; do
  for _ in 1 2 3 4 5; do
    start=$(date +%s%N)
    MEMEX_STORE=$store MEMEX_CONFIG=/dev/null "$memex" $cmd >/dev/null 2>&1 || true
    end=$(date +%s%N)
    printf 'memex %s: %d ms\n' "$cmd" $(((end - start) / 1000000))
  done
done
