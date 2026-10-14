# Cold-agent test (SPEC §17.1)

The test: a fresh agent with no knowledge of memex gets only the prompt in `PROMPT.md`, with
`__TASK__`, `__STORE__` and `__ENV__` filled in by `setup.sh`, and a store seeded from the help
fixture. It must find its way from `AGENT.md` and `memex help`: run `preflight`, read the
results, do the task, `capture` with a proper title, summary and body, and reach `pushed`.
Every stumble is recorded in `round-N.md` (commands and stumbles only); the pages are fixed,
not the tool; the next round uses a different task.

Rounds:

| round | task | model | result |
|---|---|---|---|
| 1 | rotate the backup key on alpha | Sonnet, 2026-10-14 | completed the loop; one harness stumble (git identity), three page fixes (`round-1.md`) |
| 2 | add a disk-usage alert for beta | Sonnet, 2026-10-14 | completed the loop with three `[[id]]` links; led to the `weak results` line and the AGENT.md wording (`round-2.md`) |

Status: two rounds run; both completed preflight, reading, task and a pushed capture.
A third round is run only after a substantial page change. To run a round:

```
scripts/cold-agent/setup.sh /tmp/cold-N && sed -i 's/__TASK__/<task>/' /tmp/cold-N/prompt.md
# spawn a fresh Sonnet agent with the content of /tmp/cold-N/prompt.md as its whole prompt
```
