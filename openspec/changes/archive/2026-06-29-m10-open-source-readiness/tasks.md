## 1. Public Documentation

- [x] 1.1 Rewrite `README.md` as a public-facing entry point in English.
- [x] 1.2 Document project purpose, current capabilities, intentionally unsupported features, architecture, repository layout, and roadmap.
- [x] 1.3 Document GitHub App permissions, webhook events, local smoke test, production deployment, systemd/nginx operation, and M8/M9 verification references.
- [x] 1.4 Document safe configuration defaults, especially that workspace checkout is disabled by default and requires explicit configuration.
- [x] 1.5 Link to production, operations, design, research, and E2E evidence docs without exposing secrets.

## 2. Public Project Metadata

- [x] 2.1 Add a license file if the project does not have one.
- [x] 2.2 Add contribution guidance covering tests, OpenSpec workflow, security boundaries, and non-secret evidence handling.
- [x] 2.3 Confirm generated binaries, local config, private keys, databases, raw payloads, and filled private evidence remain ignored.

## 3. Publication Safety Check

- [x] 3.1 Add a script that checks required public-readiness files and documentation anchors exist.
- [x] 3.2 The script must fail if sensitive tracked or staged files are present.
- [x] 3.3 The script must fail if generated binaries, `.env*` files except `.env.example`, private keys, local databases, raw payload captures, or private E2E/tmp evidence are staged.
- [x] 3.4 The script must avoid printing secret values or reading local env/private key contents.

## 4. Verification

- [x] 4.1 Run `go test ./...`.
- [x] 4.2 Run `go build ./cmd/server`.
- [x] 4.3 Run `scripts/smoke_local.sh`.
- [x] 4.4 Run `scripts/check_e2e_safety.sh`.
- [x] 4.5 Run the new publication safety check.
- [x] 4.6 Run `openspec validate m10-open-source-readiness --type change --strict`.
- [x] 4.7 Run `git diff --check`.
