## Why

M16 created the offline review-quality benchmark mechanics, but the project still lacks a disciplined way to collect and annotate a representative batch of real PR fixtures. Before tuning prompts, context selection, verification, reporters, or future storage, the team needs 10-20 sampled fixtures that expose false positives, false negatives, and low-value review patterns without risking private code leakage.

## What Changes

- Add a formal real-PR batch sampling workflow that can generate multiple fixtures from an explicit manifest/list using the existing read-only GitHub App fixture generator behavior.
- Define a safe manifest format for target PRs, output paths, privacy/sanitization intent, sample dimensions, and annotation status.
- Add a manual annotation workflow for fixture metadata, `golden_relevant_files`, `expected_findings`, `expected_no_findings`, `actual_findings`, quality labels, and reviewer notes.
- Add a suite report/checklist artifact that records aggregate M16 metrics and per-fixture TODOs using safe metadata only.
- Formalize quarantine and sanitization rules: unsanitized private fixtures remain in gitignored locations; only sanitized public or synthetic fixtures may move under tracked testdata.
- Define a minimum sampling matrix covering auth/security, config/CI, API/backend, tests, docs-only, Go, Python/JS, clean/no-finding cases, and noisy/low-value cases.
- Document a runbook for choosing PRs, generating fixtures, sanitizing, annotating, running the suite, classifying quality outcomes, and deciding what to tune next.
- No production review behavior changes are introduced. Tests are only required if implementation adds helper scripts or manifest parsing.

## Capabilities

### New Capabilities
- `real-pr-fixture-sampling`: Covers batch selection, generation, annotation, reporting checklist, and quarantine/sanitization workflow for real PR benchmark fixtures.

### Modified Capabilities
- `review-quality-benchmark-suite`: Extends the benchmark suite requirements with batch sampling workflow integration, manifest-driven fixture generation expectations, manual annotation workflow expectations, and safe suite checklist reporting.

## Impact

- Affected areas are expected to be benchmark docs/runbooks, optional helper scripts or CLI glue around `cmd/review-bench-from-pr` and `cmd/review-bench`, optional manifest parsing tests if helper code is added, and OpenSpec specs.
- No production webhook, reporter, Check Run, inline comment, LLM prompt, verifier, context builder, dashboard, storage, or merge policy behavior changes.
- No new dependency is required unless implementation chooses a manifest parser not already available in the Go standard library.
