+++
title = "Rotate the backup key"
status = "done"
created = 2026-01-10
summary = "Done 2026-01-15: key rotated on alpha and beta, old key revoked, restore re-tested."
tags = ["backup"]
+++

## Goal

A new restic key on alpha and beta, and the old key revoked in the vault.

## Done when

A restore with the new key succeeds on both hosts.

## Status 2026-01-15

The new key is in the vault and on alpha; beta still uses the old one until its timer
is moved, which is the next step of the overhaul plan.

## Closed 2026-01-15

Rotated on beta too; the old key is revoked in the vault and a restore of the newest
snapshot succeeded with the new key on both hosts.
