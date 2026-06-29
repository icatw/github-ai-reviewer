## Why

M8 proved the GitHub App review loop works under a real nginx-backed deployment, but the running process is currently managed by an interactive Hermes background process while the existing `github-ai-reviewer.service` unit is inactive. The next reliability risk is operational: service restarts, log access, health checks, rollback, and deployment commands need to be reproducible without depending on the current chat session.

## What Changes

- Add version-controlled systemd service material for `github-ai-reviewer.service` that runs the built server from the repository with `.env.production` supplied outside git.
- Add an operations runbook for build, install, daemon reload, start, stop, restart, status, logs, health checks, rollback, and nginx route verification.
- Add safe helper scripts or documented commands for systemd health/status checks that do not print secrets or environment values.
- Switch the live deployment from the temporary Hermes-managed process to systemd and verify `https://app.icatw.site/healthz` remains healthy.
- Preserve product behavior: M9 does not change GitHub App permissions, webhook semantics, LLM review logic, checkout behavior, analyzer behavior, PR comments, or Check Run policy.

## Capabilities

### New Capabilities

- `service-operations`: Systemd-backed service management, operational runbook, health checks, log access, and rollback requirements for the GitHub AI Reviewer deployment.

### Modified Capabilities

- `production-hardening-e2e`: Clarify that verified deployments should be managed by a durable service manager rather than an interactive process.

## Impact

- Affected deployment files: new `deploy/systemd/` service unit template or equivalent.
- Affected docs: new or updated operations/deployment runbook.
- Affected scripts: optional safe health/status helper under `scripts/`.
- Affected live environment: systemd unit may be installed/reloaded/restarted on the current host.
- No changes to review logic, GitHub API behavior, LLM prompts, repository checkout, analyzer commands, inline comments, dashboard, billing, or blocking policy.
