## Context

M15 made inline comments readable and GitHub-native, but M16/M17 quality work showed that real PR output still needs more annotated evidence before warning-level inline publication should be the default. Summary comments remain the safer place for advisory warnings while inline comments should be reserved for the strongest findings unless a repository explicitly opts into a lower threshold.

The existing Check Run reporter used stateless lookup/upsert semantics. That is useful for avoiding local persistence, but repeated reviews can end up mutating older completed runs instead of representing each new review attempt as a fresh lifecycle event.

## Decisions

### Default inline threshold

The global default and built-in inline policy should require `blocker` severity. Repository config remains the operator override surface, so a repo can still opt into warning-level inline comments through `.github/ai-review.yml` when its quality bar and tolerance are known.

This keeps summary output comprehensive while reducing high-visibility line-level noise.

### Check Run lifecycle

`JobStarted` should create a fresh `AI Review` Check Run with `in_progress` status. It should not search for and update an existing completed run.

Completion and failure paths should list Check Runs and update the newest matching run only when it is still `in_progress`, matches the same head SHA, and has a valid ID. Completed runs are history and should not be mutated by later review attempts.

The implementation remains stateless: it does not persist Check Run IDs. Matching is still by name, head SHA, and now status.

### GitHub API shape

The GitHub Check Runs list response must preserve `status` so the review layer can distinguish `in_progress` from `completed` runs.

## Non-Goals

- No persistent job or Check Run storage.
- No blocking merge policy.
- No request-changes review output.
- No change to summary comment upsert behavior.
- No prompt or verifier tuning in this milestone.
