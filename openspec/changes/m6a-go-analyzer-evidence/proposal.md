## Why

The reviewer now has repo-aware context, structured findings, and deterministic verifier outcomes, but it still cannot use standard Go tooling evidence when reviewing Go projects. A narrow optional analyzer slice can improve verification quality without expanding into a general CI runner or making AI findings blocking.

## What Changes

- Add an optional Go analyzer stage in the review worker pipeline before finding verification.
- Limit M6a analyzer execution to Go repositories and Go standard tooling only: `go test ./...` and `go vet ./...`.
- Require analyzer execution to be gated by repository context and by a bounded local workspace strategy; if a safe workspace is unavailable, record skipped/omitted analyzer evidence instead of running tools.
- Bound analyzer runtime by timeout, output size, safe path constraints, cleanup expectations, and safe logging rules.
- Convert analyzer outcomes into structured static-check evidence for the existing verifier extension point, including tool name, exit category, package/file/line when safely parsed, sanitized message, and limitation notes.
- Preserve advisory behavior: analyzer failures, timeouts, and findings must not block webhook handling, PR comments, or Check Run completion based on AI review findings.
- Add deterministic tests for command planning, output parsing/sanitization, timeout/failure handling, evidence integration, and reporter non-blocking behavior.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `github-app-review-loop`: Add optional bounded Go analyzer execution before verifier/reporter output while preserving fast webhook handling, reporter fan-out, comment marker upsert, output suppression, and advisory Check Run behavior.
- `finding-verification`: Allow verifier use of bounded static-check evidence from Go analyzer results and define safe aggregate/static evidence boundaries.

## Impact

- Affected code areas: `internal/worker`, `internal/review`, verifier code, repository context/workspace planning code, and tests.
- No new GitHub App permissions are required for M6a beyond existing repository content access used by review context.
- No new third-party analyzer dependency is introduced; M6a uses the local Go toolchain only when available and safe.
- Runtime risk increases because PR code may be checked out and analyzed locally; design and implementation must constrain workspace source, path handling, command arguments, timeout, output size, cleanup, and logging.
