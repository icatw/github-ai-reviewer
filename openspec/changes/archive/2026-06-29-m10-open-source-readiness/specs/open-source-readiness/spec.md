## ADDED Requirements

### Requirement: Public project entry point

The repository SHALL provide a public-facing README that explains the GitHub AI Reviewer project clearly enough for a new developer or evaluator to understand, run, and assess it without private context.

#### Scenario: Reader understands the project scope

- **WHEN** a reader opens the repository README
- **THEN** it describes the project purpose, current capabilities, intentionally unsupported features, architecture, repository layout, and roadmap
- **AND** it does not depend on private deployment context to explain the system

#### Scenario: Reader can configure the GitHub App safely

- **WHEN** a reader follows the setup documentation
- **THEN** it names the required GitHub App permissions and pull request webhook event
- **AND** it describes required environment variables using `.env.example` without including real secrets
- **AND** it states that workspace checkout is disabled by default and must be explicitly enabled with a dedicated workspace root

#### Scenario: Reader can verify local and production readiness

- **WHEN** a reader follows the verification documentation
- **THEN** it points to local smoke tests, production deployment guidance, operations runbook, and E2E evidence guidance
- **AND** it distinguishes safe public docs from private filled evidence and local credentials

### Requirement: Open-source project metadata

The repository SHALL include common public project metadata needed for open-source publication.

#### Scenario: License is present

- **WHEN** the repository is prepared for publication
- **THEN** it contains an explicit license file

#### Scenario: Contribution guidance is present

- **WHEN** a contributor wants to make changes
- **THEN** the repository provides contribution guidance covering tests, OpenSpec workflow, security boundaries, and non-secret evidence handling

### Requirement: Publication safety check

The repository SHALL provide an automated safety check for public-readiness and accidental sensitive file inclusion.

#### Scenario: Required public-readiness files are checked

- **WHEN** the publication safety check runs
- **THEN** it verifies required public files such as README, license, contribution guidance, environment example, production docs, operations docs, and E2E evidence template exist
- **AND** it verifies README contains required public setup and safety topics

#### Scenario: Sensitive tracked or staged files are rejected

- **WHEN** sensitive files such as `.env`, `.env.*` except `.env.example`, private keys, local databases, generated binaries, raw webhook payload captures, filled private evidence, or temporary E2E records are tracked or staged
- **THEN** the publication safety check fails
- **AND** it reports file paths only, not secret values or file contents
