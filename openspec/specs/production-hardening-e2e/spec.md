# production-hardening-e2e Specification

## Purpose
TBD - created by archiving change m7-production-hardening-e2e. Update Purpose after archive.
## Requirements
### Requirement: Production configuration example
The project SHALL provide a safe `.env.example` and production configuration documentation for the GitHub App review service.

#### Scenario: Example config contains required settings without secrets
- **WHEN** an operator reads the example configuration
- **THEN** it lists the required server, GitHub App, webhook, LLM, reporter, and workspace-related environment variables
- **AND** it uses placeholder or dummy values rather than real secrets, private keys, installation tokens, webhook secrets, LLM API keys, or private repository identifiers

#### Scenario: Optional checkout config is disabled by default
- **WHEN** an operator follows the example configuration without changing optional workspace checkout settings
- **THEN** real repository checkout remains disabled
- **AND** analyzer evidence that depends on checkout is skipped safely rather than attempted

#### Scenario: Missing production config fails safely
- **WHEN** required production settings are missing at startup
- **THEN** config validation fails with useful missing-setting names
- **AND** the error output does not include secret values, private key material, installation tokens, API keys, webhook payloads, raw prompts, or raw model responses

### Requirement: Deployment hardening documentation
The project SHALL document production hardening guidance for deploying the GitHub App review service.

#### Scenario: GitHub App setup is documented
- **WHEN** an operator prepares the GitHub App
- **THEN** documentation lists the required webhook event, supported pull request actions, webhook secret requirement, and minimum repository permissions for comments and advisory Check Runs
- **AND** it distinguishes permissions needed for PR conversation comments from permissions needed for Check Run reporting

#### Scenario: Workspace root hardening is documented
- **WHEN** an operator opts into real workspace checkout
- **THEN** documentation requires an explicit workspace root outside the repository source tree
- **AND** it describes restrictive filesystem permissions, path containment, bounded disk usage, timeout/output limits, cleanup monitoring, and private-code retention risk

#### Scenario: Rollback by disable is documented
- **WHEN** an operator needs to reduce risk after deployment
- **THEN** documentation explains how to disable workspace checkout and return to the analyzer-skipped advisory path
- **AND** it explains how to stop webhook delivery or disable reporter outputs without deleting committed code or exposing secrets

### Requirement: Local smoke testing without real credentials
The project SHALL provide local smoke-test scripts or documented commands that verify safe startup and health behavior without requiring real GitHub, LLM, checkout, or production credentials.

#### Scenario: Dummy local smoke test starts server
- **WHEN** a developer runs the documented local smoke path with dummy non-secret configuration
- **THEN** the service can be built and started far enough to exercise `/healthz`
- **AND** the smoke path does not call GitHub APIs, call an LLM provider, clone a repository, or publish a PR comment

#### Scenario: Health check smoke test succeeds
- **WHEN** the locally started service receives `GET /healthz`
- **THEN** it returns a successful response
- **AND** the response does not include secrets, private key paths with sensitive material, installation tokens, API keys, webhook payloads, raw prompts, raw model responses, or private code

#### Scenario: Smoke test failure output is safe
- **WHEN** a smoke-test script fails because a port is occupied, dummy config is invalid, or the server exits early
- **THEN** the script output identifies the failing step
- **AND** it does not print secret values, private key contents, installation tokens, API keys, full webhook payloads, raw prompts, raw model responses, or private repository code

### Requirement: Real deployment E2E verification checklist
The project SHALL provide a real deployment/E2E verification checklist for proving the GitHub App review loop under live GitHub conditions.

#### Scenario: Webhook delivery is verified
- **WHEN** an operator performs the E2E checklist against a real installed GitHub App and test repository
- **THEN** the checklist verifies that GitHub reports successful `pull_request` webhook delivery for `opened`, `synchronize`, or `reopened`
- **AND** the service records only safe delivery/job metadata rather than raw complete webhook payloads

#### Scenario: PR output is verified
- **WHEN** a supported PR event completes review processing
- **THEN** the checklist verifies that the PR conversation contains one marker-identified AI review comment
- **AND** a subsequent supported event updates the marker comment rather than creating duplicate bot review comments

#### Scenario: Advisory Check Run is verified
- **WHEN** Check Run reporting is enabled for the E2E repository
- **THEN** the checklist verifies an `AI Review` Check Run appears on the PR head SHA
- **AND** completed review findings result in advisory `success` or `neutral` behavior rather than merge-blocking failure derived from AI findings

