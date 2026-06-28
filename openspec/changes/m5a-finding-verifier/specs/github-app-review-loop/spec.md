## ADDED Requirements

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
