# Frontend → Vercel deployment pipeline

The React/Vite frontend deploys to Vercel project
[`chat-analytics-platform`](https://vercel.com/erik-williams-projects/chat-analytics-platform)
(team `erik-williams-projects`).

Production URL: https://chat-analytics-platform.vercel.app

## Active pipeline (Vercel Git integration)

GitHub is linked to the Vercel project. Every push deploys automatically:

| Trigger | Target |
| --- | --- |
| Push / merge to `main` | Production |
| Push to any other branch / PR | Preview (unique URL + PR comment) |

Project settings (dashboard):

- **Root Directory:** `frontend`
- **Framework:** Vite
- **Node.js:** 22.x
- **Ignored Build Step:** skip when files under the frontend root are unchanged  
  (`git diff --quiet HEAD^ HEAD -- .` — runs inside Root Directory `frontend`)
- **Env vars:** `VITE_API_URL`, `VITE_WS_URL` (build-time; see dashboard)

Backend Go changes alone do not rebuild the frontend.

## Optional: CI-gated GitHub Actions deploy

[`.github/workflows/deploy-vercel.yml`](../.github/workflows/deploy-vercel.yml)
builds with the Vercel CLI after lint/test, then deploys a prebuilt artifact.

It stays idle until these GitHub Actions secrets exist:

| Secret | Value |
| --- | --- |
| `VERCEL_TOKEN` | Personal/team token from https://vercel.com/account/tokens |
| `VERCEL_ORG_ID` | `team_41PVU6m02jtpyMuN9iq6Grpg` |
| `VERCEL_PROJECT_ID` | `prj_AbkqVRavr2ujsg1Lm6hvO6wnXHrN` |

IDs are also in [`frontend/.vercel/project.json`](../frontend/.vercel/project.json)
for local `vercel` CLI use.

To hand deployment exclusively to Actions (avoid double deploys), add to
`frontend/vercel.json`:

```json
"git": { "deploymentEnabled": false }
```

## Gate production on GitHub CI

Vercel builds in parallel with GitHub Actions. To keep production domains on a
tested build only:

1. Open **Project → Settings → Deployment Checks**
2. Require the GitHub check **CI / Frontend (TypeScript)** (and Backend if you want)
3. Leave auto-assign of production domains enabled

## Local CLI

```bash
cd frontend
npx vercel@41.7.8 link      # uses .vercel/project.json when present
npx vercel@41.7.8 pull --yes --environment=preview
npx vercel@41.7.8 build
npx vercel@41.7.8 deploy --prebuilt
```

## Backend note

Vercel hosts the static frontend only. The Go WebSocket API still runs via
Docker Compose locally or the ECS scaffold under `deploy/aws/`. Point
`VITE_API_URL` / `VITE_WS_URL` at that backend and add the Vercel origin to
`ALLOWED_ORIGINS`.
