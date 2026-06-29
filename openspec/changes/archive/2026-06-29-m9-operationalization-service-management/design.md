## Scope

M9 operationalizes the already verified deployment. It makes the service durable across shell/session exits, documents the operational commands, and verifies nginx and systemd agree on the service port.

## Existing State

The host already has:

```text
/etc/systemd/system/github-ai-reviewer.service
app.icatw.site nginx routes for /healthz and /github/webhook
.env.production outside git
private-key.pem outside git
```

At M9 start, the systemd unit exists but is inactive, while the service is running as an interactive background process. M9 should converge on systemd as the owner.

## Constraints

- Do not commit `.env.production`, private keys, tokens, database files, raw webhook payloads, raw prompts, raw model responses, or private source.
- Do not change GitHub App permissions or webhook event subscriptions unless a separate proposal requires it.
- Do not enable workspace checkout by default.
- Do not add analyzer commands, arbitrary CI execution, inline comments, slash commands, dashboard, billing, or AI-finding-derived blocking policy.
- Keep logs safe: operational commands should show categories and lifecycle metadata, not secret values.

## Implementation Approach

1. Add a version-controlled service unit template under `deploy/systemd/` matching the current host path and runtime assumptions.
2. Add an operations runbook under `docs/` with exact commands for installing/updating the unit and operating the service.
3. Add a script for safe service health/status checks that verifies systemd active state and `/healthz` without dumping environment or secrets.
4. Build the current server binary.
5. Install or refresh `/etc/systemd/system/github-ai-reviewer.service` from the reviewed template.
6. Stop the temporary background process.
7. Run `systemctl daemon-reload`, `enable`, and `restart`.
8. Verify local and public health checks.
9. Verify recent journald logs show safe startup metadata and no obvious secrets.

## Verification

Required checks:

```bash
go test ./...
go build ./cmd/server
scripts/smoke_local.sh
scripts/check_e2e_safety.sh
openspec validate m9-operationalization-service-management --type change --strict
systemctl is-active github-ai-reviewer.service
curl -fsS http://127.0.0.1:8095/healthz
curl -fsS https://app.icatw.site/healthz
```

## Rollback

If systemd deployment fails, stop `github-ai-reviewer.service`, inspect safe journald lines, and restart the previous binary/config with systemd after correcting the unit. Do not restore an interactive background process as the long-term state.
