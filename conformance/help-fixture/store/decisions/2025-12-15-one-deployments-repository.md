+++
title = "One deployments repository"
status = "accepted"
created = 2025-12-15
summary = "Every host is deployed from the single deployments repository, never from a per-host checkout."
tags = ["deploy"]
repos = ["deployments"]
+++

## Context

Two hosts had drifted because each was deployed from its own clone.

## Decision

One repository, `deployments`, holds every host; a host is deployed from its pushed revision.

## Consequences

A change to a shared module is deployed to every host in one run.
