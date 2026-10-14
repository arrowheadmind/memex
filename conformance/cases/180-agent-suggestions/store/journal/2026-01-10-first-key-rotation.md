+++
title = "First key rotation"
status = "kept"
created = 2026-01-10
summary = "The rotation runbook works as written; the only surprise was the key id format."
tags = ["backup"]
hosts = ["alpha"]
aliases = ["notes/key-rotation.md"]
body_sha256 = "70ceeb8e3c71cf4f8b711fb4ea9f16284d9cffb0d13a5b6e4c6234475c84c0a1"
+++

## What was attempted

The runbook [[2026-01-04-rotate-the-backup-key-on-alpha]], step by step.

## What worked

Everything. `restic key add` prompted once for the new passphrase, the restore check passed.

## What failed

Nothing failed. `restic key remove` wants the short id from `restic key list`, not the full one.

## Reusable insights

Note the short id form in the runbook.
