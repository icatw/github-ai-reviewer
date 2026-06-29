# Contributing

Thanks for working on GitHub AI Reviewer. This project is a GitHub App service that touches private repositories, model providers, and deployment credentials, so changes should be small, verified, and careful about data exposure.

## Development Workflow

Use the existing Go and OpenSpec workflow for non-trivial changes:

```bash
openspec list
openspec validate --specs --strict
```

For a new feature or behavior change, create an OpenSpec proposal before implementation, then apply and archive it after verification. Keep implementation scope aligned with the accepted proposal.

## Verification

Run these checks before opening a PR or publishing changes:

```bash
go test ./...
go build ./cmd/server
scripts/smoke_local.sh
scripts/check_e2e_safety.sh
scripts/check_publication_safety.sh
openspec validate --specs --strict
```

Run `gofmt -w` on changed Go files before committing.

## Security Boundaries

Never commit:

```text
.env
.env.* except .env.example
*.pem
*.key
private-key*.pem
installation tokens
LLM API keys
webhook secrets
local databases
server binaries
raw webhook payload captures
raw prompts
raw model responses
filled private E2E evidence
private repository source excerpts
```

The service must not log or publish secrets, raw private payloads, raw prompts, raw model responses, or unintended private source. Error messages should name configuration keys or failure categories, not values.

Workspace checkout is disabled by default. Changes that enable or expand checkout behavior need explicit design review, path validation, cleanup behavior, and tests.

## Evidence Handling

Use `docs/e2e-evidence-template.md` as a template, but keep filled E2E records outside git unless every identifier, excerpt, and URL is intentionally safe to publish. Run `scripts/check_e2e_safety.sh` and `scripts/check_publication_safety.sh` before staging docs from test runs.

## PR Expectations

- Keep changes focused on one milestone or one bug.
- Add or update tests for deterministic behavior.
- Preserve advisory/non-blocking Check Run policy for AI findings.
- Do not change GitHub App permissions, checkout behavior, analyzer behavior, or production deployment behavior without documenting the reason in OpenSpec.
