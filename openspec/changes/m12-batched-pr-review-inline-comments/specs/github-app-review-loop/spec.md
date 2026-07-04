## ADDED Requirements

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
