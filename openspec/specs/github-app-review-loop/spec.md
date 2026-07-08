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

#### Scenario: Review start creates fresh in-progress check
- **WHEN** a supported PR review job starts processing after webhook acceptance
- **THEN** the Check Run reporter creates a new `AI Review` Check Run for the job owner, repo, and head SHA
- **AND** the Check Run status is `in_progress`
- **AND** the reporter does not mutate an older completed `AI Review` Check Run during job start

#### Scenario: Review completion updates current in-progress check
- **WHEN** a supported PR review job completes without infrastructure or job execution failure
- **THEN** the Check Run reporter updates the newest matching `AI Review` Check Run for the job head SHA whose status is `in_progress` to `completed`
- **AND** completed historical `AI Review` Check Runs for the same head SHA are ignored when choosing the run to update
- **AND** the conclusion is `success` or `neutral`
- **AND** the Check Run output remains advisory and non-blocking

#### Scenario: AI findings do not fail check
- **WHEN** a completed structured review result contains findings of any allowed severity
- **THEN** the Check Run reporter does not set the conclusion to `failure` based on those findings
- **AND** the service does not request changes or block merging in M18

#### Scenario: Infrastructure failure may fail current in-progress check
- **WHEN** review processing fails because of infrastructure or job execution failure after an in-progress Check Run can be identified
- **THEN** the Check Run reporter may update the matching in-progress `AI Review` Check Run to `completed` with conclusion `failure`
- **AND** completed historical `AI Review` Check Runs for the same head SHA are ignored when choosing the run to update
- **AND** the Check Run output includes only safe failure category metadata
- **AND** the failure is not derived from AI review findings

#### Scenario: Check Run output is safe and concise
- **WHEN** the service creates or updates the `AI Review` Check Run output
- **THEN** the output identifies the review status in concise GitHub Markdown
- **AND** it does not include secrets, installation tokens, private keys, API keys, complete webhook payloads, raw prompts, raw model responses, or unbounded private diff content

### Requirement: Stateless Check Run update preference
The Check Run reporter SHALL prefer stateless GitHub API lookup or create/update semantics for matching `AI Review` Check Runs and SHALL NOT introduce durable storage for Check Run IDs unless required for correct update semantics.

#### Scenario: Existing in-progress check run can be matched
- **WHEN** an `AI Review` Check Run already exists for the job head SHA with status `in_progress` and can be deterministically identified
- **THEN** the Check Run reporter updates that Check Run for completion or failure instead of requiring persisted local state

#### Scenario: Existing completed check run is not reused for a new review start
- **WHEN** one or more completed `AI Review` Check Runs already exist for the job head SHA and a new supported review job starts
- **THEN** the Check Run reporter creates a new in-progress Check Run for that review attempt
- **AND** it does not add durable storage solely to remember the new Check Run ID unless implementation proves GitHub update semantics require it

#### Scenario: Existing in-progress check run cannot be matched safely
- **WHEN** no matching in-progress `AI Review` Check Run can be deterministically identified for a completion or failure update
- **THEN** the Check Run reporter creates or updates output according to the existing safe fallback behavior
- **AND** it does not mutate a completed historical Check Run as if it were the current review attempt

### Requirement: GitHub App Checks permission
The service documentation SHALL state that M3 Check Run reporting requires GitHub App Checks read/write permission in addition to the existing metadata, contents, pull requests, and issues permissions.

#### Scenario: Permissions are documented
- **WHEN** a deployer configures the GitHub App for M3
- **THEN** project documentation identifies Checks read/write as required for Check Run reporting
- **AND** Issues write remains documented for PR conversation comment upsert

### Requirement: Reporter and Check Run verification
The implementation SHALL include automated tests and real verification steps for reporter fan-out, Check Run lifecycle behavior, and preserved PR comment upsert behavior.

#### Scenario: Automated verification covers reporter behavior
- **WHEN** M18 implementation is complete
- **THEN** tests cover reporter fan-out to multiple reporters
- **AND** tests cover comment reporter marker upsert preservation
- **AND** tests cover Check Run start creates a fresh in-progress run
- **AND** tests cover completion/failure updating only matching in-progress runs
- **AND** tests cover advisory findings not failing checks and infrastructure failure policy

#### Scenario: Standard commands pass
- **WHEN** M18 implementation is complete
- **THEN** `gofmt -w .` has been run
- **AND** `go test ./...` passes
- **AND** `go build ./cmd/server` passes
- **AND** `openspec validate m18-tighten-inline-and-checkrun-lifecycle --type change --strict` passes

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

### Requirement: Verified review result before reporting
The worker SHALL pass parsed structured LLM review results through finding verification before rendering comments or reporting completed review output.

#### Scenario: Verified result is sent to reporters
- **WHEN** a supported PR review job receives a valid structured LLM review result
- **THEN** the worker verifies the result's findings against the current job's available PR evidence
- **AND** the worker sends the verified review result, not the raw LLM result, to the configured reporters

#### Scenario: Existing comment upsert behavior is preserved
- **WHEN** the verified review result renders to non-empty Markdown
- **THEN** the comment reporter preserves the existing stable hidden marker upsert behavior
- **AND** repeated supported PR events update the marker comment instead of creating duplicate bot comments

#### Scenario: Existing advisory Check Run policy is preserved
- **WHEN** the verified review result contains kept or downgraded findings of any allowed severity
- **THEN** the Check Run reporter keeps the review output advisory and non-blocking
- **AND** the Check Run reporter does not set a failure conclusion based on AI findings

#### Scenario: Empty verified output is suppressed
- **WHEN** finding verification drops all findings and the remaining review result has no useful summary, missing tests, or limitations
- **THEN** the worker uses the existing output-suppressed path
- **AND** no empty or noisy PR comment is published

### Requirement: Finding verification observability
The worker SHALL log or record deterministic finding verification counts for completed review jobs without exposing sensitive or private content.

#### Scenario: Verification counts are observable
- **WHEN** finding verification completes for a review job
- **THEN** the worker logs or records the total finding count and kept, downgraded, and dropped counts
- **AND** the worker logs or records reason-category counts
- **AND** the metadata is safe and bounded

#### Scenario: Verification observability excludes sensitive content
- **WHEN** verification metrics or logs are emitted
- **THEN** they do not include raw prompts, raw model output, secrets, installation tokens, API keys, private keys, complete webhook payloads, or private repository code content

### Requirement: M5a verification checks
The implementation SHALL include automated and OpenSpec verification steps for the finding verifier slice.

#### Scenario: Automated verification covers M5a behavior
- **WHEN** M5a implementation is complete
- **THEN** tests cover verifier outcomes and reason categories
- **AND** tests cover worker integration before reporter fan-out
- **AND** tests cover preservation of comment marker upsert behavior
- **AND** tests cover preservation of advisory non-blocking Check Run behavior

#### Scenario: Standard commands pass
- **WHEN** M5a implementation is complete
- **THEN** `gofmt -w .` has been run
- **AND** `go test ./...` passes
- **AND** `go build ./cmd/server` passes
- **AND** `openspec validate m5a-finding-verifier --type change --strict` passes

### Requirement: Secret-safe operation
The service SHALL avoid logging or returning secrets, private keys, installation tokens, API keys, or complete private repository payloads.

#### Scenario: Error is reported safely
- **WHEN** config, webhook, GitHub API, LLM, or comment publishing fails
- **THEN** logs and HTTP responses include only safe metadata such as event type, action, delivery ID, owner, repo, pull number, and error category
- **AND** logs and HTTP responses do not include configured secrets, private keys, installation tokens, API keys, or complete webhook payloads

### Requirement: Optional Go analyzer stage
The worker SHALL support an optional Go analyzer stage that runs after bounded repository context is collected and before finding verification when the reviewed repository is identified as a Go project and a bounded safe local workspace is available.

#### Scenario: Go analyzer runs before verification
- **WHEN** a supported PR review job has bounded repository context indicating a Go project
- **AND** the worker has a safe local workspace for the PR head under an implementation-controlled root
- **THEN** the worker runs the optional Go analyzer stage before finding verification
- **AND** the verifier receives any produced static-check evidence with the current review job evidence

