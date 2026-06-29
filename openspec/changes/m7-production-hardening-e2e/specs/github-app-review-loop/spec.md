## ADDED Requirements

### Requirement: Production startup documentation alignment
The GitHub App review loop SHALL have production-facing configuration examples and documentation aligned with the runtime config required to start the service safely.

#### Scenario: Documented required config matches startup validation
- **WHEN** the service requires an environment variable to start the production review loop
- **THEN** the production docs or `.env.example` identify that setting by name
- **AND** startup validation reports missing required settings without printing configured secret values

#### Scenario: Dummy config path does not perform downstream work
- **WHEN** a local smoke path starts the service with dummy non-secret config for health checking
- **THEN** the service does not exchange installation tokens, fetch pull request files, call an LLM, clone a repository, publish PR comments, or create Check Runs until a valid signed webhook and worker path are exercised

### Requirement: Production reporter safety
The GitHub App review loop SHALL preserve advisory, non-blocking reporter behavior in production hardening paths.

#### Scenario: Comment upsert remains stable in E2E verification
- **WHEN** repeated supported pull request events are processed for the same PR
- **THEN** the comment reporter updates the existing marker-identified AI review comment when present
- **AND** it does not create duplicate bot review comments for normal repeated review events

#### Scenario: Check Run findings remain advisory
- **WHEN** a completed structured review result contains AI findings of any allowed severity
- **THEN** the Check Run reporter does not derive a blocking failure conclusion from those findings
- **AND** production docs and E2E verification describe the Check Run as advisory unless infrastructure or job execution fails

#### Scenario: Reporter failure output remains safe
- **WHEN** comment or Check Run reporting fails during review processing
- **THEN** the worker records or logs safe failure metadata identifying the reporter and category
- **AND** no PR-facing output or log under service control includes secrets, installation tokens, checkout credentials, private keys, API keys, complete webhook payloads, raw prompts, raw model responses, or unbounded private code

### Requirement: Production workspace checkout safety
The GitHub App review loop SHALL keep real workspace checkout optional, disabled by default, and bounded when explicitly enabled.

#### Scenario: Checkout disabled uses existing skip path
- **WHEN** production config does not explicitly enable workspace checkout
- **THEN** the worker does not clone or fetch repository contents
- **AND** analyzer-dependent evidence is represented as a safe skipped limitation when relevant

#### Scenario: Checkout enablement requires hardening config
- **WHEN** production config explicitly enables workspace checkout
- **THEN** startup validation requires the configured workspace root and timeout/output bounds needed by the safe workspace provider
- **AND** invalid workspace configuration fails startup without printing secrets or private-code paths beyond safe setting names

#### Scenario: Checkout rollback returns to no-checkout behavior
- **WHEN** an operator disables workspace checkout after it was previously enabled
- **THEN** subsequent review jobs do not perform real checkout
- **AND** the core webhook, LLM review, marker comment upsert, and advisory Check Run paths continue according to their own enabled configuration
