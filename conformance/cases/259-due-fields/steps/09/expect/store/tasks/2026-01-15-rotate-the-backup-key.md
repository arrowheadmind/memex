+++
title = "Rotate the backup key"
status = "open"
created = 2026-01-15
summary = "Rotate it before the old key expires."
keywords = ["backup"]
due = 2026-01-25
due_when = "after the vault upgrade lands"
+++

Rotate the backup key on both hosts and revoke the old one in the vault; a restore from the newest snapshot proves the new key works.