#### Scenario: Non-Go repository skips analyzer
- **WHEN** a supported PR review job does not have repository context indicating a Go project
- **THEN** the worker skips Go analyzer command execution
- **AND** the review job continues through LLM review, finding verification, and configured reporters without analyzer evidence

#### Scenario: Unsafe or unavailable workspace skips analyzer
- **WHEN** a supported PR review job is for a Go project
- **AND** no bounded safe local workspace strategy is available for the PR head
- **THEN** the worker skips Go analyzer command execution
- **AND** the review context records a safe analyzer limitation or omitted-context note
- **AND** the review job continues through LLM review, finding verification, and configured reporters

### Requirement: Bounded Go standard tool execution
The optional Go analyzer stage SHALL execute only fixed Go standard tool commands, bounded by timeout, output size, safe path constraints, and minimal environment rules.

#### Scenario: Analyzer command plan is restricted
- **WHEN** the Go analyzer plans commands for execution
- **THEN** the planned command list contains only fixed argv forms for `go test ./...` and `go vet ./...`
- **AND** the commands are not built through shell interpolation
- **AND** the working directory is the safe workspace root or a validated Go module directory under that root

#### Scenario: Analyzer execution is bounded
- **WHEN** a planned Go analyzer command is executed
- **THEN** execution is bounded by a configured or implementation-defined timeout
- **AND** captured output is bounded by a configured or implementation-defined byte limit
- **AND** the command environment does not include GitHub installation tokens, LLM API keys, webhook secrets, private keys, or other service secrets

#### Scenario: Analyzer timeout is non-blocking
- **WHEN** a Go analyzer command exceeds its timeout
- **THEN** the analyzer records a timeout exit category and a safe limitation note
- **AND** the review job continues through finding verification and configured reporters

#### Scenario: Analyzer failure is non-blocking
- **WHEN** `go test ./...` or `go vet ./...` exits unsuccessfully
- **THEN** the analyzer records a failure exit category and any safely parsed static-check evidence
- **AND** the worker does not treat the analyzer failure as infrastructure failure
- **AND** the review job continues through finding verification and configured reporters

### Requirement: Analyzer output safety
The worker SHALL NOT log or publish raw analyzer stdout or stderr from private repositories unbounded and SHALL pass only bounded sanitized analyzer summaries to downstream verification or reporting.

#### Scenario: Analyzer output is sanitized before use
- **WHEN** analyzer stdout or stderr is captured
- **THEN** the analyzer bounds captured content before parsing
- **AND** it sanitizes parsed messages before creating evidence
- **AND** it records truncation or omission as safe limitation metadata when applicable

#### Scenario: Raw analyzer output is not reported
- **WHEN** comment or Check Run reporters publish review output
- **THEN** they do not include unbounded raw analyzer stdout or stderr
- **AND** Check Run conclusions remain advisory and are not set to failure based on analyzer findings, analyzer command failure, or AI findings

#### Scenario: Safe analyzer metadata may be logged
- **WHEN** analyzer execution completes, is skipped, fails, or times out
- **THEN** logs may include safe aggregate metadata such as tool name, exit category, duration bucket, output truncation status, and parsed evidence count
- **AND** logs do not include secrets, installation tokens, API keys, private keys, raw prompts, raw model responses, complete webhook payloads, private repository code, or unbounded analyzer output

### Requirement: Safe Go workspace provider
The worker SHALL use an explicitly configured safe Go workspace provider before running optional Go analyzer commands, and SHALL preserve the existing safe analyzer skip behavior when no provider is configured or when provider safety checks fail.

#### Scenario: Provider is explicitly gated
- **WHEN** a supported PR review job reaches the optional Go analyzer stage
- **AND** no safe Go workspace provider is configured or enabled
- **THEN** the worker skips Go analyzer command execution
- **AND** the review context records a safe analyzer limitation or omitted-context note
- **AND** the review job continues through LLM review, finding verification, and configured reporters

#### Scenario: Provider returns validated workspace
- **WHEN** a supported PR review job is for a Go project
- **AND** the configured provider creates a workspace that satisfies all path, checkout, credential, and bounded-operation safety checks
- **THEN** the worker may pass the returned `SafeGoWorkspace` to the existing Go analyzer
- **AND** the analyzer may run only the existing fixed `go test ./...` and `go vet ./...` command plans

#### Scenario: Provider failure skips analyzer
- **WHEN** workspace provider setup fails, times out, is unavailable, or rejects the workspace as unsafe
- **THEN** the worker skips Go analyzer command execution
- **AND** the failure is represented by a deterministic safe skip or limitation category
- **AND** the review job continues through LLM review, finding verification, PR comment reporting, and advisory Check Run reporting

### Requirement: Workspace root and path safety
The safe Go workspace provider SHALL create per-job workspaces only under an implementation-controlled temp or cache root and SHALL validate all workspace paths before returning them to the analyzer.

#### Scenario: Workspace root is implementation controlled
- **WHEN** the provider creates a workspace for a review job
- **THEN** the workspace root is under an implementation-controlled temp or cache directory
- **AND** webhook payload fields, repository content, branch names, and user-supplied paths do not determine an absolute workspace root

#### Scenario: Workspace paths are contained
- **WHEN** the provider validates the workspace root, repository checkout path, module working directory, or cleanup target
- **THEN** each path resolves within the implementation-controlled workspace root
- **AND** paths that escape the root through absolute paths, symlinks, traversal, or malformed values are rejected

#### Scenario: Unsafe path skips analyzer
- **WHEN** any workspace path cannot be validated as contained under the implementation-controlled root
- **THEN** the provider does not return a `SafeGoWorkspace`
- **AND** analyzer execution is skipped with a safe path-validation category

### Requirement: PR head pinned checkout
The safe Go workspace provider SHALL checkout or fetch the exact PR head revision for the current review job and validate that the resulting workspace `HEAD` equals the job head SHA before analyzer execution is allowed.

#### Scenario: Exact head SHA is validated
- **WHEN** the provider completes checkout or fetch for a review job
- **THEN** it resolves the workspace `HEAD`
- **AND** it returns a `SafeGoWorkspace` only when the resolved `HEAD` exactly matches `job.HeadSHA`

#### Scenario: Head mismatch skips analyzer
- **WHEN** the resolved workspace `HEAD` is missing or does not exactly match `job.HeadSHA`
- **THEN** the provider rejects the workspace
- **AND** analyzer execution is skipped with a safe checkout-mismatch category

#### Scenario: Git commands are fixed argv
- **WHEN** the provider runs git clone, fetch, checkout, or revision validation commands
- **THEN** each command uses fixed argv forms without shell interpolation
- **AND** untrusted repository names, refs, URLs, or paths are not concatenated into shell command strings

### Requirement: Bounded workspace checkout
The safe Go workspace provider SHALL bound clone, fetch, checkout, and revision validation behavior by timeout, deterministic limits, and shallow or filtered fetch strategy where feasible.

#### Scenario: Checkout operations are bounded
- **WHEN** the provider performs clone, fetch, checkout, or revision validation
- **THEN** each operation is bounded by configured or implementation-defined timeouts
- **AND** fetched history and object scope are shallow or filtered where feasible for exact PR head validation
- **AND** command output captured for diagnostics is bounded and sanitized before any logging or limitation recording

#### Scenario: Bounded checkout failure skips analyzer
- **WHEN** clone, fetch, checkout, or revision validation exceeds a timeout, output limit, or deterministic fetch limit
- **THEN** the provider rejects the workspace
- **AND** analyzer execution is skipped with a safe bounded-checkout category
- **AND** LLM review and configured reporters continue

### Requirement: Workspace credential isolation
The safe Go workspace provider SHALL prevent credentials used for repository checkout from being persisted, logged, or propagated to analyzer command environments.

#### Scenario: Checkout token is not persisted
- **WHEN** the provider needs GitHub installation credentials for clone or fetch
- **THEN** it uses credentials with limited lifetime and repository scope
- **AND** it does not write tokens to git remotes, persisted git config, logs, analyzer evidence, comments, Check Runs, or durable storage

