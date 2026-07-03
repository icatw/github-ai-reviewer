## 1. Webhook Command Parsing

- [x] 1.1 Add `issue_comment` event routing after existing webhook signature verification and before payload field trust.
- [x] 1.2 Parse `issue_comment.created` payloads into explicit typed fields for installation ID, repository owner/name, issue number, pull request marker, action, comment body, and delivery ID.
- [x] 1.3 Implement strict `/ai-review` command matching that accepts exact `/ai-review` or `/ai-review` followed by whitespace and rejects lookalikes such as `/ai-reviewer`.
- [x] 1.4 Return `204 No Content` with no job for non-command comments, plain issue comments, unsupported `issue_comment` actions, and unsupported events.

## 2. PR Metadata Resolution and Job Creation

- [x] 2.1 Introduce a narrow pull request metadata resolver boundary that can obtain PR head SHA using installation-authenticated GitHub access and can be faked in tests.
- [x] 2.2 Wire accepted `/ai-review` commands to resolve PR head SHA before enqueueing a job.
- [x] 2.3 Create the same typed `ReviewJob` shape used by supported `pull_request` events, including installation ID, owner, repo, pull number, head SHA, action, and delivery ID.
- [x] 2.4 Ensure missing required command fields and resolver failures do not enqueue partial jobs and do not log raw payloads, raw comment bodies, secrets, tokens, prompts, or model responses.

## 3. Async Review Flow Preservation

- [x] 3.1 Submit accepted manual review command jobs to the existing in-memory worker or job sink and return `202 Accepted` after job acceptance.
- [x] 3.2 Keep changed-file fetching, LLM calls, PR comment publishing, Check Run updates, auto-fix behavior, merge behavior, and inline review behavior out of the webhook handler.
- [x] 3.3 Preserve existing `pull_request` opened, synchronize, and reopened behavior and advisory Check Run reporting.

## 4. Tests and Verification

- [x] 4.1 Add webhook tests for valid signature plus exact `/ai-review` command on a PR producing expected job fields.
- [x] 4.2 Add webhook tests for `/ai-review` followed by whitespace producing expected job fields.
- [x] 4.3 Add ignored-case tests for non-command comments, `/ai-reviewer` lookalikes, plain issue comments, unsupported `issue_comment` actions, and unsupported events returning `204` with no job.
- [x] 4.4 Add tests for missing required command payload fields and PR metadata resolver failure producing the expected error response and no job.
- [x] 4.5 Add tests proving invalid or missing webhook signatures still reject before parsing `issue_comment` payload fields.
- [x] 4.6 Run `gofmt -w .`, `go test ./...`, `go build ./cmd/server`, and `openspec validate m11-ai-review-command --type change --strict`.
