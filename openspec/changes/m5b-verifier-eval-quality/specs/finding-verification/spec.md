## ADDED Requirements

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

## MODIFIED Requirements

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
