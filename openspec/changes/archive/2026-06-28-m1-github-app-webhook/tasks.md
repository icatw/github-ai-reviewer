## 1. Project Setup

- [x] 1.1 Create the M1 package structure under `cmd/server`, `internal/config`, `internal/webhook`, `internal/githubapp`, `internal/review`, `internal/llm`, `internal/comment`, and `internal/worker` if missing.
- [x] 1.2 Ensure `.gitignore` excludes `.env`, private key files, local databases, data directories, and the built `server` binary.
- [x] 1.3 Keep `go.mod` dependency changes minimal; prefer the standard library except for GitHub API and JWT support.

## 2. Config

- [x] 2.1 Implement typed config loading from environment variables for server listen address, GitHub webhook secret, GitHub App ID, GitHub private key path or value, LLM base URL, LLM API key, and LLM model.
- [x] 2.2 Implement config validation with clear missing-field errors and without logging secret values.
- [x] 2.3 Add unit tests for successful config loading and missing required M1 config.

## 3. Webhook Verification And Parsing

- [x] 3.1 Implement `X-Hub-Signature-256` verification using HMAC-SHA256 and constant-time comparison.
- [x] 3.2 Reject missing, malformed, or mismatched signatures before parsing the payload as JSON.
- [x] 3.3 Implement narrow typed parsing for `pull_request` payload fields required to create a review job.
- [x] 3.4 Filter supported actions to `opened`, `synchronize`, and `reopened`.
- [x] 3.5 Return a clean ignored response for unsupported GitHub events and unsupported pull request actions without creating jobs.
- [x] 3.6 Add fixture-based tests for valid signatures, invalid signatures, unsupported events, unsupported actions, missing fields, and supported action job extraction.

## 4. Domain And Worker Boundary

- [x] 4.1 Define a typed `review.Job` with installation ID, owner, repo, pull number, head SHA, action, and delivery ID.
- [x] 4.2 Define a job sink or worker interface used by the webhook handler.
- [x] 4.3 Implement an in-memory worker path that accepts jobs outside the webhook handler.
- [x] 4.4 Add tests proving accepted webhooks return after job submission and do not run downstream work inline.

## 5. GitHub App Client

- [x] 5.1 Implement GitHub App private key loading and JWT generation.
- [x] 5.2 Implement installation token exchange for a job installation ID.
- [x] 5.3 Implement an installation-authenticated GitHub client or narrow interface for PR files and issue comments.
- [x] 5.4 Implement PR changed-files fetching with filename, status, additions, deletions, and patch data.
- [x] 5.5 Add unit tests using fakes or mock transports without real GitHub credentials.

## 6. LLM Review

- [x] 6.1 Implement an OpenAI-compatible LLM client using configured base URL, API key, and model.
- [x] 6.2 Build a concise review prompt from PR metadata and changed file patches.
- [x] 6.3 Cap or omit oversized patch content and include limitations in the prompt context.
- [x] 6.4 Add tests for prompt construction and LLM client request/response handling using fakes.

## 7. Comment Rendering And Publishing

- [x] 7.1 Implement stable Markdown rendering for a conservative AI review summary.
- [x] 7.2 Ensure empty or whitespace-only LLM output does not produce an empty or noisy PR comment.
- [x] 7.3 Publish the rendered comment through the GitHub Issues comment API for the PR number.
- [x] 7.4 Add tests for markdown rendering and comment publishing through fake clients.

## 8. Review Orchestration

- [x] 8.1 Implement review orchestration from job to installation token, PR files, LLM summary, rendered comment, and published PR comment.
- [x] 8.2 Ensure logs contain safe job metadata only and never include secrets, installation tokens, private keys, API keys, or raw private repository payloads.
- [x] 8.3 Add orchestration tests using fake GitHub, LLM, and comment clients for success and failure paths.

## 9. HTTP Server

- [x] 9.1 Implement `GET /healthz` with a simple successful response that does not expose secrets.
- [x] 9.2 Implement `POST /github/webhook` to read the body, verify the signature, filter the event, parse the payload, and hand accepted jobs to the worker.
- [x] 9.3 Keep `cmd/server/main.go` limited to loading config, wiring dependencies/routes, and starting the HTTP server.
- [x] 9.4 Ensure accepted supported webhooks return `202 Accepted` after the job is accepted.

## 10. Verification

- [x] 10.1 Run `gofmt -w .`.
- [x] 10.2 Run `go test ./...`.
- [x] 10.3 Run `go build ./cmd/server`.
- [x] 10.4 Start the built server with required environment variables and verify `GET /healthz` with `curl`.
- [x] 10.5 Run `openspec validate m1-github-app-webhook --type change --strict`.
