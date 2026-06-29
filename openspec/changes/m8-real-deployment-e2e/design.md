## Scope

M8 is a verification milestone, not a feature expansion milestone. The work should prove that the current GitHub App review loop can run safely under live GitHub conditions using a test GitHub App installation and test repository.

## Constraints

- Do not commit `.env`, private keys, webhook secrets, installation tokens, LLM API keys, raw webhook payloads, raw prompts, raw model responses, or private repository source.
- Do not enable workspace checkout by default.
- Do not add new analyzer commands, arbitrary CI execution, static analysis integrations, inline comments, slash commands, dashboard, billing, or blocking policy.
- Keep Check Runs advisory. AI findings must not produce blocking conclusions.
- Use a test repository and non-sensitive PR content for E2E.

## Verification Approach

1. Prepare safe E2E evidence material before touching live credentials.
2. Run existing local verification first: `go test ./...`, `go build ./cmd/server`, `scripts/smoke_local.sh`, and OpenSpec strict validation.
3. Configure deployment using secret storage or local uncommitted environment only.
4. Verify `/healthz` on the deployed URL.
5. Trigger supported `pull_request` events from a test repository.
6. Record safe webhook delivery metadata: delivery ID, event/action, HTTP status, timestamp, and sanitized outcome.
7. Verify PR summary marker comment creation and update behavior.
8. Verify `AI Review` Check Run appears and completes as advisory `neutral` or `success`, never as an AI-finding-derived blocking failure.
9. Review logs, PR output, and Check Run output for leaks using sentinel checks where possible.
10. If E2E reveals a bug, fix only the smallest code/docs/test surface needed to complete M8 safely, then rerun the relevant verification.

## Evidence Handling

Evidence files must use placeholders or redacted values for external identifiers unless the identifier is already public and non-sensitive. Prefer these forms:

```text
GitHub delivery: [REDACTED_DELIVERY_ID] or last 6 chars only
Installation ID: [REDACTED_INSTALLATION_ID]
Repository: test owner/repo only if intentionally public; otherwise [REDACTED_REPO]
Tokens/keys/secrets: never recorded
Webhook payload: never recorded in full
Logs: sanitized excerpts only
```

## Rollback

If live verification exposes a safety issue, stop webhook delivery in GitHub App settings and keep `GO_WORKSPACE_PROVIDER_ENABLED=false`. Do not solve M8 by broadening checkout or credential behavior.
