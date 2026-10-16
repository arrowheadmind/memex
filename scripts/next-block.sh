#!/usr/bin/env bash
# next-block.sh: read and update the work ledger (WORK.md).
#
#   next-block.sh            print the current block, its owner and the handoff rule
#   next-block.sh --hook     same, as Claude Code hook JSON (SessionStart)
#   next-block.sh start ID   mark ID as doing
#   next-block.sh done ID    mark ID as done at HEAD, commit WORK.md, print the handoff
#
# The ledger is the table in WORK.md whose first column is a block id (B0, B1, ...).
set -eu

root=$(cd "$(dirname "$0")/.." && pwd)
ledger="$root/WORK.md"

die() { printf 'next-block: %s\n' "$*" >&2; exit 1; }

# row ID -> "id|block|owner|review|done when|status|done at"
row() {
  awk -F'|' -v id="$1" '
    $2 ~ /^ *B[0-9]+ *$/ { k=$2; gsub(/^ +| +$/, "", k); if (k == id) { print; exit } }
  ' "$ledger"
}
field() { printf '%s' "$1" | awk -F'|' -v n="$2" '{ gsub(/^ +| +$/, "", $n); print $n }'; }

# first row with status doing, else first todo
current_id() {
  awk -F'|' '
    $2 ~ /^ *B[0-9]+ *$/ {
      id=$2; st=$7; gsub(/^ +| +$/, "", id); gsub(/^ +| +$/, "", st)
      if (st == "doing") { print id; found=1; exit }
      if (st == "todo" && first == "") first=id
    }
    END { if (!found && first != "") print first }
  ' "$ledger"
}

describe() {
  local id r
  id=$(current_id)
  if [ -z "$id" ]; then
    printf 'memex work ledger: every block is done.\n'
    return
  fi
  r=$(row "$id")
  printf 'memex work ledger (WORK.md)\n'
  printf 'Current block: %s, %s\n' "$id" "$(field "$r" 3)"
  printf 'Owner: %s. Reviewer: %s. Status: %s.\n' "$(field "$r" 4)" "$(field "$r" 5)" "$(field "$r" 7)"
  printf 'Done when: %s\n' "$(field "$r" 6)"
  printf 'Rule: if you are not %s, stop and tell the operator to switch with /model %s before working on this block.\n' \
    "$(field "$r" 4)" "$(field "$r" 4)"
  printf 'When the block is done: scripts/next-block.sh done %s, then memex capture (the store is the memory).\n' "$id"
}

set_status() {
  local id="$1" status="$2" done_at="${3:-}"
  [ -n "$(row "$id")" ] || die "no block $id in WORK.md"
  awk -F'|' -v OFS='|' -v id="$id" -v st="$status" -v at="$done_at" '
    $2 ~ /^ *B[0-9]+ *$/ {
      k=$2; gsub(/^ +| +$/, "", k)
      if (k == id) { $7=" " st " "; $8=" " at " " }
    }
    { print }
  ' "$ledger" > "$ledger.tmp" && mv "$ledger.tmp" "$ledger"
}

case "${1:-}" in
  "")
    describe
    ;;
  --hook)
    text=$(describe)
    jq -cn --arg t "$text" \
      '{systemMessage: $t, hookSpecificOutput: {hookEventName: "SessionStart", additionalContext: $t}}'
    ;;
  start)
    [ $# -eq 2 ] || die "usage: next-block.sh start ID"
    set_status "$2" doing
    printf 'Block %s is now doing.\n' "$2"
    ;;
  done)
    [ $# -eq 2 ] || die "usage: next-block.sh done ID"
    sha=$(git -C "$root" rev-parse --short HEAD)
    set_status "$2" done "$sha"
    git -C "$root" add -- WORK.md
    git -C "$root" commit -q -m "work: $2 done at $sha" -- WORK.md
    printf 'Block %s closed at %s.\n\n' "$2" "$sha"
    next=$(current_id)
    if [ -z "$next" ]; then
      printf 'HANDOFF: every block is done.\n'
    else
      r=$(row "$next")
      printf 'HANDOFF: next is %s, %s.\n' "$next" "$(field "$r" 3)"
      printf 'Owner: %s. Start a NEW session and run /model %s before any work on it.\n' \
        "$(field "$r" 4)" "$(field "$r" 4)"
    fi
    printf '\nTell the operator the two lines above verbatim, then capture this session with memex capture.\n'
    ;;
  *)
    die "unknown command: $1"
    ;;
esac
