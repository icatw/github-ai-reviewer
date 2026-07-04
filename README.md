# GitHub AI Reviewer

GitHub AI Reviewer is a Go service for running an AI code review bot as a GitHub App. It receives pull request webhooks, verifies GitHub signatures, fetches changed files through installation authentication, asks an OpenAI-compatible model for structured review data, and publishes a conservative PR summary comment plus an advisory Check Run.

The project is intentionally built as a real service rather than a `git diff | prompt` demo. The current goal is a deployable GitHub App review loop with safe defaults, repeatable tests, and documented production operation.

## Current Capabilities

- GitHub App webhook endpoint for `pull_request` events.
- `X-Hub-Signature-256` verification before payload parsing.
- Supported PR actions: `opened`, `synchronize`, `reopened`, and cleanup-only `closed`.
- GitHub App JWT generation and installation token exchange.
- Pull request changed-files and patch retrieval.
- OpenAI-compatible LLM review request with bounded context.
- Structured review result parsing and validation.
- Stable PR summary comment upsert using a hidden marker, with marker-scoped inactive cleanup when a PR is closed or merged.
- Advisory `AI Review` Check Run reporting with non-blocking conclusions for AI findings.
- Optional local Go workspace checkout provider, disabled by default.
- Production deployment documentation, systemd service management, local smoke tests, and real E2E evidence guidance.

## Non-Goals For The Current Version

These are intentionally outside the current production-ready slice:

- Dashboard, billing, tenant management, or hosted SaaS account flows.
- Automatic code modification or merge blocking.
- Inline review comments on exact diff lines.
- Full repository indexing, vector search, AST call graphs, or long-term review memory.
- Arbitrary CI command execution against private repositories.
- Enabling real repository checkout without explicit operator configuration.

## Architecture

```text
GitHub pull_request webhook
  -> HTTP server
  -> webhook signature verification
  -> supported event/action filtering
  -> review job creation
  -> background worker
  -> GitHub App installation authentication
  -> PR changed files retrieval
  -> optional local workspace provider
  -> LLM review request
  -> structured result validation
  -> PR comment upsert
  -> advisory Check Run update

GitHub pull_request.closed webhook
  -> HTTP server
  -> webhook signature verification
  -> cleanup job creation
  -> existing marker-owned bot output marked inactive
```

Core packages:

```text
cmd/server/          service entrypoint and wiring
internal/config/     typed environment configuration
internal/webhook/    GitHub webhook verification and PR event parsing
internal/githubapp/  GitHub App JWT and installation token authentication
internal/review/     review orchestration, reporting, checkout planning, validation
internal/llm/        OpenAI-compatible model client
internal/comment/    PR comment rendering
internal/worker/     asynchronous job processing
deploy/              deployment artifacts, including systemd unit template
scripts/             local smoke, safety, and health checks
docs/                design, production, operations, research, and E2E docs
openspec/            project specifications and milestone changes
```

## GitHub App Setup

Create a GitHub App and grant the minimum repository permissions needed by the current service:

```text
Metadata: Read-only
Contents: Read-only
Pull requests: Read and write
Issues: Read and write
Checks: Read and write
```

`Issues: write` is required because PR conversation comments use the Issues comment API. `Checks: write` is required for the advisory Check Run. AI findings are non-blocking; Check Run failures are reserved for service execution failures.

Webhook settings:

```text
Payload URL: https://<your-public-host>/github/webhook
Content type: application/json
Secret: use the same value as GITHUB_WEBHOOK_SECRET
Events: Pull request, Issue comment
Supported actions: opened, synchronize, reopened, closed, and `/ai-review` comments on open pull requests
```

Closed or merged pull requests are cleanup-only targets. The service does not fetch PR files for review, call the LLM, run analyzers, create new inline reviews, request changes, auto-fix, auto-merge, or block merging for `pull_request.closed` events. `/ai-review` commands on closed or merged PRs are accepted only for safe cleanup/ignore handling and do not start normal review work.

## Configuration

Start from the example environment file:

```bash
cp .env.example .env
```

Required settings:

```text
HTTP_ADDR
PUBLIC_BASE_URL
GITHUB_APP_ID
GITHUB_APP_PRIVATE_KEY_PATH or GITHUB_APP_PRIVATE_KEY
GITHUB_WEBHOOK_SECRET
LLM_BASE_URL
LLM_API_KEY
LLM_MODEL
```

Prefer `GITHUB_APP_PRIVATE_KEY_PATH` in production with a mounted secret file. `GITHUB_APP_PRIVATE_KEY` is useful for local tests but increases shell-history and accidental logging risk.

Optional review language:

```text
REVIEW_LANGUAGE=zh-CN
```