#### Scenario: Analyzer environment excludes checkout secrets
- **WHEN** the Go analyzer executes commands in a safe workspace
- **THEN** the analyzer command environment does not include GitHub installation tokens, checkout credentials, LLM API keys, webhook secrets, private keys, or other service secrets
- **AND** the environment is built independently from any credential-bearing checkout command environment

### Requirement: Workspace cleanup
The worker SHALL attempt to remove each per-job workspace after analyzer execution or after a provider-created workspace is no longer needed, and SHALL record cleanup limitations safely without blocking review output.

#### Scenario: Workspace is cleaned after analyzer
- **WHEN** analyzer execution completes, times out, fails, or is skipped after a workspace was created
- **THEN** the worker or provider attempts to remove the per-job workspace
- **AND** cleanup targets are validated as contained under the implementation-controlled workspace root before removal

#### Scenario: Cleanup limitation is non-blocking
- **WHEN** workspace cleanup fails or can only be partially completed
- **THEN** the worker records a deterministic safe cleanup limitation category
- **AND** the review job continues through finding verification, PR comment reporting, and advisory Check Run reporting
- **AND** logs and reports do not include private repository code, secrets, tokens, or unbounded cleanup output

### Requirement: Workspace provider observability safety
The service SHALL expose only safe aggregate metadata for workspace provider outcomes.

#### Scenario: Provider logs are aggregate only
- **WHEN** workspace provider setup, checkout, validation, analyzer handoff, or cleanup completes, fails, or is skipped
- **THEN** logs or metrics may include aggregate categories such as provider_disabled, checkout_timeout, checkout_failed, head_mismatch, path_invalid, credential_unavailable, cleanup_failed, or workspace_ready
- **AND** logs or metrics do not include raw prompts, raw model output, tokens, secrets, private keys, complete webhook payloads, unbounded analyzer output, persisted checkout credentials, or private repository code

#### Scenario: Advisory Check Run behavior is preserved
- **WHEN** workspace provider setup, checkout, analyzer execution, or cleanup fails for a review job
- **THEN** the Check Run reporter does not set a failure conclusion based on that optional analyzer or workspace-provider outcome
- **AND** review output remains advisory and non-blocking

### Requirement: Production workspace provider wiring
The production server SHALL wire the safe Go workspace provider into the review service only when workspace checkout is explicitly enabled by validated runtime config.

#### Scenario: Workspace provider is disabled by default
- **WHEN** the service starts without explicit workspace checkout enablement config
- **THEN** production review service construction succeeds without a safe Go workspace provider
- **AND** supported PR review jobs continue through the existing optional Go analyzer skipped path
- **AND** no git clone, fetch, checkout, or credential acquisition is attempted for analyzer workspace setup

#### Scenario: Workspace provider is wired when explicitly enabled
- **WHEN** the service starts with explicit workspace checkout enablement config and valid workspace root settings
- **THEN** production review service construction provides the configured safe Go workspace provider to the optional Go analyzer path
- **AND** the provider remains bounded by existing path, checkout, head validation, cleanup, timeout, and output safety requirements

#### Scenario: Invalid workspace provider config fails startup safely
- **WHEN** workspace checkout is explicitly enabled but required workspace root or safety config is invalid
- **THEN** runtime config validation or service construction fails with a useful non-secret error
- **AND** the error does not include tokens, private keys, webhook secrets, API keys, raw payloads, or private repository content

### Requirement: Checkout-only installation credential provider
The production safe Go workspace provider SHALL acquire GitHub App installation credentials only through a checkout credential provider scoped to the current review job installation, owner, repo, and head SHA.

#### Scenario: Checkout credential is acquired for current job
- **WHEN** a supported PR review job requires checkout for the safe Go workspace provider
- **THEN** the checkout credential provider requests a short-lived GitHub App installation token for the job installation ID
- **AND** the credential is scoped to checkout for the job owner and repo
- **AND** the credential is not reused for unrelated installations, repositories, pull requests, or review jobs

#### Scenario: Credential acquisition failure skips analyzer
- **WHEN** checkout credential acquisition fails because GitHub App auth, token exchange, repository scope, rate limit, or provider availability fails
- **THEN** the safe Go workspace provider rejects workspace setup
- **AND** Go analyzer command execution is skipped with a deterministic safe credential failure category
- **AND** LLM review, finding verification, PR comment reporting, and advisory Check Run reporting continue without static-check evidence from that workspace

#### Scenario: Credential scope mismatch skips analyzer
- **WHEN** a checkout credential cannot be verified as scoped to the current job installation, owner, and repo
- **THEN** the safe Go workspace provider rejects workspace setup
- **AND** Go analyzer command execution is skipped with a deterministic safe credential scope category
- **AND** no checkout command is run with that credential

### Requirement: Safe checkout credential injection
The safe Go workspace provider SHALL inject checkout credentials only through an ephemeral mechanism that keeps tokens out of persisted git state, command plans, logs, analyzer environments, verifier evidence, reporter outputs, and durable storage.

#### Scenario: Git command plans are token-free
- **WHEN** the provider plans git clone, fetch, checkout, remote, or revision validation commands
- **THEN** planned argv, working directories, safe log fields, and command descriptions do not contain installation tokens or checkout credential values
- **AND** remote URLs recorded in plans or persisted git config do not contain installation tokens or checkout credential values

#### Scenario: Checkout environment is not reused for analyzer commands
- **WHEN** checkout requires credential-bearing environment variables, askpass plumbing, credential helper plumbing, or equivalent ephemeral injection
- **THEN** that credential-bearing environment is used only for checkout commands that require it
- **AND** Go analyzer commands receive a separately constructed minimal environment without GitHub installation tokens, checkout credentials, LLM API keys, webhook secrets, private keys, or other service secrets

#### Scenario: Credential injection failure skips analyzer
- **WHEN** ephemeral checkout credential injection cannot be prepared or fails before safe workspace validation
- **THEN** the safe Go workspace provider rejects workspace setup
- **AND** Go analyzer command execution is skipped with a deterministic safe credential injection category
- **AND** the failure metadata does not include installation tokens, checkout credential values, raw git output containing credentials, or private repository code

#### Scenario: Checkout credentials are not reported
- **WHEN** checkout, analyzer execution, verification, comment rendering, Check Run reporting, or safe logging completes, fails, or is skipped
- **THEN** emitted comments, Check Runs, verifier evidence, reporter payloads, logs, metrics, and durable records do not include installation tokens, checkout credential values, tokenized remotes, credential helper payloads, or credential-bearing environment values

### Requirement: Workspace checkout rollout safety
Real repository checkout for optional Go analyzer evidence SHALL remain opt-in, advisory, and safe to deploy disabled.

#### Scenario: Disabled deployment preserves review loop
- **WHEN** production is deployed with workspace checkout disabled
- **THEN** supported PR review jobs still fetch PR metadata and patches, request LLM review, verify findings with available non-workspace evidence, and report advisory output
- **AND** workspace checkout absence is represented only as a safe analyzer limitation or skipped category

#### Scenario: Workspace and credential failures are non-blocking
- **WHEN** workspace provider setup, credential acquisition, credential injection, checkout, head validation, analyzer execution, or cleanup fails
- **THEN** the review job does not fail solely because of that optional workspace or analyzer outcome
- **AND** the Check Run reporter does not set a failure conclusion based on that optional workspace or analyzer outcome
- **AND** no concrete static-check finding is fabricated from the failed optional outcome

#### Scenario: Operations notes document opt-in behavior
- **WHEN** M6c implementation is complete
- **THEN** operator-facing configuration or deployment documentation identifies workspace checkout as disabled by default
- **AND** the documentation states that enabling checkout requires a controlled workspace root, GitHub App installation access, bounded git operations, cleanup monitoring, and secret-free logs

### Requirement: Production startup documentation alignment
The GitHub App review loop SHALL have production-facing configuration examples and documentation aligned with the runtime config required to start the service safely.

#### Scenario: Documented required config matches startup validation
- **WHEN** the service requires an environment variable to start the production review loop
- **THEN** the production docs or `.env.example` identify that setting by name
- **AND** startup validation reports missing required settings without printing configured secret values

