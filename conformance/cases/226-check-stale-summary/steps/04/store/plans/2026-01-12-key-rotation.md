+++
title = "Key rotation"
status = "done"
created = 2026-01-12
summary = "Rotate every backup key once a year; alpha rotated 2026-01-12."
tags = ["backup"]
+++

## Context

Every host moves from rsync to restic, one at a time, with a restore test after each.

## Steps

1. alpha.
2. beta.

## Status 2026-01-12

The new key is in the vault and on alpha; beta still uses the old one until its timer
is moved, which is the next step of the overhaul plan.

## Closed 2026-01-15

Rotated on beta too; the old key is revoked in the vault and a restore of the newest
snapshot succeeded with the new key on both hosts.
