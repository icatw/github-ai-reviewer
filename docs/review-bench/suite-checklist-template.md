# M17 Real PR Fixture Sampling Checklist

Use this checklist for safe batch-level reporting only. Do not paste raw private code, raw patches, raw prompts, raw model output, secrets, tokens, private keys, API keys, installation tokens, or complete webhook payloads into this file.

## Batch Metadata

- Batch ID:
- Manifest path:
- Fixture path pattern:
- Reviewer:
- Date:
- Fixture count:
- Privacy status: private quarantine / sanitized public / synthetic mix
- Safe to commit this checklist: yes / no

## M16 Aggregate Metrics

Copy safe aggregate values from `go run ./cmd/review-bench -fixtures '...'`.

| Metric | Value | Notes |
| --- | --- | --- |
| fixture_count |  |  |
| retrieved_file_count |  |  |
| golden_file_count |  |  |
| relevant_retrieved_count |  |  |
| omitted_relevant_count |  |  |
| precision |  |  |
| recall |  |  |
| f1 |  |  |
| finding_quality.annotated_fixture_count |  |  |
| finding_quality.expected_no_finding_fixture_count |  |  |
| finding_quality.not_annotated_fixture_count |  |  |
| finding_quality.expected_count |  |  |
| finding_quality.covered_count |  |  |
| finding_quality.missed_count |  |  |
| finding_quality.unexpected_count |  |  |
| finding_quality.duplicate_count |  |  |
| finding_quality.low_value_count |  |  |

## Matrix Coverage

| Required tag | Fixture IDs | Covered? | Notes |
| --- | --- | --- | --- |
| auth-security |  | no |  |
| config-ci |  | no |  |
| api-backend |  | no |  |
| tests |  | no |  |
| docs-only |  | no |  |
| go |  | no |  |
| python-or-javascript |  | no |  |
| clean-no-finding |  | no |  |
| noisy-low-value |  | no |  |

## Per-Fixture Status

| Fixture ID | Tags | Fixture path | Privacy/sanitization | Generation | Annotation | Benchmark status | TODOs |
| --- | --- | --- | --- | --- | --- | --- | --- |
| sample-001-auth-security-go | auth-security, api-backend, go | data/review-bench/private/sample-001-auth-security-go.json | private / unsanitized | not_started | not_started | not_run | Confirm expected finding boundaries. |
| sample-002-clean-docs-only | docs-only, clean-no-finding | data/review-bench/private/sample-002-clean-docs-only.json | public / pending_review | not_started | not_started | not_run | Confirm explicit expected_no_findings. |

## Quality Classification Summary

| Class | Count | Fixture IDs | Safe notes |
| --- | --- | --- | --- |
| false_positive |  |  |  |
| false_negative |  |  |  |
| duplicate |  |  |  |
| unsupported |  |  |  |
| too_generic |  |  |  |
| style_only |  |  |  |
| low_value |  |  |  |
| correct_actionable |  |  |  |
| needs_reviewer_decision |  |  |  |

## Candidate Tuning Themes

Record evidence-backed themes only after annotation. Keep this as a decision log, not a prompt or implementation patch.

| Theme | Evidence summary | Fixture IDs | Candidate area | Priority | Next action |
| --- | --- | --- | --- | --- | --- |
|  |  |  | prompt / context / verifier / reporter / analyzer / docs | low / medium / high |  |

## Open TODOs

- [ ] Resolve fixtures with missing `golden_relevant_files`.
- [ ] Resolve fixtures that should be `expected_no_findings`.
- [ ] Normalize captured `actual_findings` into deterministic safe fields.
- [ ] Review all private fixture paths for quarantine compliance.
- [ ] Decide which sanitized fixtures, if any, are safe to move into `testdata/review-bench/`.
