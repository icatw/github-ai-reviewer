## Context

M2 introduced a typed `ReviewResult` contract and deterministic PR summary comment rendering with a stable hidden marker for upsert. The current publishing path is still effectively comment-specific: review orchestration renders the result and sends it to the GitHub Issues comments API.

M3 adds an output layer for multiple review reporters and a GitHub Check Run reporter. The Checks signal must reflect review job lifecycle state on the PR head SHA while preserving the product accuracy policy: AI findings remain advisory, summary comments remain non-blocking, and no request-changes or inline comments are added.

GitHub App installations for M3 need Checks read/write permission. Existing webhook verification, fast response behavior, installation authentication, changed file fetching, structured LLM review, validation, and comment marker upsert stay in place.

## Goals / Non-Goals

**Goals:**

- Introduce a reporter abstraction for review lifecycle events and completed structured review results.
- Keep the existing PR summary comment output as a reporter that still uses M2 marker upsert semantics.
- Add a GitHub Check Run reporter named `AI Review`.
- Report `in_progress` when supported PR review job processing starts.
- Report a non-blocking final conclusion when review completes without infrastructure failure.
- Optionally report infrastructure or job failures as failed Check Runs when enough safe metadata is available.
- Keep logs and Check Run output free of secrets, tokens, private keys, raw private payloads, and unbounded model output.

**Non-Goals:**

- Blocking PRs based on AI findings.
- Requesting changes through GitHub reviews.
- Inline review comments.
- Slash commands or issue-comment command handling.
- Durable job or Check Run persistence unless implementation discovers it is strictly required for update semantics.
- Dashboard, billing, repository indexing, static-analysis gating, or Check Run annotations.

## Decisions

1. Add reporters at the review orchestration boundary.

   Review orchestration should emit lifecycle information such as job started, review completed with a validated `ReviewResult`, output suppressed, and infrastructure failure. Reporters consume those events and decide how to publish. This keeps `ReviewResult` as the shared review data boundary while avoiding a GitHub Check Run implementation being coupled to Markdown comment rendering. An alternative was to add Check Run calls directly beside comment publishing, but that would duplicate lifecycle decisions and make future output channels harder to add.

2. Keep the comment publisher as a reporter, not a special path.

   The M2 comment behavior should move behind the reporter layer without changing its external behavior. It should still render deterministic Markdown from typed review data, include the stable hidden marker, update an existing marker comment, and suppress empty or noisy output. This preserves the established PR conversation behavior while allowing reporter fan-out.

3. Use stateless Check Run lookup and update where possible.

   The Check Run reporter should create an `AI Review` Check Run for the job head SHA when processing starts, then update the matching run on completion. To avoid durable storage in M3, the implementation should prefer listing Check Runs for the head ref/SHA and selecting the service-owned `AI Review` run associated with the current job head SHA. If GitHub API semantics make ambiguity unavoidable, the implementation may create a new run per job and complete that run without requiring database state, accepting multiple historical runs for older commits. Durable storage is a later-milestone fallback, not the default design.

4. Use non-blocking conclusions for review content.

   Completed review jobs with advisory findings should conclude `success` or `neutral`; the exact mapping can be chosen during implementation and documented in tests. AI severities, including `blocker` from the structured schema, must not produce a failed Check Run in M3. `failure` is reserved for infrastructure or job execution failures only if the implementation chooses to surface those failures through Checks. An alternative was to fail the Check Run for high-risk findings, but the project does not yet have static checks or finding verification to justify blocking.

5. Keep Check Run output concise and safe.

   The Check Run title and summary should identify the review status and point users to the PR summary comment for details when available. It should not include secrets, installation tokens, raw webhook payloads, raw prompts, raw model responses, or unbounded private diff content. The comment remains the detailed human-facing review output for M3.

6. Treat reporter failures as job output failures with clear policy.

   Reporter fan-out needs deterministic error behavior. The implementation should call reporters from the worker path, capture safe error categories, and avoid executing any reporter from the webhook handler. A comment reporter failure must not cause duplicate comments in retry logic. A Check Run reporter failure should be logged safely; if other reporters can still publish, the implementation may continue to them, but tests should document the chosen behavior.

## Risks / Trade-offs

- Check Run lookup may find multiple historical runs with the same name for a SHA -> Prefer deterministic selection of the newest service-owned `AI Review` run or create a new run per job and complete it immediately.
- Reporter fan-out can create partial output, such as comment success and Check Run failure -> Log safe per-reporter failure categories and keep webhook responses independent from reporter outcomes.
- Check Run status can become stale if the process exits after `in_progress` and before completion -> Accept for M3 unless stateless recovery is straightforward; durable reconciliation belongs to a later milestone.
- Adding Checks permission changes GitHub App setup -> Update documentation and verification tasks so real installs include Checks read/write.
- Surfacing infrastructure failures as `failure` may look like a blocked PR -> Keep failure limited to service execution failures and never derive it from AI findings.

## Migration Plan

No data migration is required. Existing PR summary comments with the M2 hidden marker remain the canonical comment upsert target. Existing repositories must update the GitHub App permissions to include Checks read/write before Check Run reporting works.

Deployment can roll forward with the reporter layer enabled. Rollback is to redeploy the M2 behavior; existing Check Runs remain historical GitHub records and do not require cleanup.

## Open Questions

- Whether successful completed reviews should use `success` for "review completed" or `neutral` to emphasize advisory status.
- Whether infrastructure failures should always complete the Check Run as `failure`, or whether some transient reporter failures should leave the Check Run unchanged and rely on logs.
