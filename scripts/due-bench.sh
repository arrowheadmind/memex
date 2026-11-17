#!/usr/bin/env bash
# due-bench.sh: time `memex due` and `memex check` on a made-up store (design note
# docs/due-and-stale.md). Usage: scripts/due-bench.sh MEMEX [N]
#
# Builds a store of N entries (default 5000) in a temporary directory: a quarter tasks, some
# with due dates, conditions and dates written in their text, the rest rules, references and
# journals with bodies of a few hundred characters; commits it in one commit, then runs each
# command five times and prints the wall time of each run. Before timing, each command is run
# once and its exit code and output are checked (due: exit 0 and a section line or JSON
# object; check: exit 0 or 1 and its summary line), so a crash, a usage error or a store
# memex refuses stops the benchmark instead of being timed. Nothing outside the temporary
# directory is read or written.
set -eu
memex=$(cd "$(dirname "$1")" && pwd)/$(basename "$1")
n=${2:-5000}
dir=$(mktemp -d)
trap 'rm -rf "$dir"' EXIT
store=$dir/store
mkdir -p "$store"/{tasks,rules,reference,journal}
printf 'format = 2\nname = "bench"\nstale_days = 14\n' >"$store/memex.toml"
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
    0) extra=$(printf 'due = 2026-%02d-%02d' $((i % 12 + 1)) $((i % 28 + 1)))'\n' ;;
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
run() { MEMEX_STORE=$store MEMEX_CONFIG=/dev/null "$memex" "$@"; }
# expect CMD OK-EXITS PATTERN: run CMD once; stop unless its exit code is one of OK-EXITS and
# its standard output matches PATTERN (an extended regular expression) and stderr is empty.
expect() {
  cmd=$1 ok=$2 pattern=$3
  set +e
  run $cmd >"$dir/out" 2>"$dir/err"
  code=$?
  set -e
  case " $ok " in *" $code "*) ;; *)
    printf 'due-bench: memex %s exited %d (want %s):\n' "$cmd" "$code" "$ok" >&2
    cat "$dir/err" >&2
    exit 1
    ;;
  esac
  if ! grep -Eq "$pattern" "$dir/out" || [ -s "$dir/err" ]; then
    printf 'due-bench: memex %s printed unexpected output:\n' "$cmd" >&2
    head -c 2000 "$dir/out" "$dir/err" >&2
    exit 1
  fi
}
expect "due" "0" '^past due: [0-9]+$'
expect "due --json" "0" '^\{"today":"[0-9-]+",.*"skipped":\[\]\}$'
expect "check" "0 1" '^check: [0-9]+ errors, [0-9]+ warnings'
for cmd in "due" "due --json" "check"; do
  for _ in 1 2 3 4 5; do
    start=$(date +%s%N)
    set +e
    run $cmd >/dev/null 2>&1
    code=$?
    set -e
    end=$(date +%s%N)
    case "$cmd:$code" in due*:0 | check:0 | check:1) ;; *)
      printf 'due-bench: memex %s exited %d while timing\n' "$cmd" "$code" >&2
      exit 1
      ;;
    esac
    printf 'memex %s: %d ms\n' "$cmd" $(((end - start) / 1000000))
  done
done
