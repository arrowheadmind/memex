+++
title = "Backup overhaul"
status = "active"
created = 2026-01-08
summary = "Move every host to restic, with a tested restore and a yearly key rotation."
tags = ["backup"]
keywords = ["restic", "restore"]
+++

## Context

[[2026-01-03-use-restic-for-backups]] decided the tool; this plan does the rollout.

## Steps

1. alpha (done 2026-01-03).
2. beta ([[2026-01-11-replace-rsync-on-beta]]).
3. Document the restore test ([[2026-01-07-document-the-restore-test]]).

## Verification

A restore from each host's latest snapshot, compared file by file.
