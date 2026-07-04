## ADDED Requirements

### Requirement: Repository-level AI review config discovery
For each supported open pull request review job, the worker SHALL attempt to discover repository-level AI review configuration from `.github/ai-review.yml` or `.github/ai-review.yaml` at the pull request repository ref when safe and available.

#### Scenario: YAML config is discovered from PR ref
- **WHEN** a signed supported review job reaches worker processing
- **AND** `.github/ai-review.yml` exists at the current pull request head ref or head SHA
- **THEN** the worker reads that file as the repository AI review config candidate
- **AND** the worker uses the config only for the current review job

#### Scenario: YAML extension fallback is discovered
- **WHEN** `.github/ai-review.yml` is absent
- **AND** `.github/ai-review.yaml` exists at the current pull request head ref or head SHA
- **THEN** the worker reads `.github/ai-review.yaml` as the repository AI review config candidate
- **AND** the worker uses the config only for the current review job

#### Scenario: Primary config wins
- **WHEN** both `.github/ai-review.yml` and `.github/ai-review.yaml` exist
- **THEN** the worker uses `.github/ai-review.yml`
- **AND** the worker does not merge both files

#### Scenario: Missing config is non-blocking
- **WHEN** neither repository config path exists or GitHub reports the file is not found
- **THEN** the review continues with service defaults and global service configuration
- **AND** the missing file does not fail the review job

#### Scenario: Config fetch failure is safe
- **WHEN** repository config discovery fails because GitHub content fetching is unavailable, unauthorized, oversized, ambiguous, or otherwise unsafe to use
- **THEN** the review continues with service defaults and global service configuration
- **AND** the worker records only a safe bounded configuration limitation category
- **AND** logs, comments, Check Runs, and prompts do not include secrets, installation tokens, raw private config content, complete webhook payloads, or unbounded private repository code

### Requirement: Repository-level AI review config schema
The repository config parser SHALL support a conservative first-slice schema and reject unknown, invalid, or unsafe values without applying partial unsafe behavior.

#### Scenario: Supported config fields are parsed
- **WHEN** repository config contains valid YAML for supported fields
- **THEN** the parser recognizes `enabled`, `language`, `summary_comment.enabled`, `check_run.enabled`, `inline_comments.enabled`, `inline_comments.max_comments`, `inline_comments.severity_threshold`, `inline_comments.confidence_threshold`, `path_ignore`, and `go_analyzer.enabled`
- **AND** omitted fields remain unset so service defaults and global configuration can apply

#### Scenario: Invalid config falls back to defaults
- **WHEN** repository config content is malformed YAML, uses invalid types, uses unsupported enum values, sets out-of-range numeric thresholds, or otherwise fails validation
- **THEN** the worker treats the repository config as invalid for that review job
- **AND** the review continues with service defaults and global service configuration
- **AND** the invalid config is reported only as a safe bounded configuration limitation without exposing raw private config content

#### Scenario: Review can be disabled
- **WHEN** a valid repository config sets `enabled: false`
- **THEN** the worker suppresses normal review work for the job before LLM calls, optional analyzer execution, summary comment creation, Check Run creation, and inline review creation
- **AND** the suppression remains advisory and does not request changes, auto-fix, auto-merge, fail merge gates, or block merging

#### Scenario: Language is limited to supported values
- **WHEN** repository config sets `language`
- **THEN** the parser accepts only implementation-supported review languages
- **AND** unsupported language values make the repository config invalid for that job

#### Scenario: Inline threshold fields are bounded
- **WHEN** repository config sets `inline_comments.max_comments`, `inline_comments.severity_threshold`, or `inline_comments.confidence_threshold`
- **THEN** the parser accepts only values that can tighten existing inline eligibility and limit behavior
- **AND** values that would increase unsafe output volume or lower quality below service defaults are ignored by effective-config merge or rejected by validation according to the implementation policy

#### Scenario: Path ignore entries are bounded
- **WHEN** repository config sets `path_ignore`
- **THEN** the parser accepts only a bounded list of deterministic repository-relative path patterns supported by the implementation
- **AND** invalid, absolute, parent-traversing, or unsupported patterns make the repository config invalid for that job

### Requirement: Effective review config safety boundary
The service SHALL merge repository config with global service configuration into an effective review config where global service configuration remains the upper safety boundary.

#### Scenario: Repo config cannot enable globally disabled Check Runs
- **WHEN** global service configuration disables Check Run reporting
- **AND** repository config sets `check_run.enabled: true`
- **THEN** the effective config keeps Check Run reporting disabled for the review job

#### Scenario: Repo config can disable globally enabled Check Runs
- **WHEN** global service configuration permits Check Run reporting
- **AND** repository config sets `check_run.enabled: false`
- **THEN** the effective config disables Check Run reporting for the review job
- **AND** summary and inline reporters remain governed by their own effective settings

#### Scenario: Repo config cannot enable globally disabled inline comments
- **WHEN** global service configuration disables inline Pull Request Review comments
- **AND** repository config sets `inline_comments.enabled: true`
- **THEN** the effective config keeps inline comment publishing disabled for the review job

