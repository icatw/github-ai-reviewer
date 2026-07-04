## ADDED Requirements

### Requirement: Public metadata reflects current implemented capabilities
The repository SHALL keep public README and agent guidance aligned with implemented production capabilities and current milestone scope.

#### Scenario: README does not list completed capabilities as future work
- **WHEN** M14 documentation updates are complete
- **THEN** README current capabilities mention implemented summary comments, advisory Check Runs, manual `/ai-review`, repo-aware context, finding verification, optional Go analyzer/workspace provider, batched inline PR review comments, production/systemd operations, E2E guidance, and PR close/merge cleanup where appropriate
- **AND** the README roadmap does not list those completed capabilities as planned future work

#### Scenario: README documents remaining future work accurately
- **WHEN** M14 documentation updates are complete
- **THEN** README future work distinguishes repository-level config follow-up, durable job storage, review history, repository context improvements beyond the current implementation, optional CLI or GitHub Action entrypoints, dashboard, billing, tenant management, and other genuinely unimplemented work

#### Scenario: AGENTS milestone guidance is current
- **WHEN** M14 documentation updates are complete
- **THEN** AGENTS.md no longer presents M1 as the current milestone or describes Check Runs, inline comments, analyzer behavior, manual commands, production operations, or close cleanup as future-only work
- **AND** AGENTS.md preserves safety rules for secrets, non-blocking AI findings, explicit configuration, small verifiable changes, OpenSpec workflow, and standard verification commands

### Requirement: Repository config documentation
The repository SHALL document `.github/ai-review.yml` enough for repository owners and contributors to understand supported fields, defaults, and safety boundaries.

#### Scenario: Repository config example is documented
- **WHEN** M14 documentation updates are complete
- **THEN** public documentation includes an example `.github/ai-review.yml` with supported fields from the conservative first-slice schema
- **AND** the example demonstrates disabling or tightening behavior rather than bypassing global service safety configuration

#### Scenario: Repository config safety boundary is documented
- **WHEN** M14 documentation updates are complete
- **THEN** documentation states that repository config cannot enable globally disabled Check Runs, inline comments, optional Go analyzer execution, safe checkout behavior, auto-fix, auto-merge, request-changes behavior, or merge blocking
- **AND** documentation states that missing config is allowed and invalid config falls back to service defaults with safe limitation reporting

#### Scenario: Local tooling config decision is documented or ignored
- **WHEN** M14 documentation and ignore updates are complete
- **THEN** local `config/mcporter.json`, if present, is treated as local tooling metadata outside the AI review runtime config feature
- **AND** git ignore rules prevent accidentally tracking that local file if the implementation chooses to add the ignore entry
