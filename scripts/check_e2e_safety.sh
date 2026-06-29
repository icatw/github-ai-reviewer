#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"

fail() {
  echo "e2e safety check failed: $*" >&2
  exit 1
}

require_file() {
  [[ -f "$1" ]] || fail "missing required file $1"
}

require_file docs/e2e-evidence-template.md
require_file docs/production.md
require_file scripts/smoke_local.sh

if ! grep -q "GO_WORKSPACE_PROVIDER_ENABLED=false" docs/production.md .env.example; then
  fail "workspace checkout default-off guidance missing"
fi

if ! grep -q "<!-- github-ai-reviewer:review-comment:v1 -->" docs/e2e-evidence-template.md; then
  fail "marker comment evidence field missing"
fi

if ! grep -q "AI Review" docs/e2e-evidence-template.md; then
  fail "check run evidence field missing"
fi

tracked_sensitive_files="$(git ls-files '.env' '.env.*' '*.pem' '*.key' 'private-key*.pem' '*.db' 'data/**' 2>/dev/null | grep -v '^\.env\.example$' || true)"
if [[ -n "${tracked_sensitive_files}" ]]; then
  echo "${tracked_sensitive_files}" >&2
  fail "sensitive local files are tracked"
fi

staged_sensitive_files="$(git diff --cached --name-only -- '.env' '.env.*' '*.pem' '*.key' 'private-key*.pem' '*.db' 'data/**' 2>/dev/null | grep -v '^\.env\.example$' || true)"
if [[ -n "${staged_sensitive_files}" ]]; then
  echo "${staged_sensitive_files}" >&2
  fail "sensitive local files are staged"
fi

if git diff --cached --name-only | grep -Eq '(^|/)e2e/.+\.md$|(^|/)tmp/'; then
  fail "filled E2E evidence should not be staged from tmp/e2e"
fi

echo "e2e safety check ok"
