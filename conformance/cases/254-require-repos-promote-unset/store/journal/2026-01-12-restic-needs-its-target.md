+++
title = "restic needs its target"
status = "raw"
created = 2026-01-12
summary = "A restore failed because restic does not create its target directory."
keywords = ["restic"]
repos = ["alpha"]
body_sha256 = "d0e0d9561da58da505035f5f1f3032033e43669d5575faf35980a9097bad06a1"
+++

## What was attempted

Restored the latest snapshot into a scratch directory that did not exist yet.

## What worked

Creating the directory first, then running the same restore again.
