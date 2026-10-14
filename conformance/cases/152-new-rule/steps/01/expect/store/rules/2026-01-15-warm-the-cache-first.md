+++
title = "Warm the cache first"
status = "active"
created = 2026-01-15
summary = "Warm the binary cache before switching hosts."
tags = ["cache", "nix"]
keywords = ["cache", "cold build"]
hosts = ["builder"]
+++

When the cache is cold the first build downloads everything again, so warm it before switching hosts.
