+++
title = "Backup overhaul"
status = "active"
created = 2026-01-10
summary = "Move every host to restic, with a tested restore; alpha done, beta next."
tags = ["backup"]
+++

## Context

Every host moves from rsync to restic, one at a time, with a restore test after each.

## Steps

1. alpha.
2. beta.

## Status 2026-01-15

The new key is in the vault and on alpha; beta still uses the old one until its timer
is moved, which is the next step of the overhaul plan.
