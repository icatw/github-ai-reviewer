#!/usr/bin/env bash
set -euo pipefail

SERVICE="${SERVICE:-github-ai-reviewer.service}"
LOCAL_HEALTH="${LOCAL_HEALTH:-http://127.0.0.1:8095/healthz}"
PUBLIC_HEALTH="${PUBLIC_HEALTH:-https://app.icatw.site/healthz}"

fail() {
  echo "service health check failed: $*" >&2
  exit 1
}

active="$(systemctl is-active "${SERVICE}" 2>/dev/null || true)"
if [[ "${active}" != "active" ]]; then
  fail "${SERVICE} is ${active:-unknown}, want active"
fi

echo "systemd active ok"

local_body="$(curl -fsS --max-time 10 "${LOCAL_HEALTH}")" || fail "local health request failed"
if [[ "${local_body}" != "ok" ]]; then
  fail "local health returned unexpected body"
fi

echo "local health ok"

public_body="$(curl -fsS --max-time 10 "${PUBLIC_HEALTH}")" || fail "public health request failed"
if [[ "${public_body}" != "ok" ]]; then
  fail "public health returned unexpected body"
fi

echo "public health ok"

if command -v nginx >/dev/null 2>&1; then
  nginx_conf="$(sudo nginx -T 2>/dev/null || true)"
  if [[ "${nginx_conf}" != *"server_name app.icatw.site"* || "${nginx_conf}" != *"proxy_pass http://127.0.0.1:8095/healthz"* || "${nginx_conf}" != *"proxy_pass http://127.0.0.1:8095/github/webhook"* ]]; then
    fail "nginx app.icatw.site proxy routes are missing expected 8095 targets"
  fi
  echo "nginx routes ok"
fi

echo "service health check ok"
