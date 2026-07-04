## ADDED Requirements

### Requirement: GitHub-native inline comment body format
When rendering an inline Pull Request Review comment for an eligible finding, the service SHALL make the visible body read as a concise line-level reviewer note while preserving the service marker and finding fingerprint for idempotency.

#### Scenario: Inline comment starts with human-facing severity label
- **WHEN** the service renders an inline comment body for an eligible finding
- **THEN** the body starts with a localized human-facing severity label
- **AND** the body does not start with the hidden inline marker
- **AND** blocker findings use a label equivalent to `🚨 Blocking risk`
- **AND** warning findings use a label equivalent to `⚠️ Potential issue`

#### Scenario: Inline comment contains concise actionable visible text
- **WHEN** the service renders an inline comment body for an eligible finding
- **THEN** the visible top section includes the finding title or direct explanation
- **AND** it includes a short localized suggestion label or sentence using the finding suggestion
- **AND** it does not render evidence, failure scenario, and confidence as top-level report bullets

#### Scenario: Evidence details are collapsed
- **WHEN** the service renders an inline comment body for an eligible finding with evidence, failure scenario, suggestion, and confidence
- **THEN** evidence appears inside a `<details>` block
- **AND** failure scenario appears inside the same `<details>` block
- **AND** confidence appears inside the same `<details>` block
- **AND** the default visible comment remains compact before the details block

#### Scenario: Optional confidence is not fabricated
- **WHEN** the service renders an inline comment body for an eligible finding without confidence
- **THEN** the details block omits confidence
- **AND** the renderer does not fabricate a confidence value

### Requirement: Inline marker placement and backward compatibility
The service SHALL keep inline hidden marker and fingerprint metadata discoverable after moving the marker after visible content, and SHALL continue to recognize existing leading-marker inline comments created by previous versions.

#### Scenario: New inline body places marker after visible content
- **WHEN** the service renders a new inline comment body
- **THEN** the service marker and fingerprint are present after the visible reviewer note and details content
- **AND** the marker is not the first non-whitespace content in the body
- **AND** the marker does not expose secrets, tokens, raw prompts, raw model responses, complete webhook payloads, or unbounded private source

#### Scenario: New trailing-marker body is discoverable
- **WHEN** existing inline comment discovery reads a bot comment whose marker and fingerprint appear after visible content
- **THEN** the service extracts the fingerprint
- **AND** the comment can be matched for update, stale marking, or inactive cleanup

#### Scenario: Old leading-marker body remains discoverable
- **WHEN** existing inline comment discovery reads a bot comment whose body begins with the historical hidden marker and fingerprint format
- **THEN** the service extracts the fingerprint
- **AND** the comment can be matched for update, stale marking, or inactive cleanup

#### Scenario: Unmarked comments remain untouched
- **WHEN** an existing Pull Request Review comment lacks this service's inline marker or lacks a valid fingerprint
- **THEN** the service does not treat it as bot-owned
- **AND** the service does not update, stale-mark, inactive-mark, minimize, resolve, or otherwise alter that comment

### Requirement: GitHub-native Pull Request Review body
When creating a submitted Pull Request Review for inline comments, the service SHALL render a concise localized review body that reads like an advisory GitHub review note.

#### Scenario: English review body is human-friendly
- **WHEN** the service creates a Pull Request Review for one or more inline comments with English review language
- **THEN** the review body says that Review Cat left the number of inline comments
- **AND** it states that findings are advisory and non-blocking
- **AND** it does not use report-like wording such as `AI Review found N inline comment(s)`

#### Scenario: Chinese review body is localized
- **WHEN** the service creates a Pull Request Review for one or more inline comments with `zh-CN` review language
- **THEN** the review body is written in Simplified Chinese
- **AND** it states the number of inline comments
- **AND** it states that findings are advisory and non-blocking

### Requirement: Summary comment milestone-neutral advisory wording
The PR summary comment SHALL remain the full advisory report while avoiding stale milestone-specific wording in fixed renderer text.

#### Scenario: English summary advisory text is milestone-neutral
- **WHEN** the service renders an English PR summary comment
- **THEN** the fixed advisory text does not contain `M2 Review`
- **AND** it still states that findings are advisory and non-blocking
- **AND** the summary comment marker and footer purpose remain unchanged

#### Scenario: Chinese summary advisory text remains localized
- **WHEN** the service renders a `zh-CN` PR summary comment
- **THEN** the fixed advisory text is localized
- **AND** it does not introduce historical milestone wording
- **AND** the summary comment marker and footer purpose remain unchanged

### Requirement: Inline formatting verification
The implementation SHALL include automated tests for compact inline formatting, marker compatibility, localized review bodies, and summary wording cleanup.

#### Scenario: Tests cover inline body structure
- **WHEN** M15 implementation is complete
- **THEN** tests assert inline bodies start with the visible severity label rather than the hidden marker
- **AND** tests assert evidence, failure scenario, and confidence are inside `<details>`
- **AND** tests assert the marker and fingerprint appear after visible content

#### Scenario: Tests cover marker compatibility
- **WHEN** M15 implementation is complete
- **THEN** tests assert new trailing-marker inline bodies are discoverable by existing inline comment detection
- **AND** tests assert old leading-marker inline bodies are still discoverable
- **AND** tests assert unrelated comments without the service marker and fingerprint are ignored

#### Scenario: Tests cover localized fixed text
- **WHEN** M15 implementation is complete
- **THEN** tests assert the Pull Request Review body is human-friendly in English
- **AND** tests assert the Pull Request Review body is localized for `zh-CN`
- **AND** tests assert summary advisory wording no longer contains stale milestone text

#### Scenario: Standard commands pass
- **WHEN** M15 implementation is complete
- **THEN** `gofmt -w .` has been run
- **AND** `go test ./...` passes
- **AND** `go build ./cmd/server` passes
- **AND** `openspec validate m15-github-native-inline-comment-format --type change --strict` passes
