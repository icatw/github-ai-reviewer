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
The implementation SHALL include deterministic unit or eval fixtures that exercise the verifier's accuracy boundary and safe quality metrics.

#### Scenario: Required verification fixtures exist
- **WHEN** M5b implementation is complete
- **THEN** automated tests or eval fixtures cover a true positive finding
- **AND** they cover an unsupported finding
- **AND** they cover a finding for an unavailable file
- **AND** they cover a line or context mismatch
- **AND** they cover an omitted-context dependency
- **AND** they cover a no-finding result

#### Scenario: Expanded realistic evidence fixtures exist
- **WHEN** M5b implementation is complete
- **THEN** automated tests or eval fixtures cover paraphrased evidence
- **AND** they cover short code snippets with safeguards
- **AND** they cover patch-only evidence
- **AND** they cover full-file evidence
- **AND** they cover related test evidence
- **AND** they cover docs or config evidence

#### Scenario: Mixed outcome fixtures exist
- **WHEN** M5b implementation is complete
- **THEN** automated tests or eval fixtures cover a review result containing multiple findings with kept, downgraded, and dropped outcomes
- **AND** expected verification stats include deterministic kept, downgraded, dropped, no-finding, and reason-category counts

#### Scenario: Missing tests and limitations fixtures exist
- **WHEN** M5b implementation is complete
- **THEN** automated tests or eval fixtures cover interactions between verified findings, missing-tests entries, and limitations
- **AND** unsupported concrete defects are not converted into missing-tests or limitation output
- **AND** omitted-context uncertainty may be represented as a downgraded question or limitation when useful

#### Scenario: Reason category distribution is stable
- **WHEN** the verifier eval fixture suite is executed repeatedly with the same inputs
- **THEN** aggregate kept, downgraded, dropped, no-finding, and reason-category counts remain stable
- **AND** the suite can detect changes that would unexpectedly keep generic unsupported findings or drop clearly supported findings

### Requirement: Conservative evidence matching quality
The verifier SHALL support deterministic evidence matching beyond exact full-string substring checks while remaining conservative.

#### Scenario: Paraphrased supported evidence is kept
- **WHEN** a finding references an available changed file and describes evidence that paraphrases a concrete code condition present in patch or full-file context
- **THEN** the verifier keeps the finding only when normalized tokens, identifiers, literals, or code phrases provide deterministic support
- **AND** verification stats count the finding under a supported reason category

#### Scenario: Short code snippet evidence is safeguarded
- **WHEN** a finding uses short evidence text
- **THEN** the verifier treats the evidence as supported only when the short text contains a meaningful identifier, literal, operator expression, config key, or exact code phrase present in available evidence
- **AND** generic words alone do not support the finding

#### Scenario: Generic overlap is not sufficient
- **WHEN** a finding overlaps available evidence only through generic words such as error, nil, test, config, timeout, handler, request, or response
- **THEN** the verifier drops the finding or downgrades it to a question only if a separate omitted-context limitation justifies uncertainty
- **AND** verification stats record the unsupported or omitted-context reason category instead of supported

### Requirement: Evidence source compatibility
The verifier SHALL consider evidence source type when deciding whether available context supports a finding.

#### Scenario: Patch-only evidence can support code findings
- **WHEN** a finding references an available changed file and its concrete evidence is present only in patch context for that file
- **THEN** the verifier may keep the finding without requiring full-file context

#### Scenario: Full-file evidence can support code findings
- **WHEN** a finding references an available changed file and its concrete evidence is present in bounded full-file context but not in the patch text
- **THEN** the verifier may keep or downgrade the finding according to line availability and evidence strength

#### Scenario: Related tests support test-specific findings
- **WHEN** a finding or missing-tests entry is about test behavior and the supporting evidence is present in related test context for the current review job
- **THEN** the verifier may keep the finding or missing-tests entry when the evidence is source-compatible

#### Scenario: Docs or config evidence does not support unrelated code defects
- **WHEN** a concrete code-defect finding references a source file but only unrelated docs or config prose overlaps the finding text
- **THEN** the verifier does not treat that docs or config evidence as support for the code-defect finding
- **AND** verification stats record an unsupported-evidence reason category

#### Scenario: Docs or config evidence supports docs or config findings
- **WHEN** a finding is explicitly about documentation, workflow configuration, repository configuration, or review limitations
- **AND** matching docs or config context is available for the current job
- **THEN** the verifier may keep or downgrade the finding according to evidence strength

### Requirement: Safe aggregate verification metrics
The verifier SHALL expose only safe aggregate quality metadata for runtime stats and deterministic eval summaries.

#### Scenario: Verification rates are aggregate only
- **WHEN** finding verification completes
- **THEN** stats may include total findings, kept count, downgraded count, dropped count, kept rate, downgraded rate, dropped rate, no-finding count, and deterministic reason-category counts
- **AND** stats do not include raw private code, raw prompts, raw model output, secrets, tokens, private keys, API keys, complete webhook payloads, or installation tokens

#### Scenario: Eval summaries are aggregate only
- **WHEN** verifier eval fixtures are executed
- **THEN** fixture summaries may include fixture names, expected and actual outcome categories, and aggregate reason-category counts
- **AND** fixture summaries do not include raw private repository content, raw prompts, raw model output, secrets, tokens, private keys, API keys, complete webhook payloads, or installation tokens
