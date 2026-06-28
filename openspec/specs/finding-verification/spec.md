## Purpose
Define how structured LLM review findings are verified against bounded PR evidence before advisory output is rendered or reported.

## Requirements

### Requirement: Structured finding verification
The service SHALL verify each structured LLM finding against bounded PR evidence before the finding is made available to comment rendering or Check Run reporting.

#### Scenario: Supported finding is kept
- **WHEN** a structured finding references an available changed file and its evidence is supported by available patch, full file, related test, docs, or config context
- **THEN** the verifier keeps the finding for downstream advisory output
- **AND** the finding severity, title, evidence, failure scenario, suggestion, and confidence are preserved

#### Scenario: No findings are handled deterministically
- **WHEN** a structured review result contains no findings
- **THEN** the verifier returns the result without adding fabricated findings
- **AND** verification stats record the no-finding case without treating it as a failure

### Requirement: Evidence source boundaries
The verifier SHALL use only evidence collected for the current review job and SHALL NOT fetch additional repository data or execute static analyzers in M5a.

#### Scenario: Available evidence sources are used
- **WHEN** a finding is verified
- **THEN** the verifier may use changed-file metadata, patch context, full changed-file context, related test context, repo docs/config context, and omitted-context notes from the current job
- **AND** the verifier does not use unbounded repository indexing, vector search, AST analysis, call graph analysis, durable stored history, or external analyzer execution

#### Scenario: Future static-check evidence remains an extension point
- **WHEN** verifier evidence sources are represented internally
- **THEN** the design allows a future static-check evidence source to be added
- **AND** M5a does not run `go test`, `go vet`, `staticcheck`, `gosec`, `semgrep`, or similar tools as part of finding verification

### Requirement: Unsupported finding filtering
The verifier SHALL drop or downgrade findings that cannot be supported by available evidence.

#### Scenario: Unsupported evidence is dropped
- **WHEN** a finding has no useful overlap with available PR evidence and does not describe an explicit omitted-context limitation
- **THEN** the verifier removes the finding from the downstream review result
- **AND** verification stats count the finding under an unsupported-evidence reason category

#### Scenario: Unavailable file is dropped
- **WHEN** a finding references a file that is not present in changed-file metadata, included repo context, or omitted-context notes for the current job
- **THEN** the verifier removes the finding from the downstream review result
- **AND** verification stats count the finding under an unavailable-file reason category

#### Scenario: Impossible line information is dropped or downgraded
- **WHEN** a finding references a line that is impossible for the available file content or unavailable from the available patch/full-file context
- **THEN** the verifier removes the finding or downgrades it to a `question` when file-level evidence remains useful
- **AND** verification stats count the finding under a line-unavailable or line-mismatch reason category

#### Scenario: Omitted context dependency is downgraded or dropped
- **WHEN** a finding depends on context that the review job explicitly omitted, skipped, filtered, truncated, or failed to fetch
- **THEN** the verifier downgrades the finding to `question` if it is useful as a limitation
- **AND** the verifier drops the finding if it asserts a concrete defect that cannot be verified from available evidence

### Requirement: Verification output safety
The verifier SHALL produce deterministic safe counts and reason categories without logging or exposing private code content.

#### Scenario: Safe verification stats are emitted
- **WHEN** finding verification completes
- **THEN** the service records counts for kept, downgraded, and dropped findings
- **AND** the service records counts by deterministic reason category
- **AND** the service does not log raw prompts, raw model output, secrets, tokens, private keys, complete webhook payloads, or private repository code content

### Requirement: Finding verification eval fixtures
The implementation SHALL include deterministic unit or eval fixtures that exercise the verifier's accuracy boundary.

#### Scenario: Required verification fixtures exist
- **WHEN** M5a implementation is complete
- **THEN** automated tests or eval fixtures cover a true positive finding
- **AND** they cover an unsupported finding
- **AND** they cover a finding for an unavailable file
- **AND** they cover a line or context mismatch
- **AND** they cover an omitted-context dependency
- **AND** they cover a no-finding result
