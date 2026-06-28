## MODIFIED Requirements

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

## ADDED Requirements

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
