## Context

M6a added an optional Go analyzer stage, command planning for `go test ./...` and `go vet ./...`, analyzer output parsing/sanitization, verifier static-check evidence, and a production-safe skip when no workspace provider is configured. M6b is the next narrow slice: provide a safe local workspace only when the service can checkout the exact PR head under strict constraints.

PR code is untrusted. A workspace provider can expose private repository code locally, invoke Git credentials, and enable later analyzer execution. The provider therefore must be explicitly gated, use implementation-controlled paths, validate the checked-out revision, avoid leaking tokens into analyzer environments, and fail closed into the existing M6a safe-skip behavior.

## Goals / Non-Goals

**Goals:**

- Add a safe Go workspace provider contract that can supply `SafeGoWorkspace` only for the current review job's PR head.
- Use an implementation-controlled temp/cache root and validate every returned path is contained under that root.
- Checkout/fetch using fixed git argv, bounded timeouts, shallow or filtered fetch behavior where feasible, and deterministic limits.
- Validate the resulting workspace `HEAD` exactly matches `job.HeadSHA` before analyzer execution is allowed.
- Keep checkout credentials short-lived and prevent them from being written to remotes, persisted config, logs, or the analyzer command environment.
- Remove each per-job workspace after analyzer execution, and record safe cleanup limitations without blocking review.
- Preserve M6a non-blocking analyzer skip/failure categories, LLM review, finding verification, PR comment behavior, and advisory Check Runs.

**Non-Goals:**

- No AST, tree-sitter, staticcheck, gosec, semgrep, arbitrary commands, configurable command execution, inline comments, slash commands, durable storage, blocking policy, dashboard, product UI, or auto-fix.
- No full repository indexing or long-lived local mirror requirement.
- No failing Check Runs based on analyzer results, analyzer command failures, or AI findings.
- No exposure of raw prompts, raw model output, tokens, private keys, complete webhook payloads, unbounded analyzer output, or private repository code in logs or reports.

## Decisions

1. Make the workspace provider explicitly configured and fail closed.

   The worker should keep the M6a default production-safe skip when no provider is configured or when provider safety checks fail. This avoids turning analyzer execution into an implicit CI runner.

   Alternative considered: always checkout repositories for Go projects. Rejected because PR code is untrusted and some deployments may not have a hardened local workspace environment.

2. Create per-job workspaces under an implementation-controlled root.

   The provider should derive per-job directories from safe service-controlled identifiers such as delivery ID, owner, repo, pull number, and head SHA after sanitization. The root must be configured or created by the implementation, not provided by webhook payloads or repository content. Every workspace, module working directory, and cleanup target must be validated with canonical path containment checks before use.

   Alternative considered: accept a repository path from configuration or webhook metadata. Rejected because user-supplied or repo-supplied paths could escape expected boundaries.

3. Checkout the exact PR head and verify it.

   Git operations should use fixed argv with no shell interpolation. The provider should fetch only the required PR head ref/SHA using bounded timeout and shallow or filtered fetch behavior where feasible. Before returning `SafeGoWorkspace`, it must resolve workspace `HEAD` and require exact equality with `job.HeadSHA`.

   Alternative considered: checkout a branch name and trust it. Rejected because branch heads can move and do not prove the workspace matches the reviewed job.

4. Keep checkout credentials out of analyzer execution.

   If checkout requires an installation token, use it only for clone/fetch through a short-lived path such as askpass or in-memory command input. The token must not be written into `origin` remote URLs, persisted git config, logs, or environment variables passed to `go test` or `go vet`. Analyzer execution receives a minimal secret-free environment as defined by M6a.

   Alternative considered: embed the token in the clone URL. Rejected because it risks leaking into remotes, process listings, logs, or persisted config.

5. Treat workspace-provider failures as analyzer skips or limitations.

   Clone/fetch timeout, checkout mismatch, path validation failure, credential failure, cleanup failure, and provider internal errors should map to safe deterministic categories. They must not stop LLM review, finding verification, comment reporting, or advisory Check Run completion. Cleanup failures are limitations to record and operational events to observe safely, not review failures.

   Alternative considered: fail the review job when workspace setup fails. Rejected because the analyzer is optional and M6b must preserve non-blocking review behavior.

## Risks / Trade-offs

- Running Go tooling can execute untrusted code -> Mitigation: M6b only supplies a workspace after strict path, checkout, credential, and bounded-operation checks; M6a still controls fixed commands, timeout, output limits, minimal env, and non-blocking results.
- Git credentials can leak through remotes, logs, process args, or analyzer env -> Mitigation: avoid tokenized remote URLs, avoid persisted credential config, redact logs by category, and construct analyzer env independently from checkout env.
- Cleanup can fail and leave private code on disk -> Mitigation: use per-job directories under a controlled root, validate cleanup targets, attempt cleanup after analyzer execution, and record bounded safe cleanup limitation categories for operator follow-up.
- Shallow or filtered fetch may not work for every repository/server state -> Mitigation: treat bounded fetch failure as analyzer skipped/unavailable and continue review without static-check evidence.
- Exact head validation can reject legitimate but unusual refs -> Mitigation: fail closed into safe skip and record the mismatch category without blocking the review.
