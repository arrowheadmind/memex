+++
title = "Replace rsync on beta"
status = "in-progress"
created = 2026-01-11
summary = "beta still runs the old rsync job; move it to restic like alpha."
tags = ["backup"]
hosts = ["beta"]
parent = "2026-01-08-backup-overhaul"
+++

## Goal

`beta` backs up with restic to the same NAS repository, with its own key.

## Done when

The rsync timer is gone from `beta` and a restore test passes.
