+++
title = "Rotate the backup key on alpha"
status = "active"
created = 2026-01-04
summary = "Yearly: add a new restic key, re-check a restore, then remove the old key."
tags = ["backup", "runbook"]
keywords = ["key", "restic", "rotation", "secret"]
hosts = ["alpha"]
+++

## When to use

Once a year, or at once if the key may have leaked.

## Steps

1. On `alpha`: `restic key add` with the new passphrase from the secrets store.
2. Run a restore of one small snapshot into `/tmp/restore-test` and compare a file.
3. `restic key remove <old id>`.
4. Update the secrets store and the `backup.nix` module, deploy `alpha`.

## Verify

`restic key list` shows one key, and the next 03:00 run succeeds (check `journalctl -u restic`).
