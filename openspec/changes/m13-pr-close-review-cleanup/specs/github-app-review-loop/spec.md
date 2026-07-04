## ADDED Requirements

### Requirement: Pull request close cleanup event handling
The webhook endpoint SHALL accept signed GitHub `pull_request` events whose action is `closed` for cleanup-only handling, and SHALL distinguish merged pull requests from closed-unmerged pull requests when the payload provides merged state.

#### Scenario: Closed unmerged pull request is accepted for cleanup
- **WHEN** a signed `pull_request` webhook has action `closed`
- **AND** the pull request payload indicates `merged` is false
- **THEN** the service accepts the event for close cleanup
- **AND** the cleanup event contains the installation ID, owner, repo, pull number, head SHA, action, delivery ID, and closed-unmerged status

#### Scenario: Merged pull request is accepted for cleanup
- **WHEN** a signed `pull_request` webhook has action `closed`
- **AND** the pull request payload indicates `merged` is true
- **THEN** the service accepts the event for close cleanup
- **AND** the cleanup event contains the installation ID, owner, repo, pull number, head SHA, action, delivery ID, and merged status

#### Scenario: Close cleanup does not create normal review job
- **WHEN** a signed `pull_request.closed` webhook is accepted for cleanup
- **THEN** the service does not create a normal LLM review job
- **AND** the close cleanup path does not fetch PR changed files for review, build LLM prompt context, run optional analyzers, call the LLM, create new inline Pull Request Reviews, request changes, auto-fix code, auto-merge, or block merging

#### Scenario: Close cleanup remains fast
- **WHEN** a signed `pull_request.closed` webhook is received
- **THEN** the webhook handler returns after accepting or ignoring cleanup work according to the service's asynchronous handling pattern
- **AND** the webhook handler does not perform LLM review work inline

#### Scenario: Close cleanup missing required field is rejected
- **WHEN** a signed `pull_request.closed` webhook is missing a field required to identify the installation, repository, pull request, head SHA, merged status, or delivery
- **THEN** the service rejects the payload with a client error
- **AND** no cleanup job or normal review job is created

### Requirement: Bot-owned output inactive lifecycle
For closed or merged pull requests, the service SHALL make bot-owned review outputs clearly inactive or archived through marker-scoped, non-destructive updates when supported, and SHALL NOT delete human-visible review history.

#### Scenario: Summary marker comment is updated inactive
- **WHEN** close cleanup runs for a pull request that has an existing summary issue comment containing the service's stable hidden marker
- **THEN** the service updates that marker-owned summary comment with concise inactive status text
- **AND** the status distinguishes closed-unmerged from merged when that state is available
- **AND** the updated comment remains advisory and non-blocking

#### Scenario: Missing summary marker does not create noise
- **WHEN** close cleanup runs for a pull request that has no existing summary issue comment containing the service's stable hidden marker
- **THEN** the service does not create duplicate bot summary comments
- **AND** any newly created inactive summary marker comment is concise, non-empty, and allowed only by the implementation's explicit cleanup output policy

#### Scenario: Bot inline comments may be marked inactive
- **WHEN** close cleanup runs and existing inline Pull Request Review comments contain this service's stable inline marker and fingerprint
- **THEN** the service may mark those bot inline comments inactive or stale through a safe supported marker-scoped action
- **AND** the inactive or stale text identifies the close or merge lifecycle without including secrets, raw prompts, raw model responses, complete webhook payloads, or unbounded private code

#### Scenario: Inline minimize or resolve is optional and marker-scoped
- **WHEN** the implementation supports a GitHub API action to minimize or resolve bot inline review comments or threads during close cleanup
- **THEN** the action applies only to comments or threads that contain this service's marker
- **AND** the action is covered by automated tests before use

#### Scenario: Destructive cleanup is not performed
- **WHEN** close cleanup runs for any bot-owned or human-owned output
- **THEN** the service does not delete issue comments, Pull Request Review comments, Pull Request Reviews, Check Runs, human comments, or comments from other bots
- **AND** the service does not alter comments that lack this service's stable marker

#### Scenario: Check Run cleanup does not create blocking output
- **WHEN** close cleanup runs for a pull request with existing `AI Review` Check Run output
- **THEN** the service does not create a new Check Run solely for cleanup
- **AND** any optional update to an existing `AI Review` Check Run remains advisory and non-blocking
- **AND** cleanup failure does not turn AI findings into a blocking conclusion

### Requirement: Closed pull request manual review command policy
The service SHALL NOT start a normal review when a signed `/ai-review` issue comment command targets a pull request that is closed or merged.

