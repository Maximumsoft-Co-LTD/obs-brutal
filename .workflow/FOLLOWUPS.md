# Follow-ups

Items surfaced by past `retro` runs, out of their original scope. `retro` appends + marks consumed; `pm` reads every interview and asks if any open item is now in scope.

## Open

<!-- ID `F-<run-id>-NN`: run folder + per-run counter from 01. -->

| ID | From run | Item | Type hint | Priority | Status |
|----|----------|------|-----------|----------|--------|


## Closed

Audit trail — keep. `retro` moves rows here on `consumed-by:` or `wont-do`.

| ID | From run | Item | Consumed by | Date consumed |
|----|----------|------|-------------|---------------|

## Conventions

- **ID** — `F-<run-id>-NN` (`<run-id>` = surfacing run's folder, `NN` per-run from `01`); collision-proof under parallel runs. `retro` mints; never "next after highest" (races). Legacy `F0001`-style IDs keep their form; history not renumbered.
- **From run** — `NNNN-type-slug` of the surfacing run.
- **Type hint** — which `/dev` run kind would consume it. Non-binding; `pm` can override.
- **Priority** — `low | med | high`. `high` = known-broken or security carry-over from `security.md`.
- **Status** — `open | in-progress | consumed-by:<run-id> | wont-do (reason)`. Move `Open`→`Closed` on `consumed-by:…` or `wont-do`.
