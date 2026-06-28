## ADDED Requirements

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
The worker SHALL request a concise, evidence-based review summary from an OpenAI-compatible LLM using PR metadata and changed file context.

#### Scenario: LLM review is requested
- **WHEN** changed file context is available for a review job
- **THEN** the service sends the configured model a prompt containing PR metadata and bounded changed-file patch context
- **AND** the prompt asks for conservative summary feedback that does not fabricate unavailable context

#### Scenario: Oversized patch context is bounded
- **WHEN** changed file patches exceed the configured or implementation-defined prompt budget
- **THEN** the service omits or truncates excess patch context
- **AND** the review prompt or result records that some context was omitted

#### Scenario: LLM request fails
- **WHEN** the LLM provider returns an error or unusable response
- **THEN** the review job stops without publishing an empty or fabricated review comment

### Requirement: PR comment rendering
The service SHALL render LLM review output as stable GitHub Markdown for a PR conversation comment.

#### Scenario: Review output is rendered
- **WHEN** the LLM returns non-empty review summary content
- **THEN** the comment renderer produces Markdown identifying the comment as an AI review summary
- **AND** the rendered comment remains conservative and non-blocking

#### Scenario: Empty review output is suppressed
- **WHEN** the LLM output is empty or whitespace-only
- **THEN** the service does not publish an empty or noisy PR comment

### Requirement: PR comment publishing
The worker SHALL publish the rendered review summary as a Pull Request conversation comment through the GitHub Issues comment API.

#### Scenario: Comment is published
- **WHEN** a review job has rendered non-empty Markdown
- **THEN** the service creates an issue comment for the job owner, repo, and pull number
- **AND** the comment appears on the PR conversation

#### Scenario: Comment publishing fails
- **WHEN** the GitHub Issues comment API returns an error
- **THEN** the review job records or logs safe failure metadata
- **AND** the service does not retry by creating duplicate comments inside the webhook handler

### Requirement: Secret-safe operation
The service SHALL avoid logging or returning secrets, private keys, installation tokens, API keys, or complete private repository payloads.

#### Scenario: Error is reported safely
- **WHEN** config, webhook, GitHub API, LLM, or comment publishing fails
- **THEN** logs and HTTP responses include only safe metadata such as event type, action, delivery ID, owner, repo, pull number, and error category
- **AND** logs and HTTP responses do not include configured secrets, private keys, installation tokens, API keys, or complete webhook payloads
