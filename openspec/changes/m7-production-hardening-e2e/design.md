## Context

The service has passed the early product milestones for webhook intake, structured review output, marker comment upsert, advisory Check Run reporting, repository-aware context, finding verification, optional Go analyzer evidence, and safe workspace checkout wiring. M6c kept checkout config-gated and credential-scoped, which is the right production posture, but the project still needs a hardening milestone before deeper analyzer expansion.

M7 treats production risk as the next product risk. The work should make a real deployment understandable, reversible, and verifiable: operators need clear config examples, GitHub App permission guidance, safe defaults, local smoke tests that do not need real secrets, and an E2E checklist that proves the GitHub App loop under real webhook conditions. The milestone must not deploy anything during proposal, must not broaden analyzer commands, and must not make AI output blocking.

## Goals / Non-Goals

**Goals:**

- Provide production-oriented docs and config examples for a GitHub App deployment.
- Make default-disabled workspace checkout and rollback-by-disable behavior explicit and testable.
- Add local smoke-test guidance and scripts that can run with dummy config and no real credentials where possible.
- Define a real deployment/E2E verification checklist for webhook delivery, supported PR events, comment upsert, advisory Check Run output, and leak checks.
- Add deterministic safety tests for startup config, redaction, ignore coverage, and reporter non-blocking behavior where missing.
- Keep the core review loop conservative and advisory.

**Non-Goals:**

- No code implementation during propose.
- No production deployment, service restart, GitHub App creation, or real webhook triggering during propose.
- No dashboard, billing, durable storage, slash commands, inline comments, auto-fix, blocking policy, AST/tree-sitter, staticcheck, gosec, semgrep, arbitrary CI command execution, or new analyzer command surface.
- No change that enables real checkout by default.

## Decisions

1. Treat production readiness as a first-class capability with docs, scripts, and tests.

   The hardening work should be captured as its own `production-hardening-e2e` capability because it spans docs, configuration examples, smoke tests, and E2E verification. This avoids hiding operational requirements inside implementation notes.

   Alternative considered: only update `README.md`. Rejected because production safety needs testable requirements, not just prose.

2. Keep `.env.example` safe and runnable only for non-secret local checks.

   `.env.example` should document every required and important optional setting, but it must not contain real secrets, private key paths that imply committed keys, installation tokens, webhook secrets, or LLM API keys. Local smoke scripts may use dummy values only for code paths that do not authenticate to GitHub or an LLM.

   Alternative considered: include a near-production example with placeholder private key material. Rejected because examples are frequently copied into unsafe locations and can normalize secret sprawl.

3. Make checkout opt-in and rollback-by-disable part of the operator contract.

   Production docs should state that real workspace checkout is disabled by default and requires explicit config, a dedicated workspace root, restrictive filesystem permissions, bounded timeouts/output, and cleanup monitoring. Rollback should be documented as disabling the workspace provider config and redeploying/restarting, returning the service to analyzer-skipped advisory limitations.

   Alternative considered: make checkout part of the default production recipe. Rejected because private-code checkout is higher risk than the core GitHub App review loop.

4. Split local smoke tests from real E2E verification.

   Local smoke tests should verify startup behavior, health checks, dummy config validation, script ergonomics, and safe failure paths without requiring real GitHub credentials. Real E2E verification should remain a checklist/runbook because it requires a GitHub App, HTTPS webhook delivery, installed repository, real PR, LLM credentials, and operator observation.

   Alternative considered: automate real GitHub E2E in a script. Rejected for M7 because it would require credential handling and external state creation beyond the proposal scope.

5. Reporter and safety failures remain non-blocking for AI findings.

   M7 should add or improve tests that reporter failures are recorded safely and do not make AI findings blocking. Check Run `failure` remains reserved for infrastructure/job execution failure categories, not finding severity, and reporter failures must not cause duplicate comments or unsafe fallback output.

   Alternative considered: fail Check Runs when hardening checks find review issues. Rejected because the project policy remains advisory until stronger verification and policy gates exist.

## Risks / Trade-offs

- Operators may mistake smoke tests for full production validation -> Mitigation: docs must clearly separate local smoke checks from real E2E verification and list what each proves.
- Examples can drift from runtime config -> Mitigation: add tests or script checks that documented env names and required config stay aligned where practical.
- Secret redaction can miss new fields -> Mitigation: use sentinel-based tests for config errors, logs/rendered output helpers, reporter output, and script output where available.
- E2E checklist can become stale -> Mitigation: keep it tied to current supported events, marker comment upsert, advisory Check Run behavior, checkout disabled-by-default posture, and explicit non-goals.
- More production docs may look like the service is production-complete -> Mitigation: include completion criteria that distinguish local readiness from real PR comment observation and safe log/output review.

## Migration Plan

1. Add `.env.example` and deployment docs with safe defaults and explicit production settings.
2. Add local smoke scripts/docs that use dummy config for health/startup checks and avoid real credentials unless an operator explicitly opts into E2E steps.
3. Add or improve tests for config validation, redaction, `.gitignore` coverage, and reporter non-blocking behavior.
4. Run `gofmt -w .`, `go test ./...`, `go build ./cmd/server`, local `/healthz` smoke checks, and the new smoke scripts.
5. Deploy with workspace checkout disabled and verify behavior against a real test repository.
6. Roll back by disabling workspace checkout and/or stopping webhook delivery; the service should stop attempting checkout and continue or cease advisory reporting according to operator config.

## Open Questions

- Which deployment target should be documented first in the implementation: Docker Compose on a single host, systemd plus reverse proxy, or both?
- Should the E2E checklist require a separate public test repository first, or can a private test repository be the primary path once leak checks are explicit?
- What minimal log format should be documented for production verification without introducing a broader observability stack in M7?
