# Chess404 Realtime Services

This folder is the backend foundation for the server-authoritative migration.

Planned services:

- `cmd/gateway`: control-plane entrypoint, session bootstrap, and future routing surface
- `cmd/match-service`: authoritative match host
- `cmd/matchmaking-service`: queueing and pairing
- `cmd/platform-service`: profiles, ratings, history, moderation
- `cmd/replay-worker`: replay compaction/export

Go is installed on the machine, but the current app shell may still require an explicit binary path until the session picks up the updated PATH.

## Current implemented surface

`cmd/gateway` now exposes a real control-plane surface:

- `GET /api/system/status`
  - aggregate readiness across match, platform, and matchmaking services
- `GET /api/session/bootstrap`
  - aggregate bootstrap envelope with backend readiness, platform capabilities, default queue, and configured service endpoints
- `POST /api/session/bootstrap`
  - create or resume both guest sessions through `platform-service`
  - recover backend match-seat claims for a requested room when guest sessions own seats there
  - resume per-seat account sessions when the browser already has stored account leases
  - self-heal stale account session tokens through the linked guest session when possible
  - self-heal stale guest-session secrets by falling back to a fresh guest session instead of leaving the web shell to patch that up itself
- `POST /api/matches/{matchId}/intents`
  - gateway-owned intent proxy
  - can resolve a backend-issued `playerClaimToken` into the actual room seat secret before forwarding the intent to `match-service`
- `GET /healthz`
  - basic gateway health payload

`cmd/match-service` now exposes an authoritative API, live match stream, and durable local archive updates:

- `POST /api/matches`
  - create a new active match
- `GET /api/matches/{matchId}`
  - fetch current snapshot and emitted events
- `POST /api/matches/{matchId}/intents`
  - apply supported intents
- `GET /api/matches/{matchId}/ws`
  - subscribe to live authoritative match snapshots
- `GET /healthz`
  - basic service health and rules-version payload

Shared archive persistence is now pluggable for both `match-service` and `platform-service`:

- `file`
  - existing JSON-backed archive used for simple local development
- `sqlite`
  - transactional SQL-backed archive for local production-shaped testing, including replay/history and private room recovery metadata
- `postgres`
  - real Postgres-backed archive for production-shaped replay/history persistence and room recovery metadata

`cmd/platform-service` now exposes the first real platform read surface:

- `GET /api/platform/capabilities`
- `POST /api/platform/guest-sessions`
  - create or resume a local guest identity
  - successful guest sessions now also return an opaque `sessionToken` plus `expiresAt` lease metadata
  - resumed sessions can validate through either the persisted private session secret or the renewable opaque session token instead of trusting bare guest ids
- `POST /api/platform/accounts/claim`
  - claim or re-open a guest-linked local account handle
  - returns an opaque account `sessionToken` plus `expiresAt` lease metadata for that account session
- `POST /api/platform/account-sessions`
  - resume an existing account session from its opaque account session token
- `GET /api/platform/accounts`
  - list claimed local accounts
  - supports `sort=rating` for an account-first leaderboard derived from linked guest stats
- `GET /api/platform/accounts/by-guest/{guestId}`
  - resolve which claimed local account, if any, is linked to a specific guest identity
- `GET /api/platform/accounts/{accountId}`
  - fetch public account profile metadata for a claimed local account
- `POST /api/platform/match-claims`
  - validate a guest session against an archived/live room and return that guest's actual seat claim plus room player secret
  - returns an opaque `claimToken` plus `expiresAt` lease metadata
- `POST /api/platform/match-claims/resolve`
  - resolve a backend-issued room claim token back into the cached seat claim
  - successful resolves renew the lease
- `POST /api/platform/guest-results`
  - finalize a guest-vs-guest match result and update ratings once
- `POST /api/platform/account-results`
  - finalize a rated result through archived account-seat ownership first, while still updating the linked guest ratings underneath
  - supports linked-guest fallback for older archive rows that predate explicit archived account ids
