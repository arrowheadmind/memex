# Known Mistakes — Programming

| Symptom | Context | Avoid | Instead |
|---------|---------|-------|---------|
| A sweep reports **zero** copies | Any `find` with `-size -1M` | Believing a clean negative | Make the sweep find a sentinel first; `a \| b` is one cell |
| Too many cells | one | two | three | four |
| Short | row, edited later | - | - |
| Added later | second commit | - | - |

---

## Extended Mistakes (from an older brain)

A section divider, not an entry. Everything below is the long-form list.

## Mistake: Trusted the filename (2026-01-03)

**Symptom:** A workflow was classified from its filename and did something else.

**Instead:** Read the workflow before classifying it.

## Known Noise: Spurious TargetDown after a scrape job is removed

Prometheus keeps firing for one lookback window. Ignore it for that long.

## Historical mistake (superseded 2026-01-02): Rebuilt without the cache

RESOLVED by the cache rule. Kept for the record: a bare rebuild left the closure
uncached and the weekly job rebuilt everything from source.

## A heading that is hard-wrapped over two lines because it is long, the
## second line continues it

Body of the wrapped-heading rule, long enough to count as real content here.
