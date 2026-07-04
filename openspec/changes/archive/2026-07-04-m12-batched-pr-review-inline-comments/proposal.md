## Why

The current inline finding path publishes line comments one-by-one through the individual Pull Request Review Comment API, which makes GitHub show multiple bot reviews with empty bodies instead of one cohesive review. GitHub's official Pull Request Review API supports submitting a single `COMMENT` review with a concise body and multiple inline comments, which better matches established review bot behavior and reduces review timeline noise.

## What Changes

- Add a GitHub client boundary for creating a submitted Pull Request Review with `event=COMMENT`, `commit_id`, a non-empty review body, and a batch of inline `comments[]`.
- Publish newly eligible, mapped inline findings through one Create Review API call whenever inline output is enabled and at least one new inline comment must be created.
- Preserve existing inline quality gates: disabled by default, maximum 10 comments, only `blocker` and `warning` severities, required evidence/failure scenario/suggestion, optional confidence at least `0.70`, and required RIGHT-side diff line mapping.
- Preserve the existing summary issue comment and advisory Check Run behavior as non-blocking outputs.
- Keep the existing bot marker and fingerprint strategy for inline comments where GitHub comment bodies permit it.
- Define a safe stale-comment lifecycle for bot inline comment fingerprints that are no longer produced by the current run, with destructive deletion out of scope.
- Preserve idempotency where GitHub permits updates: update existing bot inline comments individually, create only new inline comments in one submitted review batch, and handle stale comments separately.
- Allow the legacy individual Pull Request Review Comment endpoint only as a fallback when batch review creation fails or when there are no new comments to batch.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `github-app-review-loop`: Change inline review publication from independent one-by-one review comments to a preferred single submitted Pull Request Review containing multiple eligible inline comments and a concise review body, with safe stale inline comment lifecycle requirements.

## Impact

- Affected packages likely include `internal/githubapp`, `internal/comment`, and review reporter orchestration code that currently calls `CreatePullRequestReviewComment`.
- The GitHub API surface expands to the official Create Pull Request Review endpoint while keeping the individual review comment endpoint for updates and fallback only.
- No new durable storage is required by the proposal; stale detection can use existing marker/fingerprint data from GitHub comments.
- Security posture remains unchanged: logs must not include secrets, installation tokens, private keys, API keys, raw prompts, raw model responses, raw private payloads, or source snippets beyond intentional PR-facing comments.
