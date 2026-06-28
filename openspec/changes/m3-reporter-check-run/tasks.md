## 1. Reporter Abstraction

- [ ] 1.1 Define review lifecycle reporter interfaces and event/input types for job started, completed structured review, suppressed output, and infrastructure/job failure.
- [ ] 1.2 Wire review orchestration to call configured reporters from the worker path, not from the webhook handler.
- [ ] 1.3 Implement reporter fan-out with safe per-reporter error capture and deterministic behavior when one reporter fails.
- [ ] 1.4 Add unit tests for fan-out to multiple fake reporters, reporter ordering or concurrency policy, and safe failure metadata.

## 2. Comment Reporter Preservation

- [ ] 2.1 Move or adapt the M2 PR summary comment publishing path behind the reporter layer.
- [ ] 2.2 Preserve deterministic Markdown rendering, stable hidden marker inclusion, empty output suppression, and marker comment upsert behavior.
- [ ] 2.3 Add regression tests proving existing marker comments are updated and unrelated comments are ignored after reporter-layer integration.

## 3. GitHub Check Run Client

- [ ] 3.1 Extend the GitHub App client abstraction with Check Run create, list, and update operations needed by the reporter.
- [ ] 3.2 Implement stateless Check Run matching for `AI Review` on the job head SHA, with deterministic behavior when no safe match exists.
- [ ] 3.3 Ensure Check Run API calls use installation-authenticated GitHub clients and never log installation tokens or secret values.
- [ ] 3.4 Add fake-client tests for create, list/match, update, no-match create, API errors, and safe error categories.

## 4. Check Run Reporter

- [ ] 4.1 Implement `AI Review` Check Run `in_progress` reporting when a supported PR review job starts processing.
- [ ] 4.2 Implement completed Check Run updates with `success` or `neutral` for completed review jobs, including jobs with advisory findings.
- [ ] 4.3 Implement the chosen infrastructure/job failure policy, using `failure` only for service execution failures and not for AI findings.
- [ ] 4.4 Keep Check Run output concise and safe, without raw prompts, raw model responses, unbounded diff content, secrets, or complete payloads.
- [ ] 4.5 Add tests for start, completion, advisory findings not failing checks, suppressed review output, infrastructure failure policy, and Check Run output safety.

## 5. Configuration and Documentation

- [ ] 5.1 Update documentation for M3 GitHub App permissions to include Checks read/write while preserving Issues write for PR comments.
- [ ] 5.2 Document the non-blocking Check Run policy and the fact that AI findings do not request changes or fail Checks in M3.
- [ ] 5.3 Add or update configuration/tests only if enabling reporter selection requires explicit runtime settings.

## 6. Verification

- [ ] 6.1 Run `gofmt -w .`.
- [ ] 6.2 Run `go test ./...`.
- [ ] 6.3 Run `go build ./cmd/server`.
- [ ] 6.4 Run `openspec validate m3-reporter-check-run --type change --strict`.
- [ ] 6.5 Deploy or restart the service with a GitHub App that has Checks read/write permission.
- [ ] 6.6 Verify on a real PR that the Checks UI or GitHub API shows an `AI Review` Check Run for the PR head SHA.
- [ ] 6.7 Verify on the same real PR that the summary comment is still upserted through the M2 marker and repeated supported PR events do not create duplicate bot comments.