When unset, review prompts and PR comments default to English. Set `REVIEW_LANGUAGE=zh-CN` to ask the LLM for Simplified Chinese review content and render the bot's fixed PR comment labels in Chinese.

Optional Check Run reporting:

```text
CHECK_RUN_ENABLED=false
```

Check Runs are enabled by default. If the GitHub App installation lacks `Checks: write`, the service degrades to PR summary comments without failing the review job; set `CHECK_RUN_ENABLED=false` to disable Check Run attempts entirely.

Do not commit local environment files, private keys, installation tokens, API keys, local databases, generated binaries, raw webhook payloads, raw prompts, raw model responses, or filled private E2E evidence.

## Workspace Checkout

Local workspace checkout is disabled by default:

```text
GO_WORKSPACE_PROVIDER_ENABLED=false
```

Only enable it after the basic GitHub App review loop is already deployed and verified. When enabled, use a dedicated absolute root such as `/var/lib/github-ai-reviewer/workspaces`, owned by the service user and not shared with web roots, shell users, or backup jobs that could retain private source unexpectedly.

Rollback is configuration-only: set `GO_WORKSPACE_PROVIDER_ENABLED=false` and restart the service.

## Local Development

Run the standard verification commands before submitting changes:

```bash
go test ./...
go build ./cmd/server
scripts/smoke_local.sh
scripts/check_e2e_safety.sh
scripts/check_publication_safety.sh
```

`smoke_local.sh` builds the service, starts it with dummy non-secret configuration, checks `/healthz`, and stops it. It does not call GitHub, call an LLM, clone a repository, publish comments, or create Check Runs.

## Review Context Benchmark

Use the offline review benchmark to measure repository-context retrieval before changing prompts or calling a model:

```bash
go run ./cmd/review-bench -fixture testdata/review-bench/cross-package-auth.json
go run ./cmd/review-bench -fixture testdata/review-bench/python-fastapi-user.json
go run ./cmd/review-bench -fixtures 'testdata/review-bench/*.json'
```

A fixture contains changed PR files, an in-memory repository file map, and `golden_relevant_files`. The command runs the same `BuildRepoContext` path used by production and reports retrieved files, omissions, byte budget use, precision, recall, and F1. With `-fixtures`, the report includes per-fixture cases plus aggregate micro-averaged precision, recall, and F1 across the whole suite. This keeps global-context review work measurable without GitHub credentials or LLM calls.

Generate an offline fixture from a real pull request with the GitHub App credentials configured in an environment file:

```bash
go run ./cmd/review-bench-from-pr -env-file .env.production -owner OWNER -repo REPO -pull NUMBER -out /tmp/review-fixture.json
```

The generator is read-only: it resolves the repository installation, fetches PR metadata and changed files, records only repository files that `BuildRepoContext` actually reads, and writes a local fixture. Do not commit fixtures generated from private repositories unless they have been reviewed and intentionally sanitized.

Inline PR review comments are available behind `INLINE_COMMENTS_ENABLED=true`. The bot only creates inline comments for `blocker` or `warning` findings whose `file:line` maps to a RIGHT-side line in the PR diff and whose evidence fields pass the inline quality gate; unmapped, low-confidence, or lower-severity findings stay in the summary comment. Service logs include safe aggregate inline counters for each run so quality thresholds can be tuned without exposing source snippets.

## Production Deployment

Read the production and operations runbooks before deploying:

- [Production hardening and E2E verification](docs/production.md)
- [Operations runbook](docs/operations.md)
- [E2E evidence template](docs/e2e-evidence-template.md)

The deployed service should run under systemd or an equivalent durable service manager. This repository includes a systemd unit template at [deploy/systemd/github-ai-reviewer.service](deploy/systemd/github-ai-reviewer.service).

Basic production health checks:

```bash
systemctl is-active github-ai-reviewer.service
curl -fsS http://127.0.0.1:8095/healthz
curl -fsS https://<your-public-host>/healthz
scripts/check_service_health.sh
```

Filled E2E evidence should stay outside git unless every identifier and excerpt is intentionally safe to publish.

## Safety Checks

The publication safety check validates that the public project material exists and that sensitive files are not tracked or staged:

```bash
scripts/check_publication_safety.sh
```

It reports file paths only. It does not read or print secret file contents.

## Roadmap

Planned future work:

- Repository configuration file such as `.github/ai-review.yml`.
- Inline review comments for high-confidence findings.
- Static analyzer integration and finding verification.
- Durable job storage and review history.
- Better repository context selection for large changes.
- Optional CLI or GitHub Action entrypoints.

## License

MIT. See [LICENSE](LICENSE).
