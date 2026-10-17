+++
title = "Dashboard feed notes"
status = "kept"
created = 2026-01-10
summary = "How the dashboard feed passes its credential."
keywords = ["dashboard", "feed"]
body_sha256 = "7097352f3de8b4186218285343b04dcf71ad25f09580a42b2eb8782a9bdcbac1"
+++

Notes from wiring the dashboard feed. The feed takes its credential in the query string,
written as ?token=VALUE on the request URL, and the reader substitutes the real value at run
time from the environment, so nothing secret is stored in this entry.
