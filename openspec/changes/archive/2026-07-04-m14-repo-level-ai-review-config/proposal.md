## Why

The service has grown beyond a single global review policy: it now supports summary comments, advisory Check Runs, manual `/ai-review`, repo-aware context, finding verification, optional Go analyzer evidence, and gated inline PR review comments. Repository owners need a safe, versioned way to disable or tighten those outputs per repository without asking the service operator to change global environment configuration.

Public project metadata also still describes several already-implemented capabilities as future work, which makes the repository harder to evaluate and can lead contributors toward stale M1 assumptions.

## What Changes

- Add runtime discovery of `.github/ai-review.yml` or `.github/ai-review.yaml` for each supported review job, read from the PR repository/ref when safe and available.
- Treat missing repository config as normal and continue with service defaults.
- Treat invalid repository config as a safe non-blocking configuration limitation, fall back to service defaults, and report the limitation without exposing raw private file contents.
- Define an initial conservative repository config schema:
  - `enabled`
  - `language`
  - `summary_comment.enabled`
  - `check_run.enabled`
  - `inline_comments.enabled`
  - `inline_comments.max_comments`
  - `inline_comments.severity_threshold`
  - `inline_comments.confidence_threshold`
  - `path_ignore`
  - `go_analyzer.enabled`
- Merge repository config with global service configuration so global environment settings remain the upper safety boundary: repo config may disable features or tighten thresholds/limits, but it must not enable globally disabled Check Runs, inline comments, workspace/analyzer behavior, checkout behavior, or any blocking policy.
- Apply the effective config before LLM language/output decisions, reporter fan-out, inline eligibility and limits, optional Go analyzer execution, and changed-file/context path filtering where implemented in M14.
- Preserve the existing advisory policy: repository config must not make AI findings request changes, auto-fix, auto-merge, fail merge gates, or otherwise block merges.
- Update public README roadmap/current capabilities, AGENTS.md milestone guidance, and documentation for `.github/ai-review.yml` so completed features are not presented as future work.
- Decide that local `config/mcporter.json` is local tooling metadata and should be ignored by git if present; the implementation may add a narrow ignore entry without otherwise changing or documenting that local tool.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `github-app-review-loop`: Define repository-level AI review config discovery, parsing, effective-config safety boundaries, runtime integration points, invalid/missing config behavior, and verification requirements.
- `open-source-readiness`: Require public README, agent guidance, and repository config documentation to reflect implemented capabilities and current safety boundaries.

## Impact

- Affected implementation areas likely include GitHub content fetching, review job orchestration, typed config parsing/merging, LLM prompt options, reporter selection, inline comment gating, optional Go analyzer selection, context/path filtering, comment/check output of safe limitations, tests, README, AGENTS.md, and git ignore rules.
- No new GitHub App permissions are expected beyond existing Contents read, Pull requests write, Issues write, and Checks write permissions.
- A small YAML parsing dependency may be introduced if no suitable dependency already exists; schema parsing must remain strict enough to catch invalid types and unsupported values.
- No dashboard, billing, tenant management, durable review history, vector database, full repository indexing, arbitrary CI execution, auto-fix, request-changes behavior, auto-merge, or AI-finding-derived blocking policy is introduced.