#### Scenario: Dummy config path does not perform downstream work
- **WHEN** a local smoke path starts the service with dummy non-secret config for health checking
- **THEN** the service does not exchange installation tokens, fetch pull request files, call an LLM, clone a repository, publish PR comments, or create Check Runs until a valid signed webhook and worker path are exercised

### Requirement: Production reporter safety
The GitHub App review loop SHALL preserve advisory, non-blocking reporter behavior in production hardening paths.

#### Scenario: Comment upsert remains stable in E2E verification
- **WHEN** repeated supported pull request events are processed for the same PR
- **THEN** the comment reporter updates the existing marker-identified AI review comment when present
- **AND** it does not create duplicate bot review comments for normal repeated review events

#### Scenario: Check Run findings remain advisory
- **WHEN** a completed structured review result contains AI findings of any allowed severity
- **THEN** the Check Run reporter does not derive a blocking failure conclusion from those findings
- **AND** production docs and E2E verification describe the Check Run as advisory unless infrastructure or job execution fails

#### Scenario: Reporter failure output remains safe
- **WHEN** comment or Check Run reporting fails during review processing
- **THEN** the worker records or logs safe failure metadata identifying the reporter and category
- **AND** no PR-facing output or log under service control includes secrets, installation tokens, checkout credentials, private keys, API keys, complete webhook payloads, raw prompts, raw model responses, or unbounded private code

### Requirement: Production workspace checkout safety
The GitHub App review loop SHALL keep real workspace checkout optional, disabled by default, and bounded when explicitly enabled.

#### Scenario: Checkout disabled uses existing skip path
- **WHEN** production config does not explicitly enable workspace checkout
- **THEN** the worker does not clone or fetch repository contents
- **AND** analyzer-dependent evidence is represented as a safe skipped limitation when relevant

#### Scenario: Checkout enablement requires hardening config
- **WHEN** production config explicitly enables workspace checkout
- **THEN** startup validation requires the configured workspace root and timeout/output bounds needed by the safe workspace provider
- **AND** invalid workspace configuration fails startup without printing secrets or private-code paths beyond safe setting names

#### Scenario: Checkout rollback returns to no-checkout behavior
- **WHEN** an operator disables workspace checkout after it was previously enabled
- **THEN** subsequent review jobs do not perform real checkout
- **AND** the core webhook, LLM review, marker comment upsert, and advisory Check Run paths continue according to their own enabled configuration

### Requirement: Issue comment review command filtering
The webhook endpoint SHALL create review jobs for GitHub `issue_comment` events only when the signed delivery action is `created`, the comment is attached to a pull request, and the comment body is exactly `/ai-review` or starts with `/ai-review` followed by whitespace.

#### Scenario: Exact review command is accepted
- **WHEN** a signed `issue_comment` webhook has action `created`, is attached to a pull request, and has comment body `/ai-review`
- **THEN** the service accepts the event for review job creation

#### Scenario: Review command with trailing arguments is accepted
- **WHEN** a signed `issue_comment` webhook has action `created`, is attached to a pull request, and has a comment body that starts with `/ai-review` followed by whitespace
- **THEN** the service accepts the event for review job creation

#### Scenario: Non-command comment is ignored
- **WHEN** a signed `issue_comment` webhook has action `created` but the comment body is not exactly `/ai-review` and does not start with `/ai-review` followed by whitespace
- **THEN** the service returns `204 No Content`
- **AND** no review job is created

#### Scenario: Plain issue comment is ignored
- **WHEN** a signed `issue_comment` webhook has action `created` and a valid `/ai-review` command on a plain issue that is not a pull request
- **THEN** the service returns `204 No Content`
- **AND** no review job is created

#### Scenario: Unsupported issue comment action is ignored
- **WHEN** a signed `issue_comment` webhook has an action other than `created`
- **THEN** the service returns `204 No Content`
- **AND** no review job is created

### Requirement: Manual review command job creation
For each accepted `/ai-review` issue comment command, the service SHALL create a typed review job containing the installation ID, owner, repo, pull number, head SHA, action, and GitHub delivery ID.

#### Scenario: Accepted command produces expected job fields
- **WHEN** a signed accepted `/ai-review` command contains the required installation, repository, issue, pull request marker, and delivery fields
- **AND** the pull request metadata resolver returns the current pull request head SHA
- **THEN** the created review job contains the expected installation ID
- **AND** the job contains the expected owner, repo, pull number, head SHA, action, and delivery ID

#### Scenario: Pull request head SHA is resolved before enqueue
- **WHEN** a signed accepted `/ai-review` command is handled
- **THEN** the service obtains the pull request head SHA through GitHub App installation authentication or an equivalent safe resolver boundary before enqueueing the review job
- **AND** no review job is enqueued without a head SHA

#### Scenario: Required command job field is missing
- **WHEN** a signed accepted `/ai-review` command payload is missing a field required for review job creation
- **THEN** the service rejects the payload with a client error
- **AND** no review job is created

#### Scenario: Pull request metadata resolution fails
- **WHEN** a signed accepted `/ai-review` command cannot resolve pull request metadata or head SHA
- **THEN** the service returns an appropriate failure response according to the webhook handler error pattern
- **AND** no review job is created
- **AND** logs do not include raw payloads, raw comment bodies, secrets, installation tokens, private keys, API keys, raw prompts, or raw model responses

### Requirement: Manual review command asynchronous handling
The webhook handler SHALL return after accepting or ignoring an issue comment review command without fetching PR changed files, calling an LLM, publishing comments, or creating inline review comments inside the handler.

#### Scenario: Accepted command is submitted to worker
- **WHEN** a signed accepted `/ai-review` issue comment command has resolved the required review job fields
- **THEN** the service hands the review job to an in-memory worker or job sink
- **AND** the HTTP response is `202 Accepted` once the job is accepted

#### Scenario: Downstream review work is not executed for command in handler
- **WHEN** a signed accepted `/ai-review` issue comment command is handled
- **THEN** the handler does not fetch changed files, call an LLM, publish a PR comment, auto-fix code, auto-merge, block merging, or create inline review comments

#### Scenario: Existing pull request webhook behavior is preserved
- **WHEN** a signed supported `pull_request` webhook with action `opened`, `synchronize`, or `reopened` is received
- **THEN** the service preserves the existing review job creation and asynchronous worker behavior
- **AND** advisory Check Run status reporting remains unchanged

### Requirement: Batched pull request review inline comments
When inline review output is enabled and a completed review has newly eligible mapped inline findings, the service SHALL publish those new inline findings as one submitted GitHub Pull Request Review using the Create Review API with `event=COMMENT`, the review job head SHA as `commit_id`, a concise non-empty review body, and multiple inline `comments[]` entries.

#### Scenario: Multiple new inline findings are submitted in one review
- **WHEN** a completed review has two or more newly eligible mapped inline findings
- **AND** inline review output is enabled
- **THEN** the inline reporter creates one Pull Request Review for the job owner, repo, pull number, and head SHA
- **AND** the review request uses `event=COMMENT`
- **AND** the review body is non-empty and concise
- **AND** the review request contains one inline `comments[]` entry for each newly eligible mapped finding up to the configured maximum

#### Scenario: Single new inline finding uses the same review path
- **WHEN** a completed review has exactly one newly eligible mapped inline finding
- **AND** inline review output is enabled
- **THEN** the inline reporter creates one submitted Pull Request Review containing that one inline comment
- **AND** it does not prefer the legacy individual create-comment endpoint for the normal create path

#### Scenario: No new inline comments skips review creation
- **WHEN** a completed review has no newly eligible mapped inline findings that need creation
- **THEN** the inline reporter does not create an empty Pull Request Review
- **AND** existing comment updates and stale handling may still run according to their own requirements

### Requirement: Inline review eligibility gates
The service SHALL preserve the existing quality gates before a finding can become an inline Pull Request Review comment: inline output is disabled by default, no more than 10 inline comments are created per run, only `blocker` and `warning` severities are eligible, evidence, failure scenario, and suggestion are required, confidence MUST be at least `0.70` when present, and a RIGHT-side diff line mapping is required.