#### Scenario: Repo config can tighten inline comment policy
- **WHEN** global service configuration permits inline Pull Request Review comments
- **AND** repository config sets inline comment limits or thresholds that are stricter than service defaults
- **THEN** the effective config applies the stricter maximum comment count, severity threshold, and confidence threshold for the review job
- **AND** required evidence fields and RIGHT-side diff line mapping remain mandatory

#### Scenario: Repo config cannot enable globally disabled Go analyzer behavior
- **WHEN** global service configuration or workspace provider configuration disables optional Go analyzer execution or safe checkout
- **AND** repository config sets `go_analyzer.enabled: true`
- **THEN** the effective config keeps optional Go analyzer execution disabled or safely skipped for the review job

#### Scenario: Repo config can disable globally available Go analyzer behavior
- **WHEN** global service configuration permits optional Go analyzer execution with a safe workspace provider
- **AND** repository config sets `go_analyzer.enabled: false`
- **THEN** the effective config skips optional Go analyzer execution for the review job
- **AND** the review continues through non-analyzer context, LLM review, verification, and enabled reporters

#### Scenario: Repo config cannot change blocking policy
- **WHEN** repository config sets any supported field
- **THEN** the effective config does not allow AI findings to request changes, auto-fix code, auto-merge pull requests, fail merge gates, or block merging

### Requirement: Effective config integration points
The worker SHALL apply the effective review config before review work that depends on language, outputs, inline eligibility, analyzer execution, or path filtering decisions.

#### Scenario: Language affects LLM prompt and rendered fixed labels
- **WHEN** the effective config selects a supported review language
- **THEN** the worker uses that language for LLM prompt instructions and fixed bot-rendered review labels where language customization is supported
- **AND** unsupported or invalid repository language values are not applied

#### Scenario: Summary comment can be disabled per repo
- **WHEN** the effective config disables summary comments
- **THEN** the reporter fan-out does not create or update the normal summary issue comment for that review job
- **AND** any enabled advisory Check Run or inline output remains governed by its own effective setting

#### Scenario: Check Run can be disabled per repo
- **WHEN** the effective config disables Check Run reporting
- **THEN** the reporter fan-out does not create or update the advisory `AI Review` Check Run for that review job
- **AND** the review job does not fail solely because that reporter is disabled

#### Scenario: Inline output uses effective limits
- **WHEN** the effective config permits inline Pull Request Review comments
- **THEN** inline eligibility, maximum comment count, severity threshold, confidence threshold, required evidence fields, and RIGHT-side diff line mapping are evaluated before creating or updating inline output
- **AND** findings that do not satisfy the effective inline policy remain summary-only or Check-Run-only according to the enabled reporters

#### Scenario: Path ignore filters review inputs
- **WHEN** the effective config contains valid `path_ignore` entries
- **THEN** changed files and repository context candidates matching those entries are omitted from LLM prompt context and inline comment eligibility where implemented
- **AND** omitted files are represented only by safe bounded omitted-context or configuration limitation metadata

#### Scenario: Disabled review suppresses downstream work
- **WHEN** the effective config has review `enabled: false`
- **THEN** the worker does not fetch changed files for review beyond what is required to resolve safe config state, build LLM prompt context, run optional analyzers, call the LLM, create new summary comments, create Check Runs, create inline Pull Request Reviews, request changes, auto-fix code, auto-merge, or block merging

### Requirement: Repository config verification
The implementation SHALL include automated tests and standard verification for repository config parsing, effective-config merging, missing or invalid config behavior, and review-flow integration.

#### Scenario: Parser and defaults are tested
- **WHEN** M14 implementation is complete
- **THEN** tests cover supported schema parsing, omitted-field defaults, invalid YAML, invalid field types, unsupported enum values, out-of-range thresholds, and invalid path ignore entries

#### Scenario: Safety boundary merge is tested
- **WHEN** M14 implementation is complete
- **THEN** tests prove repository config cannot enable globally disabled inline comments, Check Runs, optional Go analyzer execution, safe checkout behavior, or blocking output policy
- **AND** tests prove repository config can disable or tighten globally permitted behavior

#### Scenario: Missing and invalid config behavior is tested
- **WHEN** M14 implementation is complete
- **THEN** tests cover missing `.github/ai-review.yml` and `.github/ai-review.yaml`
- **AND** tests cover invalid config falling back to service defaults with safe bounded limitation metadata
- **AND** tests cover that raw private config content is not emitted in logs, comments, Check Runs, prompts, or errors

#### Scenario: Review flow integration is tested
- **WHEN** M14 implementation is complete
- **THEN** tests cover disabled review, disabled summary comments, disabled Check Runs, disabled inline comments, inline max comment overrides, inline severity and confidence threshold overrides, optional Go analyzer disablement, and path ignore behavior where implemented

#### Scenario: Standard commands pass
- **WHEN** M14 implementation is complete
- **THEN** `gofmt -w .` has been run
- **AND** `go test ./...` passes
- **AND** `go build ./cmd/server` passes
- **AND** `openspec validate m14-repo-level-ai-review-config --type change --strict` passes
