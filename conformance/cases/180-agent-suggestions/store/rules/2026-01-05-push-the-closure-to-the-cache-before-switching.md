+++
title = "Push the closure to the cache before switching"
status = "active"
created = 2026-01-05
summary = "A host that switches to a closure nobody pushed rebuilds it from source on the next boot."
tags = ["deploy", "nix"]
keywords = ["cache", "nixos-rebuild", "rebuild", "switch"]
hosts = ["alpha", "beta"]
always = true
+++

**Symptom:** a deploy finishes, and the next reboot of the host spends twenty minutes compiling.

**Context:** `nixos-rebuild switch` activates whatever closure the deploying machine built, but
the binary cache only holds what was pushed to it.

**Avoid:** switching a host to a closure that was built locally and never pushed.

**Instead:** run `./deploy.sh push <host>` first, then switch; the deploy script does both when
given `--all`.