#### Scenario: Eligible finding becomes a review comment entry
- **WHEN** a verified finding has severity `blocker` or `warning`
- **AND** it includes evidence, failure scenario, and suggestion
- **AND** its confidence is absent or at least `0.70`
- **AND** it maps to a RIGHT-side diff line for the current pull request head
- **THEN** the finding may be included as an inline Pull Request Review `comments[]` entry

#### Scenario: Ineligible finding is not included inline
- **WHEN** a finding lacks required evidence, failure scenario, suggestion, supported severity, confidence threshold, or RIGHT-side diff line mapping
- **THEN** the finding is not included in the inline Pull Request Review comments
- **AND** it may still appear in the existing summary issue comment or advisory Check Run when those outputs allow it

#### Scenario: Inline output disabled by default
- **WHEN** inline review output is not explicitly enabled by configuration
- **THEN** the service does not create, update, or stale-mark inline Pull Request Review comments
- **AND** the summary issue comment and advisory Check Run behavior remain available according to their existing configuration

### Requirement: Inline comment marker and fingerprint idempotency
The service SHALL preserve marker and fingerprint based idempotency for bot inline review comments where GitHub comment bodies permit markers, and SHALL split each run into existing comments to update, new comments to batch-create, and obsolete bot comments to handle through the stale lifecycle.

#### Scenario: Existing fingerprinted inline comment is updated individually
- **WHEN** a current eligible inline finding has the same marker and fingerprint as an existing bot inline comment on the pull request
- **THEN** the service updates that existing inline comment when GitHub permits comment updates
- **AND** the service does not include that fingerprint in the new batched review creation request

#### Scenario: New fingerprint is included in the batch
- **WHEN** a current eligible inline finding has no matching existing bot inline comment fingerprint on the pull request
- **THEN** the service includes that finding in the next batched Pull Request Review creation request
- **AND** the inline comment body includes the stable marker and fingerprint when GitHub permits hidden markers in the comment body

#### Scenario: Unrelated comments are ignored
- **WHEN** existing pull request review comments do not contain this service's stable marker and fingerprint format
- **THEN** the service does not update, stale-mark, minimize, resolve, or otherwise alter those comments

### Requirement: Safe stale inline comment lifecycle
The service SHALL detect bot inline comment fingerprints that were previously produced by this service but are no longer present in the current run's eligible inline finding set, and SHALL handle them through a non-destructive stale lifecycle; deleting GitHub comments is out of scope.

#### Scenario: Obsolete bot inline comment is marked stale
- **WHEN** an existing inline comment contains this service's marker and fingerprint
- **AND** that fingerprint is absent from the current run's eligible inline finding set
- **THEN** the service marks the comment as stale through a safe supported action such as updating the body with a concise stale marker
- **AND** the stale marker does not include secrets, raw prompts, raw model responses, complete webhook payloads, or unbounded private code

#### Scenario: Destructive deletion is not performed
- **WHEN** a bot inline comment is obsolete
- **THEN** the service does not delete the GitHub comment
- **AND** any future minimize or resolve behavior must be explicit, safe, marker-scoped, and covered by tests before use

#### Scenario: Stale handling is separate from batch creation
- **WHEN** a review run has obsolete bot inline comments and new inline comments
- **THEN** stale handling occurs separately from the Create Review API batch request
- **AND** a failure to stale-mark one obsolete comment does not cause duplicate creation of new inline comments

### Requirement: Batched inline review fallback policy
The legacy individual Pull Request Review Comment create endpoint SHALL remain available only for existing comment updates and as a fallback when batched Pull Request Review creation fails or when no new comments are present; the preferred creation path SHALL be the official Create Pull Request Review API.

#### Scenario: Batch creation failure is handled safely
- **WHEN** the Create Pull Request Review API fails for a batch of new inline comments
- **THEN** the reporter records or logs safe failure metadata identifying the reporter and failure category
- **AND** it does not retry by creating duplicate comments inside the webhook handler
- **AND** it may use the legacy individual create-comment endpoint only according to the configured fallback policy

#### Scenario: Fallback does not replace preferred path
- **WHEN** batch review creation is available and new eligible inline findings are present
- **THEN** the service prefers one submitted Pull Request Review over creating independent inline comments one-by-one

### Requirement: Batched inline review output safety
Batched inline review reporting SHALL preserve existing non-blocking behavior and secret-safety constraints for all inline, summary comment, Check Run, and logging outputs.

#### Scenario: Inline findings remain advisory
- **WHEN** the service submits a Pull Request Review for inline findings
- **THEN** it uses `event=COMMENT`
- **AND** it does not request changes, approve the pull request, fail a Check Run, auto-fix code, auto-merge, or block merging based on AI findings

#### Scenario: Existing summary and Check Run outputs continue
- **WHEN** batched inline review reporting runs for a completed review
- **THEN** the existing summary issue comment behavior continues according to its marker upsert requirements
- **AND** the advisory Check Run behavior continues according to its existing non-blocking requirements

#### Scenario: Logs and failure metadata are safe
- **WHEN** batched inline review creation, individual update, fallback, or stale handling succeeds or fails
- **THEN** logs and recorded metadata do not include secrets, installation tokens, private keys, API keys, raw prompts, raw model responses, complete webhook payloads, or private source snippets beyond intentional PR-facing comments

### Requirement: Batched inline review verification
The implementation SHALL include automated tests and real E2E verification for the batched inline review behavior, preserved existing outputs, and safe stale lifecycle.

#### Scenario: Automated verification covers batched review behavior
- **WHEN** M12 implementation is complete
- **THEN** tests cover the GitHub client boundary for creating a submitted Pull Request Review with multiple inline comments
- **AND** tests cover inline eligibility gates and maximum comment limits
- **AND** tests cover the split between existing comment updates, new batched comment creation, and stale obsolete comments
- **AND** tests cover fallback behavior without duplicate creation
- **AND** tests cover preservation of summary issue comment and advisory Check Run outputs

#### Scenario: Standard commands pass
- **WHEN** M12 implementation is complete
- **THEN** `gofmt -w .` has been run
- **AND** `go test ./...` passes
- **AND** `go build ./cmd/server` passes
- **AND** `openspec validate m12-batched-pr-review-inline-comments --type change --strict` passes

#### Scenario: Real PR verification succeeds
- **WHEN** the service is deployed or restarted with inline output explicitly enabled for a non-sensitive test repository
- **AND** a real pull request produces at least two eligible mapped findings
- **THEN** the GitHub PR UI or API shows one newly submitted Pull Request Review from the bot containing multiple line-level comments
- **AND** the review body is non-empty
- **AND** the GitHub API returns the expected `path`, `line`, and `side=RIGHT` values for the created comments
- **AND** the summary issue comment and advisory Check Run remain present and non-blocking
- **AND** a later run where one previous inline fingerprint is absent makes the stale handling observable without deleting the old comment

### Requirement: Pull request close cleanup event handling
The webhook endpoint SHALL accept signed GitHub `pull_request` events whose action is `closed` for cleanup-only handling, and SHALL distinguish merged pull requests from closed-unmerged pull requests when the payload provides merged state.

#### Scenario: Closed unmerged pull request is accepted for cleanup
- **WHEN** a signed `pull_request` webhook has action `closed`
- **AND** the pull request payload indicates `merged` is false
- **THEN** the service accepts the event for close cleanup
- **AND** the cleanup event contains the installation ID, owner, repo, pull number, head SHA, action, delivery ID, and closed-unmerged status

#### Scenario: Merged pull request is accepted for cleanup
- **WHEN** a signed `pull_request` webhook has action `closed`
- **AND** the pull request payload indicates `merged` is true
- **THEN** the service accepts the event for close cleanup
- **AND** the cleanup event contains the installation ID, owner, repo, pull number, head SHA, action, delivery ID, and merged status

#### Scenario: Close cleanup does not create normal review job
- **WHEN** a signed `pull_request.closed` webhook is accepted for cleanup
- **THEN** the service does not create a normal LLM review job
- **AND** the close cleanup path does not fetch PR changed files for review, build LLM prompt context, run optional analyzers, call the LLM, create new inline Pull Request Reviews, request changes, auto-fix code, auto-merge, or block merging

