## ADDED Requirements

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
