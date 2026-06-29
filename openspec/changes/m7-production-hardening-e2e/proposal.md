## Why

M6a through M6c added optional analyzer evidence, safe workspace checkout foundations, and config-gated production wiring, but the next risk is operational rather than analytical: a real GitHub App deployment must be safe to configure, easy to disable, and verifiable end to end without leaking tokens, private code, prompts, or model output. M7 hardens deployment guidance and verification before expanding checkout or analyzer surface area.

## What Changes

- Add production hardening documentation and configuration examples for running the service as a GitHub App deployment.
- Document GitHub App permissions, webhook configuration, environment variables, `.env.example` behavior, workspace checkout disabled-by-default posture, workspace root requirements, cleanup monitoring, timeout/output bounds, and rollback-by-disable behavior.
- Add or improve local smoke-test scripts and docs that exercise server startup, `/healthz`, and safe dummy configuration paths without requiring real credentials where possible.
- Add a real deployment/E2E verification checklist covering GitHub webhook delivery, supported PR events, marker comment upsert, advisory Check Run behavior, and absence of secret/private-code leakage in logs or PR-facing output.
- Add safety checks/tests around startup config validation, secret redaction, `.gitignore` coverage, and reporter non-blocking behavior where coverage is missing.
- Preserve optional checkout and analyzer behavior: real PR checkout stays disabled unless explicitly configured, and M7 does not add analyzer commands, arbitrary CI execution, new static analysis tools, or deeper repository checkout behavior.

## Capabilities

### New Capabilities

- `production-hardening-e2e`: Production deployment hardening, local smoke testing, safety checks, and real E2E verification requirements for the GitHub App review loop.

### Modified Capabilities

- `github-app-review-loop`: Clarify production-facing guarantees for startup config examples, non-blocking reporter behavior, marker comment upsert, advisory Check Runs, and disabled-by-default workspace checkout as part of the deployed review loop.

## Impact

- Affected docs and examples: `README.md`, `docs/`, `.env.example`, deployment notes, GitHub App permission documentation, and smoke/E2E runbooks.
- Affected scripts: local smoke-test helpers under `scripts/` that must run without real GitHub, LLM, or checkout credentials unless explicitly documented otherwise.
- Affected tests: deterministic unit or integration tests for config validation/redaction, secret-safe output, `.gitignore` coverage, and reporter failure behavior.
- Affected runtime behavior is limited to hardening and validation. No deployment, production restart, GitHub App creation, webhook triggering, code checkout expansion, analyzer expansion, durable storage, dashboard, billing, slash commands, inline comments, auto-fix, blocking policy, AST/tree-sitter/staticcheck/gosec/semgrep, or arbitrary CI command execution is introduced by this proposal.
