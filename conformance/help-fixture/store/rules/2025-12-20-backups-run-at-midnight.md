+++
title = "Backups run at midnight"
status = "retired"
created = 2025-12-20
summary = "Retired: the backup timer moved to 03:00 when restic replaced rsync."
tags = ["backup"]
hosts = ["alpha"]
+++

**Constraint:** the nightly rsync job started at 00:00 and overlapped the log rotation, so
anything touching `/var/log` between midnight and 00:10 saw half-rotated files.

This rule was retired on 2026-01-03 when the restic timer took over at 03:00.
