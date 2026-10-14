+++
title = "Never deploy from a dirty tree"
status = "active"
created = 2026-01-06
summary = "A deploy from uncommitted changes cannot be reproduced or rolled back; commit first."
tags = ["deploy", "git"]
keywords = ["dirty", "rollback", "uncommitted"]
repos = ["deployments"]
+++

**Symptom:** a host runs a configuration that no commit describes, and the rollback target is a
guess.

**Avoid:** running the deploy script with uncommitted changes in the deployments checkout.

**Instead:** commit, push, and deploy the pushed revision; the script refuses a dirty tree since
2026-01-06.
