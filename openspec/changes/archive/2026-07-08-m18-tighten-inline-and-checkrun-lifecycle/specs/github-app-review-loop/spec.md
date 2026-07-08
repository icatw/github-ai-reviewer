## MODIFIED Requirements

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

## ADDED Requirements

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
