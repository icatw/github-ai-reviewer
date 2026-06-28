## ADDED Requirements

### Requirement: Repo-aware review context construction
The worker SHALL build deterministic repo-aware review prompt context for supported PR review jobs after fetching changed file metadata and before requesting structured LLM review output.

#### Scenario: Prompt includes stable context sections
- **WHEN** a supported PR review job reaches prompt construction
- **THEN** the prompt context includes PR metadata and changed-file patch context
- **AND** the prompt context includes stable sections equivalent to `patch_context`, `full_file_context`, `related_test_context`, `repo_docs_context`, and `omitted_context`
- **AND** the LLM output boundary remains the structured `ReviewResult`

#### Scenario: Webhook remains fast
- **WHEN** a supported pull request webhook is handled
- **THEN** repo-aware context construction is not performed in the webhook handler
- **AND** the handler still returns after accepting the job without fetching repository file content or calling the LLM

### Requirement: Changed full file context
The worker SHALL fetch and include bounded head-version full file content for changed files when the files are safe to include and within implementation-defined budgets.

#### Scenario: Full changed file content is included
- **WHEN** a changed file is not deleted, is textual, is not filtered, and fits within per-file and total context budgets
- **THEN** the worker fetches the file content at the PR head SHA
- **AND** the full file content is included in `full_file_context`

#### Scenario: Deleted changed file is skipped
- **WHEN** a changed file has deleted status
- **THEN** the worker does not fetch head-version full file content for that file
- **AND** `omitted_context` records that the file was skipped because it was deleted

#### Scenario: Oversized full file is omitted or truncated
- **WHEN** a changed file exceeds the implementation-defined per-file content budget
- **THEN** the worker omits or truncates that file content deterministically
- **AND** `omitted_context` records the path and whether the file was oversized or truncated

### Requirement: Related test context selection
The worker SHALL discover and include bounded related test files using deterministic naming conventions without AST analysis, call graph analysis, full repository indexing, or vector search.

#### Scenario: Direct paired test is included
- **WHEN** a changed source file has a same-directory direct paired test file by naming convention such as `foo.go` to `foo_test.go`
- **THEN** the worker fetches the paired test file at the PR head SHA when it is safe and within budget
- **AND** the paired test content is included in `related_test_context`

#### Scenario: Same package tests are bounded
- **WHEN** a changed Go source file has additional same-package `*_test.go` files
- **THEN** the worker may include a deterministic bounded set of those test files
- **AND** excess same-package test candidates are skipped with `omitted_context` entries when candidate or context budgets are reached

#### Scenario: Related test candidates are deduplicated
- **WHEN** multiple changed files map to the same related test file
- **THEN** the worker includes that related test file at most once
- **AND** candidate ordering remains deterministic

### Requirement: Lightweight repo docs and config context
The worker SHALL include bounded lightweight repository documentation and AI review config context when present and safe to include.

#### Scenario: Root README is included
- **WHEN** `README.md` exists at the PR head SHA and fits within the applicable budgets
- **THEN** the worker includes it in `repo_docs_context`

#### Scenario: Docs markdown files are bounded
- **WHEN** markdown files exist under `docs/`
- **THEN** the worker includes only an implementation-defined deterministic bounded set of `docs/*.md` files that fit within budget
- **AND** skipped or truncated docs are represented in `omitted_context`

#### Scenario: AI review config is included when present
- **WHEN** `.github/ai-review.yml` exists at the PR head SHA and fits within budget
- **THEN** the worker includes it in `repo_docs_context`
- **AND** the config content is treated as context only unless a separate requirement defines executable config semantics

### Requirement: Deterministic context filters and budgets
The worker SHALL apply deterministic filters and implementation-defined per-file and total context budgets before sending repo-aware context to the LLM.

#### Scenario: Unsupported file categories are filtered
- **WHEN** candidate context files are binary, generated, lock files, under vendor paths, or under dist/build output paths
- **THEN** the worker skips those files
- **AND** `omitted_context` records the path and filter reason without including file content

#### Scenario: Total context budget is enforced
- **WHEN** candidate patch, full file, related test, docs, and config context exceeds the implementation-defined total context budget
- **THEN** the worker includes context in deterministic priority order until the budget is exhausted
- **AND** remaining candidates are skipped or truncated with `omitted_context` entries

#### Scenario: Context budget behavior is deterministic
- **WHEN** the same PR metadata, changed files, repository contents, and budget settings are processed repeatedly
- **THEN** the worker produces the same included context ordering and the same omitted-context notes

### Requirement: Omitted context reporting
The worker SHALL report omitted context in a stable prompt section so the LLM can describe limitations without fabricating unavailable evidence.

#### Scenario: Omitted context notes are included
- **WHEN** any file or candidate context is skipped, missing, truncated, oversized, filtered, or blocked by budget
- **THEN** `omitted_context` includes a concise note with the path, context category, and omission reason
- **AND** the note does not include secrets, tokens, complete webhook payloads, raw prompts, or raw model responses

#### Scenario: Fetch failures are non-fatal for optional context
- **WHEN** fetching optional full file, related test, docs, or config context fails
- **THEN** the worker records an omitted-context note for that candidate
- **AND** the review job may continue using the remaining available patch and repo context

### Requirement: Repo-aware context verification
The implementation SHALL include automated tests and real verification steps for repo-aware context construction and preserved M1-M3 output behavior.

#### Scenario: Automated verification covers context construction
- **WHEN** M4a implementation is complete
- **THEN** unit tests cover full file fetching
- **AND** unit tests cover related test selection
- **AND** unit tests cover docs/config selection
- **AND** unit tests cover filtering, truncation, total budget enforcement, and omitted-context notes

#### Scenario: Standard commands pass
- **WHEN** M4a implementation is complete
- **THEN** `gofmt -w .` has been run
- **AND** `go test ./...` passes
- **AND** `go build ./cmd/server` passes
- **AND** `openspec validate m4a-repo-aware-context --type change --strict` passes

#### Scenario: Real PR verification preserves existing behavior
- **WHEN** the service is deployed or restarted with M4a and a real supported PR event is processed
- **THEN** the resulting review prompt/context behavior includes richer repo-aware context or explicit omitted-context limitations
- **AND** the existing marker comment upsert behavior still works
- **AND** the existing advisory/non-blocking Check Run behavior still works
