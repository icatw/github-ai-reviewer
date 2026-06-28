## Why

M5a added a bounded finding verifier, but its evidence matching and fixtures are still too small to evaluate realistic PR evidence shapes. M5b improves deterministic verifier quality and evaluation coverage so obvious false drops and unsafe keeps are easier to catch over time without expanding product policy beyond advisory output.

## What Changes

- Expand verifier eval fixtures beyond M5a's minimal unit cases to cover kept, downgraded, dropped, and mixed-outcome review results.
- Add deterministic coverage for paraphrased evidence, short snippets, patch-only support, full-file support, related test evidence, docs/config evidence, missing-tests and limitation interactions, no-finding results, and reason-category distribution.
- Improve evidence matching beyond strict full-string substring checks with conservative normalized snippet, token-overlap, and identifier-aware support.
- Add explicit safeguards so generic words alone cannot support a finding and unrelated docs/config text cannot support code-defect claims.
- Extend safe aggregate verification metadata with rates or percentages, reason-category counts, no-finding counts, and eval fixture summaries.
- Preserve existing reporter fan-out, comment marker upsert, advisory Check Run policy, and output suppression behavior.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `finding-verification`: Improve deterministic evidence matching, eval fixture coverage, false-positive safeguards, and safe aggregate verifier metrics.

## Impact

- Affected code is expected to stay in verifier-related review package code and deterministic verifier test/eval fixtures.
- No new runtime dependencies are required for AST, tree-sitter, vector search, durable storage, static analyzers, or repository indexing.
- Public GitHub behavior remains advisory and non-blocking; Check Runs must not fail from AI findings and comments must continue using the existing reporter flow.
- Logging and metrics remain aggregate only and must not include raw private code, raw prompts, raw model output, secrets, tokens, private keys, or complete webhook payloads.
