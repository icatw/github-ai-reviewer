# Real PR Fixture Sampling Runbook

M17 is a docs/templates-only milestone. It introduces no production webhook, reporter, Check Run, inline comment, prompt, context builder, verifier, dashboard, storage, SaaS, durable history, vector database, indexing, or live LLM judging behavior. It also requires no new Go tests because it adds no Go source, helper scripts, manifest parser, command planner, or runtime behavior.

Use:

- Manifest template: `docs/review-bench/real-pr-sampling-manifest.template.json`
- Checklist template: `docs/review-bench/suite-checklist-template.md`
- Existing generator: `go run ./cmd/review-bench-from-pr`
- Existing benchmark runner: `go run ./cmd/review-bench`

## 1. Choose 10-20 PRs

Start from an explicit operator-authored manifest. Do not auto-enumerate repositories or recent PRs. Select PRs that are safe for local fixture generation and useful for review-quality evidence.

Minimum matrix:

| Tag | Include at least |
| --- | --- |
| `auth-security` | Authentication, authorization, input validation, secrets handling, or security-sensitive changes. |
| `config-ci` | Workflow, build, deploy, lint, dependency, environment, or repository configuration changes. |
| `api-backend` | Handler, service, persistence, integration, or backend behavior changes. |
| `tests` | Test-only or test-heavy changes where review signal should be conservative. |
| `docs-only` | Documentation-only changes for clean/no-finding calibration. |
| `go` | Go code changes because this service has Go-aware evidence paths. |
| `python-or-javascript` | Python, JavaScript, or TypeScript changes for non-Go coverage. |
| `clean-no-finding` | PRs expected to produce no advisory findings. |
| `noisy-low-value` | PRs likely to trigger generic, style-only, duplicate, or unsupported findings. |

A single PR can cover multiple tags. Record a short `selection_rationale` for every entry without copying raw private implementation details.

## 2. Prepare the Manifest

Copy `real-pr-sampling-manifest.template.json` to a batch-specific path. The template is valid JSON so a future standard-library parser can consume it.

For each sample, record:

- `fixture_id`
- target `owner`, `repo`, and `pull`
- intended local output path
- privacy and sanitization intent
- matrix tags
- selection rationale
- generation status
- annotation status for `golden_relevant_files`, `expected_findings`, `expected_no_findings`, `actual_findings`, quality labels, reviewer notes, and unresolved TODOs

The manifest must not contain raw source code, raw patches, raw prompts, raw model output, secrets, tokens, private keys, API keys, installation tokens, complete webhook payloads, or private fixture bodies.

## 3. Quarantine and Sanitization Rules

Generated real-PR fixtures are unsanitized by default. Private or unsanitized fixtures stay in gitignored quarantine paths such as:

- `data/review-bench/private/`
- `review-bench-private/`
- `/tmp/review-bench/`

The repo already ignores `data/` and `review-bench-private/`. Do not move a private fixture into tracked `testdata/review-bench/` until a human sanitization pass confirms it contains no secrets, private keys, API keys, installation tokens, raw private repository content, or complete webhook payloads.

Only sanitized public or synthetic fixtures should be tracked. For a fixture that becomes safe to commit, set fixture metadata to a safe shape such as:

```json
"metadata": {
  "source": "sanitized-real-pr",
  "provenance": "owner/repo#123",
  "sanitized": true
}
```

## 4. Generate Fixtures Read-Only

Run the existing single-PR generator once per manifest entry:

```bash
go run ./cmd/review-bench-from-pr \
  -env-file .env.production \
  -owner OWNER \
  -repo REPO \
  -pull NUMBER \
  -out data/review-bench/private/FIXTURE_ID.json
```

This generator is read-only. It resolves the installation, fetches PR metadata and changed files, records repository files read by benchmark context building, and writes a local fixture. It does not publish GitHub comments, create Check Runs, submit PR reviews, run production review jobs, or modify the remote repository.

Update the manifest after each entry:

