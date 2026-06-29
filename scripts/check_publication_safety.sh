#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"

fail() {
  echo "publication safety check failed: $*" >&2
  exit 1
}

require_file() {
  [[ -f "$1" ]] || fail "missing required file $1"
}

require_text() {
  local file="$1"
  local text="$2"
  grep -Fq "$text" "$file" || fail "missing required text in $file: $text"
}

require_file README.md
require_file LICENSE
require_file CONTRIBUTING.md
require_file .env.example
require_file docs/production.md
require_file docs/operations.md
require_file docs/e2e-evidence-template.md
require_file deploy/systemd/github-ai-reviewer.service
require_file scripts/smoke_local.sh
require_file scripts/check_e2e_safety.sh

require_text README.md "## GitHub App Setup"
require_text README.md "## Configuration"
require_text README.md "## Workspace Checkout"
require_text README.md "GO_WORKSPACE_PROVIDER_ENABLED=false"
require_text README.md "## Local Development"
require_text README.md "## Production Deployment"
require_text README.md "## Safety Checks"
require_text README.md "## License"
require_text README.md "scripts/check_publication_safety.sh"
require_text CONTRIBUTING.md "## Security Boundaries"
require_text CONTRIBUTING.md "Workspace checkout is disabled by default"
require_text .env.example "GO_WORKSPACE_PROVIDER_ENABLED=false"

sensitive_pathspecs=(
  '.env'
  '.env.*'
  '*.pem'
  '*.key'
  'private-key*.pem'
  '*.db'
  'data/**'
  'server'
  '**/server'
  'tmp/**'
  'e2e/**'
  '**/raw*payload*'
  '**/*payload*.json'
  '**/*prompt*.txt'
  '**/*model-response*'
)

tracked_sensitive_files="$(git ls-files -- "${sensitive_pathspecs[@]}" 2>/dev/null | grep -v '^\.env\.example$' || true)"
if [[ -n "${tracked_sensitive_files}" ]]; then
  echo "${tracked_sensitive_files}" >&2
  fail "sensitive or generated files are tracked"
fi

staged_sensitive_files="$(git diff --cached --name-only -- "${sensitive_pathspecs[@]}" 2>/dev/null | grep -v '^\.env\.example$' || true)"
if [[ -n "${staged_sensitive_files}" ]]; then
  echo "${staged_sensitive_files}" >&2
  fail "sensitive or generated files are staged"
fi

staged_files="$(git diff --cached --name-only)"
if [[ -n "${staged_files}" ]] && echo "${staged_files}" | grep -Eq '(^|/)m8-e2e-evidence\.md$|(^|/)tmp/|(^|/)e2e/'; then
  echo "${staged_files}" | grep -E '(^|/)m8-e2e-evidence\.md$|(^|/)tmp/|(^|/)e2e/' >&2
  fail "filled or private E2E evidence should not be staged"
fi

echo "publication safety check ok"
