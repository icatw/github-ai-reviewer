## Why

Real PR replay showed that warning-level inline comments can be noisy before the review bot has a larger annotated quality suite. The bot should keep inline comments more conservative while continuing to report lower-confidence or lower-severity findings in the summary comment.

Repeated manual or webhook-triggered reviews also need clearer Check Run lifecycle behavior. Updating an old completed Check Run makes fresh review attempts harder to audit in GitHub's Checks UI. A new run should start as `in_progress`, and completion/failure should update the current in-progress run rather than mutating stale completed history.

## What Changes

- Change the default inline severity threshold from `warning` to `blocker` for global config and default inline policy.
- Preserve repository-level configuration so repos can explicitly lower the inline threshold when desired.
- Capture Check Run status from GitHub list responses.
- Make review start create a fresh `AI Review` Check Run with `in_progress` status instead of upserting an old run.
- Make completion/failure matching update only matching `in_progress` Check Runs for the same head SHA.
- Keep all AI findings advisory and non-blocking; this does not introduce request-changes reviews, failing checks for findings, auto-fix, or merge gates.

## Impact

- Affected code: `internal/comment`, `internal/review`, `internal/githubapp`, and tests.
- Affected behavior: default inline publishing is stricter, and Check Runs retain clearer per-review lifecycle history.
- No database/storage migration.
- No dashboard, billing, tenant management, vector indexing, automatic fixing, or blocking policy changes.
