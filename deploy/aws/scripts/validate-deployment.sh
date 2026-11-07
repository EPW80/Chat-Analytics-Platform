#!/usr/bin/env bash
# Smoke-check backend readiness and optional frontend HEAD.
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=common.sh
source "$SCRIPT_DIR/common.sh"

load_params
require_cmd curl

API_URL="${API_URL:-}"
HEALTH_URL="${HEALTH_URL:-}"
FRONTEND_URL="${FRONTEND_URL:-}"

if [[ -z "$HEALTH_URL" && -n "$API_URL" ]]; then
  HEALTH_URL="${API_URL%/}/readyz"
fi
if [[ -z "$FRONTEND_URL" && -n "${FRONTEND_ORIGIN:-}" ]]; then
  origin="$FRONTEND_ORIGIN"
  origin="${origin#https://}"
  origin="${origin#http://}"
  FRONTEND_URL="https://${origin}"
fi

fail=0

if [[ -n "$HEALTH_URL" ]]; then
  echo "==> GET ${HEALTH_URL}"
  body=$(curl -sfS --max-time 15 "$HEALTH_URL") || {
    echo "readyz request failed" >&2
    fail=1
    body=""
  }
  echo "    $body"
  if [[ -n "$body" ]] && ! echo "$body" | grep -q '"status":"ok"'; then
    echo "readyz did not report status=ok" >&2
    fail=1
  fi

  LIVEZ_URL="${HEALTH_URL%/readyz}/livez"
  if [[ "$LIVEZ_URL" != "$HEALTH_URL" ]]; then
    echo "==> GET ${LIVEZ_URL}"
    curl -sfS --max-time 15 "$LIVEZ_URL" && echo || {
      echo "livez request failed" >&2
      fail=1
    }
  fi
else
  echo "Skipping backend checks (set API_URL or HEALTH_URL in params.env)"
fi

if [[ -n "$FRONTEND_URL" ]]; then
  echo "==> HEAD ${FRONTEND_URL}"
  code=$(curl -sS -o /dev/null -w '%{http_code}' --max-time 15 -I "$FRONTEND_URL" || true)
  echo "    HTTP $code"
  if [[ "$code" != "200" && "$code" != "301" && "$code" != "302" ]]; then
    echo "unexpected frontend status" >&2
    fail=1
  fi
else
  echo "Skipping frontend check (set FRONTEND_ORIGIN or FRONTEND_URL)"
fi

if [[ "$fail" -ne 0 ]]; then
  echo "Validation FAILED" >&2
  exit 1
fi
echo "Validation OK"