- `generation.status`: `generated`, `skipped_existing`, `blocked`, or `failed`
- `generation.generated_at`: date/time of generation
- `generation.notes`: safe operational notes only

Partial progress is expected. Do not regenerate completed fixtures unless the operator intentionally refreshes them.

## 5. Annotate Fixtures

Open each generated fixture locally and annotate deterministic benchmark fields. Keep raw private model output out of committed artifacts; normalize it into safe structured summaries when needed.

Populate `golden_relevant_files` with the files a reviewer expects context retrieval to include. Use repository-relative paths only.

For findings that should be caught, populate `expected_findings` with stable IDs and safe matching metadata:

```json
"expected_findings": [
  {
    "id": "auth-required",
    "file": "handler/user.go",
    "line": 42,
    "category": "security",
    "severity": "warning",
    "title": "auth check is skipped",
    "evidence_hints": ["RequireAuth"],
    "matching_hints": ["required authentication"]
  }
]
```

For clean PRs, declare intent explicitly:

```json
"expected_no_findings": true
```

Do not treat an empty `expected_findings` array as a clean case. Empty expected findings can also mean the fixture is context-only or not annotated.

When capturing review output, normalize `actual_findings` into deterministic safe fields such as `id`, `file`, `line`, `category`, `severity`, `title`, and safe `evidence_hints`. Do not paste raw prompts, raw model responses, private source snippets, or secrets.

Use `quality_annotations` and reviewer notes to classify outcomes. Keep unresolved questions visible in the manifest and checklist TODOs.

## 6. Run the Sampled Suite

Run the benchmark across the sampled fixtures:

```bash
go run ./cmd/review-bench -fixtures 'data/review-bench/private/*.json'
```

For a mixed suite, pass comma-separated paths or globs:

```bash
go run ./cmd/review-bench -fixtures 'testdata/review-bench/*.json,data/review-bench/private/*.json'
```

Copy only safe aggregate values into `suite-checklist-template.md`:

- context aggregate counts and precision/recall/F1
- `finding_quality.expected_count`
- `finding_quality.covered_count`
- `finding_quality.missed_count`
- `finding_quality.unexpected_count`
- `finding_quality.duplicate_count`
- `finding_quality.low_value_count`
- per-fixture status and TODO summaries

Do not copy raw code, raw prompts, raw model output, complete fixture bodies, secrets, or private patch text into the checklist.

## 7. Classify Quality Outcomes

Use deterministic labels:

| Label | Meaning |
| --- | --- |
| `false_positive` | An actual finding is not supported by the fixture evidence or is not a real issue for the PR. |
| `false_negative` | An expected finding was missed or insufficiently covered. |
| `duplicate` | Multiple actual findings describe the same issue; set `duplicate_of` when possible. |
| `unsupported` | The finding needs evidence unavailable in the fixture or outside the current benchmark scope. |
| `too-generic` | The finding is broadly true but not specific enough to guide a reviewer. |
| `style-only` | The finding is stylistic and not worth advisory output for this PR. |
| `low-value` | The finding may be correct but would distract more than help. |
| `correct-actionable` | The finding is supported, specific, and useful. |
| `needs-reviewer-decision` | The reviewer could not classify it without more domain knowledge. |

Record unresolved classifications as TODOs. Do not hide ambiguous cases by forcing a pass/fail label.

## 8. Decide What to Tune Next

After the first annotated batch, group issues by evidence-backed theme before changing behavior:

- Missed relevant files: candidate context retrieval or budget tuning.
- Correct context but missed expected finding: candidate prompt, analyzer evidence, or verifier tuning.
- Unsupported or hallucinated findings: candidate verifier, prompt, or evidence policy tuning.
- Duplicate findings: candidate result normalization or reporter deduplication.
- Too-generic, style-only, or low-value findings: candidate severity/category policy or prompt tuning.
- Clean PRs with unexpected findings: candidate no-finding calibration.

Do not tune during M17. The output of this milestone is the sampled corpus, safe checklist, and a prioritized list of candidate tuning themes for a later change.
