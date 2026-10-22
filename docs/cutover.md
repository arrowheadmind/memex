# Cutover from brainctl v3 to memex

An ordered checklist for the operator. Nothing here has been run by an agent. Each step has a
command, a check, and a way back. Do the steps in order; stop at the first check that fails.

The repository is public, so this file names no machine, path or person. Set the variables
once, in the shell you run the steps from, with your own values (the examples are the
specification's; `$APPS` is split on spaces, so write `$HOME` rather than `~` in it):

```sh
STORE=~/path/to/store              # the memex store (a clone of its private remote)
WS=~/src                            # the workspace: the tree that holds your repositories
BRAIN=~/src/brain                   # the brainctl v3 brain (the primary import source)
APPS="$HOME/src/apps/alpha $HOME/src/apps/beta"   # application repositories with a .brain/
MEMEX_CFG=~/.config/memex/config.toml     # or another path, see step 3
```

`memex doctor` is the overall check: run it after every step and read the line the step
names. A FAIL line stops the cutover.

## 0. Before you start

- [ ] memex is installed from this repository's flake and is the only `memex` on `PATH`.

  ```sh
  nix profile install .#memex            # in a checkout of the tool repository
  memex version
  memex doctor | grep -E '^(PASS|WARN|FAIL) (git|path) '
  ```

  Check: `memex version` prints `memex <version> (store format 1)`; `git` and `path` PASS.
  Back: remove the package from the profile or configuration it was added to.

- [ ] The store is a clean clone of its remote, and `check` has no errors.

  ```sh
  MEMEX_STORE=$STORE memex doctor
  ```

  Check: `store`, `format`, `upstream`, `clean`, `hooks` and `check` PASS. A `hooks` WARN is
  fixed with `memex init $STORE` (it only adds what is missing).

## 1. Bring the brain's last writes over (delta import)

The brain kept receiving writes after the store was imported. This step must run **before**
the brain is renamed (step 8): the importer rewrites path references using the source paths
as given, so the paths must still be the ones the bodies mention.

- [ ] Freeze the brain: no agent session writes to it from here on (close sessions, or tell
  agents to stop capturing with brainctl).
- [ ] Import what is new.

  ```sh
  git -C $STORE status --short          # must print nothing
  memex import brain-v3 $BRAIN $APPS --into $STORE
  ```

  Check: exit 0; the summary's `imported <N> new entries` and the `already in the store` line;
  `memex --store $STORE check --errors-only` ends `check: 0 errors, …`. Read
  `$STORE/import-report-<today>.json`: `delta.added` lists what was written, `delta.changed`
  lists entries whose body differs between the brain and the store. For each changed entry,
  compare `memex show <id>` with the brain's file and edit the store entry by hand if the
  brain's version has something the store lacks.
- [ ] Commit it.

  ```sh
  memex --store $STORE save -m "import: delta from brain@$(git -C $BRAIN rev-parse --short HEAD)"
  ```

  Check: the outcome line is `pushed <sha>`.
  Back: before `save`, `git -C $STORE checkout . && git -C $STORE clean -fd`; after it,
  `git -C $STORE revert HEAD` then `memex --store $STORE sync`.

## 2. Reword the Always rules that name the old tool

`memex preflight` prints every Always rule at session start. Rules that still tell agents to
use `brainctl` or `.brain/` would contradict the new hook.

- [ ] List them and edit each one.

  ```sh
  memex --store $STORE list --type rule --fields id,title | grep -iE 'brain|\.brain' || true
  memex --store $STORE preflight --no-sync --max-chars 4000 | grep -inE 'brainctl|\.brain'
  ```

  Edit the files under `$STORE/rules/`, then `memex --store $STORE save -m "always rules: name memex"`.
  Check: the second command prints nothing.
  Back: `git -C $STORE revert HEAD`, then `memex --store $STORE sync`.

## 3. Configuration

memex reads `MEMEX_STORE` and the config file (`memex help config`). Put the store and the
workspace in the config file; `MEMEX_STORE` alone is enough for everything except
`check --workspace` and the workspace half of the leak scan.

- [ ] Write the config file.

  ```sh
  mkdir -p "$(dirname "$MEMEX_CFG")"
  printf 'store = "%s"\nworkspace = "%s"\n' "$STORE" "$WS" > "$MEMEX_CFG"
  ```

  If `~/.config` is not writable by you (owned by root), either have it created for you
  (`sudo install -d -o $USER ~/.config/memex`), or keep the file elsewhere and point memex at
  it with `MEMEX_CONFIG`, set for every login and for the hook environment (step 4), for
  example in the NixOS or home-manager session variables:

  ```sh
  export MEMEX_CONFIG=$HOME/.local/share/memex/config.toml   # or this, without a config file:
  export MEMEX_STORE=$STORE
  ```

  Check: in a new shell, from a directory outside the store, `memex doctor` shows `config`
  PASS and `store` PASS with the right path.
  Back: delete the file and unset the variables.

## 4. Session-start hook: brainctl out, memex in

The SessionStart hook is the integration (§12 rule 4). It runs `memex preflight` with no
query: sync, then the Always rules. It takes no `--store`: the store comes from the config
file or `MEMEX_STORE`, never from the directory the session starts in.

- [ ] Replace the brainctl hook wherever it is defined (a managed-settings drop-in under
  `/etc/claude-code/managed-settings.d/` written by the system configuration, or
  `~/.claude/settings.json`) with:

  ```json
  {"hooks":{"SessionStart":[{"matcher":"startup|resume","hooks":[
    {"type":"command","command":"memex preflight --max-chars 4000","timeout":60}]}]}}
  ```

  Use the absolute path of the packaged binary if the hook environment's `PATH` may not
  have `memex`, and make sure the hook environment has `MEMEX_CONFIG` or `MEMEX_STORE` when
  the config file is not at its default path. Rebuild or deploy the configuration.

  Check: run the command exactly as the hook will, from a directory that is not the store:

  ```sh
  cd /tmp && memex preflight --max-chars 4000; echo "exit=$?"
  ```

  It prints `store: up to date` (or another `store:` line), a blank line, `## Always` and the
  rules, and `exit=0`. Then start a new agent session and confirm the Always rules appear in
  its context and no brainctl output does.
  Back: restore the previous hook definition and redeploy.

## 5. Pointer files in every repository

- [ ] See what the pointer check finds.

  ```sh
  memex check --workspace --errors-only --limit 0
  memex check --workspace --limit 0 | grep -E ': (E-POINTER|W-NO-POINTER) '
  ```

- [ ] For each `E-POINTER` file: if it holds knowledge, `memex capture --from <path>` first
  (review the new entry), then replace it. For each `W-NO-POINTER` repository, write the
  files. One line each:

  ```sh
  memex check --workspace --limit 0 | sed -n 's/: W-NO-POINTER .*//p' | while read -r d; do memex init --pointers "$d"; done
  memex init --pointers <dir> --force     # for a dir with an E-POINTER CLAUDE.md, AGENTS.md or GEMINI.md
  ```

  `CLAUDE.local.md` and `.claude/CLAUDE.md` are not written by `init`: after capturing their
  content, delete them. Commit the pointer files in each repository with its own workflow.
  Check: `memex check --workspace` reports no `E-POINTER` and no `W-NO-POINTER`.
  Back: `git -C <dir> checkout -- CLAUDE.md AGENTS.md GEMINI.md` (or `git rm` new ones);
  `--force` printed every replaced file's old content, so keep that output.

## 6. Leaks: tool memory into the store

- [ ] List them.

  ```sh
  memex doctor | sed -n 's/^LEAK //p'
  ```

- [ ] Capture each one. A file the first import already brought in with `--leak` is a no-op
  (`exists <path>`, `nothing to commit`), so every `LEAK` line gets the same command:

  ```sh
  memex doctor | sed -n 's/^LEAK //p' | while read -r p; do memex capture --from "$p"; done
  ```

  Review each new entry (`memex show <id>`; fix the title or summary with `memex set`), then
  delete the files: every capture printed `remove <path> after review`. A `MEMORY.md` that
  only indexes the other files becomes a short entry of its own; retire it with
  `memex set <id> status=superseded superseded_by=<id of one of the captured files>` or keep it.
  Check: `memex doctor` shows `PASS leaks`.
  Back: the files are only deleted after their content is committed; `git -C $STORE revert`
  undoes a capture.

## 7. Switch off Claude Code's own memory

- [ ] Set `"autoMemoryEnabled": false` in `~/.claude/settings.json` (or in the managed
  settings drop-in of step 4, which every user on the machine then gets):

  ```sh
  f=~/.claude/settings.json; tmp=$(mktemp)
  jq '.autoMemoryEnabled = false' "$f" > "$tmp" && mv "$tmp" "$f"
  ```

  `CLAUDE_CODE_DISABLE_AUTO_MEMORY=1` in the session environment works too.
  Check: `memex doctor` shows `PASS tool-memory Claude Code auto memory is disabled in …`.
  Back: remove the key (or the variable).

## 8. Retire the application `.brain/` directories

Step 1 imported them; the delta report shows nothing pending from them.

- [ ] In each application repository:

  ```sh
  for a in $APPS; do git -C "$a" rm -r -q .brain && git -C "$a" commit -q -m "Retire .brain: knowledge moved to the memex store"; done
  ```

  Push each with its usual workflow. Check: `git -C <app> ls-files .brain` prints nothing,
  and `memex search "<a phrase from one of its entries>"` still finds it in the store.
  Back: `git -C <app> revert HEAD`.

## 9. Rename the old brain and take brainctl off PATH

- [ ] Rename the checkout so nothing finds it by its old path by accident:

  ```sh
  mv "$BRAIN" "$BRAIN-v3"
  ```

- [ ] Remove `brainctl` from the profile or system configuration that installs it, and
  redeploy.

  Check: `command -v brainctl` prints nothing; `memex doctor` has no FAIL; a new agent
  session starts with `memex preflight` output only.
  Back: `mv "$BRAIN-v3" "$BRAIN"` and reinstall brainctl. The renamed checkout stays as the
  archive; its remote is unchanged.

## 10. Publish the tool (the store stays private)

The tool repository is mirrored to GitHub; the store is never mirrored anywhere.

- [ ] Before the first push, audit the tool repository for operator-specific text
  (Principle 11) across the whole history, not only the tree:

  ```sh
  git -C <tool checkout> log -p --all | grep -inE '<your hostnames>|<your domains>|<your name>' | head
  ```

  Anything found (ledger notes, replay data built from the real store, a stray binary) is
  removed and, if it is in history, the history rewritten before the mirror exists.
- [ ] In Gitea, on the tool repository: Settings → Repository → Mirror Settings → Push
  Mirror: remote `https://github.com/arrowheadmind/memex.git`, username, and a GitHub
  fine-grained token with *Contents: read and write* on that one repository; enable "sync
  when new commits are pushed". Or through the API:

  ```sh
  curl -fsS -X POST -H "Authorization: token $GITEA_TOKEN" -H 'Content-Type: application/json' \
    "$GITEA_URL/api/v1/repos/<owner>/memex/push_mirrors" \
    -d '{"remote_address":"https://github.com/arrowheadmind/memex.git","remote_username":"<github user>","remote_password":"'"$GITHUB_TOKEN"'","interval":"8h0m0s","sync_on_commit":true}'
  ```

  Check: `git ls-remote https://github.com/arrowheadmind/memex.git refs/heads/main` prints the
  same hash as `git -C <tool checkout> rev-parse main`; the store's repository has no push
  mirror (Settings → Repository → Mirror Settings is empty) and stays private.
  Back: delete the push mirror in Gitea; delete or make private the GitHub repository.
  After: other machines can install straight from GitHub with
  `nix profile install github:arrowheadmind/memex` (or the flake as a configuration input).

## After the cutover

- `memex doctor` on each machine; `fetch-age` and `clean` PASS mean the store is in sync.
- `memex help agents` is what a new agent reads; `AGENT.md` in the store points there.
