# Chess404 Architecture

## Overview

Chess404 is a chess + cards platform with a server-authoritative architecture. The backend enforces all game rules, card effects, and clock management. The frontend is a thin rendering layer.

## Monorepo Structure

```
chess404/
├── apps/web/              # Next.js frontend
├── packages/
│   ├── contracts/         # Shared TypeScript types
│   └── game-core/         # Card pool, rules, RNG
├── services/realtime/
│   ├── cmd/
│   │   ├── gateway/               # HTTP reverse proxy, auth, CORS
│   │   ├── match-service/         # WebSocket host, game state
│   │   ├── platform-service/      # Accounts, guests, history
│   │   ├── matchmaking-service/   # Queue, ticketing
│   │   ├── replay-worker/         # Async replay archival
│   │   └── migrate/               # DB migrations
│   └── internal/
│       ├── contracts/             # Go domain types
│       ├── match/                 # Game engine, state machine
│       ├── engine/                # Custom chess engine (eval, search, AI)
│       ├── platform/              # Postgres stores
│       ├── matchmaking/           # Queue logic
│       ├── messaging/             # NATS/Redis Streams bus
│       ├── sharding/              # Consistent hash ring
│       ├── featureflags/          # Feature flag store
│       ├── tracing/               # OpenTelemetry setup
│       ├── metrics/               # Prometheus /metrics
│       ├── httputil/              # CORS, CSRF, retry, circuit breaker
│       ├── logging/               # Structured slog JSON
│       └── rate_limit/            # Per-IP and per-player limits
└── deploy/
    ├── railway/           # Dockerfiles for all services
    └── grafana/           # Dashboard JSON
```

## Service Architecture

```
Browser ──► Next.js (web) ──► Gateway ──┬── Match Service (WebSocket + HTTP)
                                        ├── Platform Service (Postgres)
                                        └── Matchmaking Service (Redis)
```

- **Gateway**: Single entry point. Auth, CORS, CSRF, rate limiting. Proxies to backend services.
- **Match Service**: Hosts live games via WebSocket. Server-authoritative state machine. 37 card effects resolved server-side. Custom chess engine for computer opponent.
- **Platform Service**: Guest accounts, registered accounts, match history, rankings, friendships, notifications, moderation.
- **Matchmaking Service**: Queue ticketing, Elo-based matching, direct challenges.
- **Replay Worker**: Async archival of finished matches.

## Game Engine

The match service contains a full chess engine (`internal/engine/`) with:

- **Evaluation**: Piece-square tables, material, positional bonuses, king safety
- **Search**: Alpha-beta with iterative deepening, transposition table, move ordering
- **Card evaluation**: Scores all 37 card mechanics based on board state
- **Computer opponent**: 5 difficulty levels (Beginner=2ply through Expert=8ply)

## Card System

37 unique cards with mechanics: freeze, shield, sniper, heal, swap, promote, demote, clone, teleport, jump, borrow, mindcontrol, parasite, lava, invisible, bomb, fog, fortress, doublemove, reverse, undo, mirror, fakepiece, blackhole, sacrifice, gambler, radar, cheater, joker.

Card lifecycle: pool → hand (1 per round) → play → resolve. Server validates and resolves all effects.

## Data Flow

1. Client sends intent via WebSocket or HTTP
2. Gateway authenticates and forwards to match-service
3. Match-service validates intent against game state
4. State machine applies move/card/effect
5. New state broadcast to all connected clients
6. State dual-written to memory + Redis
7. Events published to NATS/Redis Streams

## Infrastructure

- **State storage**: In-memory with Redis backup (dual-write)
- **Cross-instance**: Redis Pub/Sub for broadcast, NATS for event bus
- **Sharding**: Consistent hash ring (150 virtual nodes) for horizontal scaling
- **Observability**: slog JSON logs, Prometheus metrics, OpenTelemetry traces, Grafana dashboards
- **Deployment**: Docker multi-stage builds, GitHub Actions CI/CD, Railway hosting
- **Feature flags**: JSON-based store with per-user rollout percentages

## Security

- Server-authoritative: no client trust for game state
- CSRF protection via double-submit cookie pattern
- CORS with explicit origin allow-list
- Rate limiting: 60/min API, 10/sec per-player intents
- Auth tokens: cryptographic random (SHA-256 fallback)
- CSP headers evaluated at request time (not build time)
- Credentials redacted from all log output
