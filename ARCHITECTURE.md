# Chess404 Foundation Status

This repository has been restructured toward a backend-authoritative architecture without redesigning the current UI baseline.

## Implemented in this pass

- Root monorepo scaffolding with workspaces and Turbo
- `apps/web` Next.js host for the current visual prototype
- `packages/contracts` for shared domain and message types
- `packages/game-core` for extracted deterministic rules, constants, card pool, and RNG
- `services/realtime` Go service scaffolds for gateway, matchmaking, match host, platform APIs, and replay worker

## Current boundary

The current UI baseline still lives in `apps/web/src`, but its reusable rules/types now have a shared home so we can continue moving authority out of `App.tsx` without changing the design.

## Next priorities

1. Move additional turn and card resolution out of `apps/web/src/App.tsx`
2. Define authoritative websocket handlers in Go
3. Replace client-owned RNG and match progression with server snapshots/events
4. Add deterministic replay tests
