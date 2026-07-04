## Context

The service currently accepts signed `pull_request` events for `opened`, `synchronize`, and `reopened`, plus signed `issue_comment.created` commands that match `/ai-review` on pull requests. Accepted review requests enqueue a normal review job that can fetch PR files, build repo-aware context, call the LLM, verify findings, update the summary marker issue comment, report an advisory Check Run, and optionally publish batched inline Pull Request Review comments.

M12 changed inline output to prefer one submitted Pull Request Review with multiple inline comments and to stale-mark obsolete bot inline comments without deleting human-visible review history. A closed or merged PR should now stop being treated as an active review target. Cleanup must be safe, marker-scoped, non-blocking, and non-destructive.

## Goals / Non-Goals

**Goals:**

- Accept `pull_request.closed` as a cleanup-only event.
- Preserve fast webhook handling by routing close/merge cleanup through a lightweight cleanup job or equivalent asynchronous path.
- Avoid normal review work for closed or merged PRs: no LLM request, no analyzer run, no new review findings, no new inline Pull Request Review, and no blocking status.
- Mark bot-owned summary output inactive when a marker comment exists or when the implementation explicitly chooses summary-marker upsert as the cleanup output.
- Optionally stale-mark, minimize, or resolve bot inline review outputs only when they are marker-scoped and the GitHub API/client boundary supports the action safely.
- Treat `/ai-review` on closed PRs as cleanup or ignored traffic, never as a normal review request.
- Preserve existing behavior for open PR `opened`, `synchronize`, `reopened`, and `/ai-review` flows.

**Non-Goals:**

- No durable storage for PR lifecycle state, review IDs, comment IDs, or cleanup history.
- No dashboard, billing, tenant management, repository indexing, vector database, auto-fix, auto-merge, request-changes review, or merge blocking.
- No deletion of summary comments, inline comments, Check Runs, reviews, or human-authored content.
- No raw payload, raw prompt, raw model response, token, key, secret, or private-source logging.

## Decisions

### Model close/merge as cleanup, not review

`pull_request.closed` will parse into a cleanup-oriented event that carries installation ID, owner, repo, pull number, head SHA, delivery ID, and merged state. The server should not pass this event to the normal review queue. It should enqueue a cleanup job or call an injected cleanup sink that performs only marker-scoped output lifecycle work.

Alternative considered: treat `closed` as another review action with a special prompt. That is rejected because there is no active PR review target, it spends unnecessary LLM/analyzer resources, and it risks publishing new findings after the conversation is over.

### Preserve closed-unmerged versus merged status text

Cleanup output should distinguish `closed` from `merged` in concise status text: closed-unmerged means inactive because the PR was closed; merged means inactive because the PR was merged. Both paths remain advisory and non-destructive.

Alternative considered: one generic inactive state. That is simpler but less useful in PR history because merge and abandonment have different product meaning.

### Keep summary marker as the primary lifecycle surface

The summary issue comment already has a stable hidden marker and upsert behavior. Close/merge cleanup should update that marker-owned comment with an inactive/archive note when it exists. The implementation may create the marker summary only if the product policy chooses to make cleanup visible even when no prior summary exists, but it must avoid empty or noisy comments.

Alternative considered: post a new close/merge comment every time. That is rejected because it creates timeline noise and can duplicate bot-owned output.

### Make inline cleanup optional and marker-scoped

Inline cleanup may stale-mark bot inline comments using the existing marker/fingerprint strategy. If GitHub supports minimizing or resolving review threads cleanly through the chosen client boundary and permissions, the implementation may add that as a later step after marker-scoped body stale marking is tested. It must never alter comments that lack the service marker and must never delete comments.

Alternative considered: delete obsolete inline comments or all bot reviews on close. That is rejected because it destroys review history and can remove useful context for future audit or learning.

### Closed PR `/ai-review` does not start review work

For `/ai-review` commands, the PR metadata resolver should include PR state and merged status, or an equivalent boundary should be introduced. If the target PR is closed or merged, the handler must not enqueue a normal review job. It may enqueue cleanup or return a safe ignored response after recording safe metadata. No LLM or analyzer work is performed.

Alternative considered: respond with a new user-visible note for every closed-PR command. That is noisier than necessary and creates output after a PR is no longer active.

### Check Runs remain advisory and are not recreated for cleanup

Close/merge cleanup should not create a new `AI Review` Check Run. If an existing Check Run can be safely identified for the PR head SHA, the implementation may update its output with inactive status, but this is optional and must not turn AI findings into blocking conclusions or make cleanup failure block anything.

Alternative considered: always create a close/merge Check Run. That adds little value after a PR is closed and can create confusing status history.

## Risks / Trade-offs

- Cleanup may race with a synchronize review already queued -> Cleanup output should be idempotent and marker-scoped; workers should avoid publishing active-looking review output after they can cheaply determine the PR is closed.
- GitHub API support differs for inline thread minimize/resolve -> Start with body stale marking and keep minimize/resolve optional until a safe client boundary and tests exist.
- Closed-PR command state requires metadata lookup -> Reuse or extend the existing PR metadata resolver so the handler can decide without raw payload dependence.
- Summary update could create noise when no prior bot comment exists -> Prefer updating an existing marker comment; if creation is supported, require concise inactive content and tests.
- Cleanup failure should not look like review failure -> Log safe cleanup categories separately from normal review execution failures and keep Check Runs non-blocking.

## Migration Plan

1. Add parsing and tests for `pull_request.closed` with merged and unmerged fixtures.
2. Add cleanup job or sink routing so close/merge events do not enter the normal review worker.
3. Add summary marker inactive rendering/upsert behavior and optional marker-scoped inline stale handling.
4. Extend `/ai-review` PR metadata resolution to identify closed/merged PRs before enqueueing normal review work.
5. Deploy with existing GitHub App pull request webhook subscription; no new webhook event type is required for close/merge.
6. Verify on a non-sensitive PR by producing bot output, closing without merge, reopening if needed, and merging a separate test PR or equivalent safe test path.

Rollback is to ignore `pull_request.closed` again and keep existing open-PR review behavior. Existing bot comments and reviews remain in GitHub because cleanup is non-destructive.

## Open Questions

- Should cleanup create an inactive summary marker when no previous summary marker exists, or only update an existing marker to avoid timeline noise?
- Should the first implementation include inline body stale marking only, or also a GitHub minimize/resolve action if supported cleanly by the available API and permissions?
- Should existing Check Run output be updated on close/merge when it can be found, or should Check Runs be left as historical review status only?
