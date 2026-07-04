## 1. Renderer Tests

- [x] 1.1 Add inline renderer tests asserting English blocker and warning bodies start with visible severity labels and not the hidden marker.
- [x] 1.2 Add inline renderer tests asserting evidence, failure scenario, and confidence render inside a `<details>` block and not as top-level report bullets.
- [x] 1.3 Add inline renderer tests asserting confidence is omitted rather than fabricated when the finding has no confidence value.
- [x] 1.4 Add zh-CN inline renderer tests for localized severity, suggestion, and details labels.
- [x] 1.5 Add Pull Request Review body tests for human-friendly English wording and localized zh-CN wording.
- [x] 1.6 Add summary renderer tests proving English advisory text no longer contains `M2 Review` while preserving advisory/non-blocking meaning.

## 2. Marker Compatibility Tests

- [x] 2.1 Add tests proving new trailing-marker inline bodies remain discoverable by `existingInlineComments` and fingerprint extraction.
- [x] 2.2 Add tests proving old leading-marker inline bodies remain discoverable for update, stale, and inactive handling.
- [x] 2.3 Add tests proving unmarked comments and comments without valid fingerprints remain ignored.
- [x] 2.4 Add stale and inactive rendering tests proving marker/fingerprint metadata remains present after body updates.

## 3. Inline Comment Formatting Implementation

- [x] 3.1 Introduce centralized localized inline labels for severity, suggestion, details summary, evidence, failure scenario, and confidence.
- [x] 3.2 Update inline finding rendering so the body starts with the visible severity label and concise finding explanation.
- [x] 3.3 Render the actionable suggestion in the visible section and move evidence, failure scenario, and optional confidence into a GitHub-supported `<details>` block.
- [x] 3.4 Move the hidden inline marker and fingerprint to the end of newly rendered inline bodies.
- [x] 3.5 Preserve markdown safety by using existing typed finding fields only and not adding raw prompts, raw model responses, webhook payloads, secrets, or unbounded source.

## 4. Review And Summary Text Implementation

- [x] 4.1 Update Pull Request Review body rendering to use GitHub-native advisory wording in English.
- [x] 4.2 Update Pull Request Review body rendering to use localized Simplified Chinese wording for `zh-CN`.
- [x] 4.3 Remove stale milestone wording such as `M2 Review` from summary advisory renderer text.
- [x] 4.4 Preserve the summary comment marker, footer purpose, full-report structure, and upsert behavior.

## 5. Compatibility And Behavior Preservation

- [x] 5.1 Ensure marker and fingerprint extraction scans the full body and supports both leading-marker and trailing-marker formats.
- [x] 5.2 Verify existing update/create/stale split behavior still uses fingerprints correctly after marker relocation.
- [x] 5.3 Verify inline eligibility, thresholds, diff mapping, batching, fallback, cleanup, repo config, analyzer behavior, Check Runs, and summary upsert behavior are unchanged.
- [x] 5.4 Keep stale and inactive inline comment updates non-destructive and marker-scoped.

## 6. Verification

- [x] 6.1 Run `gofmt -w .`.
- [x] 6.2 Run `go test ./...`.
- [x] 6.3 Run `go build ./cmd/server`.
- [x] 6.4 Run `openspec validate m15-github-native-inline-comment-format --type change --strict`.
- [x] 6.5 If deployed for manual verification, confirm the GitHub PR UI shows compact line-local inline comments and the summary comment remains the full report.
