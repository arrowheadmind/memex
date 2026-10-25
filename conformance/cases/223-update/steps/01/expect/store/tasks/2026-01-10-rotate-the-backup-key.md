+++
title = "Rotate the backup key"
status = "open"
created = 2026-01-10
summary = "Half done 2026-01-15: new key on alpha and in the vault; beta next."
tags = ["backup"]
+++

## Goal

A new restic key on alpha and beta, and the old key revoked in the vault.

## Done when

A restore with the new key succeeds on both hosts.

## Status 2026-01-15

The new key is in the vault and on alpha; beta still uses the old one until its timer
is moved, which is the next step of the overhaul plan.
