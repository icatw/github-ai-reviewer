## Why

The service now produces structured `ReviewResult` data and upserts a stable PR summary comment, but it has only one publishing path. Real GitHub App installs also need a visible, non-blocking PR Checks signal so reviewers can tell whether AI review is running, completed, or failed due to infrastructure without changing the advisory nature of findings.

## What Changes

- Add a reporter layer that receives the existing structured review lifecycle and can publish to multiple output channels.
- Preserve the M2 PR summary comment renderer and stable marker upsert behavior as one reporter output.
- Add GitHub Check Run status reporting named `AI Review` for supported PR review jobs.
- Create or update a Check Run to `in_progress` when a supported PR review job starts processing.
- Complete the Check Run with `success` or `neutral` when review finishes without infrastructure failure and without making AI findings blocking.
- Allow `failure` only for infrastructure or job failures if implementation design chooses to report those failures to Checks.
- Keep all outputs non-blocking: no request-changes behavior, no failing checks based on AI findings, and no inline review comments.
- Prefer stateless Check Run lookup/update semantics; do not add durable storage unless strictly necessary for correct Check Run updates.

## Capabilities

### New Capabilities

### Modified Capabilities
- `github-app-review-loop`: Add reporter fan-out and GitHub Check Run status reporting to the existing structured review and comment upsert flow.

## Impact

- Affected packages likely include review orchestration, GitHub App client wrappers, comment publishing, and new or existing reporter-facing code.
- GitHub App permissions must include Checks: read and write in addition to the existing M1/M2 permissions.
- Tests must cover reporter fan-out, Check Run lifecycle transitions, non-blocking conclusions, comment upsert preservation, and safe failure behavior.
- Deployment verification must include a real PR where the Checks tab shows `AI Review` and the PR summary comment is still upserted rather than duplicated.
