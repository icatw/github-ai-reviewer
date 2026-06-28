## 1. Verifier Model and Evidence Index

- [x] 1.1 Add verifier outcome, reason category, stats, and evidence source types in the review package.
- [x] 1.2 Build a deterministic evidence index from `RepoContext` paths, patches, full files, related tests, repo docs/config, and omitted-context notes.
- [x] 1.3 Add path normalization and bounded line-availability helpers for patch and full-file evidence.

## 2. Verification Logic

- [x] 2.1 Implement pure finding verification that returns a verified `ReviewResult` and `VerificationStats` without mutating raw LLM output.
- [x] 2.2 Keep supported findings unchanged when file, line, and evidence are supported by available context.
- [x] 2.3 Drop unsupported findings, unavailable-file findings, and unverifiable concrete defects that depend on omitted context.
- [x] 2.4 Downgrade partially supported or omitted-context limitation findings to `question` with deterministic limitation text.
- [x] 2.5 Preserve no-finding results without fabricating findings and record the no-finding stats category.

## 3. Worker Integration and Reporting Preservation

- [x] 3.1 Wire verification into `Service.Process` after the LLM returns a parsed result and before `ReviewCompleted` reporter fan-out.
- [x] 3.2 Emit safe aggregate verification counts and reason-category counts without raw prompts, raw model output, secrets, tokens, webhook payloads, or private code content.
- [x] 3.3 Preserve existing output suppression when the verified result has no useful content.
- [x] 3.4 Preserve stable summary comment marker upsert behavior for verified non-empty results.
- [x] 3.5 Preserve advisory non-blocking Check Run behavior for kept or downgraded findings.

## 4. Tests and Eval Fixtures

- [x] 4.1 Add verifier fixtures for a true positive finding, unsupported finding, unavailable file, line/context mismatch, omitted-context dependency, and no-finding result.
- [x] 4.2 Add unit tests for outcome counts and deterministic reason categories.
- [x] 4.3 Add worker integration tests proving reporters receive the verified result rather than the raw LLM result.
- [x] 4.4 Add regression tests for comment marker upsert preservation and advisory Check Run preservation.

## 5. Verification

- [x] 5.1 Run `gofmt -w .`.
- [x] 5.2 Run `go test ./...`.
- [x] 5.3 Run `go build ./cmd/server`.
- [x] 5.4 Run `openspec validate m5a-finding-verifier --type change --strict`.
