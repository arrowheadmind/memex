#!/usr/bin/env bash
# setup.sh: build the world for one cold-agent round (SPEC §17.1, WORK.md B6).
#
#   scripts/cold-agent/setup.sh ROUND_DIR
#
# Creates ROUND_DIR with: bin/memex (the binary built from this checkout), store/ (a git
# repository seeded from conformance/help-fixture/store with a bare origin and upstream),
# home/notes/body.md, and env.sh to source. The agent under test gets ONLY the prompt in
# PROMPT.md with the paths filled in; it must learn memex from AGENT.md and `memex help`.
set -eu
round=${1:?usage: setup.sh ROUND_DIR}
repo=$(cd "$(dirname "$0")/../.." && pwd)
mkdir -p "$round/bin" "$round/home"
( cd "$repo" && go build -o "$round/bin/memex" ./cmd/memex )
cp -r "$repo/conformance/help-fixture/store" "$round/store"
cp -r "$repo/conformance/help-fixture/home/." "$round/home/"
git -C "$round/store" init -q
git -C "$round/store" -c user.name=test -c user.email=test@example.invalid add -A
git -C "$round/store" -c user.name=test -c user.email=test@example.invalid commit -q -m "seed"
git clone -q --bare "$round/store" "$round/origin.git"
git -C "$round/store" remote add origin "$round/origin.git"
git -C "$round/store" fetch -q origin
git -C "$round/store" branch -q --set-upstream-to=origin/main 2>/dev/null \
  || git -C "$round/store" branch -q --set-upstream-to=origin/master
cat > "$round/env.sh" <<ENV
export PATH="$round/bin:\$PATH"
export MEMEX_STORE="$round/store"
export HOME="$round/home"
export MEMEX_CONFIG="$round/home/memex.toml"
ENV
: > "$round/home/memex.toml"
sed "s|__STORE__|$round/store|g; s|__ENV__|$round/env.sh|g" "$repo/scripts/cold-agent/PROMPT.md" > "$round/prompt.md"
echo "round ready: $round"
echo "prompt: $round/prompt.md"
