## 1. E2E Evidence Preparation

- [x] 1.1 Add a real deployment E2E evidence template that records only safe metadata and redacted identifiers.
- [x] 1.2 Add or update E2E runbook steps for deployment URL health, GitHub webhook delivery, supported PR action triggering, PR comment upsert, Check Run advisory behavior, and leak review.
- [x] 1.3 Add safe evidence collection guidance or helper commands that do not print secrets, private keys, installation tokens, LLM API keys, raw webhook payloads, raw prompts, raw model responses, or private source.

## 2. Preflight Verification

- [x] 2.1 Run `go test ./...`.
- [x] 2.2 Run `go build ./cmd/server`.
- [x] 2.3 Run `scripts/smoke_local.sh`.
- [x] 2.4 Run `openspec validate m8-real-deployment-e2e --type change --strict`.
- [x] 2.5 Confirm `GO_WORKSPACE_PROVIDER_ENABLED=false` for E2E unless a later explicit proposal opts into checkout.

## 3. Real GitHub App E2E

- [x] 3.1 Deploy or run the service with real credentials from uncommitted secret storage.
- [x] 3.2 Verify deployed `GET /healthz` returns success without exposing secrets.
- [x] 3.3 Configure a test GitHub App webhook to `/github/webhook` with the `pull_request` event.
- [x] 3.4 Trigger a supported `opened` pull request event in a test repository and record safe delivery metadata.
- [x] 3.5 Trigger a `synchronize` event and verify the marker comment is updated rather than duplicated.
- [x] 3.6 Trigger or confirm `reopened` handling where practical, or record why it could not be run in the current E2E window.
- [x] 3.7 Verify the PR contains one marker-identified AI review comment.
- [x] 3.8 Verify the `AI Review` Check Run appears on the PR head SHA and completes as advisory `neutral` or `success` for completed findings.
- [x] 3.9 Review service logs, PR comment, and Check Run output for absence of credentials, raw payloads, raw prompts, raw model responses, and unintended private source excerpts.

## 4. Bounded Fix Loop

- [x] 4.1 If E2E fails, record each issue with safe symptoms, suspected component, and reproduction steps without sensitive data.
- [x] 4.2 Apply only fixes required for safe E2E completion; do not expand checkout, analyzer commands, or blocking policy.
- [x] 4.3 Add or update deterministic tests for any code fix.
- [x] 4.4 Rerun affected verification and update the E2E evidence record.

## 5. Final Verification

- [x] 5.1 Run `gofmt -w .` if Go files changed.
- [x] 5.2 Run `go test ./...`.
- [x] 5.3 Run `go build ./cmd/server`.
- [x] 5.4 Run `scripts/smoke_local.sh`.
- [x] 5.5 Run `openspec validate m8-real-deployment-e2e --type change --strict`.
- [x] 5.6 Confirm no secrets, `.env`, private keys, tokens, raw payloads, raw prompts, raw model responses, or private source evidence are staged.