#### Scenario: Close cleanup remains fast
- **WHEN** a signed `pull_request.closed` webhook is received
- **THEN** the webhook handler returns after accepting or ignoring cleanup work according to the service's asynchronous handling pattern
- **AND** the webhook handler does not perform LLM review work inline

#### Scenario: Close cleanup missing required field is rejected
- **WHEN** a signed `pull_request.closed` webhook is missing a field required to identify the installation, repository, pull request, head SHA, merged status, or delivery
- **THEN** the service rejects the payload with a client error
- **AND** no cleanup job or normal review job is created

### Requirement: Bot-owned output inactive lifecycle
For closed or merged pull requests, the service SHALL make bot-owned review outputs clearly inactive or archived through marker-scoped, non-destructive updates when supported, and SHALL NOT delete human-visible review history.

#### Scenario: Summary marker comment is updated inactive
- **WHEN** close cleanup runs for a pull request that has an existing summary issue comment containing the service's stable hidden marker
- **THEN** the service updates that marker-owned summary comment with concise inactive status text
- **AND** the status distinguishes closed-unmerged from merged when that state is available
- **AND** the updated comment remains advisory and non-blocking

#### Scenario: Missing summary marker does not create noise
- **WHEN** close cleanup runs for a pull request that has no existing summary issue comment containing the service's stable hidden marker
- **THEN** the service does not create duplicate bot summary comments
- **AND** any newly created inactive summary marker comment is concise, non-empty, and allowed only by the implementation's explicit cleanup output policy

#### Scenario: Bot inline comments may be marked inactive
- **WHEN** close cleanup runs and existing inline Pull Request Review comments contain this service's stable inline marker and fingerprint
- **THEN** the service may mark those bot inline comments inactive or stale through a safe supported marker-scoped action
- **AND** the inactive or stale text identifies the close or merge lifecycle without including secrets, raw prompts, raw model responses, complete webhook payloads, or unbounded private code

#### Scenario: Inline minimize or resolve is optional and marker-scoped
- **WHEN** the implementation supports a GitHub API action to minimize or resolve bot inline review comments or threads during close cleanup
- **THEN** the action applies only to comments or threads that contain this service's marker
- **AND** the action is covered by automated tests before use

#### Scenario: Destructive cleanup is not performed
- **WHEN** close cleanup runs for any bot-owned or human-owned output
- **THEN** the service does not delete issue comments, Pull Request Review comments, Pull Request Reviews, Check Runs, human comments, or comments from other bots
- **AND** the service does not alter comments that lack this service's stable marker

#### Scenario: Check Run cleanup does not create blocking output
- **WHEN** close cleanup runs for a pull request with existing `AI Review` Check Run output
- **THEN** the service does not create a new Check Run solely for cleanup
- **AND** any optional update to an existing `AI Review` Check Run remains advisory and non-blocking
- **AND** cleanup failure does not turn AI findings into a blocking conclusion

### Requirement: Closed pull request manual review command policy
The service SHALL NOT start a normal review when a signed `/ai-review` issue comment command targets a pull request that is closed or merged.

#### Scenario: Closed pull request command does not enqueue review
- **WHEN** a signed accepted `/ai-review` issue comment command targets a pull request
- **AND** the pull request metadata resolver reports that the pull request is closed and unmerged
- **THEN** the service does not enqueue a normal LLM review job
- **AND** it may enqueue cleanup work or return a safe ignored response according to the cleanup policy

#### Scenario: Merged pull request command does not enqueue review
- **WHEN** a signed accepted `/ai-review` issue comment command targets a pull request
- **AND** the pull request metadata resolver reports that the pull request is merged
- **THEN** the service does not enqueue a normal LLM review job
- **AND** it may enqueue cleanup work or return a safe ignored response according to the cleanup policy

#### Scenario: Open pull request command behavior is preserved
- **WHEN** a signed accepted `/ai-review` issue comment command targets an open pull request
- **THEN** the service preserves existing manual review command job creation
- **AND** the downstream worker and reporter behavior remains advisory and non-blocking

#### Scenario: Closed pull request command avoids downstream review work
- **WHEN** a signed `/ai-review` command targets a closed or merged pull request
- **THEN** the service does not fetch changed files for review, build LLM prompt context, run optional analyzers, call the LLM, create new inline Pull Request Reviews, auto-fix code, auto-merge, request changes, or block merging

### Requirement: Close cleanup safety and observability
Close and merge cleanup SHALL preserve the service's existing secret-safety constraints and SHALL emit only bounded safe metadata for cleanup outcomes.

#### Scenario: Cleanup logs are safe
- **WHEN** close cleanup is accepted, skipped, succeeds, or fails
- **THEN** logs or metrics may include safe metadata such as event type, action, delivery ID, owner, repo, pull number, head SHA suffix or safe identifier, merged state, and cleanup category
- **AND** logs or metrics do not include secrets, installation tokens, checkout credentials, private keys, API keys, raw prompts, raw model responses, complete webhook payloads, raw comment bodies, or unbounded private repository code

#### Scenario: Cleanup failure is non-blocking
- **WHEN** updating marker-scoped summary, inline, or Check Run output fails during close cleanup
- **THEN** the service records or logs safe failure metadata
- **AND** the service does not retry by creating duplicate comments inside the webhook handler
- **AND** the failure does not create a normal review job or blocking output

#### Scenario: Existing open pull request review behavior is unchanged
- **WHEN** a signed supported `pull_request` webhook with action `opened`, `synchronize`, or `reopened` is received
- **THEN** the service preserves existing review job creation and asynchronous worker behavior
- **AND** summary issue comments, advisory Check Runs, and enabled inline Pull Request Reviews keep their existing behavior

### Requirement: Close cleanup verification
The implementation SHALL include automated tests and real verification steps for close/merge cleanup behavior, closed-PR manual command behavior, preserved open-PR review behavior, and non-destructive output lifecycle.

#### Scenario: Automated verification covers close cleanup
- **WHEN** M13 implementation is complete
- **THEN** tests cover `pull_request.closed` parsing for closed-unmerged and merged payloads
- **AND** tests cover that close cleanup does not enqueue a normal LLM review job
- **AND** tests cover summary marker inactive rendering or upsert behavior
- **AND** tests cover marker-scoped inline inactive or stale behavior when implemented
- **AND** tests cover that unrelated comments and human comments are not altered

#### Scenario: Automated verification covers closed PR command policy
- **WHEN** M13 implementation is complete
- **THEN** tests cover `/ai-review` on closed-unmerged and merged pull requests
- **AND** tests prove those commands do not enqueue normal review jobs or call downstream review work
- **AND** tests cover that `/ai-review` on open pull requests still creates normal manual review jobs

#### Scenario: Standard commands pass
- **WHEN** M13 implementation is complete
- **THEN** `gofmt -w .` has been run
- **AND** `go test ./...` passes
- **AND** `go build ./cmd/server` passes
- **AND** `openspec validate m13-pr-close-review-cleanup --type change --strict` passes

#### Scenario: Real PR verification succeeds
- **WHEN** the service is deployed or restarted with M13 on a non-sensitive test repository
- **AND** a real pull request with existing bot summary output and, when enabled, bot inline output is closed without merge or merged
- **THEN** the webhook delivery for `pull_request.closed` is accepted or safely ignored according to the cleanup path
- **AND** no new LLM review output is produced after close or merge
- **AND** bot-owned marker output is inactive or archived according to the cleanup policy
- **AND** no bot-owned or human-visible review history is deleted
- **AND** logs, comments, and Check Run output do not expose secrets, raw prompts, raw model responses, complete webhook payloads, or unbounded private source content

### Requirement: Repository-level AI review config discovery
For each supported open pull request review job, the worker SHALL attempt to discover repository-level AI review configuration from `.github/ai-review.yml` or `.github/ai-review.yaml` at the pull request repository ref when safe and available.

#### Scenario: YAML config is discovered from PR ref
- **WHEN** a signed supported review job reaches worker processing
- **AND** `.github/ai-review.yml` exists at the current pull request head ref or head SHA
- **THEN** the worker reads that file as the repository AI review config candidate
- **AND** the worker uses the config only for the current review job

