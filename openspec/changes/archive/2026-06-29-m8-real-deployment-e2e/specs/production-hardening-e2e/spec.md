## ADDED Requirements

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
