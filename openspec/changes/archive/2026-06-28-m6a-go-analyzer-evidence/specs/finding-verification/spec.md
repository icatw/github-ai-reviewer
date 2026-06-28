## ADDED Requirements

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
