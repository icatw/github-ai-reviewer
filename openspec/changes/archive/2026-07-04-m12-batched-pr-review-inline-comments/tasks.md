## 1. GitHub Client Boundary

- [x] 1.1 Add typed request/response structures for creating a Pull Request Review with `commit_id`, `body`, `event=COMMENT`, and inline `comments[]`.
- [x] 1.2 Implement a GitHub client method that calls the official Create Pull Request Review API.
- [x] 1.3 Keep the existing individual Pull Request Review Comment endpoint available for existing comment updates and configured fallback only.
- [x] 1.4 Add client tests that assert the batched review request includes path, body, line, side, commit ID, event, and review body.

## 2. Inline Comment Planning

- [x] 2.1 Preserve existing inline enablement, severity, required-field, confidence, maximum-count, and RIGHT-side diff line mapping gates.
- [x] 2.2 Parse existing bot inline review comments by marker and fingerprint without touching unrelated comments.
- [x] 2.3 Split a review run into existing matching comments to update, new comments to batch-create, and obsolete bot comments to stale-mark.
- [x] 2.4 Add tests for eligibility filtering, max 10 behavior, fingerprint matching, unrelated comment ignoring, and update/create/stale split results.

## 3. Batched Inline Reporter

- [x] 3.1 Render a concise non-empty Pull Request Review body for batched inline comments.
- [x] 3.2 Render inline comment bodies that preserve the service marker and finding fingerprint where GitHub permits comment-body markers.
- [x] 3.3 Create all newly eligible inline comments for a run in one submitted Pull Request Review when at least one new comment exists.
- [x] 3.4 Update existing matching inline comments individually when GitHub permits updates and exclude them from the create-review batch.
- [x] 3.5 Skip empty Pull Request Review creation when there are no new inline comments.
- [x] 3.6 Add reporter tests for multi-comment batch creation, single-comment batch creation, no-new-comment behavior, and existing-comment update behavior.

## 4. Stale Lifecycle and Fallback

- [x] 4.1 Implement non-destructive stale marking for obsolete bot inline comments, preferably by updating the comment body with a concise stale marker.
- [x] 4.2 Ensure stale handling is marker-scoped and cannot modify human comments or other bot comments.
- [x] 4.3 Implement or preserve the configured fallback path for batch creation failure without duplicate creation inside the webhook handler.
- [x] 4.4 Add tests for stale marking, stale failure isolation, destructive deletion absence, and fallback behavior.

## 5. Existing Output Preservation and Safety

- [x] 5.1 Verify the summary issue comment reporter still upserts the marker comment independently of batched inline review reporting.
- [x] 5.2 Verify the advisory Check Run reporter remains non-blocking and does not fail checks based on AI findings.
- [x] 5.3 Audit logging and failure metadata so inline create, update, fallback, and stale paths do not log secrets, tokens, raw prompts, raw model responses, complete webhook payloads, or private source snippets beyond intentional PR-facing comments.
- [x] 5.4 Add tests for preserved summary comment and Check Run behavior when inline batch reporting succeeds and fails.

## 6. Verification

- [x] 6.1 Run `gofmt -w .`.
- [x] 6.2 Run `go test ./...`.
- [x] 6.3 Run `go build ./cmd/server`.
- [x] 6.4 Run `openspec validate m12-batched-pr-review-inline-comments --type change --strict`.
- [x] 6.5 Perform real E2E verification on a non-sensitive PR where the LLM produces at least two eligible mapped findings.
- [x] 6.6 Confirm GitHub shows one submitted bot Pull Request Review with multiple line-level comments and a non-empty review body.
- [x] 6.7 Confirm GitHub API responses for created inline comments include expected `path`, `line`, and `side=RIGHT`.
- [x] 6.8 Confirm stale handling is observable on a later run where a previous bot inline fingerprint is absent, without deleting the old comment.
