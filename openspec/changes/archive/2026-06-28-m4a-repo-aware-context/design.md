## Context

The current review loop accepts supported PR webhooks quickly, processes review jobs in the worker path, fetches PR changed files and patches, asks the LLM for a structured `ReviewResult`, and publishes advisory output through the existing comment/check reporters. This keeps M1-M3 behavior stable but gives the LLM limited context for changed code and nearby tests.

M4a adds deterministic repo-aware context gathering inside review processing, after installation-authenticated GitHub access is available and before prompt construction. The feature must remain bounded, predictable, and safe for private repositories: no raw prompts in logs, no broad repository indexing, no new webhook latency, and no durable storage requirement.

## Goals / Non-Goals

**Goals:**

- Preserve PR metadata and changed-file patch context in review prompts.
- Add bounded full head-version content for changed files when files are safe, textual, not deleted, and within budget.
- Add bounded related test context using simple file naming conventions.
- Add lightweight repo docs/config context from `README.md`, limited `docs/*.md`, and `.github/ai-review.yml`.
- Add stable omitted-context reporting for filtered, missing, truncated, oversized, and budget-limited inputs.
- Keep prompt structure deterministic and testable.
- Keep structured `ReviewResult` as the only LLM output boundary.

**Non-Goals:**

- AST, tree-sitter, symbol graph, call graph, vector search, or full repository indexing.
- Slash commands, `issue_comment` webhook handling, inline review comments, request-changes behavior, or merge-blocking policy.
- Durable context storage or storing prompts/model responses.
- Changing the existing comment marker upsert or advisory Check Run semantics.

## Decisions

1. **Add a review context builder between GitHub fetching and prompt rendering.**

   The worker will continue to own downstream review work. A deterministic context builder will receive PR metadata, changed file metadata/patches, and a GitHub content reader scoped to the PR head SHA. It will return structured prompt context with stable sections: `patch_context`, `full_file_context`, `related_test_context`, `repo_docs_context`, and `omitted_context`.

   Alternative considered: embed context fetching directly inside the LLM client. That would couple provider-specific prompt rendering with GitHub repository traversal and make budget/filter tests harder.

2. **Use conservative content filters before fetching or including context.**

   The builder will skip deleted files, binary files, generated files, lock files, vendor/dist paths, and files exceeding implementation-defined size limits. Binary/generated detection should use deterministic path, extension, metadata, and content heuristics available in the existing codebase; failures to fetch or classify content become omitted-context notes instead of job failures unless the original PR changed-files fetch fails.

   Alternative considered: attempt best-effort inclusion of every changed file. That increases prompt noise, token cost, and risk of unreadable generated/vendor content.

3. **Budgeting is deterministic and centrally testable.**

   M4a will define implementation-level constants or configuration for per-file and total context budgets. Patch context remains bounded. Full changed files, related tests, and docs/config context each consume from a total context budget in a stable order. When the budget is exhausted, later candidates are skipped or truncated with omitted-context entries.

   Alternative considered: rely on provider token counting. That would be less deterministic, require provider-specific accounting, and complicate unit tests.

4. **Related tests use naming conventions only.**

   For a changed source file, the builder will look for direct paired tests such as `foo.go` to `foo_test.go`. For Go packages, it may also include bounded same-package `*_test.go` files when doing so stays within candidate and context budgets. Candidate order must be stable and duplicate paths must be removed.

   Alternative considered: parse imports or symbols to discover tests. That belongs to later repository intelligence milestones, not M4a.

5. **Lightweight docs/config context is allowlisted.**

   The builder will look for `README.md`, a bounded stable list of `docs/*.md`, and `.github/ai-review.yml`. Missing files are not errors. Existing repo config may be included as context but M4a does not require new config semantics.

   Alternative considered: recursive docs traversal. That risks unbounded context and surprising private repository exposure to the LLM.

6. **Omitted context is part of the prompt contract, not a log dump.**

   The prompt will include concise omitted-context notes that identify path, category, and reason such as `deleted`, `binary`, `generated`, `lock_file`, `vendor_or_dist`, `oversized`, `truncated`, `budget_exhausted`, `missing`, or `fetch_error`. Notes must not include secrets, raw payloads, tokens, raw prompts, or raw model responses.

   Alternative considered: only log omitted context. The LLM then cannot accurately state limitations and may overstate confidence.

## Risks / Trade-offs

- **Risk: Larger prompts increase cost and latency.** -> Mitigation: enforce per-file and total budgets, deterministic ordering, and truncation/omission notes.
- **Risk: Extra GitHub content calls hit rate limits.** -> Mitigation: skip unsafe candidates before fetching where possible, bound same-package test discovery, and treat non-critical fetch failures as omissions.
- **Risk: Omitted context exposes private repository path names.** -> Mitigation: include only concise repository-relative paths already relevant to the PR/repo context, and never log raw prompt bodies or secrets.
- **Risk: Simple related-test matching misses important tests.** -> Mitigation: report limitations and leave semantic discovery to later AST/indexing milestones.
- **Risk: Budget order biases docs or tests.** -> Mitigation: define stable section and candidate ordering, then cover total-budget behavior with unit tests.

## Migration Plan

Implementation can be deployed as an internal review pipeline change with no database migration. Existing webhook behavior, review result validation, comment marker upsert, and Check Run policy remain unchanged.

Roll back by disabling or removing the repo-aware context builder from prompt construction, returning prompts to PR metadata plus patch context. The GitHub App already needs contents read access for repository content fetching.

After implementation, run standard build/test checks, validate the OpenSpec change, restart the deployed service, and verify a real PR shows richer context or explicit limitations while preserving existing PR comment and Check Run behavior.
