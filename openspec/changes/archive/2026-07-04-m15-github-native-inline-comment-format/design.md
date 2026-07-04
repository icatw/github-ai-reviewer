## Context

M12 moved newly eligible inline findings into a submitted Pull Request Review, but the rendered inline body still mirrors the full summary report structure. The current body leads with the hidden inline marker/fingerprint and then lists evidence, failure scenario, and suggestion as top-level bullets. That is useful for machine idempotency, but it makes GitHub API readbacks noisy and makes the PR UI feel less like a normal reviewer note.

The current implementation already has the right behavioral boundaries: inline comments are gated, mapped to RIGHT-side diff lines, batched through Create Review for new comments, updated individually for existing fingerprints, and marked stale or inactive non-destructively. This change is a formatting and compatibility pass, not a new review behavior.

## Goals / Non-Goals

**Goals:**

- Make inline PR review comments compact, human-facing, and line-local by default.
- Keep evidence, failure scenario, and confidence available in a collapsed `<details>` block.
- Preserve marker/fingerprint idempotency after moving hidden metadata to the end of the body.
- Keep old leading-marker inline bodies discoverable for update, stale, inactive, and cleanup paths.
- Make the Pull Request Review body sound like a GitHub review bot note and keep the advisory policy clear.
- Remove stale milestone wording from summary comment advisory text.
- Localize fixed English/Chinese labels consistently with the effective review language.

**Non-Goals:**

- Do not change inline eligibility, severity/confidence thresholds, line mapping, batching, fallback, stale handling, cleanup, repository config, analyzer behavior, Check Runs, summary upsert semantics, or LLM prompt scope.
- Do not introduce GitHub suggested-change blocks, auto-fix, request-changes reviews, failing checks, merge gates, durable storage, or new GitHub permissions.
- Do not remove markers or fingerprints from PR-facing comments.

## Decisions

### Render visible inline content before machine metadata

Inline comment bodies will start with a localized severity label and a concise title/explanation. The hidden marker/fingerprint will move to the final lines of the body, after the visible reviewer note and `<details>` block.

Alternative considered: keep the marker first and rely on GitHub hiding comments. That preserves current extraction but leaves API/readback output noisy and risks exposing implementation metadata first in renderers that do not hide HTML comments.

### Use a compact visible structure with collapsed details

The default visible body should contain:

- a severity label such as `🚨 Blocking risk` or `⚠️ Potential issue`;
- a one-sentence finding title or explanation;
- a short actionable suggestion.

Evidence, failure scenario, and confidence will move under `<details><summary>Details</summary>...</details>`. This preserves traceability without forcing every line comment to read like a full report. If confidence is absent, the details block omits the confidence row rather than fabricating one.

Alternative considered: drop evidence/failure scenario from inline comments entirely because the summary comment remains the full report. That would make inline comments shorter, but it weakens traceability and would discard already-verified PR-facing finding data.

### Keep extraction marker-position agnostic

Marker and fingerprint extraction will scan the full comment body and continue to require this service's inline marker plus a valid fingerprint before treating a comment as bot-owned. Tests should cover both the old leading-marker body and the new trailing-marker body. Stale and inactive renderers should preserve or append marker/fingerprint data so future runs can still discover the comment.

Alternative considered: migrate only new comments and ignore old leading-marker comments. That would strand existing inline comments and break stale/update behavior on active PRs.

### Localize fixed renderer text without touching model content

Fixed labels supplied by the renderer, including severity labels, suggestion/detail labels, review body text, stale/inactive notes where touched, and summary advisory text, should follow the effective language. The finding title, evidence, failure scenario, and suggestion remain model/result content and are not translated by the renderer.

### Keep summary as the full report

The summary issue comment remains the complete report and keeps its existing marker/upsert/footer purpose. The only summary cleanup in scope is milestone-neutral wording, especially replacing stale English text such as `Findings are advisory and non-blocking in this M2 review.` with current advisory language.

## Risks / Trade-offs

- Hidden marker moved to the end could break idempotency if extraction assumes the marker is first -> make extraction position-agnostic and add old/new body tests.
- `<details>` markdown can render differently across clients -> use simple GitHub-supported HTML details markup with plain markdown content inside.
- Severity labels with emoji improve scanability but may complicate exact assertions -> centralize label mapping and assert stable prefixes in renderer tests.
- More compact inline comments may omit context reviewers expect at first glance -> keep details collapsed and the summary comment as the full report.
- Localized fixed labels can drift from English behavior -> add zh-CN tests for review body and inline labels.
