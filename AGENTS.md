# AGENTS.md

This file gives Hermes, Codex, and other coding agents project-specific context for `github-ai-reviewer`.

## Project Summary

`github-ai-reviewer` is a Go service for a GitHub App based AI code review bot. The product goal is to let a user install the GitHub App on selected repositories, then automatically review Pull Requests when they are opened or updated.

The first milestone is not a full Codex-like repository intelligence system. The first milestone is the smallest real GitHub App loop:

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

Read these docs before non-trivial work:

- `README.md` for project scope and layout.
- `docs/design.md` for architecture, permissions, event flow, accuracy policy, and phase plan.
- `docs/research.md` for references from PR-Agent, reviewdog, Probot, and GitHub official APIs.

## Current Milestone

Focus on M1: GitHub App minimum loop.

M1 includes:

- HTTP server with `/healthz` and `/github/webhook`.
- Config loading from environment variables / `.env`.
- GitHub webhook signature verification using `X-Hub-Signature-256`.
- Pull request event parsing for `opened`, `synchronize`, and `reopened`.
- Review job creation with `installation_id`, owner, repo, pull number, and head SHA.
- GitHub App JWT generation and installation token exchange.
- Pull Request changed files fetching.
- LLM review summary.
- PR issue comment publishing.

Do not implement these in M1 unless explicitly requested:

- Dashboard.
- Billing or tenant management.
- Vector database.
- Full repository indexing.
- Tree-sitter / AST call graph.
- Automatic code fixing.
- Inline review comments.
- Check Run gating.

Those are later milestones.

## Tech Stack

Use Go for the service.

Preferred libraries:

- HTTP: standard library `net/http` unless Gin becomes useful for routing.
- GitHub API: `github.com/google/go-github/v68/github` or current stable major.
- JWT: `github.com/golang-jwt/jwt/v5`.
- Env loading: keep simple; use standard env first. A lightweight dotenv dependency is acceptable if needed.
- Storage: SQLite later. M1 may start without persistence if job state is in memory and tests cover behavior.

Keep the dependency set small until M1 is verified end to end.

## Directory Conventions

Use the existing layout:

```text
cmd/server/          HTTP server entrypoint
internal/config/     service config loading and validation
internal/webhook/    GitHub webhook parsing and signature verification
internal/githubapp/  GitHub App auth, JWT, installation token, GitHub client
internal/review/     review orchestration and domain types
internal/llm/        OpenAI-compatible LLM client
internal/comment/    GitHub comment rendering and publishing
internal/storage/    persistence, later phase
internal/worker/     async worker, in-memory for M1
deploy/              Docker and deployment files
scripts/             local/test automation scripts
docs/                design and research docs
```

Avoid putting core logic in `cmd/server/main.go`. Keep `main.go` as wiring only.

## Implementation Rules

- Make small, verifiable changes.
- Prefer explicit types over generic maps after payload parsing.
- Validate config at startup and return clear errors for missing required values.
- Never log secrets, private keys, installation tokens, API keys, or complete webhook payloads from private repositories.
- Webhook handlers must return quickly. Long review work belongs in a worker/job path.
- Treat GitHub webhook payload as untrusted until the signature is verified.
- Do not make AI findings blocking until static checks and finding verification exist.
- Do not fabricate GitHub or LLM results in tests. Use fixtures, mocks, or clearly named fake clients.
- Keep comments short and useful. Avoid narrating obvious code.

## Testing Expectations

For every core package, add unit tests when behavior is deterministic.

Minimum tests for M1:

- `internal/webhook`: valid signature accepted.
- `internal/webhook`: invalid signature rejected.
- `internal/webhook`: unsupported event ignored or rejected cleanly.
- `internal/webhook`: unsupported PR action ignored.
- `internal/webhook`: supported PR action produces expected review job fields.
- `internal/config`: missing required config returns a useful error.
- `internal/comment`: markdown rendering is stable and does not emit empty/noisy comments.

Run before reporting completion:

```bash
gofmt -w .
go test ./...
go build ./cmd/server
```

When a server is implemented, also verify with real commands:

```bash
./server
curl -sS http://127.0.0.1:<port>/healthz
```

Use a different port if the default is already occupied.

## GitHub App Notes

Minimum repository permissions for M1:

```text
Metadata: Read-only
Contents: Read-only
Pull requests: Read and write
Issues: Read and write
```

`Issues: write` is needed because PR conversation comments use the Issues comment API.

Future Check Run integration requires:

```text
Checks: Read and write
```

Webhook events for M1:

```text
Pull request
```

Supported actions for M1:

```text
opened
synchronize
reopened
```

Future command interaction can add:

```text
Issue comment
```

## Accuracy Policy

MVP should be conservative.

- Post a summary comment only.
- Do not request changes automatically.
- Do not fail checks in M1.
- Ask the LLM for concise, evidence-based feedback.
- If context is insufficient, output a question or limitation instead of a blocker.

Later finding objects should include:

```text
severity
category
file
line
title
evidence
failure_scenario
suggestion
confidence
```

High severity findings require code evidence and a plausible failure scenario.

## Agent Workflow

For coding tasks in this repository:

1. Load relevant skills when available: `go-api-service`, `github-workflows`, `systematic-debugging`, `requesting-code-review`, and `codex`.
2. Inspect existing files before editing.
3. For multi-file implementation, prefer OpenSpec + Codex rather than ad hoc planning.
4. Implement one milestone slice at a time.
5. Run `gofmt`, `go test ./...`, and `go build ./cmd/server` after code changes.
6. Report actual command output summaries. Do not claim verification that was not run.

If using Codex CLI from Hermes, give Codex a high-level goal plus the relevant files and acceptance checks. Do not over-specify every line unless necessary.

## OpenSpec + Codex Workflow

OpenSpec is configured for Codex in this repo. The correct integration is generated by:

```bash
openspec init --tools codex --force
openspec update --force
```

This creates project-local Codex skills in `.codex/skills/openspec-*`. For non-trivial features, use Codex with the OpenSpec slash workflow:

```text
/opsx:propose "change description"
/opsx:apply <change-name>
/opsx:archive <change-name>
```

Do not manually guess OpenSpec artifact paths or old commands. Use CLI outputs as source of truth:

```bash
openspec status --change <change-name> --json
openspec instructions <artifact-id> --change <change-name> --json
openspec instructions apply --change <change-name> --json
openspec validate <change-name> --type change --strict
```

Hermes should act as commander: update OpenSpec/Codex, write or trigger the high-level Codex prompt, then verify diffs and command output. Codex should act as executor for project-level source code changes.

## Security Constraints

Never commit:

- `.env`
- GitHub App private key files
- API keys
- installation tokens
- webhook secrets
- raw private repository payload dumps
- local SQLite databases containing job data

Add or maintain `.gitignore` before generated files appear.

Sensitive files to ignore include:

```text
.env
*.pem
*.key
private-key*.pem
data/
*.db
server
```

## Completion Criteria for M1

M1 is complete only when all are true:

- Unit tests pass.
- Server builds.
- `/healthz` responds locally.
- Webhook signature verification has tests.
- PR event parsing has tests using fixtures.
- A real GitHub App can be configured with the documented permissions.
- A real test PR can trigger the service and receive a PR comment.

Until the real PR comment is observed through GitHub API or PR page, the project is not end-to-end complete.
