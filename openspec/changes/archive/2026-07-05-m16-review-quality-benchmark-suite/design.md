## Context

The service already has an offline benchmark path:

- `cmd/review-bench` reads one or more JSON fixtures and runs `review.BuildRepoContext`.
- `cmd/review-bench-from-pr` creates a read-only fixture from a real PR using GitHub App credentials.
- `internal/reviewbench` reports retrieved files, omissions, budget use, `golden_relevant_files`, context precision/recall/F1, source-only metrics, and suite aggregates.
- Fixtures already accept `expected_findings`, but finding quality is not yet evaluated against actual review output.

M16 turns that foundation into a formal review quality benchmark suite. The benchmark remains offline and replayable. It may call the existing review pipeline or load captured structured review output when explicitly requested, but it must not publish comments, create Check Runs, fetch unbounded repository data, or introduce production behavior changes.

## Goals / Non-Goals

**Goals:**

- Support real-PR fixture generation, sanitization, annotation, replay, and suite aggregation.
- Preserve the existing context retrieval benchmark while adding finding quality metrics.
- Make expected no-finding cases first-class so the suite measures noise as well as coverage.
- Report deterministic per-fixture and suite-level finding quality metrics suitable for regression comparison.
- Keep generated private fixtures out of git until intentionally sanitized.
- Keep benchmark reports safe by omitting raw private source content, raw prompts, raw model output, secrets, tokens, and complete webhook payloads.
- Add deterministic tests for decoding, reporting, and aggregate calculations.

**Non-Goals:**

- No dashboard, hosted SaaS, billing, tenant management, or durable job storage.
- No vector database, full repository indexing, tree-sitter call graph, or long-term review memory.
- No request-changes reviews, failing merge gates, auto-merge, auto-fix, or AI-finding-derived blocking policy.
- No production webhook, reporter, Check Run, or inline comment behavior change.
- No arbitrary analyzer or CI command execution beyond existing safe analyzer paths already available to the service.

## Decisions

### Fixture schema evolves in-place with backward compatibility

Add optional fields around the existing fixture format rather than replacing it. Existing fields such as `files`, `repo_files`, `golden_relevant_files`, and `expected_findings` remain valid. New optional metadata can describe fixture provenance, sanitization status, expected no-finding intent, expected finding IDs, categories, severity, matching hints, and low-value or duplicate labels.

Alternative considered: create a separate benchmark fixture schema. That would avoid overloading the current context benchmark, but it would split the replay path and make existing fixtures less useful.

### Finding quality comparison is deterministic and text-bounded

Expected finding matching should use deterministic fields such as file, line or line range, category, severity, stable IDs, title tokens, and evidence hints. The benchmark should not rely on a second LLM as a judge. When model wording changes, fixture annotations should allow maintainers to update matching hints explicitly.

Alternative considered: use LLM-as-judge quality scoring. That may be useful later, but it is too non-deterministic for a regression gate and creates extra private-content handling risk.

### No-finding cases are explicit

A fixture may state that no findings are expected. Any verified finding in that case becomes unexpected output and contributes to false-positive or low-value reporting depending on classification.

Alternative considered: infer no-finding intent from an empty `expected_findings` list. That is ambiguous because many existing fixtures focus only on context retrieval.

### Reports prioritize safe summaries over raw evidence

Benchmark output may include fixture names, IDs, file paths, line numbers, metric counts, category counts, and sanitized finding labels. It must not include raw private source content, raw prompts, raw model responses, secrets, tokens, private keys, API keys, installation tokens, or complete webhook payloads.

Alternative considered: include snippets to ease debugging. Snippets are useful for public fixtures, but the default report format must be safe for private PR workflows and CI logs.

### Private fixture workflow uses quarantine before sanitization

`review-bench-from-pr` should continue writing generated real-PR fixtures to an operator-selected path. Documentation and safeguards should steer private fixtures to ignored local paths such as `/tmp`, `data/`, or another gitignored quarantine directory until they are sanitized and intentionally moved into `testdata`.

Alternative considered: write directly under `testdata/review-bench`. That is convenient but increases the risk of committing private code.

## Risks / Trade-offs

- Private fixture leakage -> Mitigate with documentation, gitignore coverage, explicit sanitization metadata, report redaction, and generated-private-fixture warnings.
- Matching expected findings is too brittle -> Mitigate with stable expected IDs, file/line ranges, category/severity, and evidence hint matching rather than exact full-title matching.
- Low-value classification is subjective -> Mitigate by starting with deterministic labels and counts, such as duplicate-of expected ID, style-only, unsupported, too-generic, or expected-no-finding violation.
- Suite metrics hide per-case regressions -> Mitigate by reporting both aggregate metrics and per-fixture missed/unexpected lists.
- Benchmark accidentally changes production behavior -> Mitigate by keeping changes inside `cmd/review-bench`, `cmd/review-bench-from-pr`, `internal/reviewbench`, fixtures, docs, and tests.

## Migration Plan

Existing fixtures must continue to decode and run as context-only fixtures. New finding-quality fields are optional. Implementation can add new sample fixtures or update existing public fixtures after preserving their current context retrieval assertions.

Rollback is source-only: remove the benchmark schema/reporting additions and continue using the current context-only benchmark commands. No runtime migration is required.

## Open Questions

- Should the first implementation compare actual findings from a captured JSON review result, from a live LLM invocation, or from the existing review pipeline behind an explicit benchmark flag?
- Which low-value labels should be required in M16 versus reserved for future manual annotation?
- Should CI enforce thresholds immediately, or should M16 first emit stable reports and leave threshold policy to a later change?