#### Scenario: Leak review is verified
- **WHEN** E2E verification is complete
- **THEN** the checklist requires review of logs, PR comments, and Check Run output for absence of installation tokens, private keys, webhook secrets, LLM API keys, checkout credentials, raw prompts, raw model responses, complete webhook payloads, and unbounded private code

### Requirement: Production safety regression checks
The project SHALL include deterministic tests or script checks for production safety boundaries that are practical to verify locally.

#### Scenario: Secret redaction is covered
- **WHEN** tests exercise startup config errors, safe logging helpers, rendered PR-facing output, or reporter failure output with sentinel secret values
- **THEN** the sentinel values do not appear in returned errors, logs under test, rendered comments, Check Run output, or script output

#### Scenario: Ignore coverage is covered
- **WHEN** tests or script checks inspect repository ignore rules
- **THEN** `.env`, private key files, generated server binaries, local data directories, and local database files are ignored
- **AND** the check does not require those sensitive files to exist

#### Scenario: Reporter non-blocking behavior is covered
- **WHEN** a reporter fails while another reporter can still run or the core review result remains advisory
- **THEN** tests verify the failure is recorded safely
- **AND** AI finding severity alone does not cause a blocking Check Run conclusion or duplicate fallback comment creation

### Requirement: Executed real deployment E2E evidence
The project SHALL record a safe evidence trail for a real GitHub App deployment E2E verification against a test repository.

#### Scenario: E2E evidence template is safe
- **WHEN** an operator records real deployment E2E results
- **THEN** the evidence format captures deployment health, GitHub delivery metadata, supported PR action coverage, PR comment upsert results, Check Run results, and leak-review status
- **AND** it uses redacted or placeholder values for secrets, private keys, installation tokens, LLM API keys, webhook secrets, raw webhook payloads, raw prompts, raw model responses, checkout credentials, and private source

#### Scenario: Deployed health is verified
- **WHEN** M8 E2E begins against the deployed service
- **THEN** the deployed `GET /healthz` endpoint is checked and recorded
- **AND** the health output does not expose secrets or dependency-specific credential details

#### Scenario: Supported live PR events are verified
- **WHEN** the test GitHub App receives live `pull_request` events for a test repository
- **THEN** the E2E record captures safe evidence for `opened` and `synchronize` handling
- **AND** it captures `reopened` handling when practical or records a safe reason if `reopened` could not be exercised in the E2E window

#### Scenario: Marker comment upsert is evidenced
- **WHEN** multiple supported PR events run for the same test PR
- **THEN** the E2E record shows the marker-identified AI review comment is updated rather than duplicated
- **AND** it does not include the full raw PR body, raw model response, or private source beyond the intended sanitized review output

#### Scenario: Advisory Check Run is evidenced
- **WHEN** Check Run reporting is enabled during E2E
- **THEN** the E2E record shows the `AI Review` Check Run on the PR head SHA
- **AND** completed AI findings result in advisory `neutral` or `success` behavior rather than a merge-blocking failure derived from AI finding severity

#### Scenario: Leak review is evidenced
- **WHEN** E2E completes or fails
- **THEN** the E2E record includes a leak-review result for service logs, PR comments, and Check Run output
- **AND** the record confirms absence of installation tokens, private keys, webhook secrets, LLM API keys, checkout credentials, raw prompts, raw model responses, complete webhook payloads, and unintended private source excerpts

### Requirement: Bounded E2E fix loop
The project SHALL constrain fixes found during real deployment E2E to the minimum changes required for safe verification completion.

#### Scenario: E2E issue records are safe
- **WHEN** live E2E exposes a bug or operational gap
- **THEN** the issue record captures safe symptoms, suspected component, reproduction outline, and verification status
- **AND** it omits secrets, raw payloads, raw prompts, raw model responses, tokens, private keys, and private source

#### Scenario: E2E fixes stay within M8 scope
- **WHEN** a code, script, or documentation fix is needed to complete E2E safely
- **THEN** the fix is limited to deployment verification, safe output, reporter behavior, config validation, or documentation needed by the E2E loop
- **AND** it does not introduce default workspace checkout, arbitrary CI execution, new analyzer commands, inline comments, slash commands, dashboard, billing, or AI-finding-derived blocking policy

#### Scenario: E2E fixes are verified
- **WHEN** a code fix is applied during M8
- **THEN** deterministic tests are added or updated where practical
- **AND** local verification and the affected real E2E step are rerun before marking the task complete

