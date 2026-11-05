#!/usr/bin/env bash
# Shared helpers for deploy/aws scripts.
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
AWS_DIR="$ROOT_DIR/deploy/aws"
PARAMS_FILE="${PARAMS_FILE:-$AWS_DIR/params.env}"

load_params() {
  if [[ ! -f "$PARAMS_FILE" ]]; then
    echo "Missing $PARAMS_FILE — copy deploy/aws/params.example.env and fill values." >&2
    exit 1
  fi
  # shellcheck disable=SC1090
  set -a
  source "$PARAMS_FILE"
  set +a

  : "${AWS_REGION:?}"
  : "${AWS_ACCOUNT_ID:?}"
}

require_cmd() {
  for c in "$@"; do
    command -v "$c" >/dev/null 2>&1 || {
      echo "Required command not found: $c" >&2
      exit 1
    }
  done
}

render_task_definition() {
  local src="$AWS_DIR/ecs-task-definition.json"
  local dest="${TASK_DEFINITION_FILE:-$AWS_DIR/.rendered-task-definition.json}"
  local tag="${IMAGE_TAG:-latest}"
  local origin="${FRONTEND_ORIGIN:?}"
  # Normalize to a full HTTPS origin for ALLOWED_ORIGINS.
  if [[ "$origin" != https://* && "$origin" != http://* ]]; then
    origin="https://${origin}"
  fi

  sed \
    -e "s/AWS_ACCOUNT_ID/${AWS_ACCOUNT_ID}/g" \
    -e "s/AWS_REGION/${AWS_REGION}/g" \
    -e "s/IMAGE_TAG/${tag}/g" \
    -e "s|FRONTEND_ORIGIN|${origin}|g" \
    "$src" >"$dest"
  echo "$dest"
}