- account stores now persist direct ladder stats and finalized-match idempotency across file, SQLite, and Postgres backends
- account stores now also persist direct account-owned rating history across file, SQLite, and Postgres backends
- account detail responses now include derived season summaries built from that rating history
- `POST /api/platform/guest-results` now also backfills linked account ladder stats when both seat guests already belong to claimed accounts, so older guest-first rooms do not leave account profiles behind
- `GET /api/platform/rankings`
  - sorted guest leaderboard by rating
- `GET /api/platform/guests`
  - recent guest profiles with persisted record stats
- `GET /api/platform/matches`
  - recent archived matches, including guest player metadata and linked account identity when available
  - accepts `guestId` to filter history to one guest's archived matches
  - accepts `accountId` to filter history to one claimed account, with linked-guest fallback for older archive rows
- `GET /api/platform/matches/{matchId}`
  - archived match detail including latest snapshot, replay frames, full stored event history, player metadata, and linked account handles when available

`platform-service` guest persistence is now pluggable:

- `file`
  - existing JSON-backed guest profile store
- `sqlite`
  - transactional SQL-backed guest profile and finalized-result store for local production-shaped testing
- `postgres`
  - real Postgres-backed guest profile and result-finalization store for the first production-shaped relational backend slice

`platform-service` account persistence is now pluggable too:

- `file`
  - JSON-backed local account store for the first guest-to-account upgrade path
- `sqlite`
  - transactional SQL-backed local account store for production-shaped account/session testing without changing the product API
- `postgres`
  - real Postgres-backed local account store for account handles, linked guest ownership, and renewable account sessions

`platform-service` live room claim recovery is now pluggable too:

- `memory`
  - simple in-process cache for local development
- `redis`
  - shared Redis-backed room claim cache for backend-owned seat recovery across restarts and multiple service instances

`matchmaking-service` ticket persistence is now pluggable too:

- `file`
  - existing JSON-backed queue ticket store
- `sqlite`
  - transactional SQL-backed queue ticket store for restart-safe local production-shaped queue testing
- `redis`
  - real Redis-backed queue ticket store for shared volatile queue durability and a cleaner path toward distributed matchmaking

Current web product surfaces now connected through the platform layer:

- `Queue` can create matched rooms and open authoritative matches
- `Queue` now resumes saved tickets after browser reloads when the local matchmaking service still has them
- `Queue` now also survives matchmaking-service restarts through a file-backed ticket store
- `Account` can claim reusable guest-linked handles and keep a renewable local account session per seat
- `Account` now also shows recent archived matches for each claimed account seat
- `Account` now also shows current season momentum plus recent season summaries derived from account-owned rating history
- `Rankings` now renders an account-first leaderboard derived from linked guest ratings and records
- `Rankings` now also shows current season momentum when an account already has direct account-owned match history
- rated match finalization now prefers the account-owned platform path when a room has linked account seats, so result validation lines up with the same account identity shown in rankings/history/community
- `Community` now shows linked account handles and account metadata inside focused guest profiles
- `History` can inspect archived matches, player metadata, and linked account identity
- `History` can scrub through archived replay frames instead of only showing the final board snapshot
- `Community` can inspect guest profiles and jump into that guest's recent matches
- `Rankings` can jump directly into guest profiles

Currently supported intents:

- `make_move`
- `play_card`
- `select_target`
- `send_chat`
- `offer_draw`
- `respond_draw`
- `resign`

Current backend-owned mechanics:

- standard move validation
- clocks and timeout finishes
- draw / resign / chat flow
- pawn promotion on move
- frozen pieces cannot move
- shield blocks one capture and expires
- `Freeze`
- `Shield`
- `Sniper`
- `Bad Sniper`

This is still not fully production-ready yet:

- Redis is now in use for shared queue tickets and optional live room claim recovery
- Postgres is now in use for guest profiles and shared match archives
- account/auth is only in its first guest-linked local account phase, not full user auth yet
- backend authority is much further along, but frontend orchestration and stress hardening still need work

## Local archive

By default, `match-service` and `platform-service` share:

