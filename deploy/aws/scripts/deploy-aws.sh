#!/usr/bin/env bash
# Register a rendered ECS task definition and rolling-update the service.
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=common.sh
source "$SCRIPT_DIR/common.sh"

load_params
require_cmd aws

: "${ECS_CLUSTER:?}"
: "${ECS_SERVICE:?}"
: "${TASK_FAMILY:?}"

echo "==> Rendering task definition"
TASK_DEF_FILE="$(render_task_definition)"
echo "    $TASK_DEF_FILE"

echo "==> Registering task definition family=${TASK_FAMILY}"
TASK_ARN=$(aws ecs register-task-definition \
  --cli-input-json "file://${TASK_DEF_FILE}" \
  --region "$AWS_REGION" \
  --query 'taskDefinition.taskDefinitionArn' \
  --output text)
echo "    $TASK_ARN"

echo "==> Updating service ${ECS_CLUSTER}/${ECS_SERVICE}"
aws ecs update-service \
  --cluster "$ECS_CLUSTER" \
  --service "$ECS_SERVICE" \
  --task-definition "$TASK_ARN" \
  --force-new-deployment \
  --region "$AWS_REGION" \
  --query 'service.{status:status,desired:desiredCount,taskDef:taskDefinition}' \
  --output table

echo "==> Waiting for service stability (can take several minutes)"
aws ecs wait services-stable \
  --cluster "$ECS_CLUSTER" \
  --services "$ECS_SERVICE" \
  --region "$AWS_REGION"

echo "Service is stable on $TASK_ARN"
