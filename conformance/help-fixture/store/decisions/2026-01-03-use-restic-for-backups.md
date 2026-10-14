+++
title = "Use restic for backups"
status = "accepted"
created = 2026-01-03
summary = "restic replaces the rsync job: deduplicated, encrypted, and restorable per snapshot."
tags = ["backup"]
keywords = ["encryption", "rsync", "snapshot"]
hosts = ["alpha"]
+++

## Context

The rsync job copied `/srv` to the NAS nightly with no history and no encryption, and a
restore was a manual copy back.

## Decision

Run restic from `alpha` against the NAS repository at 03:00, with a key kept in the secrets
store and rotated yearly.

## Consequences

Restores are per snapshot; the key rotation needs a runbook ([[2026-01-04-rotate-the-backup-key-on-alpha]]).
