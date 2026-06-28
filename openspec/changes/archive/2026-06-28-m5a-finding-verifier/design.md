## Context

The service already verifies webhooks, builds deterministic repo-aware PR context, asks the LLM for structured `ReviewResult` output, renders a stable marker summary comment, and reports advisory Check Runs. The remaining accuracy gap is that a structurally valid finding can still be unsupported by the evidence available to the service, point at a file or line outside the available PR context, or rely on content that was explicitly omitted.

M5a adds a bounded verifier between LLM result parsing and reporter fan-out. It is not a repository intelligence system. It uses only the evidence already collected for the job: changed-file metadata and patches, full changed-file context, related tests, repo docs/config, and omitted-context notes.

## Goals / Non-Goals

**Goals:**

- Verify structured findings before comments or Check Runs receive the result.
- Preserve valid findings and existing advisory output behavior.
- Drop or downgrade unsupported findings deterministically with reason categories.
- Keep metrics/logging safe by emitting counts and categories only.
- Add eval/test fixtures for representative supported and unsupported cases.
- Keep a clear extension point for future static-check evidence.

**Non-Goals:**

- No AST, tree-sitter, call graph, symbol index, vector database, or full repository indexing.
- No inline review comments, slash commands, issue_comment handling, durable storage, request-changes behavior, failing Check Runs due to AI findings, or merge-blocking policy.
- No execution of `go test`, `go vet`, `staticcheck`, `gosec`, `semgrep`, or other analyzers in M5a.

## Decisions

### Verifier placement

Run verification after the LLM returns a parsed and normalized `ReviewResult` and before `ReviewCompleted` reporter fan-out. This keeps LLM parsing concerns separate from accuracy filtering and ensures every output channel receives the same verified result.

Alternatives considered:

- Verify inside comment rendering: rejected because Check Run output could diverge from comments.
- Verify in the LLM client: rejected because the verifier needs repo context and is not provider-specific.

### Evidence model

Build an in-memory evidence index from the `RepoContext` used to build the prompt. The initial evidence sources are:

- `patch_context` from changed-file metadata and patch text.
- `full_file_context` for bounded changed files fetched at the PR head SHA.
- `related_test_context` for deterministic related test files.
- `repo_docs_context` for README/docs/config context.
- `omitted_context` for explicit limitations.

The verifier should treat future static-check results as another evidence source through an interface or typed source category, but M5a does not populate that source.

### Finding outcomes

Each finding receives one outcome:

- `kept`: file/path, optional line, and evidence are supported by available context.
- `downgraded`: the finding contains some useful support but overstates certainty or relies partly on limited context. Downgrade to `question` and add a limitation rather than publishing it as a stronger advisory.
- `dropped`: the finding has no useful support, references unavailable files, has impossible/unavailable line information, or depends primarily on omitted/unavailable context.

The verifier returns a new `ReviewResult` plus `VerificationStats`; it does not mutate raw LLM output in place. If all findings are dropped, the summary, missing tests, and limitations can still render when useful. If nothing useful remains, existing output suppression behavior applies.

### Reason categories

Use a small fixed set of reason categories for deterministic tests and safe logging:

- `supported`
- `unsupported_evidence`
- `unavailable_file`
- `line_unavailable`
- `line_mismatch`
- `omitted_context_dependency`
- `no_findings`

Counts are logged with delivery, repo, pull number, totals, and reason categories only. Logs must not include raw prompts, raw model output, tokens, secrets, webhook payloads, or private code content.

### Matching rules

The verifier should keep the first implementation intentionally simple:

- File paths must match a changed file or included context path after normalization.
- Findings with a line must be checked against available line ranges when those ranges can be derived from patch hunks or full-file line counts.
- Evidence text must overlap with available context in a bounded deterministic way, such as exact or normalized substring matching against patch/full/test/docs context.
- Findings referencing omitted paths or omitted categories should be downgraded to `question` when they raise a plausible limitation, or dropped when they assert a concrete defect that cannot be verified.

This avoids brittle semantic matching while still blocking obvious hallucinations.

## Risks / Trade-offs

- False negatives from simple text matching -> Mitigation: downgrade partially supported findings to `question` instead of dropping when they identify a real limitation but lack enough evidence for a stronger severity.
- Patch line parsing complexity -> Mitigation: accept conservative line validation; if line availability is ambiguous, downgrade or require file-level evidence rather than inventing precision.
- Reduced comment volume may look like missing review coverage -> Mitigation: keep limitation notes and stats so suppressed/downgraded findings are observable without exposing code.
- Verifier could duplicate renderer policy -> Mitigation: keep verifier focused on finding validity and let existing comment/Check Run reporters handle presentation and advisory policy.

## Migration Plan

1. Add verifier types and pure verification functions behind the review package boundary.
2. Wire verification into `Service.Process` before `ReviewCompleted` reporter fan-out.
3. Add safe stats logging and unit/eval fixtures.
4. Preserve current output behavior for verified findings and rollback by bypassing the verifier call if needed during deployment.

## Open Questions

- Whether downgraded findings should retain original severity in internal stats only or also annotate the published finding. M5a should prefer not publishing raw original severity unless a clear user-facing need appears.
- Whether summary text should be adjusted when all findings are dropped. M5a can preserve summary text initially, relying on existing suppression only when the whole result has no useful content.
