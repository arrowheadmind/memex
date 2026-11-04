+++
title = "Create the restore target first"
status = "active"
created = 2026-01-15
summary = "restic restore does not create its target directory."
keywords = ["restic"]
repos = []
sources = ["2026-01-12-restic-without-repos"]
+++

Create the restore target directory before running restic restore, because restic does not create it and the restore fails.
