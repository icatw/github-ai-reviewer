## 1. Webhook Parsing and Routing

- [x] 1.1 Add `pull_request.closed` parsing in `internal/webhook` with typed fields for installation ID, owner, repo, pull number, head SHA, delivery ID, and merged state.
- [x] 1.2 Add webhook fixtures or focused payload tests for closed-unmerged and merged PR payloads.
- [x] 1.3 Preserve ignored handling for unsupported PR actions and existing accepted handling for `opened`, `synchronize`, and `reopened`.
- [x] 1.4 Update `internal/server` routing so accepted close events are sent to a cleanup sink or cleanup job path, not the normal review worker.
- [x] 1.5 Add server tests proving `pull_request.closed` returns through the fast webhook path and does not enqueue a normal review job.

## 2. Cleanup Job and Output Policy

- [x] 2.1 Introduce a small cleanup job type or equivalent interface carrying installation ID, owner, repo, pull number, head SHA, delivery ID, and closed-unmerged or merged status.
- [x] 2.2 Implement cleanup orchestration that performs only marker-scoped output lifecycle work and never calls the LLM, analyzer, normal review worker, or inline review creation path.
- [x] 2.3 Add summary issue comment inactive rendering for closed-unmerged and merged states using the existing stable summary marker.
- [x] 2.4 Update or add comment publisher tests for inactive summary marker update, missing-marker behavior, and duplicate-comment prevention.
- [x] 2.5 Add safe cleanup logging categories that exclude secrets, raw payloads, raw comment bodies, raw prompts, raw model responses, and private source content.

## 3. Inline and Check Run Lifecycle

- [x] 3.1 Reuse existing inline marker/fingerprint discovery for cleanup-only inactive or stale marking when inline cleanup is enabled and supported.
- [x] 3.2 Add tests proving inline cleanup alters only comments containing this service's marker and does not alter human comments, unrelated bot comments, or comments without fingerprints.
- [x] 3.3 Keep destructive deletion out of the implementation and add tests or fake-client assertions proving delete APIs are not called.
- [x] 3.4 Skip Check Run cleanup for this slice and document that existing `AI Review` Check Runs remain historical advisory status.
- [x] 3.5 If Check Run cleanup is skipped, document the skip in code comments or operational docs as historical status preservation.

## 4. Closed PR `/ai-review` Command Policy

- [x] 4.1 Extend the PR metadata resolver used by `issue_comment` commands to return open/closed state and merged state.
- [x] 4.2 Update `/ai-review` command handling so open PRs preserve existing normal review job behavior.
- [x] 4.3 Update `/ai-review` command handling so closed-unmerged PRs do not enqueue normal review jobs and either enqueue cleanup or return a safe ignored response.
- [x] 4.4 Update `/ai-review` command handling so merged PRs do not enqueue normal review jobs and either enqueue cleanup or return a safe ignored response.
- [x] 4.5 Add tests for open, closed-unmerged, and merged `/ai-review` command targets, including proof that closed and merged targets do not call downstream review work.

## 5. Documentation and Real Verification

- [x] 5.1 Update README or production docs to include `pull_request.closed` cleanup behavior and closed-PR `/ai-review` behavior.
- [x] 5.2 Update E2E evidence template or add M13-specific verification notes for closing and merging a non-sensitive test PR.
- [x] 5.3 Verify a non-sensitive closed-unmerged PR receives cleanup behavior without new LLM review output and without deleting history.
- [x] 5.4 Verify a non-sensitive merged PR receives cleanup behavior without new LLM review output and without deleting history.
- [x] 5.5 Verify logs, comments, inline output, and optional Check Run output do not expose secrets, raw prompts, raw model responses, complete webhook payloads, or unbounded private source content.

## 6. Standard Verification

- [x] 6.1 Run `gofmt -w .`.
- [x] 6.2 Run `go test ./...`.
- [x] 6.3 Run `go build ./cmd/server`.
- [x] 6.4 Run `openspec validate m13-pr-close-review-cleanup --type change --strict`.
