## 1. Manifest and Checklist Templates

- [x] 1.1 Add a safe batch sampling manifest template that records PR targets, output paths, privacy/sanitization intent, sampling matrix tags, annotation status, and selection rationale without raw source content.
- [x] 1.2 Add a safe suite checklist/report template that records aggregate M16 metrics, per-fixture status, unresolved annotation TODOs, and tuning candidates without raw private code, secrets, raw prompts, or raw model output.
- [x] 1.3 Document gitignored quarantine locations for private or unsanitized generated fixtures and distinguish them from tracked sanitized public or synthetic testdata paths.

## 2. Sampling and Annotation Runbook

- [x] 2.1 Document the end-to-end workflow for choosing 10-20 PRs, preparing the manifest, generating fixtures with `cmd/review-bench-from-pr`, and preserving read-only GitHub behavior.
- [x] 2.2 Document the minimum sampling matrix covering auth/security, config/CI, API/backend, tests, docs-only, Go, Python/JS, clean/no-finding cases, and noisy/low-value cases.
- [x] 2.3 Document manual annotation steps for `golden_relevant_files`, `expected_findings`, `expected_no_findings`, `actual_findings`, quality labels, reviewer notes, and unresolved TODOs.
- [x] 2.4 Document how to run `cmd/review-bench` for the sampled suite, copy safe aggregate metrics into the checklist, and classify false positives, false negatives, duplicate findings, unsupported findings, too-generic findings, style-only findings, and other low-value findings.
- [x] 2.5 Document that this milestone gathers evidence only and does not tune prompts, context building, verification, reporters, dashboard/storage, or production webhook behavior.

## 3. Optional Helper Workflow

- [x] 3.1 If helper scripts or CLI glue are added, make them reuse the existing single-PR fixture generator behavior rather than duplicating GitHub fetching or benchmark logic. No helper scripts or CLI glue were added.
- [x] 3.2 If manifest parsing is added, validate required fields, privacy defaults, unsafe tracked output paths for private fixtures, duplicate fixture IDs, and safe checklist output. No manifest parsing was added.
- [x] 3.3 If no helper scripts or manifest parsing are added, explicitly record in the docs that the milestone is runbook/template-only and introduces no production code behavior changes.

## 4. Tests and Verification

- [x] 4.1 Add deterministic tests for any new helper scripts, manifest parser, command planning, unsafe path checks, or checklist rendering. No testable helper behavior was added.
- [x] 4.2 If implementation is docs/templates-only, state why no new Go tests are required.
- [x] 4.3 Run `gofmt -w .` if Go files changed. No Go files changed, so `gofmt` was not required.
- [x] 4.4 Run `go test ./...` if Go files or testable helper behavior changed. No Go files or testable helper behavior changed, so Go tests were not required.
- [x] 4.5 Run `go build ./cmd/server` if production Go files changed. No production Go files changed, so a server build was not required.
- [x] 4.6 Run `openspec validate m17-real-pr-fixture-sampling --type change --strict`.
