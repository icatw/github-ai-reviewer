## 1. Structured Review Model

- [x] 1.1 Define typed `ReviewResult` and `Finding` models for summary, risk score, findings, missing tests, limitations, severity, category, file, line, title, evidence, failure scenario, suggestion, and confidence.
- [x] 1.2 Implement parsing that accepts a JSON object, trims surrounding whitespace, and extracts otherwise valid JSON from a Markdown `json` code fence.
- [x] 1.3 Implement validation and normalization for useful content, allowed severities, bounded risk score, bounded confidence, optional line numbers, and required finding text fields.
- [x] 1.4 Add unit tests for valid results, empty or missing useful content, invalid severities, invalid scores, invalid confidence values, optional lines, and fenced JSON.

## 2. Structured LLM Output

- [x] 2.1 Update prompt construction to request JSON-only output matching the structured review schema and to state that findings are advisory and non-blocking.
- [x] 2.2 Update the LLM client or review orchestration boundary so successful review output becomes typed review data before rendering.
- [x] 2.3 Handle provider errors, empty choices, malformed JSON, and validation failures as job-stopping errors that do not publish comments.
- [x] 2.4 Add fake-transport or fake-client tests for request shape, response parsing, malformed output, and no-publish failure behavior.

## 3. Stable Markdown Rendering

- [x] 3.1 Render comments from typed review data with deterministic sections for summary, risk, findings, missing tests, and limitations when present.
- [x] 3.2 Include a versioned hidden marker in every rendered bot review comment.
- [x] 3.3 Keep rendered findings explicitly advisory and non-blocking in M2.
- [x] 3.4 Suppress rendering for validated results with no useful content.
- [x] 3.5 Add exact-string or snapshot-style tests for marker presence, full result rendering, partial result rendering, and empty suppression.

## 4. Comment Upsert

- [x] 4.1 Extend the GitHub comment interface to list issue comments for a PR and update an existing comment by ID.
- [x] 4.2 Implement publisher logic that finds the service marker in existing issue comments and updates that comment instead of creating a duplicate.
- [x] 4.3 Create a new comment only when no existing marker comment is present.
- [x] 4.4 Avoid updating human comments or unrelated bot comments.
- [x] 4.5 Add fake-client tests for create, update, no-op empty body, and unrelated comments.

## 5. Orchestration Integration

- [x] 5.1 Update review orchestration to pass typed review results through rendering and comment upsert.
- [x] 5.2 Preserve safe logging with parse and validation failure categories, without logging raw private repository payloads, secrets, installation tokens, or API keys.
- [x] 5.3 Ensure webhook responses remain fast and continue returning `202 Accepted` after job submission.
- [x] 5.4 Add or update service-level tests for successful publish, suppressed output, LLM parse failure, validation failure, and comment publish failure.

## 6. Verification

- [x] 6.1 Run `gofmt -w .`.
- [x] 6.2 Run `go test ./...`.
- [x] 6.3 Run `go build ./cmd/server`.
- [x] 6.4 Run `openspec validate m2-structured-review-comments --type change --strict`.
