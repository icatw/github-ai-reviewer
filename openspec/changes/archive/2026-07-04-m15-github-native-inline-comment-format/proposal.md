## Why

Inline Pull Request Review comments now work functionally, but their body format still reads like a copied summary report instead of a GitHub-native line-level reviewer note. Moving the machine marker out of the lead position and making each inline comment concise improves PR readability while preserving idempotency, stale handling, and the existing advisory safety policy.

## What Changes

- Reformat inline PR review comment bodies so visible content starts with a human-facing severity label, followed by a concise finding explanation and directly actionable suggestion.
- Keep the hidden inline marker and fingerprint for idempotency, but move them after visible content so API/readback noise does not dominate the comment body.
- Move evidence, failure scenario, and confidence into a collapsed `<details>` block to keep default inline comments compact while retaining traceability.
- Update the submitted Pull Request Review body to read like a GitHub review note and clearly state that findings are advisory and non-blocking.
- Localize inline severity labels, detail labels, suggestions, and review body fixed text when the effective review language is `zh-CN`.
- Clean up stale historical summary wording such as `M2 Review` from the summary comment advisory text while preserving the summary as the full report and preserving its marker/footer purpose.
- Preserve all existing inline eligibility, quality thresholds, diff mapping, batching, update/stale behavior, cleanup semantics, repository config behavior, analyzer behavior, Check Runs, and summary upsert behavior.
- Preserve backward compatibility so existing inline comments whose hidden marker appears at the beginning can still be detected, updated, and marked stale or inactive.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `github-app-review-loop`: Refine advisory summary and inline Pull Request Review output formatting while preserving marker/fingerprint idempotency, existing inline publication behavior, cleanup behavior, and non-blocking policy.

## Impact

- Affected implementation areas likely include `internal/comment` inline rendering, review body rendering, summary comment rendering text, inline marker/fingerprint extraction, and focused tests around renderer output and existing-comment detection.
- No new GitHub App permissions, runtime configuration, durable storage, GitHub API endpoints, LLM prompt scope, analyzer execution, or reporter channels are expected.
- Existing GitHub comments remain compatible because marker/fingerprint extraction must scan the full body and tolerate both old leading-marker bodies and new trailing-marker bodies.
- Security posture remains unchanged: generated PR-facing markdown must not include raw prompts, raw model responses, complete webhook payloads, secrets, tokens, or private source beyond intended bounded finding text.
