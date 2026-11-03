+++
title = "Check the restore target on beta"
status = "active"
created = 2026-01-15
summary = "The restore target on beta is checked before a restore."
keywords = ["restic"]
repos = ["beta"]
sources = ["2026-01-12-restic-needs-its-target"]
+++

Create the restore target directory before running restic restore, because restic does not create it and the restore fails.
