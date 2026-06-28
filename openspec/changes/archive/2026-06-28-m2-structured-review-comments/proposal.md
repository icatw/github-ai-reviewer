## Why

M1 proves the GitHub App loop can receive PR events, fetch changed files, call an LLM, and publish a PR conversation comment. M2 reduces review noise and prepares for later verification by replacing free-form model Markdown with validated structured review data and by updating the prior bot comment instead of creating duplicates.

## What Changes

- Request JSON-only structured review results from the configured OpenAI-compatible LLM.
- Parse, validate, and normalize typed review data before rendering or publishing.
- Render deterministic GitHub Markdown from typed data with a stable hidden marker.
- Publish review comments with upsert behavior: update the existing marker comment when present, otherwise create one.
- Preserve M1 boundaries: webhook handling remains fast, review work stays in the worker path, and findings remain non-blocking.
- Add unit coverage for structured parsing, validation, rendering, marker detection, and create-vs-update behavior.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `github-app-review-loop`: review jobs use structured LLM review data, stable Markdown rendering, hidden marker comments, and update-vs-create comment publishing.

## Impact

- `internal/llm/`
- `internal/review/`
- `internal/comment/`
- `internal/githubapp/`
- package tests and fixtures for the affected packages
- OpenAI-compatible chat completions response content
- GitHub Issues comments list, create, and update APIs for PR conversation comments

Out of scope: GitHub Check Runs, inline review comments, failing PR checks, durable SQLite job storage, slash commands, full repository indexing, and AST/static-analysis integration.
