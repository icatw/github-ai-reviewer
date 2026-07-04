## Why

The service now produces multiple bot-owned PR outputs across summary issue comments, advisory Check Runs, and batched inline Pull Request Reviews. After a PR is closed or merged, those outputs should clearly stop representing an active review state and the service should avoid spending LLM or analyzer work on a PR that is no longer reviewable.

## What Changes

- Add cleanup-only handling for signed `pull_request.closed` webhook events.
- Distinguish closed-unmerged and merged pull requests in the inactive status text where useful, while keeping both paths non-destructive.
- Do not enqueue a normal review job, call the LLM, fetch PR patch context for review, run optional analyzers, create new inline reviews, or create blocking outputs for closed or merged PR cleanup.
- Update the existing bot summary issue comment marker, when present or otherwise appropriate, to show that review output is inactive because the PR was closed or merged.
- Define an optional marker-scoped lifecycle for bot-owned inline review outputs on close or merge, such as stale-marking and, only when safely supported, minimizing or resolving bot inline threads without deleting comments.
- Define `/ai-review` behavior on closed PRs: do not start a new LLM review; ignore safely or update the bot summary marker with a concise inactive note according to the implementation policy.
- Preserve existing behavior for `pull_request` `opened`, `synchronize`, and `reopened` events and open-PR `/ai-review` commands.
- Preserve advisory, non-blocking output policy and all secret-safety constraints.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `github-app-review-loop`: Add cleanup-only handling for PR close/merge events and closed-PR manual review commands, including safe bot-owned output lifecycle requirements.

## Impact

- Affected packages likely include webhook event parsing/filtering, review job or cleanup job routing, comment rendering/upsert, inline review lifecycle helpers, GitHub App client boundaries for PR state lookup, and worker orchestration.
- GitHub API usage may include reading PR state for `/ai-review` commands and updating marker-scoped issue comments or review comments already owned by the bot.
- No durable storage, dashboard, billing, blocking review policy, auto-fix, auto-merge, request-changes behavior, or destructive comment deletion is introduced.
- Logs, comments, Check Runs, and failure metadata must continue to exclude secrets, installation tokens, private keys, API keys, raw prompts, raw model responses, complete private webhook payloads, and unbounded private source content.
