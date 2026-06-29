# Operations Runbook

This runbook is for operating the deployed GitHub AI Reviewer service on the current host. It keeps credentials outside git and uses systemd as the source of truth for long-running service ownership.

## Files And Paths

```text
Repository: /home/ubuntu/github-ai-reviewer
Binary: /home/ubuntu/github-ai-reviewer/server
Environment file: /home/ubuntu/github-ai-reviewer/.env.production
Systemd unit: /etc/systemd/system/github-ai-reviewer.service
Unit template: deploy/systemd/github-ai-reviewer.service
Public health URL: https://app.icatw.site/healthz
Local health URL: http://127.0.0.1:8095/healthz
Webhook URL: https://app.icatw.site/github/webhook
```

Do not commit these local files:

```text
.env
.env.production
private-key.pem
*.pem
*.key
data/
*.db
server
raw webhook payload captures
filled private E2E evidence
```

## Build

```bash
cd /home/ubuntu/github-ai-reviewer
go test ./...
go build -o server ./cmd/server
```

## Install Or Refresh The Service Unit

```bash
sudo install -m 0644 deploy/systemd/github-ai-reviewer.service /etc/systemd/system/github-ai-reviewer.service
sudo systemctl daemon-reload
sudo systemctl enable github-ai-reviewer.service
```

The unit expects `.env.production` and the GitHub App private key path referenced by that file to already exist on the host.

## Start, Stop, Restart

```bash
sudo systemctl start github-ai-reviewer.service
sudo systemctl stop github-ai-reviewer.service
sudo systemctl restart github-ai-reviewer.service
```

Use systemd rather than an interactive shell, `nohup`, or a chat-session-managed background process for long-running operation.

## Status And Logs

```bash
systemctl --no-pager status github-ai-reviewer.service
journalctl -u github-ai-reviewer.service -n 80 --no-pager
```

Do not run commands that dump `.env.production` or private key contents. Logs should contain lifecycle and review metadata only, not tokens, private keys, webhook secrets, raw payloads, raw prompts, raw model responses, or private source dumps.

## Health Checks

```bash
systemctl is-active github-ai-reviewer.service
curl -fsS http://127.0.0.1:8095/healthz
curl -fsS https://app.icatw.site/healthz
scripts/check_service_health.sh
```

Expected response body for health checks:

```text
ok
```

## Nginx Route Verification

```bash
sudo nginx -T 2>/dev/null | grep -nE 'app\.icatw\.site|8095|location = /healthz|location = /github/webhook|proxy_pass' -C 3
```

Expected routes:

```text
/healthz -> http://127.0.0.1:8095/healthz
/github/webhook -> http://127.0.0.1:8095/github/webhook
```

## Safe Deployment Update

```bash
cd /home/ubuntu/github-ai-reviewer
git pull --ff-only origin m1-github-app-webhook
go test ./...
go build -o server ./cmd/server
sudo install -m 0644 deploy/systemd/github-ai-reviewer.service /etc/systemd/system/github-ai-reviewer.service
sudo systemctl daemon-reload
sudo systemctl restart github-ai-reviewer.service
scripts/check_service_health.sh
```

## Rollback

If a deployment fails health checks:

```bash
systemctl --no-pager status github-ai-reviewer.service
journalctl -u github-ai-reviewer.service -n 80 --no-pager
sudo systemctl stop github-ai-reviewer.service
```

Then restore the last known-good binary or commit, rebuild, restart through systemd, and rerun `scripts/check_service_health.sh`. Do not roll back by exposing secrets, committing local credentials, or leaving the service under an interactive background process.
