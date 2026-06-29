## 1. Provider Contract and Configuration Gate

- [ ] 1.1 Inspect the existing M6a `GoWorkspaceProvider`, `SafeGoWorkspace`, worker analyzer wiring, and skip categories to identify the narrow provider insertion point.
- [ ] 1.2 Define or extend provider configuration so production continues to skip analyzer execution unless a safe workspace provider is explicitly enabled.
- [ ] 1.3 Add provider result categories for disabled, unavailable, checkout failed, checkout timeout, head mismatch, path invalid, credential unavailable, workspace ready, and cleanup failed.
- [ ] 1.4 Add tests proving no configured provider preserves the M6a safe analyzer skip path and does not block LLM review, verifier execution, comments, or advisory Check Runs.

## 2. Safe Workspace Root and Path Validation

- [ ] 2.1 Implement per-job workspace directory creation under an implementation-controlled temp or cache root using sanitized service-controlled identifiers.
- [ ] 2.2 Implement canonical containment validation for workspace root, repository checkout path, module working directory, and cleanup target.
- [ ] 2.3 Reject absolute, traversal, malformed, symlink-escaped, and root-escaping paths before returning a `SafeGoWorkspace`.
- [ ] 2.4 Add tests for valid containment, traversal rejection, symlink escape rejection, malformed path rejection, and safe cleanup target validation.

## 3. Bounded Git Checkout

- [ ] 3.1 Implement fixed-argv git clone/fetch/checkout/revision commands with no shell interpolation.
- [ ] 3.2 Bound checkout operations with configured or implementation-defined timeouts, output limits, and shallow or filtered fetch behavior where feasible.
- [ ] 3.3 Fetch or checkout the exact review job PR head ref/SHA and validate resolved workspace `HEAD` equals `job.HeadSHA`.
- [ ] 3.4 Add tests for exact head match, head mismatch, checkout timeout, checkout command failure, bounded output handling, and fixed argv command construction.

## 4. Credential Isolation

- [ ] 4.1 Use short-lived repository-scoped GitHub installation credentials for checkout only when required.
- [ ] 4.2 Ensure checkout credentials are not written to remotes, persisted git config, logs, analyzer evidence, comments, Check Runs, or durable storage.
- [ ] 4.3 Build analyzer command environments independently from checkout environments and exclude installation tokens, checkout credentials, LLM API keys, webhook secrets, private keys, and service secrets.
- [ ] 4.4 Add tests for token-free remote configuration, secret-free analyzer environment, safe credential failure categories, and redacted provider logging.

## 5. Cleanup and Non-Blocking Behavior

- [ ] 5.1 Attempt per-job workspace cleanup after analyzer completion, timeout, failure, or post-creation skip.
- [ ] 5.2 Validate cleanup targets before removal and record cleanup failures as safe limitations or aggregate operational categories.
- [ ] 5.3 Ensure provider setup failures, checkout failures, head mismatches, path rejections, credential failures, analyzer skips, and cleanup limitations do not block LLM review, verifier execution, comment reporter output, or advisory Check Run completion.
- [ ] 5.4 Add worker/reporter tests covering non-blocking provider failures and preserved advisory Check Run conclusions.

## 6. Verifier Evidence and Metrics

- [ ] 6.1 Ensure static-check evidence is passed to the verifier only when produced from a validated `SafeGoWorkspace` whose `HEAD` matched the current job head SHA.
- [ ] 6.2 Represent provider disabled, unavailable, timeout, checkout failure, head mismatch, path invalid, credential failure, and cleanup limitation outcomes as safe limitations or aggregate categories without fabricating concrete evidence.
- [ ] 6.3 Extend aggregate verification/provider stats without including raw analyzer output, private repository code, raw prompts, model output, secrets, tokens, private keys, complete webhook payloads, or checkout credentials.
- [ ] 6.4 Add verifier tests for valid workspace evidence, unvalidated workspace evidence ignored, provider limitation categories, cleanup limitation handling, and aggregate-only metrics.

## 7. Final Verification

- [ ] 7.1 Run `gofmt -w .`.
- [ ] 7.2 Run `go test ./...`.
- [ ] 7.3 Run `go build ./cmd/server`.
- [ ] 7.4 Run `openspec validate m6b-safe-go-workspace-provider --type change --strict`.
- [ ] 7.5 Review implementation scope for no AST/tree-sitter, no staticcheck/gosec/semgrep, no inline comments, no slash commands, no durable storage, no blocking policy, no dashboard, no product UI, no auto-fix, safe logging, secret isolation, bounded checkout behavior, cleanup handling, and preserved advisory/non-blocking behavior.
