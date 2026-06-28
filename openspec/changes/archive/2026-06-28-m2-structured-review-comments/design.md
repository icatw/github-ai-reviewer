## Context

M1 currently sends PR diff context to the LLM, accepts a free-form string, renders that string into a Markdown summary, and creates a new PR conversation comment for every supported PR event. That proves the loop but leaves two problems for real repositories: model output has no typed contract, and repeated `synchronize` events can clutter a PR with duplicate bot comments.

This change spans the LLM boundary, review orchestration, Markdown rendering, and GitHub comment publishing. It must preserve M1 constraints: webhook responses stay fast, secrets are not logged, and AI output remains advisory.

## Goals / Non-Goals

**Goals:**

- Define a typed review result contract for summary, risk score, findings, missing tests, and limitations.
- Validate and normalize model output before rendering.
- Render deterministic Markdown from typed data only.
- Include a stable hidden marker in every bot review comment.
- Update the previous marker comment for a PR when present, otherwise create one.
- Keep all review work in the existing worker path.

**Non-Goals:**

- GitHub Check Runs or failing PR checks.
- Inline PR review comments.
- Durable SQLite job storage.
- Slash-command interaction through issue comments.
- Full repository indexing, AST analysis, static-analysis integration, or finding verification.

## Decisions

1. Use an internal typed review result as the boundary between LLM and comments.

   The LLM client or review package should parse provider text into a typed `ReviewResult` before comment rendering. This keeps Markdown generation deterministic and prevents raw model prose from becoming the published contract. An alternative was to keep the existing string boundary and ask the renderer to parse ad hoc Markdown, but that would preserve the M1 ambiguity M2 is meant to remove.

2. Request JSON-only output through prompting, with tolerant parsing for common wrappers.

   The prompt should require one JSON object matching the review result schema. Parsing should tolerate surrounding whitespace and Markdown JSON fences because many OpenAI-compatible providers still return fenced content despite instructions. Malformed JSON, empty choices, and schema validation failures should stop the job without publishing. An alternative was to rely on provider-specific structured output APIs, but this service targets OpenAI-compatible providers and should not require a provider-specific feature in M2.

3. Treat validation failures as publish-stopping, not publish-degrading.

   Invalid severities, out-of-range risk scores, out-of-range confidence values, and missing useful content should fail safely. The service should log safe error categories but should not invent a fallback review comment. This follows the project accuracy policy and avoids publishing fabricated confidence.

4. Render comments from typed data with a stable hidden marker.

   The marker should be a fixed HTML comment owned by this service, such as `<!-- github-ai-reviewer:review-comment:v1 -->`. It must not contain secrets, tokens, repo payload data, or model output. The marker lets the publisher find its own previous comment while ignoring human comments and unrelated bot comments.

5. Upsert PR conversation comments through the GitHub Issues comments API.

   Publishing should list issue comments for the PR, find the first comment containing the stable marker, and update that comment. If none exists, it should create a new issue comment. The webhook handler should not perform comment listing or retries inline. An alternative was to store the last comment ID, but durable storage is out of scope and marker search is sufficient for M2.

## Risks / Trade-offs

- Provider-specific JSON behavior may vary -> Keep parsing tolerant for fences and whitespace, and fail closed on invalid content.
- Marker collision is possible if copied manually -> Use a service-specific versioned marker and update only comments containing that marker.
- Listing comments adds one GitHub API call per review publish -> Accept for M2 simplicity; later storage can avoid repeated search.
- Updating the first marker comment can leave older duplicate M1 comments untouched -> M2 prevents new duplicates but does not require cleanup of existing history.
- Strict validation may suppress some useful-but-invalid model output -> This is preferable to publishing untrusted or ambiguous review data.

## Migration Plan

No data migration is required. Existing M1 comments without the hidden marker remain untouched. The first M2 review event creates a marker comment; subsequent supported PR events update that marker comment.

Rollback is to redeploy the M1 behavior. Marker comments are ordinary PR conversation comments and do not require schema cleanup.

## Open Questions

- Whether the renderer should cap the number of findings in M2 or render all validated findings. The implementation can choose a conservative cap if tests document it.
