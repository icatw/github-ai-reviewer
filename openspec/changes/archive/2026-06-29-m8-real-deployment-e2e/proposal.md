## Why

M7 established production hardening documentation, safe example configuration, local smoke testing, and a manual E2E checklist. The project still needs a controlled real deployment verification pass against a test GitHub App and test repository before expanding checkout or analyzer behavior. M8 turns the checklist into an executed, evidence-backed verification workflow with clear pass/fail records and a bounded fix loop for issues found during the run.

## What Changes

- Add a real deployment E2E evidence template and runbook for recording deployment URL health, GitHub webhook delivery IDs/statuses, PR test repository details, marker comment upsert evidence, advisory Check Run evidence, and leak-review results.
- Add scripts or documented commands that help operators collect safe E2E evidence without printing secrets, private keys, installation tokens, raw payloads, raw prompts, raw model responses, or private source.
- Verify the service against a real GitHub App installation on a test repository using supported `pull_request` actions.
- Record any issues found as bounded M8 fix tasks and address only issues required to complete the E2E loop safely.
- Preserve the current safety posture: workspace checkout remains disabled by default, Check Runs remain advisory, and M8 does not add deeper checkout, analyzer commands, static analysis integrations, inline comments, slash commands, dashboard, billing, or blocking policy.

## Capabilities

### Modified Capabilities

- `production-hardening-e2e`: Add executed real deployment E2E evidence, safe evidence collection, and issue remediation requirements.

## Impact

- Affected docs: `docs/production.md` or a new E2E evidence template under `docs/`.
- Affected scripts: optional helper scripts under `scripts/` for safe E2E evidence collection or webhook/log review.
- Affected tests: only deterministic regression tests required by fixes found during E2E; no fabricated GitHub/LLM results.
- Affected runtime behavior: limited to fixes needed for safe real E2E completion. No default checkout enablement, production deployment automation, credential expansion, arbitrary CI execution, or AI blocking behavior is introduced.
