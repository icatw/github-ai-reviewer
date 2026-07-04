## ADDED Requirements

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
