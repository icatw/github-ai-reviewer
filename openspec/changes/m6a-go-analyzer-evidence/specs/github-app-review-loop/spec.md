## ADDED Requirements

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
