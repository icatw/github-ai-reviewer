## ADDED Requirements

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
