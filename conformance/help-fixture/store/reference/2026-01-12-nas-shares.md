+++
title = "NAS shares"
status = "active"
created = 2026-01-12
summary = "Which NAS shares exist and which host mounts each one."
verified = 2026-01-12
+++

## Current state

- `backups`: the restic repository, mounted on alpha at `/mnt/backups`.
- `media`: read-only on beta at `/srv/media`.

## How to re-verify

`showmount -e nas` from either host.
