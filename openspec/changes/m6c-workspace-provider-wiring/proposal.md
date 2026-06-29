## Why

M6b created the safe Go workspace provider foundation but left it disconnected from production startup and left checkout credential handling as a design gap. The next narrow milestone is to make real checkout explicitly opt-in and production-wired while ensuring GitHub App installation tokens can be used for checkout without leaking into durable config, logs, analyzer execution, verifier evidence, reporter output, or PR-facing text.

## What Changes

- Wire the safe Go workspace provider into `cmd/server` review service construction only when explicit config enables it.
- Keep workspace-backed Go analyzer execution disabled by default, preserving the M6a production-safe skip when no provider is configured.
- Add a checkout-only GitHub App installation credential provider contract scoped to the current review job installation, owner, repo, and head SHA.
- Require checkout credential injection to avoid tokenized remotes, persisted git config, durable storage, analyzer environment variables, logs, comments, Check Runs, verifier evidence, and reporter outputs.
- Map credential acquisition and checkout credential failures to deterministic non-blocking analyzer/workspace limitation categories.
- Add tests for default-disabled behavior, explicit enablement wiring, credential failure categories, token-free git plans/logs, non-blocking review continuation, and secret-free analyzer/verifier/reporter boundaries.
- Keep Go analyzer evidence advisory and non-blocking; analyzer or checkout failures must not fail the review job or advisory Check Run.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `github-app-review-loop`: startup config and production review service wiring SHALL explicitly enable the safe Go workspace provider only when configured, and SHALL inject checkout credentials safely for current-job GitHub App installation access.
- `finding-verification`: verifier and analyzer evidence boundaries SHALL continue to treat workspace, checkout, and credential failures as safe current-job limitations without leaking checkout credentials or promoting analyzer failures into blocking output.

## Impact

- Affected code areas for implementation: `cmd/server`, `internal/config`, `internal/review`, `internal/worker`, `internal/githubapp`, optional workspace provider code, Go analyzer orchestration, verifier evidence ingestion, reporters, and tests.
- No new external analyzer tools, CI runner behavior, durable storage, dashboard, slash commands, inline review comments, AST/tree-sitter integration, auto-fix, billing, or blocking policy.
- No required dependency changes are expected beyond existing GitHub App and Go workspace/analyzer foundations.
