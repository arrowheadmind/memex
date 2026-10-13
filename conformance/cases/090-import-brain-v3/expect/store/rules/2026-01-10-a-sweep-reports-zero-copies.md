+++
title = "A sweep reports zero copies"
status = "active"
created = 2026-01-10
aliases = [".brain/known-mistakes.md#a-sweep-reports-zero-copies"]
+++

**Symptom:** A sweep reports **zero** copies

**Context:** Any `find` with `-size -1M`

**Avoid:** Believing a clean negative

**Instead:** Make the sweep find a sentinel first; `a | b` is one cell
