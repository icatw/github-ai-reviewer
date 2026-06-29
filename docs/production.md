# Production Hardening And E2E Verification

This runbook keeps M7 focused on deployment safety and verification. It does not enable deeper checkout behavior, new analyzer commands, arbitrary CI execution, or blocking policy.

## GitHub App Setup

Minimum repository permissions:

```text
Metadata: Read-only
Contents: Read-only
Pull requests: Read and write
Issues: Read and write
Checks: Read and write
```

`Issues: write` is required for PR conversation comments. `Checks: write` is required for the advisory Check Run. AI findings must remain non-blocking; Check Run failures are reserved for service execution failures.

Webhook configuration:

```text
Payload URL: https://<public-host>/github/webhook
Content type: application/json
Secret: GITHUB_WEBHOOK_SECRET
Events: Pull request
Supported actions: opened, synchronize, reopened
```

Terminate TLS at a reverse proxy or managed platform before forwarding traffic to the service. Do not expose the service over plain public HTTP.

## Configuration

Start from `.env.example` and inject real values through the deployment secret manager. Do not commit `.env`, `.env.*`, private keys, installation tokens, API keys, payload captures, local databases, or generated binaries.

Required production settings:

```text
HTTP_ADDR
GITHUB_APP_ID
GITHUB_APP_PRIVATE_KEY_PATH or GITHUB_APP_PRIVATE_KEY
GITHUB_WEBHOOK_SECRET
LLM_BASE_URL
LLM_API_KEY
LLM_MODEL
```

Prefer `GITHUB_APP_PRIVATE_KEY_PATH` with a mounted secret file. `GITHUB_APP_PRIVATE_KEY` is useful for local smoke tests but increases accidental logging and shell-history risk.

## Workspace Checkout

Workspace checkout is disabled by default:

```text
GO_WORKSPACE_PROVIDER_ENABLED=false
```

Only enable it after deployment basics are already verified. When enabled, use a dedicated absolute root such as `/var/lib/github-ai-reviewer/workspaces`, owned by the service user and not shared with web roots, shell users, or backup jobs that retain private source unexpectedly.

Operational checks before enabling checkout:

- `GO_WORKSPACE_ROOT` is absolute and writable only by the service user.
- `GO_WORKSPACE_CHECKOUT_TIMEOUT` is bounded and positive.
- `GO_WORKSPACE_OUTPUT_LIMIT_BYTES` is bounded and positive.
- Disk cleanup and disk-usage monitoring cover the workspace root.
- Logs, PR comments, Check Runs, and verifier evidence are reviewed for absence of tokens and private source excerpts.

Rollback is configuration-only: set `GO_WORKSPACE_PROVIDER_ENABLED=false` and restart the service. This keeps the PR diff based review loop available without local checkout.

## Safe Logging And Output

Do not log or publish:

```text
GITHUB_WEBHOOK_SECRET
GITHUB_APP_PRIVATE_KEY
installation tokens
LLM_API_KEY
raw private webhook payloads
raw prompts
raw model responses
private repository source beyond the intended PR-facing review summary
```

Startup and validation errors should name setting keys, not values. Reporter and Check Run errors should surface safe categories such as `github_error`, `llm_error`, or `reporter_error`.

## Local Smoke Test

Run the local smoke script before deployment changes:

```bash
scripts/smoke_local.sh
```

The script builds the server, starts it with dummy non-secret config, checks `GET /healthz`, and stops the process. It must not call GitHub APIs, call an LLM, clone a repository, publish PR comments, or create Check Runs.

Expected output includes:

```text
build server
start server with dummy config
check healthz
smoke ok
```

## Real Deployment E2E Checklist

Use a test repository and a non-sensitive PR.

1. Confirm `/healthz` returns `ok` through the public deployment URL.
2. Confirm GitHub webhook delivery receives HTTP 202 for supported `pull_request` actions: `opened`, `synchronize`, and `reopened`.
3. Confirm unsupported actions return without starting a review job.
4. Confirm one PR summary comment is created with the marker `<!-- github-ai-reviewer:review-comment:v1 -->`.
5. Push another commit to the same PR and confirm the marker comment is updated instead of duplicated.
6. Confirm the Check Run named `AI Review` is created or updated with a `neutral` conclusion for completed AI findings.
7. Confirm Check Run `failure` appears only for infrastructure execution failures, not because an AI finding has high severity.
8. Review logs for absence of webhook secret, private key material, installation tokens, LLM API key, raw prompts, raw model responses, and private source dumps.
9. Review PR-facing comment and Check Run output for absence of credentials and unintended private source excerpts.
10. If workspace checkout was enabled for the test, confirm workspace directories are contained under `GO_WORKSPACE_ROOT` and cleanup/monitoring catches leftover directories.

## Rollback

Use configuration toggles before code changes:

```text
Disable workspace checkout: GO_WORKSPACE_PROVIDER_ENABLED=false
Disable webhook delivery: turn off Pull request webhook delivery in GitHub App settings
Disable PR output: remove Issues write / Checks write permissions or disable reporter wiring in a follow-up deployment
```

After rollback, re-run `scripts/smoke_local.sh` and verify `/healthz` still works.
