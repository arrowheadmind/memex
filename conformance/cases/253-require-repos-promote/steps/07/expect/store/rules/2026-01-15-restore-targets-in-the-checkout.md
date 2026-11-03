+++
title = "Restore targets in the checkout"
status = "active"
created = 2026-01-15
summary = "The repository is taken from the current directory."
keywords = ["restic"]
repos = ["home"]
sources = ["2026-01-12-restic-needs-its-target"]
+++

Create the restore target directory before running restic restore, because restic does not create it and the restore fails.
