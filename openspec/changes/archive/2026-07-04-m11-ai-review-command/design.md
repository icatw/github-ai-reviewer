## Context

The current review loop accepts signed `pull_request` webhooks for `opened`, `synchronize`, and `reopened`, creates a typed review job, and hands that job to asynchronous worker processing. Users currently need a new PR event, usually another push, to request a fresh review.

GitHub slash-style commands arrive as `issue_comment` webhook deliveries. Pull request conversation comments use issue comments, so the payload can identify the repository, installation, issue number, and whether the issue is a PR, but it does not include the pull request head SHA needed by the existing review job and Check Run reporting path.

## Goals / Non-Goals

**Goals:**

- Accept manual review requests from signed `issue_comment.created` events on pull requests.
- Trigger only for an exact `/ai-review` command or `/ai-review` followed by whitespace.
- Preserve a fast webhook response by enqueueing review work instead of running review work inline.
- Reuse the existing `ReviewJob` shape and downstream worker/reporting behavior.
- Resolve the PR head SHA through a bounded GitHub App installation-authenticated resolver before enqueueing, or behind an equivalent safe resolver boundary.
- Avoid logging raw webhook payloads, raw comment bodies, secrets, tokens, prompts, or model responses.

**Non-Goals:**

- No auto-fix, auto-merge, merge blocking, or inline review comments.
- No change to the existing `pull_request` event behavior.
- No new command syntax beyond `/ai-review`.
- No dashboard, persistent command audit log, or user authorization policy beyond GitHub delivery authenticity and repository installation scope.

## Decisions

1. **Use `issue_comment.created` as the only manual command event.**

   Rationale: a new comment is the explicit user action and avoids retriggering on edits or deletes. Unsupported `issue_comment` actions are ignored with `204`.

   Alternative considered: accept edited comments. This is rejected for this change because edits create ambiguity about whether the command was newly requested and can retrigger unexpectedly.

2. **Use strict command matching.**

   The command parser accepts only a body that is exactly `/ai-review` or starts with `/ai-review` followed by whitespace. Prefix lookalikes such as `/ai-reviewer`, text before the command, and unrelated comments are ignored.

   Rationale: this avoids surprising reviews from ordinary discussion while allowing future arguments after whitespace without changing the trigger contract.

   Alternative considered: substring matching. This is rejected because it would trigger from quoted text or normal prose.

3. **Treat plain issues and unsupported comments as ignored, not errors.**

   The handler returns `204` and creates no job when the event is signed but is not an accepted command on a pull request.

   Rationale: ignored deliveries are normal webhook traffic and should not look like service failures.

4. **Resolve PR head SHA behind an installation-authenticated resolver boundary before enqueueing.**

   For a valid command on a PR, the handler path obtains the installation ID, owner, repo, and pull number from the signed payload, then asks a PR metadata resolver for the current head SHA. The resolver uses GitHub App installation auth, or an equivalent injected boundary in tests, and returns only the fields needed for job creation.

   Rationale: the downstream worker and Check Run reporter require `head_sha`, but `issue_comment` payloads do not include it directly. A resolver boundary keeps secret handling centralized, lets tests use a fake resolver without fabricating GitHub results, and avoids broad payload dependence.

   Alternative considered: enqueue a partial job and resolve SHA later in the worker. This would require changing the existing job contract and could let invalid jobs enter the queue. Keeping the job complete before enqueue preserves the current worker contract.

5. **Keep webhook verification first.**

   The handler continues to verify `X-Hub-Signature-256` before parsing either `pull_request` or `issue_comment` payloads.

   Rationale: payload fields are untrusted until signature verification succeeds.

6. **Preserve advisory reporter behavior.**

   Manual review jobs flow through the same worker and reporters as automatic pull request jobs. Check Runs remain advisory status reporting for the resolved head SHA; the change does not introduce blocking conclusions or merge gates.

## Risks / Trade-offs

- **Resolver latency in webhook handling** -> Keep the resolver call limited to PR metadata only, with normal GitHub client timeout behavior, then enqueue review work asynchronously. If the resolver fails, return an appropriate error according to the existing handler pattern and do not enqueue a partial job.
- **Duplicate manual reviews from repeated comments** -> Repeated explicit commands intentionally enqueue repeated reviews. Existing comment upsert behavior prevents duplicate bot summary comments for the same PR.
- **Ambiguous future command arguments** -> The whitespace rule reserves space for future arguments, but this change ignores argument semantics. Tests must lock current matching behavior.
- **Command body privacy** -> Logs and errors must use safe categories and delivery IDs, not raw comment bodies or payload dumps.
- **Permissions/config drift** -> Operators must enable the `Issue comment` webhook event on the GitHub App. Existing read/write PR and issue permissions are sufficient for PR metadata lookup and PR conversation comments.

## Migration Plan

Deploy the code change with the existing service rollout process, then enable the `Issue comment` webhook event for the GitHub App. Rollback is to disable the `Issue comment` event or redeploy the previous build; existing automatic pull request reviews remain unchanged.

## Open Questions

- Should later milestones restrict manual commands to repository collaborators or users with write permission? This proposal does not add an authorization check beyond GitHub webhook authenticity and installed repository scope.
