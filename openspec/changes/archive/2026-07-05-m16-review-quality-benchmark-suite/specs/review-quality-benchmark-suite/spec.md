## ADDED Requirements

### Requirement: Replayable real-PR benchmark fixtures
The system SHALL support offline benchmark fixtures generated from real pull requests and replayed without GitHub credentials.

#### Scenario: Real PR fixture can be generated for offline replay
- **WHEN** an operator runs the real-PR fixture generator with GitHub App credentials, owner, repo, pull number, and output path
- **THEN** the generator writes a fixture containing changed-file metadata, bounded repository files observed through the benchmark context path, and benchmark metadata needed for replay
- **AND** the generator does not publish GitHub comments, create Check Runs, run production review jobs, or modify the remote repository

#### Scenario: Generated fixture replays without network access
- **WHEN** the benchmark runner loads a generated fixture after credentials are unavailable
- **THEN** it can run the offline context and finding-quality benchmark using only fixture content, optional captured review output, and local annotations
- **AND** it does not call GitHub APIs or fetch additional repository content

#### Scenario: Existing context fixtures remain valid
- **WHEN** the benchmark runner loads a fixture that only contains the existing context benchmark fields
- **THEN** it continues to report context precision, recall, F1, budget, omissions, and suite aggregates
- **AND** finding-quality metrics are omitted or reported as not annotated rather than treated as failures

### Requirement: Safe private fixture handling
The benchmark workflow SHALL protect private repository content during generation, sanitization, storage, and reporting.

#### Scenario: Private generated fixture is treated as unsanitized
- **WHEN** a fixture is generated from a private repository or without an explicit sanitized marker
- **THEN** documentation and command output instruct the operator to keep it in a gitignored or temporary location until reviewed and sanitized
- **AND** generated private fixtures are not written directly to tracked fixture paths by default

#### Scenario: Sanitized fixture can be intentionally committed
- **WHEN** an operator sanitizes a fixture for repository inclusion
- **THEN** the fixture annotation path includes metadata that identifies it as sanitized and replayable
- **AND** the fixture contains no secrets, API keys, private keys, installation tokens, raw webhook payloads, or unintended private repository content

#### Scenario: Benchmark report omits private raw content
- **WHEN** the benchmark runner emits per-fixture or suite-level output
- **THEN** the output may include fixture names, expected IDs, file paths, line numbers, categories, severities, and aggregate counts
- **AND** it does not include raw private source content, raw prompts, raw model output, secrets, tokens, private keys, API keys, installation tokens, or complete webhook payloads

### Requirement: Finding expectation annotations
The benchmark fixture schema SHALL support deterministic annotations for expected findings and expected no-finding cases.

#### Scenario: Expected finding is annotated
- **WHEN** a fixture includes an expected finding
- **THEN** the expected finding can identify the expected issue using stable fields such as ID, file, line or line range, title, category, severity, evidence hint, and matching notes
- **AND** the benchmark can compare actual findings against the annotation deterministically

#### Scenario: Expected no-finding case is annotated
- **WHEN** a fixture represents a PR where no AI findings are expected
- **THEN** the fixture can declare the expected no-finding intent explicitly
- **AND** any actual finding produced during replay is reported as unexpected unless it is explicitly classified otherwise by the fixture annotations

#### Scenario: Duplicate or low-value annotation is available
- **WHEN** expected or actual finding comparisons identify findings that are duplicates, too generic, unsupported, style-only, or otherwise low-value
- **THEN** the benchmark can represent those classifications using deterministic labels
- **AND** those classifications are counted separately from covered expected findings

### Requirement: Deterministic finding quality metrics
The benchmark runner SHALL report deterministic finding quality metrics for annotated fixtures.

#### Scenario: Expected finding is covered
- **WHEN** an actual finding deterministically matches an expected finding annotation
- **THEN** the benchmark counts the expected finding as covered
- **AND** the report includes expected finding coverage metrics for the fixture

#### Scenario: Expected finding is missed
- **WHEN** no actual finding deterministically matches an expected finding annotation
- **THEN** the benchmark counts the expected finding as missed
- **AND** the report lists the missed expected finding by safe identifier and location metadata

#### Scenario: Unexpected finding is reported
- **WHEN** an actual finding does not match any expected finding annotation
- **THEN** the benchmark counts it as unexpected
- **AND** the report lists it using safe metadata such as file, line, severity, category, title, and classification label without raw private evidence content

#### Scenario: Duplicate and low-value findings are reported separately
- **WHEN** actual findings are deterministically classified as duplicate or low-value
- **THEN** the benchmark includes duplicate and low-value counts and lists
- **AND** these counts do not improve expected finding coverage

### Requirement: Suite-level regression output
The benchmark runner SHALL emit suite-level aggregate output suitable for regression comparison.

#### Scenario: Suite aggregate includes context and finding metrics
- **WHEN** multiple fixtures are run as a suite
- **THEN** the suite report includes aggregate context precision, recall, F1, source-only metrics, fixture count, and annotated finding quality totals
- **AND** annotated finding totals include expected count, covered count, missed count, unexpected count, duplicate count, and low-value count when annotations are present

#### Scenario: Per-fixture details remain available
- **WHEN** the suite report is emitted
- **THEN** it includes per-fixture metric summaries and safe missed or unexpected finding lists
- **AND** a regression can be investigated without exposing raw private fixture content in standard output

#### Scenario: Output is stable for repeated runs
- **WHEN** the same fixture suite and same captured or deterministic review inputs are run repeatedly
- **THEN** aggregate counts, per-fixture classifications, and ordering of reported lists remain stable

### Requirement: Benchmark documentation
The project SHALL document the full offline review quality benchmark workflow.

#### Scenario: Documentation covers fixture lifecycle
- **WHEN** an operator reads the benchmark documentation
- **THEN** it explains how to generate a real-PR fixture, keep unsanitized private fixtures out of git, sanitize fixture content, annotate expected findings or expected no-finding cases, and run one fixture or a suite
- **AND** it explains what benchmark reports do and do not contain

#### Scenario: Documentation covers regression use
- **WHEN** a developer wants to compare prompt, context, verifier, or reporting changes
- **THEN** the documentation explains how to use suite-level metrics and per-fixture missed or unexpected finding lists for regression review
- **AND** it states that benchmark metrics are advisory and do not create production merge-blocking behavior

### Requirement: Benchmark behavior tests
The implementation SHALL include deterministic tests for fixture decoding, finding-quality reporting, and suite aggregation.

#### Scenario: Fixture decoding tests cover annotation variants
- **WHEN** benchmark tests run
- **THEN** they cover fixtures with expected findings, expected no-finding annotations, legacy context-only fixtures, and invalid conflicting annotations

#### Scenario: Reporting tests cover finding quality categories
- **WHEN** benchmark tests run
- **THEN** they cover covered expected findings, missed expected findings, unexpected findings, duplicate findings, and low-value findings
- **AND** report assertions verify that raw private content and secret-like values are not emitted in safe report fields

#### Scenario: Aggregate tests cover suite metrics
- **WHEN** benchmark tests run across multiple fixtures
- **THEN** they verify deterministic aggregate context metrics and aggregate finding quality counts
- **AND** they verify stable ordering of per-fixture missed and unexpected finding summaries
