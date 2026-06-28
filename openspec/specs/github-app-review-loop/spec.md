# github-app-review-loop Specification

## Purpose
TBD - created by archiving change m1-github-app-webhook. Update Purpose after archive.
## Requirements
### Requirement: Health endpoint
The service SHALL expose a `GET /healthz` endpoint that reports the server is running without requiring a GitHub webhook delivery.

#### Scenario: Health check succeeds
- **WHEN** a client sends `GET /healthz`
- **THEN** the service responds with a successful HTTP status
- **AND** the response does not expose secrets or dependency-specific credential details

### Requirement: Runtime config loading
The service SHALL load typed runtime configuration from environment variables and validate all settings required for the M1 GitHub App review loop.

#### Scenario: Required config is missing
- **WHEN** the service starts without a required M1 setting
- **THEN** config validation fails with a useful error identifying the missing setting
- **AND** the error does not include secret values

#### Scenario: Required config is present
- **WHEN** the service starts with the required server, GitHub App, webhook, and LLM settings
- **THEN** config loading succeeds

### Requirement: Webhook signature verification
The webhook endpoint SHALL verify `X-Hub-Signature-256` with the configured webhook secret before parsing the request body as a GitHub payload.

#### Scenario: Valid signature is accepted
- **WHEN** a webhook request contains a body and a matching `sha256=` HMAC signature
- **THEN** signature verification succeeds
- **AND** the webhook handler may parse the payload

#### Scenario: Invalid signature is rejected
- **WHEN** a webhook request contains a body and a mismatched `X-Hub-Signature-256`
- **THEN** the service rejects the request
- **AND** no review job is created

#### Scenario: Missing or malformed signature is rejected
- **WHEN** a webhook request omits `X-Hub-Signature-256` or uses an unsupported format
- **THEN** the service rejects the request
- **AND** no payload fields are trusted

### Requirement: Pull request event filtering
The webhook endpoint SHALL create review jobs only for GitHub `pull_request` events whose action is `opened`, `synchronize`, or `reopened`.

#### Scenario: Unsupported event is ignored
- **WHEN** a signed webhook request uses an `X-GitHub-Event` value other than `pull_request`
- **THEN** the service returns a clean ignored response
- **AND** no review job is created

#### Scenario: Unsupported pull request action is ignored
- **WHEN** a signed `pull_request` webhook has an action other than `opened`, `synchronize`, or `reopened`
- **THEN** the service returns a clean ignored response
- **AND** no review job is created

#### Scenario: Supported pull request action is accepted
- **WHEN** a signed `pull_request` webhook has action `opened`, `synchronize`, or `reopened`
- **THEN** the service accepts the event for review job creation

### Requirement: Review job creation
For each supported pull request event, the service SHALL create a typed review job containing the installation ID, owner, repo, pull number, head SHA, action, and GitHub delivery ID.

#### Scenario: Supported payload produces expected job fields
- **WHEN** a signed supported `pull_request` webhook contains the required installation, repository, pull request, and delivery fields
- **THEN** the created review job contains the expected installation ID
- **AND** the job contains the expected owner, repo, pull number, head SHA, action, and delivery ID

#### Scenario: Required job field is missing
- **WHEN** a signed supported `pull_request` webhook is missing a field required for review job creation
- **THEN** the service rejects the payload with a client error
- **AND** no review job is created

### Requirement: Fast webhook response
The webhook handler SHALL return after accepting or ignoring the webhook without fetching GitHub PR files, calling an LLM, or publishing comments inline.

#### Scenario: Supported event is submitted to worker
- **WHEN** a signed supported pull request webhook is received
- **THEN** the service hands the review job to an in-memory worker or job sink
- **AND** the HTTP response is `202 Accepted` once the job is accepted

#### Scenario: Downstream review work is not executed in handler
- **WHEN** a supported webhook is handled
- **THEN** the handler does not exchange installation tokens
- **AND** the handler does not fetch changed files, call an LLM, or post a PR comment

### Requirement: GitHub App installation authentication
The worker SHALL authenticate as the GitHub App and exchange a job installation ID for an installation access token before calling repository APIs.

#### Scenario: Installation token is exchanged
- **WHEN** a review job is processed
- **THEN** the service generates a GitHub App JWT from configured App credentials
- **AND** the service exchanges the job installation ID for an installation access token

