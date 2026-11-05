# Chat Analytics Frontend

React + TypeScript UI for the real-time chat analytics platform. Vite serves the
app in development; production builds are static assets behind nginx.

## Scripts

```bash
npm install
npm run dev      # http://localhost:5173
npm run test     # Vitest unit tests
npm run lint
npm run build
```

## Environment

| Variable       | Default                     | Purpose              |
|----------------|-----------------------------|----------------------|
| `VITE_WS_URL`  | `ws://localhost:8080/ws`    | WebSocket endpoint   |
| `VITE_API_URL` | `http://localhost:8080`     | REST API base URL    |

Values are baked in at build time for production images.