#### Scenario: Closed pull request command does not enqueue review
- **WHEN** a signed accepted `/ai-review` issue comment command targets a pull request
- **AND** the pull request metadata resolver reports that the pull request is closed and unmerged
- **THEN** the service does not enqueue a normal LLM review job
- **AND** it may enqueue cleanup work or return a safe ignored response according to the cleanup policy

#### Scenario: Merged pull request command does not enqueue review
- **WHEN** a signed accepted `/ai-review` issue comment command targets a pull request
- **AND** the pull request metadata resolver reports that the pull request is merged
- **THEN** the service does not enqueue a normal LLM review job
- **AND** it may enqueue cleanup work or return a safe ignored response according to the cleanup policy

#### Scenario: Open pull request command behavior is preserved
- **WHEN** a signed accepted `/ai-review` issue comment command targets an open pull request
- **THEN** the service preserves existing manual review command job creation
- **AND** the downstream worker and reporter behavior remains advisory and non-blocking

#### Scenario: Closed pull request command avoids downstream review work
- **WHEN** a signed `/ai-review` command targets a closed or merged pull request
- **THEN** the service does not fetch changed files for review, build LLM prompt context, run optional analyzers, call the LLM, create new inline Pull Request Reviews, auto-fix code, auto-merge, request changes, or block merging

### Requirement: Close cleanup safety and observability
Close and merge cleanup SHALL preserve the service's existing secret-safety constraints and SHALL emit only bounded safe metadata for cleanup outcomes.

#### Scenario: Cleanup logs are safe
- **WHEN** close cleanup is accepted, skipped, succeeds, or fails
- **THEN** logs or metrics may include safe metadata such as event type, action, delivery ID, owner, repo, pull number, head SHA suffix or safe identifier, merged state, and cleanup category
- **AND** logs or metrics do not include secrets, installation tokens, checkout credentials, private keys, API keys, raw prompts, raw model responses, complete webhook payloads, raw comment bodies, or unbounded private repository code

#### Scenario: Cleanup failure is non-blocking
- **WHEN** updating marker-scoped summary, inline, or Check Run output fails during close cleanup
- **THEN** the service records or logs safe failure metadata
- **AND** the service does not retry by creating duplicate comments inside the webhook handler
- **AND** the failure does not create a normal review job or blocking output

#### Scenario: Existing open pull request review behavior is unchanged
- **WHEN** a signed supported `pull_request` webhook with action `opened`, `synchronize`, or `reopened` is received
- **THEN** the service preserves existing review job creation and asynchronous worker behavior
- **AND** summary issue comments, advisory Check Runs, and enabled inline Pull Request Reviews keep their existing behavior

### Requirement: Close cleanup verification
The implementation SHALL include automated tests and real verification steps for close/merge cleanup behavior, closed-PR manual command behavior, preserved open-PR review behavior, and non-destructive output lifecycle.

#### Scenario: Automated verification covers close cleanup
- **WHEN** M13 implementation is complete
- **THEN** tests cover `pull_request.closed` parsing for closed-unmerged and merged payloads
- **AND** tests cover that close cleanup does not enqueue a normal LLM review job
- **AND** tests cover summary marker inactive rendering or upsert behavior
- **AND** tests cover marker-scoped inline inactive or stale behavior when implemented
- **AND** tests cover that unrelated comments and human comments are not altered

#### Scenario: Automated verification covers closed PR command policy
- **WHEN** M13 implementation is complete
- **THEN** tests cover `/ai-review` on closed-unmerged and merged pull requests
- **AND** tests prove those commands do not enqueue normal review jobs or call downstream review work
- **AND** tests cover that `/ai-review` on open pull requests still creates normal manual review jobs

#### Scenario: Standard commands pass
- **WHEN** M13 implementation is complete
- **THEN** `gofmt -w .` has been run
- **AND** `go test ./...` passes
- **AND** `go build ./cmd/server` passes
- **AND** `openspec validate m13-pr-close-review-cleanup --type change --strict` passes

#### Scenario: Real PR verification succeeds
- **WHEN** the service is deployed or restarted with M13 on a non-sensitive test repository
- **AND** a real pull request with existing bot summary output and, when enabled, bot inline output is closed without merge or merged
- **THEN** the webhook delivery for `pull_request.closed` is accepted or safely ignored according to the cleanup path
- **AND** no new LLM review output is produced after close or merge
- **AND** bot-owned marker output is inactive or archived according to the cleanup policy
- **AND** no bot-owned or human-visible review history is deleted
- **AND** logs, comments, and Check Run output do not expose secrets, raw prompts, raw model responses, complete webhook payloads, or unbounded private source content