#### Scenario: Authentication failure stops the job
- **WHEN** GitHub App JWT generation or installation token exchange fails
- **THEN** the review job stops without calling the LLM
- **AND** no PR comment is published for that failed job

### Requirement: Pull request changed files fetching
The worker SHALL fetch changed file metadata and patches for the job pull request using an installation-authenticated GitHub client.

#### Scenario: Changed files are fetched
- **WHEN** a review job has a valid installation token
- **THEN** the service requests the pull request changed files for the job owner, repo, and pull number
- **AND** the review context includes filename, status, additions, deletions, and patch data for each returned file when available

#### Scenario: Changed files request fails
- **WHEN** the GitHub API returns an error while fetching pull request files
- **THEN** the review job stops without calling the LLM
- **AND** no PR comment is published for that failed job

### Requirement: LLM review summary
The worker SHALL request structured, evidence-based review data from an OpenAI-compatible LLM using PR metadata and changed file context.

#### Scenario: Structured LLM review is requested
- **WHEN** changed file context is available for a review job
- **THEN** the service sends the configured model a prompt containing PR metadata and bounded changed-file patch context
- **AND** the prompt asks for JSON-only output matching the structured review result schema
- **AND** the prompt asks for conservative feedback that does not fabricate unavailable context

#### Scenario: Oversized patch context is bounded
- **WHEN** changed file patches exceed the configured or implementation-defined prompt budget
- **THEN** the service omits or truncates excess patch context
- **AND** the review prompt or structured result records that some context was omitted

#### Scenario: LLM request fails
- **WHEN** the LLM provider returns an error or unusable response
- **THEN** the review job stops without publishing an empty or fabricated review comment

#### Scenario: LLM output is malformed
- **WHEN** the LLM provider returns non-JSON, malformed JSON, empty choices, or JSON that fails review result validation
- **THEN** the review job stops without publishing a review comment
- **AND** the job logs safe failure metadata including the failing stage and validation category

### Requirement: PR comment rendering
The service SHALL render typed review data as deterministic GitHub Markdown for a PR conversation comment.

#### Scenario: Structured review output is rendered
- **WHEN** the LLM returns a valid structured review result with useful content
- **THEN** the comment renderer produces deterministic Markdown identifying the comment as an AI review summary
- **AND** the rendered comment includes stable sections for summary, risk, findings, missing tests, and limitations when those fields are present
- **AND** the rendered comment remains conservative and non-blocking

#### Scenario: Bot marker is included
- **WHEN** the service renders a review comment
- **THEN** the comment includes a stable hidden marker that identifies it as this service's review comment
- **AND** the marker does not expose secrets, tokens, or private repository payload data

#### Scenario: Empty review output is suppressed
- **WHEN** the structured review result has no useful summary, findings, missing tests, or limitations after validation
- **THEN** the service does not publish an empty or noisy PR comment

### Requirement: PR comment publishing
The worker SHALL publish the rendered review summary as a Pull Request conversation comment through the GitHub Issues comment API, updating a previous bot review comment when one exists.

#### Scenario: New review comment is created
- **WHEN** a review job has rendered non-empty Markdown
- **AND** no existing issue comment on the PR contains the service's stable hidden marker
- **THEN** the service creates an issue comment for the job owner, repo, and pull number
- **AND** the comment appears on the PR conversation

#### Scenario: Existing review comment is updated
- **WHEN** a review job has rendered non-empty Markdown
- **AND** an existing issue comment on the PR contains the service's stable hidden marker
- **THEN** the service updates that issue comment with the newly rendered Markdown
- **AND** the service does not create a duplicate review comment for the same PR

#### Scenario: Unrelated comments are ignored
- **WHEN** the PR contains comments from humans or other bots that do not include the service's stable hidden marker
- **THEN** the service does not update those comments
- **AND** it creates a new review comment if no marker comment exists

#### Scenario: Comment publishing fails
- **WHEN** the GitHub Issues comments list, create, or update API returns an error
- **THEN** the review job records or logs safe failure metadata
- **AND** the service does not retry by creating duplicate comments inside the webhook handler

### Requirement: Structured review result validation
The service SHALL parse, validate, and normalize LLM review output into a typed review result before rendering or publishing it.

#### Scenario: Valid structured review is accepted
- **WHEN** the LLM output contains a valid review result with a summary or at least one useful finding
- **THEN** the service parses the result into typed fields
- **AND** the result can be passed to the comment renderer

