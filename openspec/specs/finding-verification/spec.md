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

### Requirement: Static-check evidence ingestion
The verifier SHALL accept bounded static-check evidence from the optional Go analyzer as an `EvidenceSourceStaticCheck` source for the current review job.

#### Scenario: Static-check evidence is available to verifier
- **WHEN** the Go analyzer produces bounded static-check evidence for the current review job
- **THEN** the verifier includes that evidence when evaluating structured findings
- **AND** the evidence source type is `static_check_context`
- **AND** the evidence identifies the tool name, exit category, and any safely parsed package, file, line, message, and limitation fields

#### Scenario: Static-check evidence is current-job scoped
- **WHEN** static-check evidence is ingested by the verifier
- **THEN** the evidence is scoped only to the current review job and PR head being reviewed
- **AND** the verifier does not fetch additional repository data, run additional analyzer commands, or use durable stored analyzer history during verification

#### Scenario: Skipped analyzer is represented safely
- **WHEN** Go analyzer execution is skipped, unavailable, timed out, or internally failed
- **THEN** the verifier may receive a safe static-check limitation or omitted-context note
- **AND** no concrete analyzer finding is fabricated from the skipped, unavailable, timed-out, or internally failed execution

### Requirement: Static-check evidence matching
The verifier SHALL use static-check evidence conservatively and compatibly with existing evidence source matching rules when keeping, downgrading, or dropping findings.

#### Scenario: Matching static-check evidence can support finding
- **WHEN** a structured finding references a file or package that is supported by bounded static-check evidence from `go test ./...` or `go vet ./...`
- **AND** the finding evidence, title, or failure scenario has deterministic overlap with the sanitized static-check message or parsed file and line
- **THEN** the verifier may keep the finding according to existing evidence strength rules
- **AND** verification stats count the finding under a supported static-check or supported evidence reason category

#### Scenario: Static-check evidence does not override unavailable file rules
- **WHEN** a structured finding references a file that is not present in changed-file metadata, included repo context, static-check evidence, or omitted-context notes for the current job
- **THEN** the verifier removes the finding or downgrades it according to existing unavailable-file rules
- **AND** static-check evidence from unrelated files or packages is not used as support

#### Scenario: Generic analyzer overlap is insufficient
- **WHEN** a structured finding overlaps static-check evidence only through generic words or broad failure terms
- **THEN** the verifier drops or downgrades the finding according to existing unsupported-evidence rules
- **AND** static-check evidence does not make the finding supported by itself

### Requirement: Static-check evidence safety metrics
The verifier SHALL expose only safe aggregate metrics for static-check evidence integration.

#### Scenario: Static-check stats are aggregate only
- **WHEN** finding verification completes with static-check evidence available
- **THEN** verification stats may include aggregate counts for static-check evidence items, supported findings using static-check evidence, dropped or downgraded findings, skipped analyzer categories, and deterministic reason categories
- **AND** stats do not include raw analyzer stdout, raw analyzer stderr, private repository code, raw prompts, raw model output, secrets, tokens, private keys, API keys, complete webhook payloads, or installation tokens

#### Scenario: Static-check eval fixtures remain deterministic
- **WHEN** verifier tests or eval fixtures exercise static-check evidence
- **THEN** expected kept, downgraded, dropped, skipped, and reason-category counts are deterministic
- **AND** fixture summaries remain aggregate and safe

### Requirement: Safe workspace-scoped static-check evidence
The verifier SHALL treat static-check evidence from the optional Go analyzer as valid only when it is scoped to the current review job and produced from a `SafeGoWorkspace` whose checkout was validated against the job head SHA.

#### Scenario: Valid workspace evidence is accepted
- **WHEN** the Go analyzer produces bounded static-check evidence from a `SafeGoWorkspace`
- **AND** the workspace provider validated that the checkout `HEAD` exactly matched the current review job head SHA before analyzer execution
- **THEN** the verifier may consider the evidence under the existing static-check evidence matching rules
- **AND** the evidence remains scoped only to the current review job

#### Scenario: Unvalidated workspace evidence is ignored
- **WHEN** analyzer evidence is produced without a validated safe workspace or without exact current-job head SHA validation
- **THEN** the verifier does not use that evidence to keep or downgrade findings
- **AND** verification stats record a deterministic skipped or unsafe-workspace reason category

### Requirement: Workspace provider limitations in verification
The verifier SHALL represent safe Go workspace provider skip, setup failure, checkout failure, head mismatch, path rejection, credential unavailability, timeout, and cleanup limitation categories without fabricating concrete static-check findings.

#### Scenario: Workspace setup skip is a limitation only
- **WHEN** the workspace provider is disabled, unavailable, times out, or rejects checkout before analyzer execution
- **THEN** the verifier may receive a safe limitation or omitted-context note for the current job
- **AND** no concrete static-check evidence item is fabricated from the skipped workspace provider outcome

#### Scenario: Cleanup limitation does not affect finding support
- **WHEN** analyzer evidence was produced from a validated safe workspace but workspace cleanup later fails or is partially completed
- **THEN** cleanup status is represented only as safe limitation or aggregate operational metadata
- **AND** cleanup failure does not make unrelated findings supported

### Requirement: Workspace provider verification metrics safety
The verifier SHALL expose only safe aggregate metrics for workspace-provider-related static-check evidence and limitation handling.

#### Scenario: Workspace metrics are aggregate only
- **WHEN** finding verification completes with workspace-provider evidence or limitations
- **THEN** stats may include aggregate counts for workspace-ready evidence, provider skips, unsafe workspace rejections, checkout mismatches, checkout timeouts, credential failures, cleanup limitations, and deterministic reason categories
- **AND** stats do not include raw analyzer stdout, raw analyzer stderr, private repository code, raw prompts, raw model output, secrets, tokens, private keys, API keys, complete webhook payloads, installation tokens, or checkout credentials

