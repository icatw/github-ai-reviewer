#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SERVER_BIN="${ROOT_DIR}/server"
HTTP_ADDR="${HTTP_ADDR:-127.0.0.1:18088}"
PRIVATE_KEY_FILE="$(mktemp)"
LOG_FILE="$(mktemp)"
PID=""

cleanup() {
  if [[ -n "${PID}" ]] && kill -0 "${PID}" 2>/dev/null; then
    kill "${PID}" 2>/dev/null || true
    wait "${PID}" 2>/dev/null || true
  fi
  rm -f "${PRIVATE_KEY_FILE}" "${LOG_FILE}"
}
trap cleanup EXIT

if ! command -v openssl >/dev/null 2>&1; then
  echo "openssl is required for local smoke private key generation" >&2
  exit 1
fi
openssl genrsa -out "${PRIVATE_KEY_FILE}" 2048 >/dev/null 2>&1

cd "${ROOT_DIR}"

echo "build server"
go build -o "${SERVER_BIN}" ./cmd/server

echo "start server with dummy config"
GITHUB_WEBHOOK_SECRET="dummy-webhook-secret" \
GITHUB_APP_ID="123" \
GITHUB_APP_PRIVATE_KEY_PATH="${PRIVATE_KEY_FILE}" \
LLM_BASE_URL="http://127.0.0.1:9/v1" \
LLM_API_KEY="dummy-llm-api-key" \
LLM_MODEL="dummy-model" \
GO_WORKSPACE_PROVIDER_ENABLED="false" \
HTTP_ADDR="${HTTP_ADDR}" \
"${SERVER_BIN}" >"${LOG_FILE}" 2>&1 &
PID="$!"

for _ in $(seq 1 50); do
  if curl -fsS "http://${HTTP_ADDR}/healthz" >/dev/null 2>&1; then
    echo "check healthz"
    echo "smoke ok"
    exit 0
  fi
  if ! kill -0 "${PID}" 2>/dev/null; then
    echo "server exited during smoke startup" >&2
    grep -E "config error|github app setup error|server error|listening" "${LOG_FILE}" >&2 || true
    exit 1
  fi
  sleep 0.1
done

echo "healthz did not become ready" >&2
grep -E "config error|github app setup error|server error|listening" "${LOG_FILE}" >&2 || true
exit 1
