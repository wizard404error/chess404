# Chess404

Chess404 is a realtime chess variant platform with card mechanics, an authoritative Go backend, and a Next.js web client.

## Stack

- `apps/web`: Next.js frontend and proxy routes
- `services/realtime`: Go services for gateway, live matches, platform APIs, and matchmaking
- `packages/contracts`: shared TypeScript contracts
- `packages/game-core`: shared frontend game helpers

## Local development

Start the full local stack:

```powershell
cd "C:\Users\Expert Gaming\Desktop\chess404"
powershell -ExecutionPolicy Bypass -File .\start-local.ps1
```

Main local URLs:

- Web: `http://localhost:3000`
- Gateway health: `http://localhost:8090/healthz`
- Match service health: `http://localhost:8082/healthz`
- Platform service health: `http://localhost:8083/healthz`
- Matchmaking service health: `http://localhost:8084/healthz`

## Quality checks

From the repo root:

```powershell
pnpm lint
pnpm build
```

From `services/realtime`:

```powershell
& "C:\Program Files\Go\bin\go.exe" test ./...
```

## Railway staging deploy

The repo is prepared for Railway with service-specific Dockerfiles under `deploy/railway/`.

Use the deployment guide here:

- [DEPLOY_RAILWAY.md](./DEPLOY_RAILWAY.md)

Recommended Railway services:

- `web`
- `gateway`
- `match-service`
- `platform-service`
- `matchmaking-service`
- `postgres`
- `redis`
