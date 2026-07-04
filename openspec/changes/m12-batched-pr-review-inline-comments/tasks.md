## 1. GitHub Client Boundary

- [ ] 1.1 Add typed request/response structures for creating a Pull Request Review with `commit_id`, `body`, `event=COMMENT`, and inline `comments[]`.
- [ ] 1.2 Implement a GitHub client method that calls the official Create Pull Request Review API.
- [ ] 1.3 Keep the existing individual Pull Request Review Comment endpoint available for existing comment updates and configured fallback only.
- [ ] 1.4 Add client tests that assert the batched review request includes path, body, line, side, commit ID, event, and review body.

## 2. Inline Comment Planning

- [ ] 2.1 Preserve existing inline enablement, severity, required-field, confidence, maximum-count, and RIGHT-side diff line mapping gates.
- [ ] 2.2 Parse existing bot inline review comments by marker and fingerprint without touching unrelated comments.
- [ ] 2.3 Split a review run into existing matching comments to update, new comments to batch-create, and obsolete bot comments to stale-mark.
- [ ] 2.4 Add tests for eligibility filtering, max 10 behavior, fingerprint matching, unrelated comment ignoring, and update/create/stale split results.

## 3. Batched Inline Reporter

- [ ] 3.1 Render a concise non-empty Pull Request Review body for batched inline comments.
- [ ] 3.2 Render inline comment bodies that preserve the service marker and finding fingerprint where GitHub permits comment-body markers.
- [ ] 3.3 Create all newly eligible inline comments for a run in one submitted Pull Request Review when at least one new comment exists.
- [ ] 3.4 Update existing matching inline comments individually when GitHub permits updates and exclude them from the create-review batch.
- [ ] 3.5 Skip empty Pull Request Review creation when there are no new inline comments.
- [ ] 3.6 Add reporter tests for multi-comment batch creation, single-comment batch creation, no-new-comment behavior, and existing-comment update behavior.

## 4. Stale Lifecycle and Fallback

- [ ] 4.1 Implement non-destructive stale marking for obsolete bot inline comments, preferably by updating the comment body with a concise stale marker.
- [ ] 4.2 Ensure stale handling is marker-scoped and cannot modify human comments or other bot comments.
- [ ] 4.3 Implement or preserve the configured fallback path for batch creation failure without duplicate creation inside the webhook handler.
- [ ] 4.4 Add tests for stale marking, stale failure isolation, destructive deletion absence, and fallback behavior.

## 5. Existing Output Preservation and Safety

- [ ] 5.1 Verify the summary issue comment reporter still upserts the marker comment independently of batched inline review reporting.
- [ ] 5.2 Verify the advisory Check Run reporter remains non-blocking and does not fail checks based on AI findings.
- [ ] 5.3 Audit logging and failure metadata so inline create, update, fallback, and stale paths do not log secrets, tokens, raw prompts, raw model responses, complete webhook payloads, or private source snippets beyond intentional PR-facing comments.
- [ ] 5.4 Add tests for preserved summary comment and Check Run behavior when inline batch reporting succeeds and fails.

## 6. Verification

- [ ] 6.1 Run `gofmt -w .`.
- [ ] 6.2 Run `go test ./...`.
- [ ] 6.3 Run `go build ./cmd/server`.
- [ ] 6.4 Run `openspec validate m12-batched-pr-review-inline-comments --type change --strict`.
- [ ] 6.5 Perform real E2E verification on a non-sensitive PR where the LLM produces at least two eligible mapped findings.
- [ ] 6.6 Confirm GitHub shows one submitted bot Pull Request Review with multiple line-level comments and a non-empty review body.
- [ ] 6.7 Confirm GitHub API responses for created inline comments include expected `path`, `line`, and `side=RIGHT`.
- [ ] 6.8 Confirm stale handling is observable on a later run where a previous bot inline fingerprint is absent, without deleting the old comment.
