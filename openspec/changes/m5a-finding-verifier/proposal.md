## Why

The reviewer now produces structured results with repo-aware context and advisory reporting, but LLM findings can still be published when they are weakly supported, point at unavailable files, or depend on context that was omitted. M5a adds a bounded verification step so published findings stay tied to available PR evidence while preserving the non-blocking review loop.

## What Changes

- Add a verifier layer after structured LLM review parsing and before comment rendering or Check Run reporting.
- Verify each finding against available evidence from changed-file patches, bounded full file context, related tests, repo docs/config, and explicit omitted-context notes.
- Keep valid findings unchanged when their file, line, and evidence are supported by available context.
- Drop or downgrade findings that reference unavailable files, impossible or unavailable line information, insufficient evidence, or omitted/unavailable context.
- Preserve existing summary comment marker upsert behavior and advisory/non-blocking Check Run reporting.
- Add deterministic safe metrics/logging counts for kept, downgraded, and dropped findings by reason category.
- Add eval and unit fixtures for true positive, unsupported finding, unavailable file, line/context mismatch, omitted context, and no-finding cases.
- Leave an extension point for future static-check evidence without running static analyzers in M5a.

## Capabilities

### New Capabilities

- `finding-verification`: Verifies structured review findings against bounded PR evidence before any advisory output is rendered or reported.

### Modified Capabilities

- `github-app-review-loop`: The review loop now passes validated structured results through finding verification before reporter fan-out while preserving comment upsert and non-blocking Check Run behavior.

## Impact

- Affected code areas: `internal/review` orchestration and result types, repo-aware context/evidence data structures, comment rendering inputs, reporter fan-out inputs, worker logging, and tests/eval fixtures.
- API behavior: no new external HTTP endpoints and no GitHub permission changes.
- Output behavior: fewer unsupported findings may be published; supported findings, summary comments, and advisory Check Runs keep their existing behavior.
- Dependencies: no new durable storage, vector database, AST/tree-sitter index, inline review comments, slash commands, merge-blocking policy, or static analyzer execution.
