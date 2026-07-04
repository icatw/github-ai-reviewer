## Context

The service is past the original M1 loop. It now has multiple review outputs and safety layers: summary issue comments, advisory Check Runs, manual `/ai-review`, repo-aware context, finding verification, optional Go analyzer evidence through a safe workspace provider, batched inline Pull Request Review comments, production operations, and close/merge cleanup. Configuration is still primarily operator-controlled through environment variables, while `.github/ai-review.yml` is currently only treated as lightweight context for prompt and benchmark purposes.

M14 introduces runtime repository-level configuration as a conservative policy overlay. Repository owners can opt out of outputs or tighten review behavior per repository, but global service configuration remains the upper safety boundary controlled by the operator.

## Goals / Non-Goals

**Goals:**

- Discover `.github/ai-review.yml` or `.github/ai-review.yaml` for each supported review job from the PR repository/ref when safe.
- Parse a small typed schema and produce an effective per-job review config.
- Allow repository config to disable review, disable outputs, select supported language, tighten inline limits/thresholds, ignore paths, and disable optional Go analyzer execution.
- Ensure repository config cannot enable globally disabled features or broaden checkout/analyzer behavior.
- Continue reviews on missing or invalid config with safe limitation reporting.
- Update public docs and agent guidance so they describe the current implemented service rather than stale M1 roadmap language.

**Non-Goals:**

- No tenant management, dashboard, billing, durable per-repository settings UI, or database-backed config.
- No inline comment enablement when global `INLINE_COMMENTS_ENABLED` is false.
- No Check Run enablement when global `CHECK_RUN_ENABLED` is false.
- No checkout or Go analyzer enablement when global workspace/analyzer configuration does not permit it.
- No request-changes reviews, auto-fix, auto-merge, merge blocking, or AI-finding-derived failing Check Runs.
- No arbitrary analyzer commands, full repository indexing, vector database, or long-term review memory.

## Decisions

1. Fetch config in the worker after installation authentication and before expensive review work.

   The worker already has the installation-scoped GitHub client and job identity needed to read repository contents. Fetching config there avoids trusting webhook payload file hints and lets `enabled: false` suppress changed-file retrieval, LLM calls, analyzer execution, and reporter fan-out. The fetch should prefer `.github/ai-review.yml`, then `.github/ai-review.yaml`, from the current PR head SHA/ref when available. If the implementation cannot safely read from a fork head with the installation token, it should treat config as unavailable and continue with defaults.

   Alternative considered: fetch config in the webhook handler. Rejected because webhook handlers must return quickly and must not perform GitHub API or policy work beyond safe event parsing.

2. Use a typed parser with tri-state/optional fields, then merge into an effective config.

   Repository config fields need to distinguish "unset" from explicit false or zero-like values. A typed representation with pointer or optional fields avoids accidental default overwrites. After parsing and validation, a merge step combines service defaults, global env config, and repository config into one effective per-job config.

   Alternative considered: parse YAML into generic maps at integration points. Rejected because feature flags and thresholds are safety-sensitive and need deterministic validation and tests.

3. Invalid config falls back to service defaults for the whole repo config file.

   A malformed or invalid config should not fail a PR review because repository owners can break YAML accidentally. Falling back to global defaults keeps the service available and avoids partial application surprises. The worker should record a safe bounded limitation category such as `repo_config_invalid` without exposing raw config content. This is stricter than silently ignoring only the invalid field and simpler to reason about in tests.

   Alternative considered: apply valid fields and ignore invalid fields. Rejected for M14 because mixed partial application can make output behavior hard to explain and may hide unsafe assumptions.

4. Repository config is a tightening overlay, not an operator override.

   Global service configuration remains the maximum allowed behavior. Repository config may turn off summary comments, Check Runs, inline comments, or Go analyzer execution; it may lower inline maximums or raise quality thresholds; it may ignore paths. It must not enable outputs that the operator disabled globally, lower inline quality gates below service defaults, run analyzer/checkout when globally disabled, or change non-blocking policy.

   Alternative considered: let repo owners fully configure all review behavior. Rejected because this service runs as a GitHub App across repositories and the operator owns deployment, secrets, credentials, checkout risk, and permission boundaries.

5. Path ignore filtering should apply before prompt and inline decisions.

   `path_ignore` should suppress matching changed files and context candidates before they reach LLM prompt construction and inline comment eligibility. The implementation should use bounded repository-relative patterns with deterministic behavior and reject absolute, parent-traversing, or unsupported patterns. Omitted paths should be represented as safe metadata without source content.

   Alternative considered: only mention ignored files in the prompt. Rejected because ignored paths are intended to reduce review surface and output noise.

6. Keep repository config documentation close to the user-facing setup path.

   README should include or link an example `.github/ai-review.yml` and explain that missing config is fine, invalid config falls back to defaults, and global env config remains authoritative. AGENTS.md should be updated away from stale M1-only guidance while preserving safety and verification rules.

   Local `config/mcporter.json` is unrelated local tooling metadata. It should be left out of runtime docs and ignored narrowly if present, rather than folded into service config or OpenSpec behavior.

## Risks / Trade-offs

- Repo config fetched from an untrusted PR head can be changed by the PR author -> Mitigation: config can only disable or tighten behavior, never broaden globally disabled capabilities or create blocking output.
- Invalid config fallback may surprise repository owners who expected one field to apply -> Mitigation: report a safe limitation category and document the all-or-default fallback rule.
- Fetching config before changed files adds one or two GitHub content calls per review -> Mitigation: only fetch two fixed paths, prefer primary path, and treat not found as a cheap normal path.
- Path ignore semantics can become complex -> Mitigation: support a bounded first-slice pattern set and reject unsafe patterns rather than implementing a permissive glob language.
- Docs cleanup can drift again -> Mitigation: update publication safety checks or tests where practical to assert README contains current safety topics and does not advertise completed features as future-only.

## Migration Plan

1. Add typed repository config parsing, validation, and effective-config merge tests.
2. Add GitHub content discovery and safe missing/invalid behavior tests with fakes.
3. Apply effective config to review orchestration and reporter/analyzer/inline/path-filter integration points.
4. Update README, AGENTS.md, config docs/examples, and a narrow git ignore entry for `config/mcporter.json` if that file remains local-only.
5. Run `gofmt -w .`, `go test ./...`, `go build ./cmd/server`, and `openspec validate m14-repo-level-ai-review-config --type change --strict`.

Rollback is configuration and code rollback only: without a valid repository config, the service continues using existing global defaults. If M14 must be disabled after deployment, the implementation can ignore repository config discovery behind a service-level kill switch if one is added during implementation; otherwise reverting the feature returns behavior to global-only configuration.

## Open Questions

- Which exact path pattern syntax should `path_ignore` support in M14: Go `path.Match`, gitignore-like patterns, or a deliberately smaller prefix/exact/glob subset?
- Should invalid config limitations appear in the summary comment when summary comments are otherwise enabled, or only in logs/Check Run metadata? The implementation should choose the least noisy safe option and cover it with tests.
- Should `language` initially support only existing service language values such as default English and `zh-CN`, or define a broader allowlist?