- `services/realtime/data/match-archive.json`
- `services/realtime/data/guest-profiles.json`
- `services/realtime/data/accounts.json`
- `services/realtime/data/matchmaking-tickets.json`

Override it with:

- `MATCH_ARCHIVE_PATH`
- `MATCH_ARCHIVE_BACKEND`
- `MATCH_ARCHIVE_SQLITE_PATH`
- `MATCH_ARCHIVE_POSTGRES_URL`
- `GUEST_STORE_BACKEND`
- `GUEST_STORE_PATH`
- `GUEST_STORE_SQLITE_PATH`
- `GUEST_STORE_POSTGRES_URL`
- `ACCOUNT_STORE_PATH`
- `ACCOUNT_STORE_BACKEND`
- `ACCOUNT_STORE_SQLITE_PATH`
- `ACCOUNT_STORE_POSTGRES_URL`
- `MATCH_CLAIM_STORE_BACKEND`
- `MATCH_CLAIM_STORE_REDIS_URL`
- `MATCH_CLAIM_STORE_REDIS_KEY`
- `MATCH_CLAIM_STORE_TTL_SECONDS`
- `MATCHMAKING_TICKET_STORE_BACKEND`
- `MATCHMAKING_TICKET_STORE_PATH`
- `MATCHMAKING_TICKET_STORE_SQLITE_PATH`
- `MATCHMAKING_TICKET_STORE_REDIS_URL`
- `MATCHMAKING_TICKET_STORE_REDIS_KEY`

## Local development

Recommended local port setup now:

- Go gateway: `http://localhost:8090`
- Next.js web app: `http://localhost:3000`
- Go match service: `http://localhost:8082`
- Go platform service: `http://localhost:8083`
- Go matchmaking service: `http://localhost:8084`

Start the backend:

```powershell
cd "C:\Users\Expert Gaming\Desktop\chess404\services\realtime"
$env:GATEWAY_ADDR=':8090'
& "C:\Program Files\Go\bin\go.exe" run .\cmd\gateway
```

Start the match service:

```powershell
cd "C:\Users\Expert Gaming\Desktop\chess404\services\realtime"
$env:MATCH_SERVICE_ADDR=':8082'
& "C:\Program Files\Go\bin\go.exe" run .\cmd\match-service
```

Start the web app:

```powershell
cd "C:\Users\Expert Gaming\Desktop\chess404"
pnpm --filter @chess404/web dev
```

Start the platform service:

```powershell
cd "C:\Users\Expert Gaming\Desktop\chess404\services\realtime"
$env:PLATFORM_ADDR=':8083'
& "C:\Program Files\Go\bin\go.exe" run .\cmd\platform-service
```

With all three running, the web app `History` page can read archived matches through:

- `http://localhost:3000/api/platform/matches`
- `http://localhost:3000/api/platform/matches/{matchId}`

Start the matchmaking service:

```powershell
cd "C:\Users\Expert Gaming\Desktop\chess404\services\realtime"
$env:MATCHMAKING_ADDR=':8084'
& "C:\Program Files\Go\bin\go.exe" run .\cmd\matchmaking-service
```

With the matchmaking service running, the web app `Queue` page can create and inspect local tickets through:

- `http://localhost:3000/api/matchmaking/queues/tickets`

Saved queue tickets now survive browser refreshes in local dev, and the backend queue store also survives matchmaking-service restarts so matched rooms can still be reopened later from the same browser.

## Recent local hardening

