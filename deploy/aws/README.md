# AWS deployment (ECS Fargate + DynamoDB + S3/CloudFront)

Production topology for the Chat Analytics Platform:

```
Browser
  ├─ HTTPS  → CloudFront → S3 (Vite static build)
  └─ WSS/HTTPS → ALB → ECS Fargate task (Go hub + REST)
                            └─ DynamoDB (chat-messages)
                            └─ optional EFS → BRIEFING_AUDIO_DIR
```

This directory is a **scaffold**: task definition, CodeBuild spec, and scripts.
**DynamoDB table schemas and IAM policies are hand-authored** (see
`HAND_AUTHORED.md`) — do not paste AI-generated schema/IAM into production.

## Prerequisites

- AWS CLI v2, Docker, Node 22+, Go 1.27+ (local only)
- ECR repository for the backend image
- ECS cluster (Fargate)
- ALB + target group (see ALB notes below)
- DynamoDB table `chat-messages` (hand-created; matches `pkg/storage`)
- S3 bucket + CloudFront distribution for the frontend
- Secrets Manager (or SSM) entries for `AUTH_SECRET` / ElevenLabs if enabled

Copy `params.example.env` → `params.env` (gitignored locally) and fill values.

## Quick path

```bash
# 1. Build + push backend image
./deploy/aws/scripts/push-to-ecr.sh

# 2. Register a new task definition revision and update the service
./deploy/aws/scripts/deploy-aws.sh

# 3. Build frontend against production API/WS URLs and sync to S3
./deploy/aws/scripts/sync-frontend.sh

# 4. Smoke-check livez/readyz + CloudFront
./deploy/aws/scripts/validate-deployment.sh
```

## ALB notes (WebSocket)

Long-lived chat sockets need ALB settings that default HTTP apps often miss:

| Setting | Recommendation | Why |
|---------|----------------|-----|
| Listener | HTTPS :443 → target group | Terminate TLS at the edge; app speaks plain HTTP inside the VPC |
| Target group protocol | HTTP :8080 | Tasks listen on `PORT` (default 8080) |
| Health check path | `/readyz` | Fails when DynamoDB is configured but unreachable; use `/livez` only if you want traffic during storage outages |
| Health check matcher | `200` | `/readyz` returns 503 when not ready |
| Stickiness | **Enable** (load-balancer cookie, e.g. 1h) | Hub state is in-memory per task; without stickiness, reconnects land on a different task and drop room membership |
| Idle timeout | **3600s** (max) | Default 60s kills quiet WebSocket connections |
| Deregistration delay | 30–120s | Lets write pumps drain on deploy |
| Attributes | Prefer HTTP/1.1 to targets | gorilla/websocket upgrade path is HTTP/1.1 |

Target group health checks hit the task IP directly (not the ALB hostname).
Security groups must allow ALB → task `:8080` and egress from tasks to DynamoDB
(VPC gateway endpoint or public egress).

### Scaling caveat

The hub keeps connections in process memory. Horizontal scale works for
fan-in HTTP and new sockets (with stickiness), but **broadcasts do not cross
tasks**. Multi-task production needs a shared pub/sub (e.g. Redis / SNS+SQS)
before `desiredCount > 1` for a single logical room. Until then, run
`desiredCount: 1` or shard rooms by sticky key deliberately.

## Frontend sync

`sync-frontend.sh` builds with:

- `VITE_API_URL=https://<alb-or-api-dns>`
- `VITE_WS_URL=wss://<alb-or-api-dns>/ws`

then `aws s3 sync` to the bucket and invalidates CloudFront `/*`.

Set `ALLOWED_ORIGINS` on the backend to the CloudFront origin
(e.g. `https://d123.cloudfront.net` or your custom domain) — never `*` in prod.

## Environment (ECS task)

| Variable | Production value |
|----------|------------------|
| `PORT` | `8080` |
| `DYNAMODB_ENDPOINT` | **empty** (use AWS DynamoDB + task role) |
| `DYNAMODB_REGION` | e.g. `us-east-1` |
| `ALLOWED_ORIGINS` | CloudFront / custom frontend origin |
| `AUTH_SECRET` | from Secrets Manager (add a `secrets` block to the task def once the secret exists) |
| `BRIEFING_ENABLED` | `false` unless TTS is configured |
| `BRIEFING_AUDIO_DIR` | EFS mount path if briefings are on |

Leave `DYNAMODB_ENDPOINT` **unset** in ECS so the task uses real AWS DynamoDB
via the task role. Local Compose sets the endpoint explicitly.

Do not set static `AWS_ACCESS_KEY_ID` / `AWS_SECRET_ACCESS_KEY` on the task —
use the task IAM role.

To inject `AUTH_SECRET` after creating it in Secrets Manager, add:

```json
"secrets": [
  {
    "name": "AUTH_SECRET",
    "valueFrom": "arn:aws:secretsmanager:REGION:ACCOUNT:secret:chat-analytics/auth-secret"
  }
]
```

## Files

| Path | Purpose |
|------|---------|
| `ecs-task-definition.json` | Fargate task def template (substitute placeholders) |
| `buildspec.yml` | CodeBuild: docker build/push backend image |
| `params.example.env` | Variable names for the scripts |
| `HAND_AUTHORED.md` | Checklist for DynamoDB + IAM (human-written) |
| `scripts/push-to-ecr.sh` | Build `backend/Dockerfile` → ECR |
| `scripts/deploy-aws.sh` | Register task def + update ECS service |
| `scripts/sync-frontend.sh` | Vite build → S3 → CloudFront invalidation |
| `scripts/validate-deployment.sh` | Curl livez/readyz + optional frontend HEAD |
