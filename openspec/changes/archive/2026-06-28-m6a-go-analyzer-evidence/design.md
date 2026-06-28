## Context

The service already processes supported PR webhook jobs asynchronously, builds bounded repo-aware review context, requests a structured `ReviewResult`, verifies findings against available evidence, and reports through comment and Check Run reporters. M5b added deterministic verifier outcomes, safe aggregate stats, source-compatible matching, and an `EvidenceSourceStaticCheck` extension point, but M5 intentionally did not run external analyzers.

M6a introduces a narrow analyzer evidence slice for Go projects only. The key constraint is that PR code is untrusted: running `go test ./...` or `go vet ./...` can execute or load code, trigger module downloads, consume CPU, and produce private repository output. The design therefore treats analyzer execution as optional, bounded, and advisory.

## Goals / Non-Goals

**Goals:**

- Run `go test ./...` and `go vet ./...` only when repository context indicates a Go project and a safe bounded local workspace is available.
- Place analyzer execution after PR/repo context collection and before finding verification.
- Convert analyzer outcomes into bounded static-check evidence for verifier support.
- Preserve webhook responsiveness, comment marker upsert, reporter fan-out, output suppression, safe logging, and advisory Check Run behavior.
- Make skipped, unavailable, failed, or timed-out analyzer execution visible as safe limitation evidence rather than job-blocking failure.

**Non-Goals:**

- No non-standard analyzers or arbitrary commands.
- No language support beyond Go.
- No AST, tree-sitter, call graph, symbol index, vector DB, durable storage, dashboard, inline comments, slash commands, request-changes behavior, or automatic fixes.
- No merge-blocking or failing Check Runs based on analyzer or AI findings.
- No unbounded clone/fetch strategy.

## Decisions

1. Add an internal analyzer abstraction before implementing execution details.

   The worker should call a small analyzer interface that accepts review job metadata plus the already collected repository/workspace context and returns analyzer evidence plus safe stats. This keeps verifier integration testable without requiring real command execution in every test.

   Alternative considered: call `go test` and `go vet` directly inside the worker. Rejected because it couples orchestration, workspace safety, parsing, timeout handling, and verifier evidence construction in one place.

2. Gate execution on both Go project detection and workspace safety.

   The analyzer should only plan commands when repo-aware context indicates a Go project, such as a safely available `go.mod`, changed Go files, or existing bounded repo context that identifies Go module/project layout. It must also require a local workspace rooted inside an implementation-controlled directory, pinned to the PR head SHA/ref being reviewed, with safe path validation and cleanup rules. If the implementation cannot safely provide this workspace in M6a, it should implement planner/parser/evidence integration and return a skipped limitation for real execution.

   Alternative considered: clone or fetch the full repository on demand. Rejected unless bounded by ref, size/depth strategy, timeout, credential handling, cleanup, and path constraints because untrusted PR code and private repository data require stricter controls.

3. Restrict commands to literal standard Go tool invocations.

   M6a may plan only:

   ```text
   go test ./...
   go vet ./...
   ```

   Commands must use fixed argv arrays, not shell interpolation. Working directory must be the safe workspace root or a validated module root under it. Environment should be minimal and must not include GitHub installation tokens, LLM API keys, webhook secrets, or private keys.

   Alternative considered: configurable analyzer commands. Rejected because it would turn M6a into a general CI runner and increase security risk.

4. Treat analyzer result categories as evidence, not job status.

   Analyzer output should be represented with safe categories such as `skipped`, `unavailable`, `success`, `failure`, `timeout`, or `internal_error`. Non-zero exits and timeouts are valid analyzer outcomes and should not stop LLM review, verifier execution, comment publishing, or advisory Check Run completion.

   Alternative considered: fail the review job when Go tools fail. Rejected because failing tests may be the evidence under review, and M6a must remain advisory.

5. Store/pass only bounded sanitized summaries.

   Analyzer parsing should extract only structured fields needed for verification: source type `static_check_context`, tool name, package, file, line when available, sanitized message, exit category, and omitted/limitation notes. Raw stdout/stderr must be bounded before parsing, sanitized, and never logged or emitted to PR comments or Check Run output unbounded.

   Alternative considered: include raw tool output in verifier context. Rejected because private repository output can include source snippets, paths, secrets, and large logs.

## Risks / Trade-offs

- Running PR code may execute untrusted tests -> Mitigation: keep analyzer optional, require safe workspace gating, fixed commands, timeout, minimal environment, output limits, cleanup, and no secret propagation.
- `go test ./...` can download modules or be slow -> Mitigation: bounded timeout and output size; record timeout or unavailable evidence instead of blocking review.
- Parser coverage for Go tool output will be incomplete -> Mitigation: parse conservative file/line/package patterns where available and preserve only safe sanitized messages; keep aggregate categories deterministic.
- Static-check evidence could over-support weak AI findings -> Mitigation: verifier must apply existing evidence compatibility and conservative matching rules; static-check context does not override unsupported finding rules by itself.
- Local workspace strategy may not be safe enough in M6a -> Mitigation: implement the analyzer interface, planner, parser, and evidence integration with real execution skipped until bounded checkout/workspace constraints are satisfied.
