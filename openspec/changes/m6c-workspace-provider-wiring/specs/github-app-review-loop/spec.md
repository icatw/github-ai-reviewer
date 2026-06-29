## ADDED Requirements

### Requirement: Production workspace provider wiring
The production server SHALL wire the safe Go workspace provider into the review service only when workspace checkout is explicitly enabled by validated runtime config.

#### Scenario: Workspace provider is disabled by default
- **WHEN** the service starts without explicit workspace checkout enablement config
- **THEN** production review service construction succeeds without a safe Go workspace provider
- **AND** supported PR review jobs continue through the existing optional Go analyzer skipped path
- **AND** no git clone, fetch, checkout, or credential acquisition is attempted for analyzer workspace setup

#### Scenario: Workspace provider is wired when explicitly enabled
- **WHEN** the service starts with explicit workspace checkout enablement config and valid workspace root settings
- **THEN** production review service construction provides the configured safe Go workspace provider to the optional Go analyzer path
- **AND** the provider remains bounded by existing path, checkout, head validation, cleanup, timeout, and output safety requirements

#### Scenario: Invalid workspace provider config fails startup safely
- **WHEN** workspace checkout is explicitly enabled but required workspace root or safety config is invalid
- **THEN** runtime config validation or service construction fails with a useful non-secret error
- **AND** the error does not include tokens, private keys, webhook secrets, API keys, raw payloads, or private repository content

### Requirement: Checkout-only installation credential provider
The production safe Go workspace provider SHALL acquire GitHub App installation credentials only through a checkout credential provider scoped to the current review job installation, owner, repo, and head SHA.

#### Scenario: Checkout credential is acquired for current job
- **WHEN** a supported PR review job requires checkout for the safe Go workspace provider
- **THEN** the checkout credential provider requests a short-lived GitHub App installation token for the job installation ID
- **AND** the credential is scoped to checkout for the job owner and repo
- **AND** the credential is not reused for unrelated installations, repositories, pull requests, or review jobs

#### Scenario: Credential acquisition failure skips analyzer
- **WHEN** checkout credential acquisition fails because GitHub App auth, token exchange, repository scope, rate limit, or provider availability fails
- **THEN** the safe Go workspace provider rejects workspace setup
- **AND** Go analyzer command execution is skipped with a deterministic safe credential failure category
- **AND** LLM review, finding verification, PR comment reporting, and advisory Check Run reporting continue without static-check evidence from that workspace

#### Scenario: Credential scope mismatch skips analyzer
- **WHEN** a checkout credential cannot be verified as scoped to the current job installation, owner, and repo
- **THEN** the safe Go workspace provider rejects workspace setup
- **AND** Go analyzer command execution is skipped with a deterministic safe credential scope category
- **AND** no checkout command is run with that credential

### Requirement: Safe checkout credential injection
The safe Go workspace provider SHALL inject checkout credentials only through an ephemeral mechanism that keeps tokens out of persisted git state, command plans, logs, analyzer environments, verifier evidence, reporter outputs, and durable storage.

#### Scenario: Git command plans are token-free
- **WHEN** the provider plans git clone, fetch, checkout, remote, or revision validation commands
- **THEN** planned argv, working directories, safe log fields, and command descriptions do not contain installation tokens or checkout credential values
- **AND** remote URLs recorded in plans or persisted git config do not contain installation tokens or checkout credential values

#### Scenario: Checkout environment is not reused for analyzer commands
- **WHEN** checkout requires credential-bearing environment variables, askpass plumbing, credential helper plumbing, or equivalent ephemeral injection
- **THEN** that credential-bearing environment is used only for checkout commands that require it
- **AND** Go analyzer commands receive a separately constructed minimal environment without GitHub installation tokens, checkout credentials, LLM API keys, webhook secrets, private keys, or other service secrets

#### Scenario: Credential injection failure skips analyzer
- **WHEN** ephemeral checkout credential injection cannot be prepared or fails before safe workspace validation
- **THEN** the safe Go workspace provider rejects workspace setup
- **AND** Go analyzer command execution is skipped with a deterministic safe credential injection category
- **AND** the failure metadata does not include installation tokens, checkout credential values, raw git output containing credentials, or private repository code

#### Scenario: Checkout credentials are not reported
- **WHEN** checkout, analyzer execution, verification, comment rendering, Check Run reporting, or safe logging completes, fails, or is skipped
- **THEN** emitted comments, Check Runs, verifier evidence, reporter payloads, logs, metrics, and durable records do not include installation tokens, checkout credential values, tokenized remotes, credential helper payloads, or credential-bearing environment values

### Requirement: Workspace checkout rollout safety
Real repository checkout for optional Go analyzer evidence SHALL remain opt-in, advisory, and safe to deploy disabled.

#### Scenario: Disabled deployment preserves review loop
- **WHEN** production is deployed with workspace checkout disabled
- **THEN** supported PR review jobs still fetch PR metadata and patches, request LLM review, verify findings with available non-workspace evidence, and report advisory output
- **AND** workspace checkout absence is represented only as a safe analyzer limitation or skipped category

#### Scenario: Workspace and credential failures are non-blocking
- **WHEN** workspace provider setup, credential acquisition, credential injection, checkout, head validation, analyzer execution, or cleanup fails
- **THEN** the review job does not fail solely because of that optional workspace or analyzer outcome
- **AND** the Check Run reporter does not set a failure conclusion based on that optional workspace or analyzer outcome
- **AND** no concrete static-check finding is fabricated from the failed optional outcome

#### Scenario: Operations notes document opt-in behavior
- **WHEN** M6c implementation is complete
- **THEN** operator-facing configuration or deployment documentation identifies workspace checkout as disabled by default
- **AND** the documentation states that enabling checkout requires a controlled workspace root, GitHub App installation access, bounded git operations, cleanup monitoring, and secret-free logs
