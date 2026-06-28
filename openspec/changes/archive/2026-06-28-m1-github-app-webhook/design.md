## Context

`github-ai-reviewer` is at M1: the smallest real GitHub App loop. The project documents define the loop as:

```text
pull_request webhook
  -> verify GitHub signature
  -> parse supported PR event
  -> create review job
  -> exchange installation token
  -> fetch PR changed files / patches
  -> call OpenAI-compatible LLM
  -> post PR comment
```

The implementation must be a Go service using the existing package layout. Core logic belongs in `internal/*`; `cmd/server/main.go` should only load config, wire dependencies, and start the server. Webhook payloads are untrusted until signature verification succeeds. Secrets, private keys, API keys, installation tokens, and raw private repository payloads must not be logged.

## Goals / Non-Goals

**Goals:**

- Provide an HTTP server with `/healthz` and `/github/webhook`.
- Load and validate M1 config from environment variables.
- Verify `X-Hub-Signature-256` with HMAC-SHA256 before JSON parsing.
- Accept only `pull_request` events with `opened`, `synchronize`, or `reopened` actions.
- Create typed review jobs with installation ID, owner, repo, pull number, head SHA, action, and delivery ID.
- Return quickly from the webhook handler after handing accepted jobs to a worker boundary.
- Generate GitHub App JWTs from the configured App ID and private key.
- Exchange installation tokens and use them to fetch PR changed files and patches.
- Call an OpenAI-compatible LLM with a concise review prompt.
- Render and publish one conservative PR issue comment containing the review summary.
- Cover deterministic behavior with unit tests, fixtures, fakes, or mocks.
- Verify the server builds and `/healthz` responds locally.

**Non-Goals:**

- Dashboard, billing, tenant management, or installation management UI.
- Durable persistence or production queueing.
- Full repository indexing, vector search, AST analysis, or tree-sitter call graphs.
- Automatic code fixing.
- Inline review comments, Check Runs, request-changes decisions, or merge-blocking behavior.
- Issue comment commands or bot command routing.

## Decisions

### Use `net/http` and keep `main.go` as wiring

The service only needs health and webhook endpoints for M1. Standard `net/http` keeps dependencies small and is sufficient for route registration, body reading, status responses, and tests. `cmd/server/main.go` should construct config, GitHub/LLM clients, comment publisher, worker, and handlers, then call the HTTP server.

Alternative considered: introduce Gin or another router. That is unnecessary until route count, middleware, or binding needs grow.

### Fail fast on config required by the full M1 loop

`internal/config` should load typed settings for listen address, GitHub webhook secret, GitHub App ID, private key path or value, LLM base URL, LLM API key, and LLM model. Missing required settings must produce clear startup errors. The package may support `.env` only if a lightweight dependency is deliberately chosen; standard environment variables are enough for M1.

Alternative considered: validate only webhook settings first. That would let the server accept webhooks but fail every accepted job later, which is not the M1 end-to-end loop.

### Verify before parsing

The webhook handler should read request bytes, validate `X-Hub-Signature-256` using the configured secret, and only then parse JSON. Verification must reject missing, malformed, and mismatched signatures using constant-time comparison.

Alternative considered: parse JSON first and verify afterward. That conflicts with the security rule that payloads are untrusted before verification.

### Parse narrow payload structs into a domain job

`internal/webhook` should define only the fields required for M1 job creation: action, installation ID, repository owner and name, PR number, and head SHA. It should return a typed `review.Job` or equivalent domain type rather than generic maps.

Alternative considered: use full GitHub webhook SDK structs. That adds broad dependency surface and can make fixtures harder to keep focused.

### Ignore unsupported events and actions cleanly

Unsupported GitHub events and unsupported PR actions should return a clean ignored response and create no job. Invalid signatures, malformed signed JSON for supported events, and missing required fields should return client errors and create no job.

Alternative considered: return errors for all unsupported deliveries. GitHub Apps can receive setup or future events; clean ignores reduce operational noise.

### Process review work behind a worker boundary

The webhook handler should enqueue or submit the job to an in-memory worker/job sink and return `202 Accepted` after acceptance. The worker orchestrates GitHub token exchange, PR file fetching, LLM review, Markdown rendering, and comment publishing.

Alternative considered: perform GitHub and LLM calls inline in the handler. That risks webhook timeouts and violates the project rule that long work belongs in a worker path.

### Keep GitHub App authentication isolated

`internal/githubapp` should own private key parsing, JWT generation, installation token exchange, and creation of authenticated GitHub clients. Installation tokens should not be logged and do not need durable storage for M1. In-memory short-lived caching is optional but not required.

Alternative considered: spread JWT and token exchange logic through the review worker. That would make security-sensitive code harder to test and reuse.

### Publish a single conservative summary comment

`internal/review` should build a compact prompt from PR metadata and changed file patches, ask the LLM for concise evidence-based feedback, and pass the result to `internal/comment` for Markdown rendering. `internal/comment` should publish through the GitHub Issues comment API because PR conversation comments are issue comments.

Alternative considered: publish inline comments or Check Runs immediately. Those are later milestones and require stronger finding validation.

## Risks / Trade-offs

- Signature verification bugs could accept forged payloads -> Cover valid, invalid, missing, and malformed signatures with deterministic tests.
- Long-running worker jobs can be lost on process restart -> Accept for M1; durable storage is a later milestone.
- Large PR patches can exceed LLM context limits -> Cap prompt input, summarize omitted files, and make limitations explicit in the LLM prompt/comment.
- LLM output can be noisy or unsupported by evidence -> Ask for conservative summary-only feedback and never block merges in M1.
- GitHub API failures or LLM failures can leave a job without a comment -> Return webhook acceptance only after enqueue; log safe job metadata and expose errors through tests/fakes, with retries deferred unless simple.
- Private key and token handling is security-sensitive -> Keep secrets out of logs, isolate auth code, and test without real credentials.

## Migration Plan

This is the first implementation milestone, so no data migration is required.

Deployment steps:

1. Build the server.
2. Configure environment variables for the server, GitHub App, webhook secret, and LLM provider.
3. Configure the GitHub App with the documented M1 permissions and the `/github/webhook` URL.
4. Start the service and verify `/healthz`.
5. Open or update a test PR and confirm a PR comment appears.

Rollback is to stop the service or disable the GitHub App webhook. No durable state is introduced by M1.

## Open Questions

- Should the first implementation support `GITHUB_APP_PRIVATE_KEY` content in addition to `GITHUB_APP_PRIVATE_KEY_PATH`, or only the path form documented in `.env.example`?
- Should worker failures post a short failure comment, or only log safe metadata in M1? The preferred M1 default is no failure comment to avoid noisy PRs.
- Should unsupported events return `204 No Content` or another explicit ignored status? The implementation should choose one and lock it with tests.
