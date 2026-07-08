# real-pr-fixture-sampling Specification

## Purpose
TBD - created by archiving change m17-real-pr-fixture-sampling. Update Purpose after archive.
## Requirements
### Requirement: Manifest-driven real PR fixture batch
The system SHALL define a batch sampling workflow that generates multiple real-PR benchmark fixtures from an explicit operator-provided manifest.

#### Scenario: Batch manifest identifies target PRs and outputs
- **WHEN** an operator prepares a batch fixture sampling manifest
- **THEN** each target entry identifies the owner, repository, pull number, intended output path, privacy status, sanitization intent, sampling dimensions, and annotation status
- **AND** the manifest does not embed raw source content, raw patches, secrets, tokens, private keys, API keys, installation tokens, complete webhook payloads, raw prompts, or raw model output

#### Scenario: Batch generation reuses read-only fixture behavior
- **WHEN** an operator runs the batch sampling workflow for manifest entries
- **THEN** each fixture is generated through the existing read-only real-PR fixture generation behavior
- **AND** the workflow does not publish GitHub comments, create Check Runs, submit PR reviews, run production review jobs, or modify remote repositories

#### Scenario: Batch workflow supports partial progress
- **WHEN** some manifest entries are already generated or annotated
- **THEN** the workflow lets the operator continue remaining entries without regenerating completed fixtures by default
- **AND** the manifest or checklist records per-fixture status using safe metadata only

### Requirement: Private fixture quarantine and sanitization
The batch sampling workflow SHALL keep private or unsanitized fixture content out of tracked repository paths until an operator intentionally sanitizes it.

#### Scenario: Private target defaults to quarantine
- **WHEN** a manifest entry is marked private or unsanitized
- **THEN** its generated fixture output path is required or documented to be a gitignored quarantine path such as a local data directory or temporary directory
- **AND** the workflow warns operators not to move it into tracked testdata until sanitization is complete

#### Scenario: Sanitized fixture can move to tracked testdata
- **WHEN** an operator intentionally sanitizes a fixture for repository inclusion
- **THEN** the fixture metadata records sanitized status and replay intent
- **AND** the operator can move the fixture to a tracked public or synthetic testdata path only after checking that it contains no secrets, private keys, API keys, installation tokens, raw private repository content, or complete webhook payloads

#### Scenario: Checklist remains safe for private batches
- **WHEN** the workflow emits or updates a suite checklist for private fixtures
- **THEN** the checklist contains only safe identifiers, status, tags, aggregate counts, metric summaries, and TODOs
- **AND** it does not contain raw private code, raw prompts, raw model output, secrets, tokens, private keys, API keys, installation tokens, or complete webhook payloads

### Requirement: Manual annotation workflow
The system SHALL document a manual annotation workflow for 10-20 real PR fixtures using the M16 benchmark fixture schema.

#### Scenario: Reviewer annotates relevant files and expected findings
- **WHEN** a reviewer annotates a generated fixture
- **THEN** the workflow instructs them to populate `golden_relevant_files`, expected findings with stable IDs and safe matching metadata, or explicit expected no-finding intent
- **AND** the workflow distinguishes missing annotation from an intentional clean/no-finding case

#### Scenario: Reviewer captures actual findings
- **WHEN** a reviewer captures benchmark or review output for a fixture
- **THEN** the workflow instructs them to normalize `actual_findings` into deterministic fields suitable for M16 metrics
- **AND** raw private model output is not required in the committed fixture or checklist

#### Scenario: Reviewer classifies quality outcomes
- **WHEN** actual findings are compared with expected annotations
- **THEN** the workflow instructs the reviewer to classify false positives, false negatives, duplicate findings, unsupported findings, too-generic findings, style-only findings, and low-value findings using deterministic quality labels and reviewer notes
- **AND** unresolved classification questions remain visible as per-fixture TODOs

### Requirement: Minimum sampling matrix
The real PR fixture sampling workflow SHALL define a minimum matrix for selecting the first 10-20 fixtures.

#### Scenario: Batch includes diverse domains and outcomes
- **WHEN** an operator assembles the initial sampling manifest
- **THEN** the selected fixtures cover auth or security changes, config or CI changes, API or backend changes, tests, docs-only changes, Go changes, Python or JavaScript changes, clean/no-finding cases, and noisy or low-value review cases
- **AND** each fixture records its matrix tags in safe metadata

#### Scenario: Batch records selection rationale
- **WHEN** an operator adds a PR to the sampling manifest
- **THEN** the manifest or checklist records a brief safe rationale for inclusion
- **AND** the rationale avoids raw private implementation details

### Requirement: Sampling runbook
The project SHALL document how to run the real PR fixture sampling milestone from selection through tuning decision.

#### Scenario: Runbook covers end-to-end sampling steps
- **WHEN** an operator reads the runbook
- **THEN** it explains how to choose PRs, prepare the manifest, generate fixtures, quarantine private outputs, sanitize fixtures, annotate expected and actual findings, run the benchmark suite, and update the checklist

#### Scenario: Runbook keeps tuning out of scope
- **WHEN** an operator completes the first annotated batch
- **THEN** the runbook explains how to classify false positives, false negatives, and low-value findings before deciding what to tune next
- **AND** it states that prompt, context builder, verifier, reporter, dashboard, storage, and production behavior changes are out of scope for this milestone

