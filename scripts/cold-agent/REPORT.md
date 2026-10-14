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
| 1 | rotate the backup key on alpha | (not yet run) | |
| 2 | (a different task) | (not yet run) | |

Status: the harness is ready; the rounds were not run in block B6 because the block was
implemented by a fork that cannot spawn fresh agents. The manager runs them:

```
scripts/cold-agent/setup.sh /tmp/cold-1 && sed -i 's/__TASK__/rotate the backup key on alpha/' /tmp/cold-1/prompt.md
# spawn a fresh Sonnet agent with the content of /tmp/cold-1/prompt.md as its whole prompt
```

Then write `round-1.md`, fix the pages, repeat with a second task.
