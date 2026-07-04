## Context

The service already has a reporter-style review loop that preserves the PR summary issue comment and advisory Check Run outputs. Inline review output exists behind quality gates and currently uses the individual Pull Request Review Comment API (`POST /repos/{owner}/{repo}/pulls/{pull_number}/comments`) through `internal/githubapp.Client.CreatePullRequestReviewComment`.

Real PR verification showed that individually created inline comments appear as separate bot reviews with empty review bodies. GitHub's Create Pull Request Review API can instead submit one review with `event=COMMENT`, a review `body`, `commit_id`, and multiple inline `comments[]`. That is the preferred GitHub-native shape for a review bot that has several eligible line-level findings in one run.

The design must preserve existing safety constraints: inline output remains disabled by default, only high-signal findings are eligible, mapped comments must target RIGHT-side diff lines, and logs must not include secrets, raw prompts, raw model responses, raw private payloads, or private source snippets beyond intentional PR-facing comments.

## Goals / Non-Goals

**Goals:**

- Add a GitHub client/reporter boundary for creating one submitted Pull Request Review with multiple inline comments.
- Prefer the official Create Review API for newly created inline findings.
- Keep the existing inline eligibility rules and maximum comment count.
- Preserve existing fingerprint/marker based idempotency where GitHub comment bodies support it.
- Define a safe lifecycle for obsolete bot inline comments without destructive deletion.
- Preserve the summary issue comment and advisory Check Run behavior.
- Define automated and real E2E verification for the batched review path.

**Non-Goals:**

- Do not make AI findings blocking, request changes, or fail Check Runs based on findings.
- Do not add inline suggestions, auto-fixes, or Check Run gating.
- Do not delete GitHub comments.
- Do not introduce durable storage solely for inline comment review IDs or fingerprints.
- Do not broaden inline eligibility beyond the existing quality gates.

## Decisions

### Prefer Create Pull Request Review for new inline comments

The inline reporter will collect newly eligible mapped findings and create one submitted Pull Request Review with `event=COMMENT`, the job `head SHA` as `commit_id`, a concise non-empty review body, and one `comments[]` entry per new inline finding. Each comment entry will use modern line mapping fields: `path`, `body`, `line`, `side=RIGHT`, and optional `start_line`/`start_side` only if the existing mapping layer can prove a supported multi-line range. Legacy `position` is not the preferred path.

Alternative considered: continue creating one inline comment per finding. That keeps the current API boundary but produces noisy GitHub review history and empty review bodies. It remains acceptable only as a fallback for batch creation failure.

### Split idempotency into update, create, and stale phases

The reporter will identify existing bot inline comments using the existing hidden marker and finding fingerprint. The run will split findings into:

- existing matching fingerprints: update those comments individually if the GitHub API permits updates;
- new fingerprints: create them together in one submitted review;
- old bot fingerprints absent from the current eligible set: handle through the stale lifecycle.

Create Review cannot update existing review comments in the same request, so this split keeps idempotency without sacrificing the preferred batch creation path. The batch review is skipped when there are no new comments; existing-comment updates and stale marking may still run.

Alternative considered: always create a fresh review and ignore previous bot comments. That would preserve batching but duplicate comments and make obsolete findings confusing.

### Use a non-destructive stale lifecycle

Stale handling will be safe and explicit. The implementation will detect obsolete bot inline comments by comparing marker/fingerprint data from GitHub against the current run's eligible fingerprint set for the PR and head context. Destructive deletion is out of scope. The preferred later apply behavior is to mark obsolete bot comments as stale by updating their body with a short stale marker and current-run reference when update is permitted. If GitHub supports a safe minimize or resolve operation for the comment/review thread in the chosen client boundary and permissions, it may be added as an optional strategy after body marking is working and tested.

This keeps human-visible history intact while making it clear that the finding no longer applies to the current run.

### Preserve existing output channels and safety boundaries

The summary issue comment and advisory Check Run reporters continue to run independently of inline review batching. Inline reporter failure records only safe failure metadata and must not leak raw prompts, raw model responses, installation tokens, private keys, API keys, complete webhook payloads, or private source snippets into logs. PR-facing inline comment bodies remain the intentional place where bounded evidence, failure scenario, and suggestion text can appear.

### Fallback policy

The legacy individual Pull Request Review Comment endpoint may remain available for updates and as a fallback only. If batch review creation fails for new comments, the reporter may either record safe failure metadata and skip new inline comments, or use the legacy individual create endpoint according to the implementation's existing failure policy. The preferred path remains Create Pull Request Review. The fallback must not run in the webhook handler and must not create duplicates for fingerprints already known to exist.

## Risks / Trade-offs

- Batch review API validation is all-or-nothing for newly created comments -> validate path, line, side, commit ID, body, and max count before submission; on failure, log safe categories and avoid duplicate retries inside the webhook handler.
- Existing comment update semantics differ from review creation semantics -> keep update and create operations as separate GitHub client methods with focused tests.
- Stale detection could mark comments from another bot or user -> require the service marker and fingerprint before any stale action.
- Obsolete findings can be head-SHA sensitive -> include run/head context in the stale marker where useful, but do not require durable storage.
- Fallback individual creation can reintroduce multiple empty reviews -> keep it explicitly secondary and observable so production can prefer skipping fallback if needed.

## Migration Plan

1. Add the new GitHub client method and inline reporter split behind the existing inline enablement configuration.
2. Keep existing individual comment creation available for updates and fallback while the batch path is verified.
3. Deploy with inline output still disabled by default.
4. Enable inline output on a non-sensitive test repository and verify one submitted review contains multiple line-level comments.
5. If production issues occur, disable inline output or disable batch creation fallback while retaining summary issue comments and advisory Check Runs.

## Open Questions

- Which stale action should be implemented first in apply: body update only, or body update plus an optional GitHub minimize/resolve action if the API/client supports it cleanly?
- Should batch creation failure fall back to individual creation by default, or should production prefer skipping new inline comments to avoid noisy empty reviews?
