## 1. Context Builder Shape

- [ ] 1.1 Inspect current review orchestration, GitHub client abstractions, LLM prompt construction, and tests to identify the smallest integration point for repo-aware context.
- [ ] 1.2 Define typed context structures for `patch_context`, `full_file_context`, `related_test_context`, `repo_docs_context`, and `omitted_context`.
- [ ] 1.3 Define deterministic per-file, per-section, candidate-count, and total context budget constants or config defaults.
- [ ] 1.4 Add omitted-context note types with stable reason values for deleted, binary, generated, lock file, vendor/dist, oversized, truncated, budget exhausted, missing, and fetch error cases.

## 2. Full Changed File Context

- [ ] 2.1 Add a GitHub content reader path that fetches repository file content at the PR head SHA without logging raw content or tokens.
- [ ] 2.2 Include safe textual full content for changed non-deleted files within per-file and total budgets.
- [ ] 2.3 Skip deleted, binary, generated, lock, vendor, dist/build, and oversized changed files with omitted-context notes.
- [ ] 2.4 Add unit tests for full file fetch inclusion, deleted-file skipping, filtered file categories, oversized files, truncation, and fetch-error omission.

## 3. Related Test Context

- [ ] 3.1 Implement direct paired test discovery such as `foo.go` to `foo_test.go` using stable repository-relative paths.
- [ ] 3.2 Implement bounded same-package Go `*_test.go` discovery with deterministic ordering and deduplication.
- [ ] 3.3 Apply the same safe-content filters and budgets to related test files.
- [ ] 3.4 Add unit tests for paired test selection, same-package bounded selection, deduplication, filtering, truncation, total budget behavior, and omitted-context notes.

## 4. Repo Docs and Config Context

- [ ] 4.1 Select `README.md`, a deterministic bounded set of `docs/*.md`, and `.github/ai-review.yml` when present at the PR head SHA.
- [ ] 4.2 Treat `.github/ai-review.yml` as lightweight context only, without adding executable config semantics.
- [ ] 4.3 Apply safe-content filters and budgets to docs/config context.
- [ ] 4.4 Add unit tests for README inclusion, bounded docs selection, AI review config inclusion, missing files, truncation, and omitted-context notes.

## 5. Prompt Integration

- [ ] 5.1 Integrate the context builder into the worker review path after changed-file fetching and before LLM request construction.
- [ ] 5.2 Preserve existing PR metadata and changed-file patch context in prompts.
- [ ] 5.3 Render stable prompt sections for patch, full file, related test, repo docs/config, and omitted context.
- [ ] 5.4 Keep structured `ReviewResult` parsing, validation, comment marker upsert, and Check Run reporter behavior unchanged.
- [ ] 5.5 Ensure raw prompts, raw model responses, private repository payloads, tokens, API keys, and private keys are not logged.

## 6. Verification

- [ ] 6.1 Run `gofmt -w .`.
- [ ] 6.2 Run `go test ./...`.
- [ ] 6.3 Run `go build ./cmd/server`.
- [ ] 6.4 Run `openspec validate m4a-repo-aware-context --type change --strict`.
- [ ] 6.5 Deploy or restart the service.
- [ ] 6.6 Verify on a real PR that review prompt/context behavior includes richer repo-aware context or explicit omitted-context limitations.
- [ ] 6.7 Verify on the same real PR that marker comment upsert still works and Check Run reporter behavior remains advisory/non-blocking.
