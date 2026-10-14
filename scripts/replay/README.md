# Retrieval replay harness

The harness that produced the evidence for gate G1 (block B4) and the reruns after curation
(B7): a set of real queries, run through `memex search` (and, in B4, through the predecessor's
`brainctl preflight`), scored against blind relevance grades.

## Where the data is

The data is knowledge about one operator's store, so it lives in that store, not in this
public repository: the reference entry
`2026-10-14-memex-retrieval-replay-queries-blind-grades-and-results-gate-g1-evidence` holds it
as attachments under `files/<that id>/`: `queries.json`, `grades.json`, `grades-sonnet.json`,
`results.json`, `REPORT.md`, `rerun-2026-10-14.md` and `rerun-2026-10-14-final.md`.

Every script takes that directory as `--data DIR`, default `$REPLAY_DATA`:

```sh
export REPLAY_DATA="$MEMEX_STORE/files/2026-10-14-memex-retrieval-replay-queries-blind-grades-and-results-gate-g1-evidence"
```

## Scripts

| script | does |
|---|---|
| `extract.py OUT.json` | Collects `brainctl preflight` queries from coding-tool transcripts (the B4 query source) |
| `run.py OUT.json` | B4 replay: `memex search` and `brainctl preflight` for each query in `DATA/queries.json`; needs `MEMEX_STORE`, `MEMEX_BIN`, `REPLAY_SCRATCH` and `--brain DIR` (or `REPLAY_BRAIN`) |
| `metrics.py [RESULTS]` | Hit rates, sizes and graded scores of a results file (default `DATA/results.json`) with `DATA/grades.json` |
| `packets.py [RESULTS]` | Blind judging packets for ungraded items, into `REPLAY_SCRATCH` |
| `merge-grades.py` | Merges the judges' raw grades from `REPLAY_SCRATCH` into `DATA/grades-sonnet.json` |
| `rerun.py OUT.json` | memex-only rerun against the current store, compared with `DATA/results.json` |

## A rerun

After a curation batch, or any change to search (§8):

```sh
nix shell nixpkgs#go_1_27 -c go build -o memex ./cmd/memex
PATH=$PWD:$PATH MEMEX_STORE=<store> REPLAY_DATA=<data dir> \
  python3 scripts/replay/rerun.py /tmp/rerun.json > /tmp/rerun-report.md
```

Items the original judges never saw count as grade 0, so a lower score is a prompt to grade
them (`packets.py`, then `merge-grades.py`), not by itself a regression. Keep a report worth
keeping by adding it to the store entry's attachments and running `memex save`.
