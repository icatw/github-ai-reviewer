## Why

Current review prompts are primarily patch-based, which limits the LLM's ability to reason about changed code in context and to identify missing or affected tests. M4a adds a deterministic, bounded repo context layer before deeper repository intelligence, improving review quality while preserving the existing non-blocking GitHub App loop.

## What Changes

- Add deterministic review context gathering that keeps PR metadata and changed-file patches, then enriches prompts with safe full head-version content for changed files.
- Add simple related test discovery by naming convention, including direct pairs such as `foo.go` to `foo_test.go` and bounded same-package test files.
- Add lightweight repository docs/config context from `README.md`, bounded `docs/*.md`, and `.github/ai-review.yml` when present.
- Add stable omitted-context reporting for skipped, truncated, filtered, and budget-limited context.
- Define stable prompt sections such as `patch_context`, `full_file_context`, `related_test_context`, `repo_docs_context`, and `omitted_context`.
- Add deterministic filters and implementation-defined budgets for deleted files, binary files, generated files, lock files, vendor/dist paths, oversized files, per-file limits, and total context limits.
- Preserve M1-M3 behavior: fast webhook response, structured `ReviewResult` as the LLM output boundary, marker comment upsert, and advisory/non-blocking Check Run reporting.
- Do not add AST/tree-sitter/call graph analysis, full repository indexing, vector search, slash commands, `issue_comment` handling, inline review comments, blocking policy, or durable storage.

## Capabilities

### New Capabilities

### Modified Capabilities
- `github-app-review-loop`: The review loop now builds bounded repo-aware prompt context from changed full files, related tests, lightweight docs/config, and omitted-context notes before requesting structured LLM review output.

## Impact

- Affected code areas: review orchestration/context building, GitHub repository content fetching, LLM prompt construction, tests, and lightweight documentation.
- No required database or durable storage change.
- No GitHub permission change beyond existing contents read access already required for PR file and repository content access.
- Deployment remains the existing service process; verification must include a real PR after restart to confirm richer context or limitations appear without changing comment/check behavior.
