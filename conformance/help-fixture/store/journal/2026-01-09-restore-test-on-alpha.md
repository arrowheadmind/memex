+++
title = "Restore test on alpha"
status = "raw"
created = 2026-01-09
summary = "First restore from restic on alpha worked; the mount point must exist before restic runs."
tags = ["backup"]
repos = ["deployments"]
hosts = ["alpha"]
body_sha256 = "ff0e2a694b3faa17f0e5419899629f2cdd9061888856d2054f93768dd51e5e7a"
+++

## What was attempted

Restore the latest snapshot's `/srv/photos/2025` into `/tmp/restore-test` on alpha.

## What worked

`restic restore latest --target /tmp/restore-test --include /srv/photos/2025` finished in
40 s and the files compared equal.

## What failed

The first attempt failed because `/tmp/restore-test` did not exist; restic does not create the
target directory.

## Reusable insights

Create the target directory first. Worth a line in the rotation runbook.