- authoritative match creation can now carry per-seat `whitePlayerSecret` / `blackPlayerSecret`
- the Go match service enforces those seat secrets on move/card/chat/draw/resign intents when they are configured
- the main web app persists those seat secrets in room metadata so refresh/reconnect keeps the same seat claim locally
- guest session resume is now backed by a persisted private session secret across file / SQLite / Postgres guest stores, so a forged bare `guestId` no longer silently reclaims that guest profile
- the main web app and queue room handoff now prefer those backend-issued guest session secrets as the source for seat claims before falling back to browser-generated room secrets
- the main app can now also recover missing active-room seat secrets from those validated guest sessions during room bootstrap, reducing dependence on room-local browser metadata alone
- `platform-service` now also exposes a backend match-claim flow, so the web app can recover the actual room-specific seat secret for older archived rooms instead of guessing from guest-session state alone
- the web shell now uses `gateway` as the first backend-owned bootstrap path for guest resume and room-claim recovery, so startup no longer has to coordinate those platform calls directly in browser logic
- that same gateway bootstrap path now also resumes stored account sessions and can refresh stale account leases through linked guest identity, so account recovery is less browser-shaped too
- `platform-service` now caches validated room seat claims in a pluggable claim store, and that claim layer can run on Redis so room recovery stops depending only on archive lookups plus browser-stored metadata
- room claims now also carry opaque backend-issued claim tokens, and gateway can use those tokens to forward active match intents without requiring the browser to keep sending raw room seat secrets
- room claims now expire on a backend lease timer and renew on active use, so abandoned claim tokens age out while active rooms keep a fresh backend-owned claim
- the web shell now renews active room claim leases proactively through `gateway` before expiry, so long-running matches stay on opaque claim tokens instead of dropping back to stored seat secrets after a lease timeout
- guest sessions now behave similarly: the browser can resume through the backend-issued `sessionToken` lease, while the raw guest secret stays as a compatibility fallback instead of the default bootstrap path
- guest-linked local accounts now exist on top of that guest session layer:
  - `platform-service` can claim a reusable handle from a validated guest session
  - account sessions also use backend-issued opaque renewable `sessionToken` leases
  - the web app now has an `Account` tab for claiming and resuming those local account sessions
- guest rating finalization is now treated as a `rated`-queue-only action; direct sandbox matches and `casual` queue rooms should no longer mutate the leaderboard
- `platform-service` now enforces that rule too, so direct posts to `/api/platform/guest-results` are rejected unless:
  - the archived room exists
  - the archived room is `rated`
  - the archived room is already `finished`
  - the posted winner and seat guest ids match the archived room metadata
- archived match history now also carries queue mode metadata so product surfaces can distinguish `rated`, `casual`, and direct rooms
- archived room persistence now also keeps private seat-secret metadata and full internal position history, so `match-service` can rehydrate an existing room after restart without dropping seat ownership or breaking restart-sensitive cards like `Reverse`
- new rooms now also stamp linked account ids into authoritative match/archive metadata, and `platform-service` enriches archive reads with claimed account handles for history/community surfaces
- the shared match archive can now run against a transactional SQLite store instead of only the local JSON archive file, while keeping the same history, replay, and room-recovery behavior
- the shared match archive can now also run against a real Postgres-backed store, which extends the actual Postgres footprint beyond guest profiles into replay/history and room recovery metadata
- `platform-service` guest persistence can now run against a transactional SQLite store instead of only the local JSON profile file, while keeping the same API surface for guest sessions, ratings, rankings, and community/history pages
- `platform-service` can now also run against a real Postgres-backed guest store, which is the first actual Postgres slice in the repo instead of another local file replacement
- `platform-service` account persistence can now also run against transactional SQLite or real Postgres backends instead of only the local JSON account file, while keeping the same account claim/session API surface
- `matchmaking-service` ticket persistence can now run against a transactional SQLite store instead of only the local JSON ticket file, while keeping the same queue API and room handoff behavior
- `matchmaking-service` can now also run against a real Redis-backed ticket store, which is the first actual Redis slice in the project instead of another local file replacement
- each backend now exposes a lightweight status endpoint for local ops visibility:
  - `gateway`: `GET /api/system/status`
  - `match-service`: `GET /api/system/status`
  - `platform-service`: `GET /api/platform/status`
  - `matchmaking-service`: `GET /api/status`
- the web app proxies those into the new in-product `Status` tab so you can inspect control-plane readiness plus room, archive, guest, queue counts, and the active archive / guest-store / ticket-store backends without checking raw service logs
