## Context

M5a introduced a verifier that runs after structured `ReviewResult` parsing and before reporter fan-out. It builds an in-memory evidence index from bounded `RepoContext`, verifies each finding, and returns kept, downgraded, or dropped outcomes with safe reason-category counts.

The current matching boundary is intentionally simple: exact or normalized substring matching against available evidence. That blocks many hallucinations, but it can also drop useful findings when the model paraphrases a concrete code issue or quotes a short but meaningful snippet. M5b improves this boundary without adding AST analysis, static analyzers, full repository indexing, vector search, or any blocking product policy.

## Goals / Non-Goals

**Goals:**

- Expand deterministic verifier eval coverage over realistic PR evidence shapes.
- Improve conservative evidence matching for paraphrases and snippets while preventing generic text from supporting findings.
- Keep false-positive prevention explicit for unrelated docs/config evidence and generic wording.
- Emit only safe aggregate verifier metadata and eval summaries.
- Preserve existing reporter fan-out, stable comment marker upsert, advisory Check Run behavior, and output suppression.

**Non-Goals:**

- No AST, tree-sitter, call graph, symbol index, vector database, durable storage, or full repository indexing.
- No inline review comments, slash commands, issue_comment webhook handling, dashboard, UI, billing, or tenant features.
- No `go test`, `go vet`, `staticcheck`, `gosec`, `semgrep`, or other analyzer execution.
- No request-changes behavior, failing Check Runs from AI findings, or merge-blocking policy.

## Decisions

### Layered deterministic matching

Evidence matching will stay deterministic and conservative. The verifier should attempt support checks in layers:

1. Exact and normalized substring matching for clear quoted evidence.
2. Normalized snippet matching for code fragments after whitespace, punctuation, and casing normalization.
3. Token overlap matching with thresholds that require meaningful code or domain tokens, not only common words.
4. Identifier-aware matching that gives weight to function names, method names, field names, constants, file names, and config keys present in both the finding and evidence.

Alternatives considered:

- Semantic embeddings or vector search: rejected because it expands scope, adds dependencies, and is hard to test deterministically.
- AST/tree-sitter symbol matching: rejected for M5b because AST work is explicitly a later milestone.
- LLM-based re-verification: rejected because the verifier must be deterministic and cheap to evaluate.

### Short-evidence safeguards

Short finding evidence is accepted only when it contains a strong signal such as an identifier, literal, operator expression, config key, or exact code phrase. Generic words like "error", "nil", "test", "config", "timeout", or "handler" alone must not support a finding.

This allows useful snippets such as `if err == nil` or `timeout: 0` to match while preventing broad prose overlap from keeping unsupported findings.

### Source-aware support

Evidence sources are not equally valid for every claim. Patch and full-file evidence can support concrete code-defect findings for the referenced file. Related tests can support missing-test and test-behavior findings, and may corroborate code findings only when the referenced code file also has support. Docs/config evidence can support docs/config findings and limitation context, but unrelated docs/config prose must not support a concrete code-defect claim.

Omitted-context notes remain a limitation source. They can justify a downgrade to `question` when the finding is framed as uncertainty, but they must not verify a concrete defect.

### Eval fixture shape

Eval fixtures should be table-driven and deterministic, with each fixture specifying:

- Bounded repo context inputs.
- Raw structured review result inputs.
- Expected verified finding outcomes.
- Expected reason-category counts.
- Expected safe aggregate stats such as no-finding count or kept/downgraded/dropped percentages.

Fixtures should include realistic PR shapes: patch-only evidence, full-file-only support, related test evidence, docs/config evidence, paraphrased evidence, short snippets, omitted context, missing tests, no findings, and multi-finding mixed outcomes.

### Safe aggregate metadata

Verifier stats may include total findings, kept/downgraded/dropped counts, rates or percentages, reason-category counts, no-finding count, and eval fixture summaries. These values must be derived from counts and stable categories only.

Stats, logs, and Check Run output must not include raw private code, raw prompts, raw model output, secrets, tokens, private keys, API keys, complete webhook payloads, or installation tokens.

## Risks / Trade-offs

- More permissive matching could keep unsupported findings -> Mitigation: require source compatibility, meaningful tokens, identifier/literal support for short evidence, and regression fixtures for generic-word false positives.
- Threshold tuning could become brittle -> Mitigation: keep thresholds fixed, documented in tests, and evaluated through fixture summaries rather than production code samples.
- Paraphrase support may still miss real issues -> Mitigation: prefer downgrade to `question` only when bounded evidence remains useful; otherwise keep dropping unsupported concrete claims.
- Metrics could accidentally expose content if expanded casually -> Mitigation: restrict metadata to counts, percentages, categories, fixture names, and outcome summaries.

## Migration Plan

1. Add or extend verifier eval fixtures before changing matching behavior.
2. Implement layered deterministic matching behind existing verifier interfaces.
3. Add source-compatibility and short-evidence safeguards.
4. Extend safe aggregate stats and tests.
5. Run the existing verification commands plus OpenSpec validation.

Rollback is straightforward: revert to the M5a matcher while leaving reporter fan-out and advisory output unchanged.

## Open Questions

- Exact token-overlap thresholds should be chosen during implementation from fixture behavior and kept conservative.
- Eval fixture summaries may live only in tests initially unless production logging has an existing safe metadata path.
