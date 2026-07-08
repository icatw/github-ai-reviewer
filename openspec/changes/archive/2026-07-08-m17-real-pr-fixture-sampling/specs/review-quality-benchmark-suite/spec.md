## ADDED Requirements

### Requirement: Batch sampling workflow integration
The benchmark suite SHALL support a documented workflow for running M16 metrics across a manually sampled batch of real PR fixtures.

#### Scenario: Batch checklist references benchmark output safely
- **WHEN** an operator runs the benchmark suite for a sampled fixture batch
- **THEN** the workflow records aggregate context metrics, aggregate finding-quality metrics, and per-fixture follow-up TODOs in a safe checklist artifact
- **AND** the checklist does not include raw private source content, raw prompts, raw model output, secrets, tokens, private keys, API keys, installation tokens, or complete webhook payloads

#### Scenario: Batch workflow preserves existing fixture compatibility
- **WHEN** batch-sampled fixtures are replayed with `cmd/review-bench`
- **THEN** legacy context-only fixtures, annotated finding-quality fixtures, expected no-finding fixtures, and sanitized public or synthetic fixtures remain compatible with the benchmark runner
- **AND** the batch workflow does not require live LLM judging as a benchmark gate

#### Scenario: Helper code is tested when added
- **WHEN** implementation adds helper scripts, manifest parsing, or command planning for batch fixture sampling
- **THEN** deterministic tests cover manifest validation, privacy defaults, unsafe output path rejection or warning behavior, and safe checklist output
- **AND** if implementation is documentation-only, the milestone explicitly records that no production code behavior changed and no new helper parsing tests are required
