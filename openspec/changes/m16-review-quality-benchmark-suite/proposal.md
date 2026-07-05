## Why

Review quality is now the main product risk before dashboard, storage, or SaaS work: the bot can publish advisory output, but accuracy changes are not yet regression-safe across replayable PR cases. The existing `review-bench` and `review-bench-from-pr` commands provide context retrieval metrics and real-PR fixture generation, so M16 should formalize them into an offline benchmark suite that measures finding quality without changing production review behavior.

## What Changes

- Extend the offline benchmark milestone around sanitized, replayable fixtures generated from real pull requests.
- Define a fixture annotation path for expected findings and expected no-finding cases.
- Add deterministic finding quality reporting for expected finding coverage, missed expected findings, unexpected findings, and low-value or duplicate finding classification where feasible.
- Add suite-level aggregate output suitable for regression comparison in local development and CI.
- Require private repository fixture and benchmark report safety rules so generated private content stays out of git until sanitized and reports avoid raw private content and secrets.
- Document how to generate, sanitize, annotate, and run benchmark fixtures.
- Add tests for fixture decoding, quality reporting, and suite aggregation behavior.
- No production review behavior, dashboard, hosted SaaS, durable storage, vector search, full indexing, or blocking policy changes are included.

## Capabilities

### New Capabilities
- `review-quality-benchmark-suite`: Offline replayable PR benchmark suite for context retrieval and finding quality regression measurement.

### Modified Capabilities

## Impact

- Affected code: `cmd/review-bench`, `cmd/review-bench-from-pr`, `internal/reviewbench`, benchmark fixtures under `testdata/review-bench`, and documentation.
- APIs: no public service API or GitHub App runtime API changes.
- Dependencies: no new external dependency is expected.
- Systems: offline local/CI benchmark workflow only; production webhook, review publishing, Check Run, and inline comment behavior remain unchanged.
