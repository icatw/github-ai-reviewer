#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ENV_FILE="${ENV_FILE:-${ROOT_DIR}/.env.production}"
OWNER="${OWNER:-}"
REPO="${REPO:-}"
PULL="${PULL:-}"
BIN="${ROOT_DIR}/diagnose-github"

usage() {
  cat >&2 <<'USAGE'
usage: OWNER=icatw REPO=interview-pilot PULL=7 scripts/diagnose_github_app.sh

Optional:
  ENV_FILE=/home/ubuntu/github-ai-reviewer/.env.production
USAGE
}

if [[ -z "${OWNER}" || -z "${REPO}" || -z "${PULL}" ]]; then
  usage
  exit 2
fi

cd "${ROOT_DIR}"
go build -o "${BIN}" ./cmd/diagnose-github
"${BIN}" -env-file "${ENV_FILE}" -owner "${OWNER}" -repo "${REPO}" -pull "${PULL}"
