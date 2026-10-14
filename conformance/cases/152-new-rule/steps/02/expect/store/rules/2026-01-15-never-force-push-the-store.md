+++
title = "Never force-push the store"
status = "active"
created = 2026-01-15
summary = "Force-pushing loses other hosts' commits."
repos = ["app", "infra"]
always = true
+++

Never force-push the store: other hosts may have pushed commits that a force-push would silently drop.
