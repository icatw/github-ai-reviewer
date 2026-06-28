## Why

The project needs the first real GitHub App loop before later review intelligence can be useful. M1 proves that a pull request webhook can be trusted, converted into a review job, processed with GitHub App credentials, sent to an OpenAI-compatible LLM, and written back to the PR as a conservative summary comment.

This change is needed now because the repository scope and architecture are documented, but the implementation milestone needs a precise OpenSpec contract for the minimum end-to-end product loop.

## What Changes

- Add a Go HTTP server with `GET /healthz` and `POST /github/webhook`.
- Add typed config loading and startup validation for the M1 server, GitHub App, webhook, and LLM settings.
- Verify GitHub webhook requests with `X-Hub-Signature-256` before parsing payloads.
- Parse `pull_request` events and accept only `opened`, `synchronize`, and `reopened` actions.
- Create typed review jobs containing installation ID, owner, repo, pull number, head SHA, action, and delivery ID.
- Process accepted jobs outside the webhook handler through an in-memory worker boundary.
- Generate a GitHub App JWT and exchange it for an installation token.
- Fetch Pull Request changed files and patches through the GitHub API.
- Build a concise, evidence-based review prompt from PR metadata and changed file patches.
- Call an OpenAI-compatible LLM for a conservative review summary.
- Render stable Markdown and publish one PR conversation comment through the Issues comment API.
- Add deterministic tests for config, webhook signature verification, event filtering, job extraction, comment rendering, and orchestrator behavior using fixtures, fakes, or mocks.
- Add or maintain `.gitignore` entries for local secrets, private keys, databases, and build artifacts.

No breaking changes are expected because this is the first implementation milestone.

## Capabilities

### New Capabilities

- `github-app-review-loop`: Receive trusted PR webhooks, create review jobs, fetch PR changes with GitHub App authentication, request an LLM review summary, and publish a PR comment.

### Modified Capabilities

None.

## Impact

Affected code and artifacts:

- `cmd/server/`
- `internal/config/`
- `internal/webhook/`
- `internal/githubapp/`
- `internal/review/`
- `internal/llm/`
- `internal/comment/`
- `internal/worker/`
- `testdata/` or package-local fixture files
- `.gitignore`
- `go.mod` / `go.sum` if dependencies are added

Affected external systems:

- GitHub App webhook deliveries for the `pull_request` event.
- GitHub App authentication and installation access token exchange.
- GitHub Pull Request Files API.
- GitHub Issues comment API for PR conversation comments.
- OpenAI-compatible chat or responses API.

Out of scope for this change:

- Dashboard, billing, tenant management, or repository installation UI.
- Durable job persistence, SQLite schema, or retry queues beyond a simple in-memory M1 worker.
- Vector database, full repository indexing, AST analysis, or tree-sitter call graphs.
- Automatic code fixing, inline review comments, Check Run gating, or failing PR checks.
- Issue comment commands or slash command interaction.
