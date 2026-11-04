+++
title = "restic without repos"
status = "promoted"
created = 2026-01-12
summary = "A restore failed because the target directory was missing."
keywords = ["restic"]
body_sha256 = "ffca9ade64360a5fffb6d5e26ebea0b72929a40c480c079c1225306b6559ef07"
+++

## What was attempted

Restored the latest snapshot into a directory that did not exist; restic failed until it was created.
