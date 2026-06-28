## 1. Eval Fixture Expansion

- [x] 1.1 Add table-driven verifier fixture structure that records repo context, raw review input, expected verified output, and expected stats.
- [x] 1.2 Add fixtures for true positive, unsupported evidence, unavailable file, line/context mismatch, omitted-context dependency, and no-finding baseline coverage.
- [x] 1.3 Add fixtures for paraphrased evidence, short code snippets, patch-only evidence, full-file evidence, related test evidence, and docs/config evidence.
- [x] 1.4 Add fixtures for missing-tests and limitations interactions without converting unsupported concrete defects into useful output.
- [x] 1.5 Add a mixed-result fixture containing kept, downgraded, and dropped findings with stable expected reason-category distribution.

## 2. Evidence Matching Quality

- [x] 2.1 Extend normalized evidence matching to handle bounded snippets without requiring exact full evidence substring matches.
- [x] 2.2 Add deterministic token-overlap matching with conservative thresholds and tests for accepted and rejected cases.
- [x] 2.3 Add identifier-aware support for function names, method names, field names, constants, literals, file names, and config keys.
- [x] 2.4 Add short-evidence safeguards so generic words alone cannot support a finding.
- [x] 2.5 Add source-compatibility checks so unrelated docs/config text cannot support concrete code-defect findings.

## 3. Safe Metrics and Stats

- [x] 3.1 Extend verification stats with kept, downgraded, dropped, no-finding, rate or percentage, and reason-category aggregate fields.
- [x] 3.2 Add tests proving stats are deterministic and contain only aggregate categories and counts.
- [x] 3.3 Add eval fixture summary output or assertions that avoid raw private code, raw prompts, raw model output, secrets, tokens, private keys, API keys, complete webhook payloads, and installation tokens.

## 4. Integration Preservation

- [x] 4.1 Verify reporter fan-out still receives the verified result rather than raw LLM findings.
- [x] 4.2 Preserve stable PR comment marker upsert and existing empty-output suppression behavior.
- [x] 4.3 Preserve advisory Check Run policy so AI findings never produce request-changes behavior, merge blocking, or Check Run failure.
- [x] 4.4 Preserve extension points for future static-check evidence without executing static analyzers in M5b.

## 5. Verification

- [x] 5.1 Run `gofmt -w .`.
- [x] 5.2 Run `go test ./...`.
- [x] 5.3 Run `go build ./cmd/server`.
- [x] 5.4 Run `openspec validate m5b-verifier-eval-quality --type change --strict`.
