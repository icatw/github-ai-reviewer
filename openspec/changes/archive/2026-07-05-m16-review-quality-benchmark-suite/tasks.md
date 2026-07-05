## 1. Fixture Schema and Safety

- [x] 1.1 Extend `internal/reviewbench` fixture types with backward-compatible metadata for provenance, sanitization status, expected no-finding cases, expected finding IDs, matching hints, and quality labels.
- [x] 1.2 Add fixture decode validation for invalid combinations such as explicit no-finding intent with expected findings unless the schema permits a clearly documented exception.
- [x] 1.3 Update `cmd/review-bench-from-pr` output messaging and docs so generated private fixtures are directed to gitignored or temporary paths until sanitized.
- [x] 1.4 Ensure `.gitignore` or documentation covers local private fixture quarantine paths without ignoring sanitized public benchmark fixtures.

## 2. Finding Quality Comparison

- [x] 2.1 Add deterministic actual-vs-expected finding comparison using safe fields such as ID, file, line range, category, severity, title tokens, and evidence hints.
- [x] 2.2 Report covered expected findings, missed expected findings, unexpected findings, duplicate findings, and low-value findings per fixture.
- [x] 2.3 Support explicit expected no-finding fixtures and count any unclassified actual finding as unexpected.
- [x] 2.4 Keep standard benchmark output free of raw private source content, raw prompts, raw model output, secrets, tokens, private keys, API keys, installation tokens, and complete webhook payloads.

## 3. Suite Aggregation and CLI Output

- [x] 3.1 Extend `reviewbench.Report` and `reviewbench.SuiteReport` with annotated finding quality metrics while preserving existing context precision, recall, F1, source metrics, budget, and omission reporting.
- [x] 3.2 Update `cmd/review-bench` to emit stable per-fixture and suite-level finding quality summaries when annotations and actual findings are available.
- [x] 3.3 Keep legacy context-only fixtures valid and mark finding-quality sections as omitted or not annotated rather than failing them.
- [x] 3.4 Add stable ordering for missed, unexpected, duplicate, and low-value finding lists.

## 4. Documentation

- [x] 4.1 Document how to generate a real-PR fixture from `review-bench-from-pr`.
- [x] 4.2 Document the private fixture sanitization workflow and the metadata required before moving a fixture into tracked testdata.
- [x] 4.3 Document how to annotate expected findings, expected no-finding cases, and low-value or duplicate classifications.
- [x] 4.4 Document how to run a single fixture, run a suite, and compare aggregate metrics for regressions.

## 5. Tests and Verification

- [x] 5.1 Add decode tests for legacy fixtures, expected finding annotations, expected no-finding annotations, sanitized metadata, and invalid conflicting annotations.
- [x] 5.2 Add reporting tests for covered, missed, unexpected, duplicate, and low-value finding categories.
- [x] 5.3 Add suite aggregation tests for mixed annotated and context-only fixtures.
- [x] 5.4 Run `gofmt -w .`, `go test ./...`, `go build ./cmd/server`, and `openspec validate m16-review-quality-benchmark-suite --type change --strict`.