#### Scenario: YAML extension fallback is discovered
- **WHEN** `.github/ai-review.yml` is absent
- **AND** `.github/ai-review.yaml` exists at the current pull request head ref or head SHA
- **THEN** the worker reads `.github/ai-review.yaml` as the repository AI review config candidate
- **AND** the worker uses the config only for the current review job

#### Scenario: Primary config wins
- **WHEN** both `.github/ai-review.yml` and `.github/ai-review.yaml` exist
- **THEN** the worker uses `.github/ai-review.yml`
- **AND** the worker does not merge both files

#### Scenario: Missing config is non-blocking
- **WHEN** neither repository config path exists or GitHub reports the file is not found
- **THEN** the review continues with service defaults and global service configuration
- **AND** the missing file does not fail the review job

#### Scenario: Config fetch failure is safe
- **WHEN** repository config discovery fails because GitHub content fetching is unavailable, unauthorized, oversized, ambiguous, or otherwise unsafe to use
- **THEN** the review continues with service defaults and global service configuration
- **AND** the worker records only a safe bounded configuration limitation category
- **AND** logs, comments, Check Runs, and prompts do not include secrets, installation tokens, raw private config content, complete webhook payloads, or unbounded private repository code

### Requirement: Repository-level AI review config schema
The repository config parser SHALL support a conservative first-slice schema and reject unknown, invalid, or unsafe values without applying partial unsafe behavior.

#### Scenario: Supported config fields are parsed
- **WHEN** repository config contains valid YAML for supported fields
- **THEN** the parser recognizes `enabled`, `language`, `summary_comment.enabled`, `check_run.enabled`, `inline_comments.enabled`, `inline_comments.max_comments`, `inline_comments.severity_threshold`, `inline_comments.confidence_threshold`, `path_ignore`, and `go_analyzer.enabled`
- **AND** omitted fields remain unset so service defaults and global configuration can apply

#### Scenario: Invalid config falls back to defaults
- **WHEN** repository config content is malformed YAML, uses invalid types, uses unsupported enum values, sets out-of-range numeric thresholds, or otherwise fails validation
- **THEN** the worker treats the repository config as invalid for that review job
- **AND** the review continues with service defaults and global service configuration
- **AND** the invalid config is reported only as a safe bounded configuration limitation without exposing raw private config content

#### Scenario: Review can be disabled
- **WHEN** a valid repository config sets `enabled: false`
- **THEN** the worker suppresses normal review work for the job before LLM calls, optional analyzer execution, summary comment creation, Check Run creation, and inline review creation
- **AND** the suppression remains advisory and does not request changes, auto-fix, auto-merge, fail merge gates, or block merging

#### Scenario: Language is limited to supported values
- **WHEN** repository config sets `language`
- **THEN** the parser accepts only implementation-supported review languages
- **AND** unsupported language values make the repository config invalid for that job

#### Scenario: Inline threshold fields are bounded
- **WHEN** repository config sets `inline_comments.max_comments`, `inline_comments.severity_threshold`, or `inline_comments.confidence_threshold`
- **THEN** the parser accepts only values that can tighten existing inline eligibility and limit behavior
- **AND** values that would increase unsafe output volume or lower quality below service defaults are ignored by effective-config merge or rejected by validation according to the implementation policy

#### Scenario: Path ignore entries are bounded
- **WHEN** repository config sets `path_ignore`
- **THEN** the parser accepts only a bounded list of deterministic repository-relative path patterns supported by the implementation
- **AND** invalid, absolute, parent-traversing, or unsupported patterns make the repository config invalid for that job

### Requirement: Effective review config safety boundary
The service SHALL merge repository config with global service configuration into an effective review config where global service configuration remains the upper safety boundary.

#### Scenario: Repo config cannot enable globally disabled Check Runs
- **WHEN** global service configuration disables Check Run reporting
- **AND** repository config sets `check_run.enabled: true`
- **THEN** the effective config keeps Check Run reporting disabled for the review job

#### Scenario: Repo config can disable globally enabled Check Runs
- **WHEN** global service configuration permits Check Run reporting
- **AND** repository config sets `check_run.enabled: false`
- **THEN** the effective config disables Check Run reporting for the review job
- **AND** summary and inline reporters remain governed by their own effective settings

#### Scenario: Repo config cannot enable globally disabled inline comments
- **WHEN** global service configuration disables inline Pull Request Review comments
- **AND** repository config sets `inline_comments.enabled: true`
- **THEN** the effective config keeps inline comment publishing disabled for the review job

#### Scenario: Repo config can tighten inline comment policy
- **WHEN** global service configuration permits inline Pull Request Review comments
- **AND** repository config sets inline comment limits or thresholds that are stricter than service defaults
- **THEN** the effective config applies the stricter maximum comment count, severity threshold, and confidence threshold for the review job
- **AND** required evidence fields and RIGHT-side diff line mapping remain mandatory

#### Scenario: Repo config cannot enable globally disabled Go analyzer behavior
- **WHEN** global service configuration or workspace provider configuration disables optional Go analyzer execution or safe checkout
- **AND** repository config sets `go_analyzer.enabled: true`
- **THEN** the effective config keeps optional Go analyzer execution disabled or safely skipped for the review job

#### Scenario: Repo config can disable globally available Go analyzer behavior
- **WHEN** global service configuration permits optional Go analyzer execution with a safe workspace provider
- **AND** repository config sets `go_analyzer.enabled: false`
- **THEN** the effective config skips optional Go analyzer execution for the review job
- **AND** the review continues through non-analyzer context, LLM review, verification, and enabled reporters

#### Scenario: Repo config cannot change blocking policy
- **WHEN** repository config sets any supported field
- **THEN** the effective config does not allow AI findings to request changes, auto-fix code, auto-merge pull requests, fail merge gates, or block merging

### Requirement: Effective config integration points
The worker SHALL apply the effective review config before review work that depends on language, outputs, inline eligibility, analyzer execution, or path filtering decisions.

#### Scenario: Language affects LLM prompt and rendered fixed labels
- **WHEN** the effective config selects a supported review language
- **THEN** the worker uses that language for LLM prompt instructions and fixed bot-rendered review labels where language customization is supported
- **AND** unsupported or invalid repository language values are not applied

#### Scenario: Summary comment can be disabled per repo
- **WHEN** the effective config disables summary comments
- **THEN** the reporter fan-out does not create or update the normal summary issue comment for that review job
- **AND** any enabled advisory Check Run or inline output remains governed by its own effective setting

#### Scenario: Check Run can be disabled per repo
- **WHEN** the effective config disables Check Run reporting
- **THEN** the reporter fan-out does not create or update the advisory `AI Review` Check Run for that review job
- **AND** the review job does not fail solely because that reporter is disabled

#### Scenario: Inline output uses effective limits
- **WHEN** the effective config permits inline Pull Request Review comments
- **THEN** inline eligibility, maximum comment count, severity threshold, confidence threshold, required evidence fields, and RIGHT-side diff line mapping are evaluated before creating or updating inline output
- **AND** findings that do not satisfy the effective inline policy remain summary-only or Check-Run-only according to the enabled reporters

#### Scenario: Path ignore filters review inputs
- **WHEN** the effective config contains valid `path_ignore` entries
- **THEN** changed files and repository context candidates matching those entries are omitted from LLM prompt context and inline comment eligibility where implemented
- **AND** omitted files are represented only by safe bounded omitted-context or configuration limitation metadata

#### Scenario: Disabled review suppresses downstream work
- **WHEN** the effective config has review `enabled: false`
- **THEN** the worker does not fetch changed files for review beyond what is required to resolve safe config state, build LLM prompt context, run optional analyzers, call the LLM, create new summary comments, create Check Runs, create inline Pull Request Reviews, request changes, auto-fix code, auto-merge, or block merging

### Requirement: Repository config verification
The implementation SHALL include automated tests and standard verification for repository config parsing, effective-config merging, missing or invalid config behavior, and review-flow integration.

#### Scenario: Parser and defaults are tested
- **WHEN** M14 implementation is complete
- **THEN** tests cover supported schema parsing, omitted-field defaults, invalid YAML, invalid field types, unsupported enum values, out-of-range thresholds, and invalid path ignore entries

