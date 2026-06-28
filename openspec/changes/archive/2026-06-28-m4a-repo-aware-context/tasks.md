## 1. Context Builder Shape

- [x] 1.1 Inspect current review orchestration, GitHub client abstractions, LLM prompt construction, and tests to identify the smallest integration point for repo-aware context.
- [x] 1.2 Define typed context structures for `patch_context`, `full_file_context`, `related_test_context`, `repo_docs_context`, and `omitted_context`.
- [x] 1.3 Define deterministic per-file, per-section, candidate-count, and total context budget constants or config defaults.
- [x] 1.4 Add omitted-context note types with stable reason values for deleted, binary, generated, lock file, vendor/dist, oversized, truncated, budget exhausted, missing, and fetch error cases.

## 2. Full Changed File Context

- [x] 2.1 Add a GitHub content reader path that fetches repository file content at the PR head SHA without logging raw content or tokens.
- [x] 2.2 Include safe textual full content for changed non-deleted files within per-file and total budgets.
- [x] 2.3 Skip deleted, binary, generated, lock, vendor, dist/build, and oversized changed files with omitted-context notes.
- [x] 2.4 Add unit tests for full file fetch inclusion, deleted-file skipping, filtered file categories, oversized files, truncation, and fetch-error omission.

## 3. Related Test Context

- [x] 3.1 Implement direct paired test discovery such as `foo.go` to `foo_test.go` using stable repository-relative paths.
- [x] 3.2 Implement bounded same-package Go `*_test.go` discovery with deterministic ordering and deduplication.
- [x] 3.3 Apply the same safe-content filters and budgets to related test files.
- [x] 3.4 Add unit tests for paired test selection, same-package bounded selection, deduplication, filtering, truncation, total budget behavior, and omitted-context notes.

## 4. Repo Docs and Config Context

- [x] 4.1 Select `README.md`, a deterministic bounded set of `docs/*.md`, and `.github/ai-review.yml` when present at the PR head SHA.
- [x] 4.2 Treat `.github/ai-review.yml` as lightweight context only, without adding executable config semantics.
- [x] 4.3 Apply safe-content filters and budgets to docs/config context.
- [x] 4.4 Add unit tests for README inclusion, bounded docs selection, AI review config inclusion, missing files, truncation, and omitted-context notes.

## 5. Prompt Integration

- [x] 5.1 Integrate the context builder into the worker review path after changed-file fetching and before LLM request construction.
- [x] 5.2 Preserve existing PR metadata and changed-file patch context in prompts.
- [x] 5.3 Render stable prompt sections for patch, full file, related test, repo docs/config, and omitted context.
- [x] 5.4 Keep structured `ReviewResult` parsing, validation, comment marker upsert, and Check Run reporter behavior unchanged.
- [x] 5.5 Ensure raw prompts, raw model responses, private repository payloads, tokens, API keys, and private keys are not logged.

## 6. Verification

- [x] 6.1 Run `gofmt -w .`.
- [x] 6.2 Run `go test ./...`.
- [x] 6.3 Run `go build ./cmd/server`.
- [x] 6.4 Run `openspec validate m4a-repo-aware-context --type change --strict`.
- [x] 6.5 Deploy or restart the service.
- [x] 6.6 Verify on a real PR that review prompt/context behavior includes richer repo-aware context or explicit omitted-context limitations.
- [x] 6.7 Verify on the same real PR that marker comment upsert still works and Check Run reporter behavior remains advisory/non-blocking.
