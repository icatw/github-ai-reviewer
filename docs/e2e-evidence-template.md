# Real Deployment E2E Evidence

Use this file as a template for M8 real GitHub App E2E. Keep committed examples redacted. For an actual run, copy it to an untracked location such as `tmp/e2e/<date>.md` or keep the filled version outside the repository if it contains private repository metadata.

## Run Metadata

```text
Run ID: YYYY-MM-DD-operator-initials
Operator: [REDACTED_OPERATOR]
Service version/commit: [COMMIT_SHA]
Deployment URL: https://[REDACTED_HOST]
Workspace checkout enabled: false
Test repository: [REDACTED_OWNER]/[REDACTED_REPO] or intentionally-public-test-repo
Test PR: [REDACTED_PR_URL]
```

## Preflight

```text
go test ./...: [PASS|FAIL]
go build ./cmd/server: [PASS|FAIL]
scripts/smoke_local.sh: [PASS|FAIL]
openspec validate m8-real-deployment-e2e --type change --strict: [PASS|FAIL]
GO_WORKSPACE_PROVIDER_ENABLED: false
```

## Deployment Health

```text
Checked at: [TIMESTAMP_UTC]
GET /healthz status: [STATUS]
Response body: [SAFE_SUMMARY_ONLY]
Secret exposure observed: [NO|YES - describe safely]
```

## GitHub Webhook Delivery

Record safe metadata only. Do not paste raw webhook payloads.

| Event | Action | Delivery ID | GitHub status | Service status | Safe notes |
| --- | --- | --- | --- | --- | --- |
| pull_request | opened | [REDACTED_OR_SUFFIX] | [STATUS] | [202/OTHER] | [SAFE_NOTE] |
| pull_request | synchronize | [REDACTED_OR_SUFFIX] | [STATUS] | [202/OTHER] | [SAFE_NOTE] |
| pull_request | reopened | [REDACTED_OR_SUFFIX_OR_NOT_RUN] | [STATUS_OR_REASON] | [STATUS_OR_REASON] | [SAFE_NOTE] |
| pull_request | closed, unmerged | [REDACTED_OR_SUFFIX] | [STATUS] | [202/OTHER] | cleanup-only; no LLM review |
| pull_request | closed, merged | [REDACTED_OR_SUFFIX] | [STATUS] | [202/OTHER] | cleanup-only; no LLM review |

## PR Comment Upsert

```text
Marker: <!-- github-ai-reviewer:review-comment:v1 -->
Comment created after opened: [YES|NO]
Comment updated after synchronize: [YES|NO]
Duplicate marker comments observed: [NO|YES - count]
Comment URL or ID: [REDACTED_OR_SAFE_PUBLIC_URL]
Secret/private-source exposure observed in comment: [NO|YES - describe safely]
Inactive after closed-unmerged: [YES|NO|NOT_RUN]
Inactive after merged: [YES|NO|NOT_RUN]
History deleted or unrelated comments altered: [NO|YES - describe safely]
Closed PR `/ai-review` started normal review work: [NO|YES]
```

## Check Run

```text
Check Run name: AI Review
Head SHA: [REDACTED_OR_SHORT_SHA]
Conclusion after completed review: [neutral|success|failure|missing]
Failure caused by AI finding severity: [NO|YES]
Check Run URL or ID: [REDACTED_OR_SAFE_PUBLIC_URL]
Secret/private-source exposure observed in Check Run output: [NO|YES - describe safely]
New Check Run created solely for close/merge cleanup: [NO|YES]
```

## Leak Review

Check logs, PR comment, and Check Run output for these values. Record only pass/fail and safe notes, not the values themselves.

| Surface | Tokens/keys absent | Raw payload absent | Raw prompt/model output absent | Unintended private source absent | Notes |
| --- | --- | --- | --- | --- | --- |
| Service logs | [YES|NO] | [YES|NO] | [YES|NO] | [YES|NO] | [SAFE_NOTE] |
| PR comment | [YES|NO] | [YES|NO] | [YES|NO] | [YES|NO] | [SAFE_NOTE] |
| Check Run | [YES|NO] | [YES|NO] | [YES|NO] | [YES|NO] | [SAFE_NOTE] |

## Issues Found

| ID | Safe symptom | Suspected component | Repro outline | Fix commit | Verification status |
| --- | --- | --- | --- | --- | --- |
| M8-E2E-001 | [SAFE_SYMPTOM] | [COMPONENT] | [NO_SECRETS] | [SHA_OR_NA] | [OPEN|VERIFIED] |

## Final Result

```text
M8 E2E result: [PASS|FAIL|BLOCKED]
Blocked reason if any: [SAFE_REASON]
Follow-up proposal needed: [NONE|CHANGE_ID]
```
