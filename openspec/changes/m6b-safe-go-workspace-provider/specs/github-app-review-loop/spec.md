## ADDED Requirements

### Requirement: Safe Go workspace provider
The worker SHALL use an explicitly configured safe Go workspace provider before running optional Go analyzer commands, and SHALL preserve the existing safe analyzer skip behavior when no provider is configured or when provider safety checks fail.

#### Scenario: Provider is explicitly gated
- **WHEN** a supported PR review job reaches the optional Go analyzer stage
- **AND** no safe Go workspace provider is configured or enabled
- **THEN** the worker skips Go analyzer command execution
- **AND** the review context records a safe analyzer limitation or omitted-context note
- **AND** the review job continues through LLM review, finding verification, and configured reporters

#### Scenario: Provider returns validated workspace
- **WHEN** a supported PR review job is for a Go project
- **AND** the configured provider creates a workspace that satisfies all path, checkout, credential, and bounded-operation safety checks
- **THEN** the worker may pass the returned `SafeGoWorkspace` to the existing Go analyzer
- **AND** the analyzer may run only the existing fixed `go test ./...` and `go vet ./...` command plans

#### Scenario: Provider failure skips analyzer
- **WHEN** workspace provider setup fails, times out, is unavailable, or rejects the workspace as unsafe
- **THEN** the worker skips Go analyzer command execution
- **AND** the failure is represented by a deterministic safe skip or limitation category
- **AND** the review job continues through LLM review, finding verification, PR comment reporting, and advisory Check Run reporting

### Requirement: Workspace root and path safety
The safe Go workspace provider SHALL create per-job workspaces only under an implementation-controlled temp or cache root and SHALL validate all workspace paths before returning them to the analyzer.

#### Scenario: Workspace root is implementation controlled
- **WHEN** the provider creates a workspace for a review job
- **THEN** the workspace root is under an implementation-controlled temp or cache directory
- **AND** webhook payload fields, repository content, branch names, and user-supplied paths do not determine an absolute workspace root

#### Scenario: Workspace paths are contained
- **WHEN** the provider validates the workspace root, repository checkout path, module working directory, or cleanup target
- **THEN** each path resolves within the implementation-controlled workspace root
- **AND** paths that escape the root through absolute paths, symlinks, traversal, or malformed values are rejected

#### Scenario: Unsafe path skips analyzer
- **WHEN** any workspace path cannot be validated as contained under the implementation-controlled root
- **THEN** the provider does not return a `SafeGoWorkspace`
- **AND** analyzer execution is skipped with a safe path-validation category

### Requirement: PR head pinned checkout
The safe Go workspace provider SHALL checkout or fetch the exact PR head revision for the current review job and validate that the resulting workspace `HEAD` equals the job head SHA before analyzer execution is allowed.

#### Scenario: Exact head SHA is validated
- **WHEN** the provider completes checkout or fetch for a review job
- **THEN** it resolves the workspace `HEAD`
- **AND** it returns a `SafeGoWorkspace` only when the resolved `HEAD` exactly matches `job.HeadSHA`

#### Scenario: Head mismatch skips analyzer
- **WHEN** the resolved workspace `HEAD` is missing or does not exactly match `job.HeadSHA`
- **THEN** the provider rejects the workspace
- **AND** analyzer execution is skipped with a safe checkout-mismatch category

#### Scenario: Git commands are fixed argv
- **WHEN** the provider runs git clone, fetch, checkout, or revision validation commands
- **THEN** each command uses fixed argv forms without shell interpolation
- **AND** untrusted repository names, refs, URLs, or paths are not concatenated into shell command strings

### Requirement: Bounded workspace checkout
The safe Go workspace provider SHALL bound clone, fetch, checkout, and revision validation behavior by timeout, deterministic limits, and shallow or filtered fetch strategy where feasible.

#### Scenario: Checkout operations are bounded
- **WHEN** the provider performs clone, fetch, checkout, or revision validation
- **THEN** each operation is bounded by configured or implementation-defined timeouts
- **AND** fetched history and object scope are shallow or filtered where feasible for exact PR head validation
- **AND** command output captured for diagnostics is bounded and sanitized before any logging or limitation recording

#### Scenario: Bounded checkout failure skips analyzer
- **WHEN** clone, fetch, checkout, or revision validation exceeds a timeout, output limit, or deterministic fetch limit
- **THEN** the provider rejects the workspace
- **AND** analyzer execution is skipped with a safe bounded-checkout category
- **AND** LLM review and configured reporters continue

### Requirement: Workspace credential isolation
The safe Go workspace provider SHALL prevent credentials used for repository checkout from being persisted, logged, or propagated to analyzer command environments.

#### Scenario: Checkout token is not persisted
- **WHEN** the provider needs GitHub installation credentials for clone or fetch
- **THEN** it uses credentials with limited lifetime and repository scope
- **AND** it does not write tokens to git remotes, persisted git config, logs, analyzer evidence, comments, Check Runs, or durable storage

#### Scenario: Analyzer environment excludes checkout secrets
- **WHEN** the Go analyzer executes commands in a safe workspace
- **THEN** the analyzer command environment does not include GitHub installation tokens, checkout credentials, LLM API keys, webhook secrets, private keys, or other service secrets
- **AND** the environment is built independently from any credential-bearing checkout command environment

### Requirement: Workspace cleanup
The worker SHALL attempt to remove each per-job workspace after analyzer execution or after a provider-created workspace is no longer needed, and SHALL record cleanup limitations safely without blocking review output.

#### Scenario: Workspace is cleaned after analyzer
- **WHEN** analyzer execution completes, times out, fails, or is skipped after a workspace was created
- **THEN** the worker or provider attempts to remove the per-job workspace
- **AND** cleanup targets are validated as contained under the implementation-controlled workspace root before removal

#### Scenario: Cleanup limitation is non-blocking
- **WHEN** workspace cleanup fails or can only be partially completed
- **THEN** the worker records a deterministic safe cleanup limitation category
- **AND** the review job continues through finding verification, PR comment reporting, and advisory Check Run reporting
- **AND** logs and reports do not include private repository code, secrets, tokens, or unbounded cleanup output

### Requirement: Workspace provider observability safety
The service SHALL expose only safe aggregate metadata for workspace provider outcomes.

#### Scenario: Provider logs are aggregate only
- **WHEN** workspace provider setup, checkout, validation, analyzer handoff, or cleanup completes, fails, or is skipped
- **THEN** logs or metrics may include aggregate categories such as provider_disabled, checkout_timeout, checkout_failed, head_mismatch, path_invalid, credential_unavailable, cleanup_failed, or workspace_ready
- **AND** logs or metrics do not include raw prompts, raw model output, tokens, secrets, private keys, complete webhook payloads, unbounded analyzer output, persisted checkout credentials, or private repository code

#### Scenario: Advisory Check Run behavior is preserved
- **WHEN** workspace provider setup, checkout, analyzer execution, or cleanup fails for a review job
- **THEN** the Check Run reporter does not set a failure conclusion based on that optional analyzer or workspace-provider outcome
- **AND** review output remains advisory and non-blocking
