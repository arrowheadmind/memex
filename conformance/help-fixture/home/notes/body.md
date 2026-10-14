## What was attempted

Deploy the new restic timer to beta with `./deploy.sh beta`.

## What worked

The timer is active and the first run finished in four minutes.

## What failed

Nothing; the dirty-tree check caught an uncommitted edit on the first try, as intended.

## Reusable insights

Commit before deploying; the script's refusal is the rule working.