#### Scenario: Review result fields are validated
- **WHEN** the LLM output contains structured review result fields
- **THEN** the result accepts `summary`, `risk_score`, `findings`, `missing_tests`, and `limitations`
- **AND** unknown or unavailable context is represented as a limitation instead of fabricated evidence

#### Scenario: Finding fields are validated
- **WHEN** the LLM output contains findings
- **THEN** each finding severity is one of `blocker`, `warning`, `suggestion`, or `question`
- **AND** each finding confidence is within `0.0` through `1.0` when present
- **AND** each finding risk details are treated as advisory and non-blocking in M2

#### Scenario: Risk score is bounded
- **WHEN** the LLM output contains a risk score
- **THEN** the service accepts only bounded risk score values from `0` through `100`
- **AND** invalid score values cause validation failure instead of silent publication

#### Scenario: Fenced JSON is tolerated
- **WHEN** the LLM output wraps otherwise valid JSON in a Markdown code fence
- **THEN** the service extracts and parses the JSON content

#### Scenario: Invalid structured review is rejected
- **WHEN** the LLM output is malformed or lacks useful review content
- **THEN** validation fails with a safe error category
- **AND** no PR comment is published for that job

### Requirement: Review reporter fan-out
The worker SHALL publish review lifecycle output through a reporter layer that can send the same supported PR review job and structured review result to multiple output channels.

#### Scenario: Completed review is sent to configured reporters
- **WHEN** a supported PR review job completes with a validated structured review result
- **THEN** the worker sends the job metadata and review result to each configured reporter
- **AND** the reporter layer does not require the webhook handler to perform downstream publishing work

#### Scenario: Comment upsert remains a reporter output
- **WHEN** the comment reporter receives a completed review with rendered non-empty Markdown
- **THEN** it preserves the existing stable marker upsert behavior for the PR conversation comment
- **AND** it does not create duplicate bot review comments when a marker comment already exists

#### Scenario: Reporter failure is recorded safely
- **WHEN** a reporter fails while publishing review output
- **THEN** the worker records or logs safe failure metadata identifying the reporter and failure category
- **AND** logs do not include secrets, installation tokens, private keys, API keys, complete webhook payloads, raw prompts, or raw model responses

### Requirement: GitHub Check Run review status reporting
The worker SHALL report supported PR review job status through a GitHub Check Run named `AI Review` on the pull request head SHA.

#### Scenario: Review start creates or updates in-progress check
- **WHEN** a supported PR review job starts processing after webhook acceptance
- **THEN** the Check Run reporter creates or updates an `AI Review` Check Run for the job owner, repo, and head SHA
- **AND** the Check Run status is `in_progress`

#### Scenario: Review completion updates check
- **WHEN** a supported PR review job completes without infrastructure or job execution failure
- **THEN** the Check Run reporter updates the matching `AI Review` Check Run for the job head SHA to `completed`
- **AND** the conclusion is `success` or `neutral`
- **AND** the Check Run output remains advisory and non-blocking

#### Scenario: AI findings do not fail check
- **WHEN** a completed structured review result contains findings of any allowed severity
- **THEN** the Check Run reporter does not set the conclusion to `failure` based on those findings
- **AND** the service does not request changes or block merging in M3

#### Scenario: Infrastructure failure may fail check
- **WHEN** review processing fails because of infrastructure or job execution failure after an in-progress Check Run can be identified
- **THEN** the Check Run reporter may update `AI Review` to `completed` with conclusion `failure`
- **AND** the Check Run output includes only safe failure category metadata
- **AND** the failure is not derived from AI review findings

#### Scenario: Check Run output is safe and concise
- **WHEN** the service creates or updates the `AI Review` Check Run output
- **THEN** the output identifies the review status in concise GitHub Markdown
- **AND** it does not include secrets, installation tokens, private keys, API keys, complete webhook payloads, raw prompts, raw model responses, or unbounded private diff content

### Requirement: Stateless Check Run update preference
The Check Run reporter SHALL prefer stateless GitHub API lookup or create/update semantics for matching `AI Review` Check Runs and SHALL NOT introduce durable storage for Check Run IDs unless required for correct update semantics.

#### Scenario: Existing check run can be matched
- **WHEN** an `AI Review` Check Run already exists for the job head SHA and can be deterministically identified
- **THEN** the Check Run reporter updates that Check Run instead of requiring persisted local state

