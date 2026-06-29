## ADDED Requirements

### Requirement: Checkout credential boundary in verification
The verifier SHALL accept only sanitized workspace/analyzer evidence and limitation categories, never checkout credentials or credential-bearing git metadata.

#### Scenario: Credential failure is represented as limitation only
- **WHEN** checkout credential acquisition, scope validation, or ephemeral injection fails before analyzer execution
- **THEN** the verifier may receive a safe limitation or omitted-context note for the current review job
- **AND** the verifier does not receive installation tokens, checkout credential values, tokenized remote URLs, credential helper payloads, credential-bearing environment values, or raw git output containing credentials
- **AND** no concrete static-check evidence item is fabricated from the credential failure

#### Scenario: Verified static-check evidence remains credential-free
- **WHEN** the Go analyzer produces bounded static-check evidence from a validated safe workspace
- **THEN** verifier evidence identifies only safe static-check fields such as tool name, exit category, package, file, line, sanitized message, and limitation categories
- **AND** verifier evidence does not include installation tokens, checkout credential values, tokenized remotes, persisted git config, credential helper payloads, analyzer secret environment, or raw checkout logs

### Requirement: Credential-safe verification metrics
The verifier SHALL expose only safe aggregate metrics for checkout credential and workspace-provider outcomes.

#### Scenario: Credential metrics are aggregate only
- **WHEN** finding verification completes with workspace-provider credential outcomes or limitations
- **THEN** stats may include aggregate counts for credential acquisition failures, credential scope failures, credential injection failures, workspace-ready evidence, provider skips, checkout mismatches, checkout timeouts, cleanup limitations, and deterministic reason categories
- **AND** stats do not include raw analyzer stdout, raw analyzer stderr, raw git output, private repository code, raw prompts, raw model output, secrets, tokens, private keys, API keys, complete webhook payloads, installation tokens, checkout credentials, tokenized remotes, or credential-bearing environment values

#### Scenario: Reporter outputs remain credential-free
- **WHEN** verification stats or limitations are passed to comment rendering, Check Run reporting, logs, metrics, or durable records
- **THEN** downstream outputs contain only safe categories and aggregate counts
- **AND** downstream outputs do not include installation tokens, checkout credential values, tokenized remotes, credential helper payloads, credential-bearing environment values, raw checkout logs, raw analyzer output, or private repository code