#### Scenario: Safety boundary merge is tested
- **WHEN** M14 implementation is complete
- **THEN** tests prove repository config cannot enable globally disabled inline comments, Check Runs, optional Go analyzer execution, safe checkout behavior, or blocking output policy
- **AND** tests prove repository config can disable or tighten globally permitted behavior

#### Scenario: Missing and invalid config behavior is tested
- **WHEN** M14 implementation is complete
- **THEN** tests cover missing `.github/ai-review.yml` and `.github/ai-review.yaml`
- **AND** tests cover invalid config falling back to service defaults with safe bounded limitation metadata
- **AND** tests cover that raw private config content is not emitted in logs, comments, Check Runs, prompts, or errors

#### Scenario: Review flow integration is tested
- **WHEN** M14 implementation is complete
- **THEN** tests cover disabled review, disabled summary comments, disabled Check Runs, disabled inline comments, inline max comment overrides, inline severity and confidence threshold overrides, optional Go analyzer disablement, and path ignore behavior where implemented

#### Scenario: Standard commands pass
- **WHEN** M14 implementation is complete
- **THEN** `gofmt -w .` has been run
- **AND** `go test ./...` passes
- **AND** `go build ./cmd/server` passes
- **AND** `openspec validate m14-repo-level-ai-review-config --type change --strict` passes

### Requirement: GitHub-native inline comment body format
When rendering an inline Pull Request Review comment for an eligible finding, the service SHALL make the visible body read as a concise line-level reviewer note while preserving the service marker and finding fingerprint for idempotency.

#### Scenario: Inline comment starts with human-facing severity label
- **WHEN** the service renders an inline comment body for an eligible finding
- **THEN** the body starts with a localized human-facing severity label
- **AND** the body does not start with the hidden inline marker
- **AND** blocker findings use a label equivalent to `🚨 Blocking risk`
- **AND** warning findings use a label equivalent to `⚠️ Potential issue`

#### Scenario: Inline comment contains concise actionable visible text
- **WHEN** the service renders an inline comment body for an eligible finding
- **THEN** the visible top section includes the finding title or direct explanation
- **AND** it includes a short localized suggestion label or sentence using the finding suggestion
- **AND** it does not render evidence, failure scenario, and confidence as top-level report bullets

#### Scenario: Evidence details are collapsed
- **WHEN** the service renders an inline comment body for an eligible finding with evidence, failure scenario, suggestion, and confidence
- **THEN** evidence appears inside a `<details>` block
- **AND** failure scenario appears inside the same `<details>` block
- **AND** confidence appears inside the same `<details>` block
- **AND** the default visible comment remains compact before the details block

#### Scenario: Optional confidence is not fabricated
- **WHEN** the service renders an inline comment body for an eligible finding without confidence
- **THEN** the details block omits confidence
- **AND** the renderer does not fabricate a confidence value

### Requirement: Inline marker placement and backward compatibility
The service SHALL keep inline hidden marker and fingerprint metadata discoverable after moving the marker after visible content, and SHALL continue to recognize existing leading-marker inline comments created by previous versions.

#### Scenario: New inline body places marker after visible content
- **WHEN** the service renders a new inline comment body
- **THEN** the service marker and fingerprint are present after the visible reviewer note and details content
- **AND** the marker is not the first non-whitespace content in the body
- **AND** the marker does not expose secrets, tokens, raw prompts, raw model responses, complete webhook payloads, or unbounded private source

#### Scenario: New trailing-marker body is discoverable
- **WHEN** existing inline comment discovery reads a bot comment whose marker and fingerprint appear after visible content
- **THEN** the service extracts the fingerprint
- **AND** the comment can be matched for update, stale marking, or inactive cleanup

#### Scenario: Old leading-marker body remains discoverable
- **WHEN** existing inline comment discovery reads a bot comment whose body begins with the historical hidden marker and fingerprint format
- **THEN** the service extracts the fingerprint
- **AND** the comment can be matched for update, stale marking, or inactive cleanup

#### Scenario: Unmarked comments remain untouched
- **WHEN** an existing Pull Request Review comment lacks this service's inline marker or lacks a valid fingerprint
- **THEN** the service does not treat it as bot-owned
- **AND** the service does not update, stale-mark, inactive-mark, minimize, resolve, or otherwise alter that comment

### Requirement: GitHub-native Pull Request Review body
When creating a submitted Pull Request Review for inline comments, the service SHALL render a concise localized review body that reads like an advisory GitHub review note.

#### Scenario: English review body is human-friendly
- **WHEN** the service creates a Pull Request Review for one or more inline comments with English review language
- **THEN** the review body says that Review Cat left the number of inline comments
- **AND** it states that findings are advisory and non-blocking
- **AND** it does not use report-like wording such as `AI Review found N inline comment(s)`

#### Scenario: Chinese review body is localized
- **WHEN** the service creates a Pull Request Review for one or more inline comments with `zh-CN` review language
- **THEN** the review body is written in Simplified Chinese
- **AND** it states the number of inline comments
- **AND** it states that findings are advisory and non-blocking

### Requirement: Summary comment milestone-neutral advisory wording
The PR summary comment SHALL remain the full advisory report while avoiding stale milestone-specific wording in fixed renderer text.

#### Scenario: English summary advisory text is milestone-neutral
- **WHEN** the service renders an English PR summary comment
- **THEN** the fixed advisory text does not contain `M2 Review`
- **AND** it still states that findings are advisory and non-blocking
- **AND** the summary comment marker and footer purpose remain unchanged

#### Scenario: Chinese summary advisory text remains localized
- **WHEN** the service renders a `zh-CN` PR summary comment
- **THEN** the fixed advisory text is localized
- **AND** it does not introduce historical milestone wording
- **AND** the summary comment marker and footer purpose remain unchanged

### Requirement: Inline formatting verification
The implementation SHALL include automated tests for compact inline formatting, marker compatibility, localized review bodies, and summary wording cleanup.

#### Scenario: Tests cover inline body structure
- **WHEN** M15 implementation is complete
- **THEN** tests assert inline bodies start with the visible severity label rather than the hidden marker
- **AND** tests assert evidence, failure scenario, and confidence are inside `<details>`
- **AND** tests assert the marker and fingerprint appear after visible content

#### Scenario: Tests cover marker compatibility
- **WHEN** M15 implementation is complete
- **THEN** tests assert new trailing-marker inline bodies are discoverable by existing inline comment detection
- **AND** tests assert old leading-marker inline bodies are still discoverable
- **AND** tests assert unrelated comments without the service marker and fingerprint are ignored

#### Scenario: Tests cover localized fixed text
- **WHEN** M15 implementation is complete
- **THEN** tests assert the Pull Request Review body is human-friendly in English
- **AND** tests assert the Pull Request Review body is localized for `zh-CN`
- **AND** tests assert summary advisory wording no longer contains stale milestone text

#### Scenario: Standard commands pass
- **WHEN** M15 implementation is complete
- **THEN** `gofmt -w .` has been run
- **AND** `go test ./...` passes
- **AND** `go build ./cmd/server` passes
- **AND** `openspec validate m15-github-native-inline-comment-format --type change --strict` passes

### Requirement: Conservative default inline publication
The service SHALL default inline PR review comments to blocker-severity findings only unless repository configuration explicitly lowers the inline severity threshold.

#### Scenario: Default inline policy requires blocker severity
- **WHEN** inline comments are enabled and no repository-level inline severity override is supplied
- **THEN** blocker findings may be considered for inline publication when all other inline gates pass
- **AND** warning, suggestion, and question findings stay out of inline comments by default
- **AND** those non-blocker findings may still appear in summary output when otherwise valid

#### Scenario: Repository config can lower inline threshold
- **WHEN** a safe repository review config explicitly sets the inline severity threshold to `warning`
- **THEN** warning findings may be considered for inline publication when all other inline gates pass
- **AND** global safety gates such as inline enablement, max comments, diff-line mapping, and confidence threshold still apply

#### Scenario: Warning rendering remains supported
- **WHEN** a warning finding is rendered for an inline body through an explicit policy or renderer test
- **THEN** warning severity keeps its localized human-facing label
- **AND** marker placement and fingerprint compatibility remain unchanged