#### Scenario: Existing check run cannot be matched safely
- **WHEN** no matching `AI Review` Check Run can be deterministically identified for the job head SHA
- **THEN** the Check Run reporter creates a new Check Run for that head SHA
- **AND** it does not add durable storage solely to remember the new Check Run ID unless implementation proves GitHub update semantics require it

### Requirement: GitHub App Checks permission
The service documentation SHALL state that M3 Check Run reporting requires GitHub App Checks read/write permission in addition to the existing metadata, contents, pull requests, and issues permissions.

#### Scenario: Permissions are documented
- **WHEN** a deployer configures the GitHub App for M3
- **THEN** project documentation identifies Checks read/write as required for Check Run reporting
- **AND** Issues write remains documented for PR conversation comment upsert

### Requirement: Reporter and Check Run verification
The implementation SHALL include automated tests and real verification steps for reporter fan-out, Check Run lifecycle behavior, and preserved PR comment upsert behavior.

#### Scenario: Automated verification covers reporter behavior
- **WHEN** M3 implementation is complete
- **THEN** tests cover reporter fan-out to multiple reporters
- **AND** tests cover comment reporter marker upsert preservation
- **AND** tests cover Check Run start, completion, advisory findings not failing checks, and infrastructure failure policy

#### Scenario: Standard commands pass
- **WHEN** M3 implementation is complete
- **THEN** `gofmt -w .` has been run
- **AND** `go test ./...` passes
- **AND** `go build ./cmd/server` passes
- **AND** `openspec validate m3-reporter-check-run --type change --strict` passes

#### Scenario: Real PR verification succeeds
- **WHEN** the service is deployed or restarted with M3 configuration and a real supported PR event is processed
- **THEN** the PR Checks UI or GitHub API shows an `AI Review` Check Run for the PR head SHA
- **AND** the PR conversation contains the service review summary comment
- **AND** repeated supported PR events update the marker comment instead of creating duplicate bot comments

### Requirement: Repo-aware review context construction
The worker SHALL build deterministic repo-aware review prompt context for supported PR review jobs after fetching changed file metadata and before requesting structured LLM review output.

#### Scenario: Prompt includes stable context sections
- **WHEN** a supported PR review job reaches prompt construction
- **THEN** the prompt context includes PR metadata and changed-file patch context
- **AND** the prompt context includes stable sections equivalent to `patch_context`, `full_file_context`, `related_test_context`, `repo_docs_context`, and `omitted_context`
- **AND** the LLM output boundary remains the structured `ReviewResult`

#### Scenario: Webhook remains fast
- **WHEN** a supported pull request webhook is handled
- **THEN** repo-aware context construction is not performed in the webhook handler
- **AND** the handler still returns after accepting the job without fetching repository file content or calling the LLM

### Requirement: Changed full file context
The worker SHALL fetch and include bounded head-version full file content for changed files when the files are safe to include and within implementation-defined budgets.

#### Scenario: Full changed file content is included
- **WHEN** a changed file is not deleted, is textual, is not filtered, and fits within per-file and total context budgets
- **THEN** the worker fetches the file content at the PR head SHA
- **AND** the full file content is included in `full_file_context`

#### Scenario: Deleted changed file is skipped
- **WHEN** a changed file has deleted status
- **THEN** the worker does not fetch head-version full file content for that file
- **AND** `omitted_context` records that the file was skipped because it was deleted

#### Scenario: Oversized full file is omitted or truncated
- **WHEN** a changed file exceeds the implementation-defined per-file content budget
- **THEN** the worker omits or truncates that file content deterministically
- **AND** `omitted_context` records the path and whether the file was oversized or truncated

### Requirement: Related test context selection
The worker SHALL discover and include bounded related test files using deterministic naming conventions without AST analysis, call graph analysis, full repository indexing, or vector search.

#### Scenario: Direct paired test is included
- **WHEN** a changed source file has a same-directory direct paired test file by naming convention such as `foo.go` to `foo_test.go`
- **THEN** the worker fetches the paired test file at the PR head SHA when it is safe and within budget
- **AND** the paired test content is included in `related_test_context`

#### Scenario: Same package tests are bounded
- **WHEN** a changed Go source file has additional same-package `*_test.go` files
- **THEN** the worker may include a deterministic bounded set of those test files
- **AND** excess same-package test candidates are skipped with `omitted_context` entries when candidate or context budgets are reached

