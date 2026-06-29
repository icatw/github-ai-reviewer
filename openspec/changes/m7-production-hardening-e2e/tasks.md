## 1. Production Docs And Config Examples

- [ ] 1.1 Add or update `.env.example` with all required server, GitHub App, webhook, LLM, reporter, and workspace settings using only placeholder or dummy values.
- [ ] 1.2 Document GitHub App webhook setup, supported PR events, supported PR actions, and minimum permissions for PR comments and advisory Check Runs.
- [ ] 1.3 Document production deployment notes including reverse proxy/HTTPS expectations, secret handling, safe log/output boundaries, timeout/output bounds, and no raw payload/prompt/model-output logging.
- [ ] 1.4 Document workspace checkout as disabled by default, including explicit opt-in settings, workspace root placement, filesystem permissions, path containment, disk cleanup monitoring, and private-code retention risk.
- [ ] 1.5 Document rollback-by-disable behavior for workspace checkout, reporter outputs, and webhook delivery without requiring code removal or secret exposure.

## 2. Local Smoke Tests And Runbooks

- [ ] 2.1 Add or update local smoke-test scripts or documented commands that build and start the server with dummy non-secret config.
- [ ] 2.2 Ensure the local smoke path verifies `GET /healthz` without calling GitHub APIs, calling an LLM, cloning a repository, publishing PR comments, or creating Check Runs.
- [ ] 2.3 Make smoke-test failure output identify the failing step without printing secrets, private key contents, installation tokens, API keys, webhook payloads, raw prompts, raw model responses, or private code.
- [ ] 2.4 Add a real deployment/E2E checklist that operators can follow manually for GitHub webhook delivery, supported PR events, marker comment upsert, advisory Check Run behavior, and leak review.

## 3. Safety Checks And Tests

- [ ] 3.1 Add or improve startup config tests so missing required production settings return useful setting names without secret values.
- [ ] 3.2 Add sentinel-based redaction tests for safe error/log/rendered output paths that are practical to verify locally.
- [ ] 3.3 Add or improve `.gitignore` coverage checks for `.env`, private key files, generated server binaries, local data directories, and local database files without requiring sensitive files to exist.
- [ ] 3.4 Add or improve reporter tests so reporter failures are recorded safely and do not create duplicate fallback comments.
- [ ] 3.5 Add or improve Check Run reporter tests so AI finding severity alone cannot produce a blocking failure conclusion.
- [ ] 3.6 Add or improve workspace checkout config tests so checkout remains disabled by default and invalid opt-in workspace config fails safely.

## 4. Verification

- [ ] 4.1 Run `gofmt -w .`.
- [ ] 4.2 Run `go test ./...`.
- [ ] 4.3 Run `go build ./cmd/server`.
- [ ] 4.4 Run the local smoke-test script or documented smoke commands and capture the safe output summary.
- [ ] 4.5 Review all M7 docs, examples, scripts, and tests for consistency with the non-goals: no deployment automation, no new analyzer commands, no arbitrary CI execution, no blocking policy, and no default checkout.
- [ ] 4.6 Run `openspec validate m7-production-hardening-e2e --type change --strict`.
