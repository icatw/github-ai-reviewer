## Context

M6a added optional Go analyzer execution that safely skips in production when no workspace provider is configured. M6b added the safe Go workspace provider foundation: provider contract, path containment, fixed git command planning, exact head validation, cleanup handling, validated static-check evidence, and default-disabled config parsing. It intentionally did not wire that provider into `cmd/server` production startup and did not define how GitHub App installation credentials are injected into checkout.

M6c is the next narrow step. Real checkout must remain opt-in, but when an operator explicitly enables it, the production review service should construct the safe workspace provider and pass it into the existing analyzer path. Checkout for private repositories may require a short-lived installation token; that credential must be scoped to the current review job and unavailable to analyzer execution, verifier evidence, reporter output, durable storage, and logs.

## Goals / Non-Goals

**Goals:**

- Wire the safe Go workspace provider into production review service construction only when explicit config enables it.
- Preserve default-disabled behavior and the M6a analyzer skip path when workspace checkout is not configured.
- Define a checkout-only credential provider that obtains a GitHub App installation token for the current review job installation and repository.
- Inject checkout credentials through a short-lived mechanism that does not persist tokens in git config, remotes, logs, analyzer environment, verifier evidence, comments, Check Runs, or storage.
- Add deterministic failure categories for credential acquisition, credential scope mismatch, credential injection failure, checkout failure, and cleanup limitations.
- Keep analyzer evidence advisory and non-blocking.

**Non-Goals:**

- No implementation in the propose phase.
- No real checkout against private repositories or real PRs.
- No general CI runner behavior or configurable command execution.
- No AST, tree-sitter, staticcheck, gosec, semgrep, inline review comments, slash commands, dashboard UI, auto-fix, billing, durable storage, or blocking policy.
- No failing Check Runs based on analyzer findings, analyzer failures, workspace failures, or checkout credential failures.

## Decisions

1. Production wiring is config-gated and nil-provider remains the default.

   `cmd/server` should create the safe Go workspace provider only when an explicit enablement config is true and all required workspace config is valid. Otherwise, the review service receives no workspace provider and follows the existing analyzer skipped limitation path. This makes the feature safe to deploy with disabled defaults and avoids accidentally turning the service into a checkout runner.

   Alternative considered: always construct the provider with runtime no-op behavior. Rejected because explicit nil wiring makes default-disabled behavior easier to test and reduces the chance of hidden checkout attempts.

2. Checkout credentials are acquired per review job, not globally.

   The credential provider should accept job installation ID, owner, repo, and head SHA context and should request an installation token through the existing GitHub App auth path only for checkout. The returned credential handle should be short-lived, non-stringifiable where practical, and limited to the checkout operation. It must not be cached beyond token expiry or shared across unrelated jobs.

   Alternative considered: reuse a process-wide GitHub client token for checkout. Rejected because checkout credentials need a tighter boundary and clearer leak tests than API client construction.

3. Credential injection uses an ephemeral git mechanism with sanitized command plans.

   Git command plans should remain token-free. The provider should use a short-lived injection path such as `GIT_ASKPASS`, credential helper over stdin, or equivalent in-memory command plumbing so the token is not placed in argv, remote URLs, persisted git config, command logs, or analyzer environment. The checkout command environment may contain credential plumbing only for clone/fetch; the analyzer environment is built separately and secret-free.

   Alternative considered: embed the installation token in an HTTPS remote URL. Rejected because tokens can leak through persisted remotes, process listings, shell history, logs, and errors.

4. Credential and checkout failures become analyzer limitations.

   Failures to acquire a token, authorize the repository, inject credentials, clone/fetch, validate `HEAD`, or clean up the workspace should map to deterministic safe categories. These categories may appear in aggregate metrics and limitations, but they do not stop LLM review, finding verification, comment upsert, or advisory Check Run completion.

   Alternative considered: fail the whole review when checkout fails. Rejected because M6 analyzer execution remains optional, and disabled or failed checkout should not degrade the core PR review loop.

5. Reporting and verification receive only sanitized metadata.

   Verifier inputs may include workspace/analyzer limitation categories and bounded static-check evidence from a validated workspace. They must not receive installation tokens, checkout credential handles, raw git command output containing credentials, analyzer secret environment, raw private code, raw prompts, or raw model output. Reporters may summarize limitations and aggregate categories only.

   Alternative considered: include checkout logs for operator debugging in Check Runs. Rejected because Check Runs and comments are PR-facing and can expose private repository or credential details.

## Risks / Trade-offs

- Checkout token leaks through git state, process args, logs, or reports -> Mitigation: token-free git plans, no tokenized remotes, ephemeral credential injection, sanitized failure categories, and tests that assert no known token sentinel appears in command plans, logs, analyzer env, verifier evidence, comments, or Check Run output.
- Operators may enable checkout in an environment that is not isolated enough for untrusted code -> Mitigation: keep real checkout disabled by default, document opt-in deployment expectations, preserve fixed Go commands and timeouts, and keep analyzer advisory.
- Installation token acquisition can fail due to permissions, repository scope, rate limits, or GitHub API outages -> Mitigation: classify failures deterministically and continue review without static-check evidence.
- Cleanup can fail and leave private code on disk -> Mitigation: per-job workspace roots, containment validation before cleanup, safe cleanup limitation categories, and operator metrics/logs without private code content.
- Sanitizing every boundary is easy to regress -> Mitigation: add focused tests with sentinel tokens across config wiring, git plan/log rendering, analyzer env construction, verifier inputs, comment rendering, and Check Run reporter output.

## Migration Plan

- Add M6c config documentation with disabled defaults and explicit opt-in variables for safe workspace checkout.
- Deploy with checkout disabled; behavior should match M6a/M6b safe skip.
- Enable only in a controlled environment after validating workspace root permissions, disk cleanup, GitHub App permissions, and timeout/output bounds.
- Roll back by disabling the workspace provider config; the worker returns to analyzer skipped limitations without changing the core review loop.

## Open Questions

- Which ephemeral credential mechanism should be preferred for the initial implementation (`GIT_ASKPASS`, a temporary credential helper, or stdin-based credential plumbing) given the existing git command runner abstraction?
- Should installation tokens be reused within a single review job across clone and fetch commands, or should each checkout operation request its own token handle?
- What minimal operator metric set is needed for credential failures without exposing repository-specific private details?