#### Scenario: Related test candidates are deduplicated
- **WHEN** multiple changed files map to the same related test file
- **THEN** the worker includes that related test file at most once
- **AND** candidate ordering remains deterministic

### Requirement: Lightweight repo docs and config context
The worker SHALL include bounded lightweight repository documentation and AI review config context when present and safe to include.

#### Scenario: Root README is included
- **WHEN** `README.md` exists at the PR head SHA and fits within the applicable budgets
- **THEN** the worker includes it in `repo_docs_context`

#### Scenario: Docs markdown files are bounded
- **WHEN** markdown files exist under `docs/`
- **THEN** the worker includes only an implementation-defined deterministic bounded set of `docs/*.md` files that fit within budget
- **AND** skipped or truncated docs are represented in `omitted_context`

#### Scenario: AI review config is included when present
- **WHEN** `.github/ai-review.yml` exists at the PR head SHA and fits within budget
- **THEN** the worker includes it in `repo_docs_context`
- **AND** the config content is treated as context only unless a separate requirement defines executable config semantics

### Requirement: Deterministic context filters and budgets
The worker SHALL apply deterministic filters and implementation-defined per-file and total context budgets before sending repo-aware context to the LLM.

#### Scenario: Unsupported file categories are filtered
- **WHEN** candidate context files are binary, generated, lock files, under vendor paths, or under dist/build output paths
- **THEN** the worker skips those files
- **AND** `omitted_context` records the path and filter reason without including file content

#### Scenario: Total context budget is enforced
- **WHEN** candidate patch, full file, related test, docs, and config context exceeds the implementation-defined total context budget
- **THEN** the worker includes context in deterministic priority order until the budget is exhausted
- **AND** remaining candidates are skipped or truncated with `omitted_context` entries

#### Scenario: Context budget behavior is deterministic
- **WHEN** the same PR metadata, changed files, repository contents, and budget settings are processed repeatedly
- **THEN** the worker produces the same included context ordering and the same omitted-context notes

### Requirement: Omitted context reporting
The worker SHALL report omitted context in a stable prompt section so the LLM can describe limitations without fabricating unavailable evidence.

#### Scenario: Omitted context notes are included
- **WHEN** any file or candidate context is skipped, missing, truncated, oversized, filtered, or blocked by budget
- **THEN** `omitted_context` includes a concise note with the path, context category, and omission reason
- **AND** the note does not include secrets, tokens, complete webhook payloads, raw prompts, or raw model responses

#### Scenario: Fetch failures are non-fatal for optional context
- **WHEN** fetching optional full file, related test, docs, or config context fails
- **THEN** the worker records an omitted-context note for that candidate
- **AND** the review job may continue using the remaining available patch and repo context

### Requirement: Repo-aware context verification
The implementation SHALL include automated tests and real verification steps for repo-aware context construction and preserved M1-M3 output behavior.

#### Scenario: Automated verification covers context construction
- **WHEN** M4a implementation is complete
- **THEN** unit tests cover full file fetching
- **AND** unit tests cover related test selection
- **AND** unit tests cover docs/config selection
- **AND** unit tests cover filtering, truncation, total budget enforcement, and omitted-context notes

#### Scenario: Standard commands pass
- **WHEN** M4a implementation is complete
- **THEN** `gofmt -w .` has been run
- **AND** `go test ./...` passes
- **AND** `go build ./cmd/server` passes
- **AND** `openspec validate m4a-repo-aware-context --type change --strict` passes

#### Scenario: Real PR verification preserves existing behavior
- **WHEN** the service is deployed or restarted with M4a and a real supported PR event is processed
- **THEN** the resulting review prompt/context behavior includes richer repo-aware context or explicit omitted-context limitations
- **AND** the existing marker comment upsert behavior still works
- **AND** the existing advisory/non-blocking Check Run behavior still works

### Requirement: Secret-safe operation
The service SHALL avoid logging or returning secrets, private keys, installation tokens, API keys, or complete private repository payloads.

#### Scenario: Error is reported safely
- **WHEN** config, webhook, GitHub API, LLM, or comment publishing fails
- **THEN** logs and HTTP responses include only safe metadata such as event type, action, delivery ID, owner, repo, pull number, and error category
- **AND** logs and HTTP responses do not include configured secrets, private keys, installation tokens, API keys, or complete webhook payloads

