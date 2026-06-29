## 1. Config and Production Wiring

- [ ] 1.1 Add or finalize explicit runtime config fields for enabling safe Go workspace checkout, workspace root, and required safety bounds with disabled defaults.
- [ ] 1.2 Validate enabled workspace config at startup with useful non-secret errors for missing or unsafe settings.
- [ ] 1.3 Wire `cmd/server` production review service construction to pass no workspace provider when disabled.
- [ ] 1.4 Wire `cmd/server` production review service construction to build and pass the safe Go workspace provider when explicitly enabled.
- [ ] 1.5 Add tests proving disabled-by-default startup does not construct a provider or attempt checkout/credential acquisition.
- [ ] 1.6 Add tests proving explicit enablement wires the provider into the review/analyzer path.

## 2. Checkout Credential Provider

- [ ] 2.1 Define a checkout-only credential provider interface scoped by review job installation ID, owner, repo, and head SHA.
- [ ] 2.2 Implement GitHub App installation token acquisition for checkout through the existing app auth path without durable token storage.
- [ ] 2.3 Enforce current-job repository scope checks before credentials are used for git checkout.
- [ ] 2.4 Map token exchange, permission/scope, rate-limit, and provider availability failures to deterministic safe credential categories.
- [ ] 2.5 Add unit tests for credential acquisition success, auth failure, scope mismatch, and category mapping without real GitHub calls.

## 3. Safe Credential Injection

- [ ] 3.1 Implement ephemeral git checkout credential injection that keeps tokens out of argv, command plans, persisted remotes, persisted git config, and safe logs.
- [ ] 3.2 Ensure checkout credential-bearing environment or helper plumbing is used only for clone/fetch operations that require it.
- [ ] 3.3 Ensure Go analyzer command environments are built independently and exclude GitHub tokens, checkout credentials, LLM API keys, webhook secrets, private keys, and other service secrets.
- [ ] 3.4 Add sentinel-token tests proving git plans, rendered logs, persisted remote/config inspection points, analyzer env, and failure metadata are token-free.

## 4. Non-Blocking Review and Evidence Boundaries

- [ ] 4.1 Route credential acquisition, credential injection, checkout, head validation, analyzer, and cleanup failures into safe analyzer/workspace limitations.
- [ ] 4.2 Ensure those optional failures do not stop LLM review, finding verification, PR comment reporting, or advisory Check Run completion.
- [ ] 4.3 Ensure verifier inputs contain only sanitized static-check evidence and safe limitation categories, never checkout credentials or credential-bearing git metadata.
- [ ] 4.4 Ensure comment and Check Run reporters output only safe categories/counts and never credential values, tokenized remotes, raw checkout logs, raw analyzer output, or private code.
- [ ] 4.5 Add tests covering non-blocking failure behavior and no leakage into verifier, comment renderer, Check Run reporter, logs, metrics, or durable records.

## 5. Operations and Verification

- [ ] 5.1 Document workspace checkout as disabled by default and explicitly opt-in for controlled deployments.
- [ ] 5.2 Document required GitHub App permissions, workspace root safety expectations, cleanup monitoring, timeout/output bounds, and rollback-by-disable behavior.
- [ ] 5.3 Run `gofmt -w .`.
- [ ] 5.4 Run `go test ./...`.
- [ ] 5.5 Run `go build ./cmd/server`.
- [ ] 5.6 Run `openspec validate m6c-workspace-provider-wiring --type change --strict`.
