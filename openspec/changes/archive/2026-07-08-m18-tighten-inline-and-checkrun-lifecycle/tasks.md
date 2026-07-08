## 1. Inline Conservatism

- [x] 1.1 Change the global default inline severity threshold to `blocker`.
- [x] 1.2 Change the default inline policy used without repository config to `blocker`.
- [x] 1.3 Update inline tests so default inline publication uses blocker findings while warning rendering remains covered separately.

## 2. Check Run Lifecycle

- [x] 2.1 Include Check Run `status` when listing GitHub Check Runs.
- [x] 2.2 Make review start create a fresh `in_progress` Check Run instead of upserting an existing run.
- [x] 2.3 Make completion/failure matching update only matching `in_progress` Check Runs for the same head SHA.
- [x] 2.4 Update Check Run reporter tests and GitHub client tests for status-aware matching and create-start behavior.

## 3. Verification

- [x] 3.1 Run `gofmt -w .`.
- [x] 3.2 Run `go test ./...`.
- [x] 3.3 Run `go build ./cmd/server`.
- [x] 3.4 Run `openspec validate m18-tighten-inline-and-checkrun-lifecycle --type change --strict`.
- [x] 3.5 After deployment, run one real PR smoke test and verify GitHub shows a fresh in-progress/completed Check Run for the new review attempt.
