## Why

Users need a safe way to manually rerun an AI review after addressing feedback, changing surrounding context, or wanting a review without pushing another commit. A pull request comment command keeps the trigger inside GitHub's existing PR workflow while preserving the same non-blocking review pipeline used by automatic PR events.

## What Changes

- Add support for signed `issue_comment.created` webhook deliveries as a manual PR review trigger.
- Treat comments as commands only when the body is exactly `/ai-review` or starts with `/ai-review` followed by whitespace.
- Ignore non-command comments, unsupported comment actions, and comments on plain issues with a `204` ignored response and no review job.
- Resolve pull request metadata for valid command comments before enqueueing so the review job includes the PR head SHA.
- Enqueue the same asynchronous `ReviewJob` shape used by supported `pull_request` events, including installation ID, owner, repo, pull number, head SHA, action, and delivery ID.
- Preserve existing pull request webhook behavior, advisory Check Run behavior, and non-blocking summary review output.
- Exclude auto-fix, auto-merge, merge blocking, and inline review comments.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `github-app-review-loop`: Extend webhook event filtering and review job creation to accept a safe manual `/ai-review` command on pull request issue comments.

## Impact

- Affected code areas: webhook event routing and parsing, review job construction, GitHub App installation-authenticated PR metadata lookup, worker/job sink integration, and related unit tests.
- GitHub App configuration impact: deployments must subscribe to the `Issue comment` webhook event in addition to `Pull request`; existing repository permissions remain sufficient for reading PR metadata and posting PR comments.
- Operational impact: manual command deliveries must not log raw payloads, comment bodies, webhook secrets, installation tokens, private keys, API keys, raw prompts, or raw model responses.
