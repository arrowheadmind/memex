+++
title = "Create the restore target first"
status = "active"
created = 2026-01-15
summary = "restic restore does not create its target directory."
keywords = ["restic"]
sources = ["2026-01-12-restic-needs-its-target"]
+++

Create the restore target directory before running restic restore, because restic does not create it and the restore fails.
