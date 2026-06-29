# M8 Real Deployment E2E Evidence

This is the committed safe evidence summary for the M8 run. It intentionally omits secrets, raw webhook payloads, raw prompts, raw model responses, private keys, installation tokens, and private source excerpts.

## Run Metadata

```text
Run ID: 2026-06-29-m8-real-deployment-e2e
Service version/commit: 9785f4a
Deployment URL: https://app.icatw.site
Workspace checkout enabled: false
GitHub App: icatw-Review-Cat / icatw-review-cat
Test repository: icatw/review-cat-test-repo
Primary synchronize PR: #1
Opened/reopened PR: #3
```

## Preflight

```text
go test ./...: PASS
go build ./cmd/server: PASS
scripts/smoke_local.sh: PASS
scripts/check_e2e_safety.sh: PASS
openspec validate m8-real-deployment-e2e --type change --strict: PASS
GO_WORKSPACE_PROVIDER_ENABLED: false or unset in production config
```

## Deployment Health

```text
GET https://app.icatw.site/healthz: 200 ok
GET https://app.icatw.site/github/webhook: 405 Method Not Allowed with Allow: POST
Nginx route: app.icatw.site /healthz and /github/webhook proxy to 127.0.0.1:8095
Service process: current repository server binary started with .env.production
```

## GitHub App Installation

```text
App API authentication: PASS
Installation account: icatw
Installed repositories: 1
Test repository permissions: checks:write, contents:write, issues:write, metadata:read, pull_requests:write, statuses:write
Webhook event subscription includes pull_request
```

## Webhook Delivery Evidence

Delivery IDs are redacted to suffixes only.

| Event | Action | Delivery suffix | GitHub status | Service status | Delivered at |
| --- | --- | --- | --- | --- | --- |
| pull_request | synchronize | 276800 | delivered | 202 | 2026-06-29T15:43:25Z |
| pull_request | opened | 257024 | delivered | 202 | 2026-06-29T15:49:37Z |
| pull_request | reopened | 272064 | delivered | 202 | 2026-06-29T15:50:28Z |

## PR Comment Upsert

```text
Marker: <!-- github-ai-reviewer:review-comment:v1 -->
PR #1 synchronize marker comments: 1
PR #1 marker comment updated_at: 2026-06-29T15:43:39Z
PR #3 opened marker comments: 1
PR #3 marker comment created_at: 2026-06-29T15:49:44Z
PR #3 marker comment updated_at after reopened: 2026-06-29T15:50:38Z
Duplicate marker comments observed: NO
```

## Check Run

```text
Check Run name: AI Review
PR #1 synchronize head short SHA: c9d63c67d8f5
PR #1 Check Run conclusion: neutral
PR #1 Check Run completed_at: 2026-06-29T15:43:41Z
PR #3 opened/reopened head short SHA: 65da4fd0f9ea
PR #3 Check Run conclusion: neutral
PR #3 Check Run completed_at: 2026-06-29T15:49:46Z
AI finding severity caused blocking failure: NO
```

## Leak Review

| Surface | Tokens/keys absent | Raw payload absent | Raw prompt/model output absent | Unintended private source absent | Notes |
| --- | --- | --- | --- | --- | --- |
| PR comments | YES | YES | YES | YES | Checked marker comments and PR output for common token/key/prompt markers. |
| Check Run output | YES | YES | YES | YES | Checked Check Run rollup output for common token/key/prompt markers. |
| Safety script | YES | YES | YES | YES | `scripts/check_e2e_safety.sh` passed. |

## Issues Found

| ID | Safe symptom | Suspected component | Fix commit | Verification status |
| --- | --- | --- | --- | --- |
| N/A | No M8 blocking issue found during this run. | N/A | N/A | VERIFIED |

## Final Result

```text
M8 E2E result: PASS
Follow-up proposal needed: NONE for M8 verification. Future checkout/analyzer expansion should use a separate proposal.
```
