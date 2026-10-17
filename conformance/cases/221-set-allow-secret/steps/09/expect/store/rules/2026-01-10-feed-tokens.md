+++
title = "Feed tokens go in the query string"
status = "active"
created = 2026-01-10
summary = "The feed reads its credential from a query parameter."
keywords = ["feed"]
allow_secret = true
+++

The dashboard feed reads its credential from the query string, as ?token=VALUE, never from a
header; the value comes from the environment when the request is built at run time.
