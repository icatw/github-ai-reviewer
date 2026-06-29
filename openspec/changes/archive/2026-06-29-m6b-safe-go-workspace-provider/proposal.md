## Why

M6a defined an optional Go analyzer but production still safely skips execution when no workspace provider is configured. M6b adds the narrow workspace-provider contract needed to run `go test ./...` and `go vet ./...` only after a PR head checkout satisfies strict safety constraints.

## What Changes

- Add an explicitly gated safe Go workspace provider for review jobs.
- Require workspaces to be created under an implementation-controlled temp/cache root, never from user-supplied paths.
- Require fixed git command argv, bounded clone/fetch behavior, safe path validation, and exact PR head SHA validation before returning a `SafeGoWorkspace`.
- Require GitHub credentials used for checkout to be short-lived and not written to remotes, persisted config, logs, or analyzer command environments.
- Require per-job workspace cleanup after analyzer execution, with cleanup failures recorded as safe limitations and not treated as review-blocking failures.
- Preserve M6a behavior: if any workspace safety check fails or no provider is configured, analyzer execution is skipped and LLM review, verification, PR comment reporting, and advisory Check Runs continue.
- Preserve safe logging: aggregate categories only, with no raw prompt, model output, tokens, secrets, private keys, complete webhook payloads, unbounded analyzer output, or private repository code.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `github-app-review-loop`: Add requirements for the safe Go workspace provider lifecycle, checkout safety, non-blocking skip/failure behavior, cleanup, and preservation of advisory reporting.
- `finding-verification`: Clarify that static-check evidence is consumed only when it comes from a current-job workspace whose PR head checkout was safety-validated, and that skipped workspace-provider outcomes remain safe limitations rather than fabricated evidence.

## Impact

Affected implementation areas in a future apply phase include the review worker's optional Go analyzer workspace provider wiring, GitHub App checkout credential handling, safe git execution helpers, workspace cleanup, analyzer environment construction, and tests around workspace safety and non-blocking failure paths. No production code is changed by this proposal.
