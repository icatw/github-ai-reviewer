## 1. Service Management Artifacts

- [ ] 1.1 Add a version-controlled `github-ai-reviewer.service` systemd unit template or deployment artifact.
- [ ] 1.2 Add an operations runbook with install, build, daemon reload, enable, start, stop, restart, status, logs, health check, rollback, and nginx route verification commands.
- [ ] 1.3 Add a safe status/health helper script that does not print `.env.production`, private keys, tokens, raw payloads, raw prompts, raw model responses, or private source.
- [ ] 1.4 Document that `.env.production`, private keys, local databases, and service logs with private metadata stay outside git.

## 2. Live Service Cutover

- [ ] 2.1 Build the current server binary for the deployed host.
- [ ] 2.2 Install or refresh the systemd unit from the reviewed deployment artifact.
- [ ] 2.3 Stop the temporary Hermes-managed background process.
- [ ] 2.4 Run `systemctl daemon-reload` and enable `github-ai-reviewer.service`.
- [ ] 2.5 Start or restart `github-ai-reviewer.service` under systemd.
- [ ] 2.6 Verify `systemctl is-active github-ai-reviewer.service` reports `active`.
- [ ] 2.7 Verify local `GET /healthz` on `127.0.0.1:8095` succeeds.
- [ ] 2.8 Verify public `GET /healthz` on `https://app.icatw.site` succeeds through nginx.
- [ ] 2.9 Verify recent journald startup/status output is safe and does not print secrets.

## 3. Safety And Scope Checks

- [ ] 3.1 Confirm no `.env.production`, private keys, database files, raw webhook payloads, raw prompts, raw model responses, or private source files are staged.
- [ ] 3.2 Confirm M9 does not change GitHub App permissions, LLM prompt behavior, PR comment behavior, Check Run policy, checkout behavior, or analyzer commands.
- [ ] 3.3 Confirm workspace checkout remains disabled by default.
- [ ] 3.4 Confirm nginx routes for `/healthz` and `/github/webhook` still point to the systemd service port.

## 4. Verification

- [ ] 4.1 Run `gofmt -w .` if Go files changed.
- [ ] 4.2 Run `go test ./...`.
- [ ] 4.3 Run `go build ./cmd/server`.
- [ ] 4.4 Run `scripts/smoke_local.sh`.
- [ ] 4.5 Run `scripts/check_e2e_safety.sh`.
- [ ] 4.6 Run the M9 service health/status helper.
- [ ] 4.7 Run `openspec validate m9-operationalization-service-management --type change --strict`.
