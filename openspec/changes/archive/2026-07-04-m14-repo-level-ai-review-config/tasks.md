## 1. Repository Config Model and Parser

- [x] 1.1 Add a typed repository review config model with optional fields for `enabled`, `language`, `summary_comment.enabled`, `check_run.enabled`, `inline_comments.enabled`, `inline_comments.max_comments`, `inline_comments.severity_threshold`, `inline_comments.confidence_threshold`, `path_ignore`, and `go_analyzer.enabled`.
- [x] 1.2 Add YAML parsing and strict validation for supported fields, supported language values, supported severity values, confidence bounds, max comment bounds, and safe repository-relative `path_ignore` patterns.
- [x] 1.3 Add parser/default tests covering valid config, omitted fields, malformed YAML, invalid types, unknown or unsupported values, out-of-range thresholds, and unsafe path patterns.
- [x] 1.4 Decide and implement the M14 path pattern subset for `path_ignore`, documenting the chosen exact/prefix/glob semantics in code tests.

## 2. Effective Config Merge

- [x] 2.1 Add an effective per-job review config merge path that combines service defaults, global environment config, and validated repository config.
- [x] 2.2 Ensure repository config can disable review, summary comments, Check Runs, inline comments, and optional Go analyzer execution when those features are otherwise globally available.
- [x] 2.3 Ensure repository config cannot enable globally disabled Check Runs, inline comments, optional Go analyzer execution, safe checkout behavior, or any blocking/request-changes/auto-fix/auto-merge policy.
- [x] 2.4 Ensure inline repository settings can only tighten effective output volume and quality gates, including max comments, severity threshold, confidence threshold, required evidence, and RIGHT-side diff mapping.
- [x] 2.5 Add merge tests proving global configuration remains the upper safety boundary and repository config can only disable or tighten behavior.

## 3. GitHub Config Discovery

- [x] 3.1 Add a GitHub client or review-worker boundary for reading fixed repository config paths from the current PR head ref or SHA after installation authentication.
- [x] 3.2 Prefer `.github/ai-review.yml` over `.github/ai-review.yaml` and do not merge both files when both are present.
- [x] 3.3 Treat missing config as normal default behavior without failing the review job.
- [x] 3.4 Treat fetch failures, oversized content, unsupported content encodings, or unsafe fork/ref conditions as safe config-unavailable limitations without exposing raw private config content.
- [x] 3.5 Add fake-client tests for primary discovery, yaml fallback, both-files precedence, missing config, fetch failure, and safe limitation metadata.

## 4. Review Flow Integration

- [x] 4.1 Resolve and apply effective config before changed-file review context fetch, LLM prompt construction, optional analyzer execution, and reporter fan-out.
- [x] 4.2 Suppress downstream review work when effective config has `enabled: false`, including LLM calls, optional analyzers, summary comments, Check Runs, inline Pull Request Reviews, request-changes behavior, auto-fix, auto-merge, and merge blocking.
- [x] 4.3 Apply effective `language` to LLM prompt instructions and fixed bot-rendered labels where the service already supports language customization.
- [x] 4.4 Apply effective summary comment and Check Run settings to reporter fan-out without treating disabled reporters as job failures.
- [x] 4.5 Apply effective inline comment enablement, max comments, severity threshold, and confidence threshold before inline mapping and publication.
- [x] 4.6 Apply effective Go analyzer enablement before optional analyzer/workspace execution while preserving safe skipped-limitation behavior.
- [x] 4.7 Apply effective `path_ignore` filtering to changed files, repository context candidates, and inline comment eligibility where implemented in M14, with safe omitted-context metadata.
- [x] 4.8 Add review flow tests for disabled review, disabled summary comments, disabled Check Runs, disabled inline comments, inline limit/threshold overrides, Go analyzer disablement, missing config, invalid config fallback, and path ignore behavior where implemented.

## 5. Documentation and Project Metadata Cleanup

- [x] 5.1 Update README current capabilities and roadmap so implemented summary comments, advisory Check Runs, manual `/ai-review`, repo-aware context, finding verification, optional Go analyzer/workspace provider, inline PR review comments, production/systemd operations, E2E guidance, and PR close/merge cleanup are not listed as future-only work.
- [x] 5.2 Add README or docs coverage for `.github/ai-review.yml`, including an example, supported fields, missing/invalid behavior, and the global service safety boundary.
- [x] 5.3 Update AGENTS.md current milestone guidance away from stale M1-only instructions while preserving project layout, safety rules, OpenSpec workflow, non-blocking AI policy, and verification commands.
- [x] 5.4 Decide final handling for local `config/mcporter.json`; if it remains local tooling metadata, add a narrow ignore entry without changing runtime config behavior.
- [x] 5.5 Update publication safety checks or related tests if needed so public metadata stays aligned with the current capability set and safety topics.

## 6. Verification

- [x] 6.1 Run `gofmt -w .`.
- [x] 6.2 Run `go test ./...`.
- [x] 6.3 Run `go build ./cmd/server`.
- [x] 6.4 Run `openspec validate m14-repo-level-ai-review-config --type change --strict`.
- [x] 6.5 Review logs, comments, Check Run output, prompt construction tests, and error paths to ensure invalid config and fetch failures do not expose secrets, raw private config content, raw prompts, raw model responses, complete webhook payloads, or unbounded private repository code.
