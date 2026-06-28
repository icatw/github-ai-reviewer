## 1. Analyzer Interface and Planning

- [x] 1.1 Inspect current worker, review context, verifier evidence, and reporter interfaces to identify the narrow insertion point before verifier execution.
- [x] 1.2 Define Go analyzer domain types for command plan, tool result, exit category, parsed evidence, limitations, and safe aggregate stats.
- [x] 1.3 Implement Go project detection using existing repo-aware context and changed-file metadata without adding repository indexing.
- [x] 1.4 Implement command planning for fixed argv forms `go test ./...` and `go vet ./...` with safe working-directory validation.
- [x] 1.5 Add deterministic unit tests for Go project detection, non-Go skip behavior, unsafe workspace skip behavior, and command planning.

## 2. Bounded Execution

- [x] 2.1 Implement analyzer execution behind an interface so tests can use fake executors and real execution can be skipped when workspace constraints are not satisfied.
- [x] 2.2 Enforce timeout, output byte limit, minimal environment, fixed argv, no shell interpolation, and safe workspace-root constraints.
- [x] 2.3 Ensure analyzer success, non-zero exit, timeout, unavailable tool, unsafe workspace, and internal error map to non-blocking exit categories.
- [x] 2.4 Add tests for timeout handling, output truncation, command failure, unavailable tool, secret-free environment, and non-blocking result categories.

## 3. Parsing and Sanitization

- [x] 3.1 Implement conservative parsers for useful `go test` and `go vet` package/file/line/message patterns.
- [x] 3.2 Sanitize parsed messages and preserve only bounded summaries needed for verifier support.
- [x] 3.3 Record truncation, skipped execution, unavailable execution, timeout, and parser limitations as safe limitation metadata.
- [x] 3.4 Add tests for parser fixtures, output-size limits, message sanitization, file/line extraction, and malformed output handling.

## 4. Verifier Integration

- [x] 4.1 Convert analyzer results into `EvidenceSourceStaticCheck` / `static_check_context` evidence scoped to the current review job.
- [x] 4.2 Extend verifier matching to use static-check evidence conservatively with existing source compatibility, unavailable-file, generic-overlap, and omitted-context rules.
- [x] 4.3 Extend safe aggregate verifier stats with static-check evidence counts and deterministic reason categories without raw analyzer output.
- [x] 4.4 Add verifier fixtures covering supported static-check evidence, unrelated static-check evidence, skipped analyzer limitations, generic overlap, and mixed kept/downgraded/dropped outcomes.

## 5. Worker and Reporter Behavior

- [x] 5.1 Wire the optional analyzer stage into the review worker after repo context collection and before verifier execution.
- [x] 5.2 Ensure analyzer skips, failures, and timeouts do not stop LLM review, verifier execution, comment reporter output, Check Run reporter output, or output suppression behavior.
- [x] 5.3 Ensure comment marker upsert and reporter fan-out remain unchanged and Check Run conclusions do not fail based on analyzer results or AI findings.
- [x] 5.4 Add worker/reporter tests proving analyzer failure and timeout are non-blocking and reporter output remains safe and concise.

## 6. Final Verification

- [x] 6.1 Run `gofmt -w .`.
- [x] 6.2 Run `go test ./...`.
- [x] 6.3 Run `go build ./cmd/server`.
- [x] 6.4 Run `openspec validate m6a-go-analyzer-evidence --type change --strict`.
- [x] 6.5 Review generated implementation for scope control, safe logging, secret handling, bounded analyzer output, and advisory/non-blocking behavior.
