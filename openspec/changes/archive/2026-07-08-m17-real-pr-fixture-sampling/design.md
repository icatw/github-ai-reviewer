## Context

The project now has an offline review-quality benchmark suite from M16. Existing commands include `cmd/review-bench-from-pr` for generating a single real-PR fixture with read-only GitHub App access and `cmd/review-bench` for replaying fixtures and reporting context plus deterministic finding-quality metrics. Fixtures can represent `golden_relevant_files`, `expected_findings`, `expected_no_findings`, `actual_findings`, quality annotations, and sanitization metadata.

The next product step is not tuning. It is collecting a representative set of 10-20 real PR fixtures, annotating them manually, and using the M16 metrics to identify the dominant quality problems. This change defines the workflow and optional helper surface for that sampling milestone while keeping private code quarantined until intentionally sanitized.

## Goals / Non-Goals

**Goals:**

- Define a manifest-driven batch sampling workflow for real PR fixtures.
- Keep fixture generation read-only and reuse the existing single-PR generator behavior.
- Provide a safe manifest shape for repository/PR targets, output locations, privacy status, sampling tags, and annotation status.
- Define manual annotation steps for relevant files, expected findings, expected no-finding cases, actual finding capture, quality labels, and reviewer notes.
- Provide a suite report/checklist artifact that records aggregate M16 metrics and per-fixture TODOs without embedding raw private code, secrets, raw prompts, or raw model output.
- Require a minimum sampling matrix that includes risky, clean, noisy, and language-diverse PRs.
- Document the complete runbook from PR selection through tuning decision.

**Non-Goals:**

- No production webhook, Check Run, PR summary, inline comment, verifier, context builder, or prompt behavior changes.
- No dashboard, SaaS workflow, durable review history, tenant management, vector database, or full repository indexing.
- No benchmark requirement for live LLM judging.
- No committing unsanitized private repository fixture contents.
- No tuning of prompts, context retrieval, verification, or reporting in this milestone.

## Decisions

### Batch sampling starts from an explicit manifest

The workflow should require an operator-authored manifest rather than discovering candidate PRs automatically. Each entry should identify the owner, repo, pull number, intended output path, privacy/sanitization intent, sample dimensions, and annotation status. This makes the selected corpus auditable and prevents broad repository enumeration.

Alternative considered: auto-sample from recent installed repositories. That would be faster but creates privacy and representativeness risks, and it could fetch PRs the operator did not intend to preserve as fixtures.

### Private fixture outputs stay in quarantine by default

Manifest examples and helper defaults should write private or unsanitized fixtures to gitignored paths such as `data/review-bench/private/` or a temporary directory. Moving a fixture under tracked `testdata` should require explicit sanitization metadata and human review.

Alternative considered: write all generated fixtures under testdata and rely on code review. That makes accidental private fixture commits too likely.

### The manifest records sampling intent, not raw content

Manifest and checklist artifacts may contain repository identifiers, PR numbers, categories, languages, safe file paths, annotation status, aggregate metric counts, and TODOs. They must not embed raw source snippets, secrets, raw prompts, raw model responses, installation tokens, private keys, complete webhook payloads, or private patch bodies.

Alternative considered: include short source snippets in notes for reviewer convenience. Public fixtures may carry sanitized evidence in fixture annotations, but the batch manifest/checklist must remain safe to share and review.

### Annotation remains manual and deterministic

Reviewers should annotate the 10-20 fixtures using the M16 schema and deterministic quality labels. The workflow should classify missed expected findings, unexpected findings, duplicates, unsupported findings, too-generic findings, style-only findings, and clean/no-finding expectations without relying on an LLM judge as a required gate.

Alternative considered: use a model to label false positives and false negatives. That may be useful later as advisory assistance, but the first sampling corpus needs a stable human-reviewed baseline.

### Implementation can be docs-first, with optional helper scripts

This milestone can be satisfied by documentation, manifest/checklist templates, and disciplined runbook updates if existing commands are sufficient. If implementation adds helper scripts or manifest parsing, tests should cover manifest validation, path safety, privacy defaults, and command planning without touching production review behavior.

Alternative considered: build a full new benchmark subcommand immediately. That may be useful after the workflow stabilizes, but it is not necessary to start collecting quality evidence.

## Risks / Trade-offs

- Private code leakage -> Mitigate with gitignored quarantine paths, explicit sanitization markers, safe manifest/checklist rules, and documentation that private fixtures cannot be committed until sanitized.
- Unrepresentative fixture set -> Mitigate with a minimum sampling matrix and per-fixture tags for domain, language, expected signal, and noise level.
- Annotation inconsistency -> Mitigate with a step-by-step runbook, deterministic labels, reviewer notes, and checklist TODOs for unresolved cases.
- Helper scripts drift from existing commands -> Mitigate by reusing `review-bench-from-pr` and `review-bench` behavior rather than duplicating GitHub or benchmark logic.
- Evidence gathering delays tuning -> Accept the delay; tuning without a representative annotated corpus risks optimizing for anecdotal failures.

## Migration Plan

No runtime migration is required. Implementation should add docs/templates and any optional helper scripts in a way that preserves all existing fixture formats and benchmark commands. Rollback is source-only: remove the new runbook/templates/helper script and continue using the M16 single-fixture lifecycle.

## Open Questions

- Should the first batch manifest format be JSON for standard-library parsing or YAML for operator readability?
- Should sanitized public fixtures be copied into a new tracked sample corpus immediately, or should M17 first produce the private checklist and only commit synthetic examples?
- Which exact 10-20 PRs should the operator select for the initial batch?
