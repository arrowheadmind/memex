+++
title = "NAS backup share"
created = 2026-01-13
status = "active"
summary = "The restic repository lives on the NAS share backup, mounted on alpha at /mnt/backup."
keywords = ["nas", "restic", "repository"]
tags = ["backup"]
verified = 2026-01-13
+++

## Current state

The restic repository is the NAS share `backup`, mounted on `alpha` at `/mnt/backup` by the
`backup.nix` module.

## How to re-verify

On `alpha`: `findmnt /mnt/backup` and `restic snapshots` both succeed.
