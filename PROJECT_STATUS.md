# Chess404 Project Status

This file tracks implementation progress against the scalable backend-authoritative plan so we always know:

- what is already done
- what is partially done
- what is still missing
- what to work on next

It should be updated as work continues.

## Goal

Build Chess404 so it can scale toward high concurrency without a future architectural rebuild.

Key rules:

- preserve the current design direction and animation feel
- remove frontend-authoritative gameplay over time
- make backend the source of truth
- keep the architecture clean enough to scale without rewriting core foundations

## Current Repo Shape

- `apps/web`
  - Next.js host for the current visual prototype
- `packages/contracts`
  - shared domain and protocol types
- `packages/game-core`
  - extracted deterministic rules foundation
- `services/realtime`
  - Go backend scaffolds for gateway, match execution, matchmaking, platform APIs, replay worker
- `client`
  - old prototype source kept as the original reference baseline

## Progress Against Plan

### 1. Foundation

Status: `in progress`

Completed:

- monorepo/workspace root created
- Turbo config added
- `pnpm` workspace added
- Next.js app created in `apps/web`
- current UI baseline copied into `apps/web/src`
- shared contracts package created
- shared game-core package created
- realtime Go service folders created
- architecture notes written in `ARCHITECTURE.md`

Still missing:

- remove long-term dependence on duplicated legacy `client` app
- decide whether `client` becomes archive-only or gets removed after migration stabilizes

### 2. Engine Extraction

Status: `started`

Completed:

- shared types extracted into `packages/contracts`
- chess helpers extracted into `packages/game-core/src/chess-engine.ts`
- constants extracted into `packages/game-core/src/constants.ts`
- card pool extracted into `packages/game-core/src/card-pool.ts`
- deterministic RNG utility added in `packages/game-core/src/rng.ts`
- web app re-export stubs now consume shared packages instead of local-only rule files
- canonical match-state module started in `packages/game-core/src/match-state.ts`

Still missing:

- move more rule resolution out of `apps/web/src/App.tsx`
- extract card effect resolution into backend-friendly pure functions
- expand the canonical `MatchState` reducer/apply-intent flow to cover cards and timed effects
- add replay serializer/deserializer utilities
- add rules version usage beyond constant definition

### 3. Realtime Platform

Status: `started`

Completed:

- gateway scaffold
- gateway now aggregates backend readiness and session bootstrap data across match, platform, and matchmaking services
- gateway now also owns session bootstrap recovery for the web shell, including guest-session resume/create and backend match-seat claims in one POST bootstrap flow
- gateway bootstrap now also resumes stored account sessions and can refresh stale account leases through linked guest identity, so startup can restore guest, room-claim, and account state through one backend-owned flow
- gateway now also proxies match intents with backend-issued room claim tokens, so the web app can act through opaque seat claims instead of always sending raw room secrets
- authoritative matches and archived rooms now also persist linked seat account ids, and `platform-service` enriches archive reads with claimed account handles for product surfaces
- match-service upgraded from scaffold to first real in-memory authoritative API
- match-service now persists authoritative match snapshots into a shared file-backed archive for durable local history
- shared archive persistence is now pluggable and can run on a transactional SQLite-backed archive instead of only the local JSON file
- shared archive persistence can now also run on Postgres, extending the real relational backend path into replay/history and room recovery metadata
- matchmaking-service scaffold
- matchmaking-service now persists local queue tickets into a file-backed store for restart recovery
- matchmaking-service ticket persistence is now pluggable and can run on a transactional SQLite-backed ticket store instead of only the local JSON file
- matchmaking-service can now also persist queue tickets in Redis, giving the project its first real Redis-backed infrastructure slice
- platform-service now exposes a first real match-history API backed by the shared archive
- replay-worker scaffold
- basic Go-side contract types
- first backend-target match-state flow now exists on the TypeScript/shared-engine side
- first Go-side match state/service layer exists in `services/realtime/internal/match`
- first match-service endpoints exist for create, fetch snapshot, and apply intent
- match-service now exposes a live WebSocket stream per match

Still missing:

- full authoritative match lifecycle for card effects and timed mechanics
- intent validation parity with the frontend ruleset
- full session ownership and reconnect claim flow
- Redis integration now started through the matchmaking ticket-store seam
- Postgres integration now started through the guest profile/rating store seam and extended into the shared match archive
- distributed queue lifecycle beyond the local file-backed service
- deepen the new Postgres-backed guest-store seam into fuller platform data ownership
- deepen the new Postgres-backed archive seam into fuller replay/history ownership
- deepen the new Postgres-backed account-store seam into fuller user/account ownership
- evolve the new Redis-backed ticket-store seam from whole-store persistence toward a more distributed matchmaking/session model
- richer rating model beyond guest Elo-style updates
- result finalization for non-guest and non-local match flows
- durable replay/event export beyond archived snapshots

### 4. Frontend Integration

Status: `partially done`

Completed:

- current UI runs through Next.js
- browser-only image preloading guarded for SSR compatibility
- build works in Next.js without breaking visible app structure
- browser-side authoritative backend lab route added at `/authoritative`
- shared web client layer added for the match service
- main app now bootstraps a backend match alongside the preserved local UI shell
- ordinary non-card moves in the main app can now be submitted to the backend first when the board is in a clean standard-chess state
- backend snapshots are now hydrated back into the main app board, move list, clocks, and check-state UI
- main app draw, resign, and chat controls now use backend intents when authoritative sync is live
- main app now shows backend-sync status in the right-side round panel
- `/authoritative` now auto-subscribes to live backend snapshots over WebSocket
- main page now auto-subscribes to the same live backend match stream
- web clients now perform periodic backend resyncs so clocks can reconcile during longer turns
- backend now owns elapsed clock updates and pushes live clock snapshots every second to subscribed clients
- main page stops running its own local countdown when backend authority is live
- Next.js now proxies normal match HTTP calls through same-origin `/api/realtime/*` routes instead of relying on browser-to-8081 CORS for create/fetch/intent requests
- Go-side piece contracts now include richer state flags used by the frontend piece model
- backend move validation now handles a first special-mechanics slice: frozen pieces cannot move, shielded captures are blocked server-side, and pawn promotion can be resolved by the backend
- backend now owns the first real card-targeting flow slice: `Freeze` and `Shield` support `play_card` and `select_target` with pending card state and card consumption
- main frontend now routes `Freeze` and `Shield` through backend authority when sync is live instead of only resolving them locally
- backend card targeting now also supports `Sniper` and `Bad Sniper`, including legality checks that reject removals which would expose either king to check
- main frontend now routes `Sniper` and `Bad Sniper` through backend authority when sync is live instead of only resolving them locally
- backend now supports the first multi-step transform card flow: `Promote` and `Demote` can enter pending target state, return safe transform options, and finalize a chosen transform with backend validation
- main frontend now hydrates backend-provided transform options into the existing promo picker for authoritative `Promote` / `Demote` flow
- backend transform flow now also supports `Promote Him` and `Demote Him`, including enemy-target and any-piece target rules with backend-generated safe options
- main frontend now routes `Promote Him` / `Demote Him` through the same authoritative transform picker path
- backend now supports a multi-step `Teleport` flow with authoritative source-piece selection, destination selection, and king-safety validation
- main frontend now hydrates backend `Teleport` pending state and routes the existing teleport interaction through backend authority when sync is live
- backend now supports a multi-step `Jump` flow with authoritative source-piece selection, destination selection, and server-side safety validation
- main frontend now hydrates backend `Jump` pending state and routes the existing jump interaction through backend authority when sync is live
- backend now supports a multi-step `Swap Me` flow with authoritative owned-piece selection, second-piece selection, and server-side no-check validation
- main frontend now hydrates backend `Swap Me` pending state and routes the existing swap interaction through backend authority when sync is live
- backend now supports a multi-step `Clone` flow with authoritative source-piece selection, adjacent empty destination selection, and server-side king-safety validation
- main frontend now hydrates backend `Clone` pending state and routes the existing clone interaction through backend authority when sync is live
- backend now supports a multi-step `Swap Us` flow with authoritative owned-piece selection, enemy-piece selection, and server-side no-check validation
- main frontend now hydrates backend `Swap Us` pending state and routes the existing swap interaction through backend authority when sync is live
- backend now supports a multi-step `Swap Him` flow with authoritative enemy-piece selection, second-enemy-piece selection, and server-side no-check validation
- main frontend now hydrates backend `Swap Him` pending state and routes the existing swap interaction through backend authority when sync is live
- backend now supports `Borrow` with authoritative temporary control transfer, server-side king-safety validation, and automatic ownership reversion after the borrowing turn ends
- main frontend now routes `Borrow` through backend authority when sync is live
- backend now supports `Mind Control` with authoritative permanent control transfer and server-side king-safety validation
- main frontend now routes `Mind Control` through backend authority when sync is live
- backend now supports `Parasite` with authoritative host/target linking, move-time link updates, and capture-triggered linked destruction checks
- main frontend now routes `Parasite` through backend authority when sync is live
- backend now supports `Lavaground` with authoritative trap placement, move-time burn resolution, and trap decay across later moves
- main frontend now routes `Lavaground` through backend authority when sync is live and hydrates backend-owned lava squares into the existing board overlay
- backend now supports `Invisible` with authoritative off-board ghost state, invisible move resolution, and materialization on check or late capture
- main frontend now hydrates backend-owned invisible ghost state into the existing ghost-piece UI and sends invisible moves back through backend authority
- backend now supports `Unabomber` with authoritative bomb attachment, bomb carrier tracking across moves, and turn-based 3×3 explosion resolution
- main frontend now routes `Unabomber` through backend authority when sync is live and hydrates backend-owned bomb state into the existing bomb overlays and warnings
- backend now supports `Half Fuse` card application with authoritative two-step selection, bishop+rook special-case queen upgrade, and fusion-aware king-safety validation
- main frontend now routes `Half Fuse` through backend authority when sync is live while preserving the existing fusion animation and fused-piece presentation
- backend now supports `Full Fusion` card application with authoritative two-step selection, redundancy checks, and fusion-aware king-safety validation without the half-fuse point cap
- main frontend now routes `Full Fusion` through backend authority when sync is live while preserving the existing fusion animation and fused-piece presentation
- backend now supports `Fog Village` with authoritative 3x3 zone placement, per-owner replacement, and full-round decay
- main frontend now hydrates backend-owned fog zones into the existing board masking effect and no longer falls back to local-only move submission just because fog is active
- backend now supports `Double Move (Twin)` and `Double Move (Solo)` as authoritative turn modifiers, including same-piece/different-piece enforcement and the no-check-on-first-move rule
- main frontend now routes authoritative double-move card activation through the backend and hydrates backend-owned double-move state into the existing banners and move flow
- backend now supports `Reverse` with authoritative rollback to the previous completed move state, while preserving current-turn ownership for the card player
- main frontend now routes authoritative `Reverse` through the backend and preserves the existing reverse feedback/animation messaging
- backend now supports `Undo` as an authoritative one-shot trap that nullifies the opponent's next card play
- main frontend now routes authoritative `Undo` through the backend and preserves the existing user-facing feedback for an armed nullification
- backend now supports `Mirror` as an immediate authoritative board mutation that repeats the last move pattern with the first valid matching own piece
- main frontend now routes authoritative `Mirror` through the backend instead of resolving it only in local board state
- backend now supports `Fake Piece` with authoritative pending placement, empty-square validation, and king-safety checks before spawning the fake pawn
- main frontend now routes authoritative `Fake Piece` through backend `play_card` + `select_target` flow instead of keeping it purely local
- backend now supports `Black Hole` with authoritative two-square targeting, persisted timed trap state, and delayed king-immune area explosions
- main frontend now routes authoritative `Black Hole` through backend `play_card` + `select_target` flow instead of keeping it in session storage
- backend now supports sacrifice selection state and authoritative `Small Sacrifice` / `Big Sacrifice` resolution, including board safety checks and backend-owned reward card draws
- main frontend now routes sacrifice-card selection through backend pending state instead of resolving the piece removals and reward cards only locally
- backend now supports `Gambler` as an immediate authoritative hand-transfer effect with deterministic backend-owned outcome resolution
- main frontend now routes authoritative `Gambler` through backend hand-state updates and preserved win/lose animations
- backend now supports `Radar` as an immediate authoritative reveal-state effect that stays active for the rest of the current turn and clears automatically when the turn passes
- main frontend now hydrates backend-owned radar reveal state instead of relying only on a local boolean toggle
- backend now supports `Cheater` as an immediate authoritative engine-helper state with owner-bound turn countdown
- main frontend now hydrates backend-owned cheater countdown state into the existing engine panel instead of relying only on a local timer
- backend now supports `Joker` as an authoritative pending hand-transform flow, so the chosen transformed card is created by the server instead of only locally in React state
- main frontend now keeps the existing Joker picker UI but sends the selected transformation back to the backend in synced matches
- backend now supports `Fortress` with authoritative 2x2 zone placement, blocked enemy entry on move/card-driven placement paths, and full-round decay
- main frontend now routes authoritative `Fortress` through backend `play_card` + `select_target` flow instead of leaving it as dead card metadata
- backend intent ownership is now stricter: unknown `playerId` values are rejected instead of falling back to the active turn color
- backend draw handling now rejects a draw response from the same side that offered the draw
- local dev defaults now point to a fresh Go match-service on `:8082`, avoiding the stale locked `:8081` backend process that was causing browser/runtime mismatches
- match-service `/healthz` now returns JSON service metadata instead of a bare string, which makes runtime verification simpler during debugging
- web match-service client now defaults to talking directly to `http://localhost:8082/api` in local dev, because the Next dev proxy routes were intermittently corrupting their chunk cache on Windows
- backend regression tests now look up starter cards by mechanic instead of brittle hand indexes, so new card additions no longer break unrelated tests
- local services now share a durable file-backed match archive, so created/updated matches survive backend restarts and can be queried from `platform-service`
- `platform-service` now exposes `/api/platform/matches` and `/api/platform/matches/{matchId}` for archived match-history reads
- web app now has a `History` page that reads persisted archived matches from `platform-service` through Next API proxy routes
- `History` now shows archived board previews, move history, active effects, hands, clocks, chat, and raw snapshot detail
- platform-service now owns basic guest identity creation/resume with file-backed guest profiles
- guest session resume now validates a persisted private session secret instead of trusting bare `guestId`
- platform-service guest persistence is now pluggable and can run on a transactional SQLite-backed guest store instead of only the local JSON profile file
- platform-service can now also persist guest profiles and rating finalization in Postgres, giving the repo its first actual Postgres-backed infrastructure slice
- platform-service account persistence is now pluggable and can run on transactional SQLite or real Postgres backends instead of only the local JSON account file
- main web shell now hydrates player names and ratings from platform guest sessions instead of only hardcoded labels
- main web shell and queue room handoff now prefer backend-issued guest session secrets as the source for match seat secrets instead of inventing fresh browser-only seat claims
- active room recovery can now rebuild missing seat secrets from validated guest session secrets when room metadata is absent or partial
- platform-service now exposes backend match-seat claims, so the web app can recover actual room-specific seat secrets for archived/restarted rooms instead of guessing from guest session state alone
- main web shell now consumes gateway-owned bootstrap recovery instead of stitching guest-session resume and seat-claim calls together directly from the browser
- platform-service now supports a pluggable live match-claim store, including a Redis-backed backend for backend-owned room claim recovery beyond browser-local metadata
- platform-service match claims now issue opaque claim tokens and can resolve them later from the live claim store, which gives gateway a backend-owned way to translate seat claims into room secrets
- live room claims now behave as renewable backend leases with expiry timestamps, so active use refreshes the claim while stale room claims age out instead of living forever
- main web shell now proactively renews active room claim leases through gateway before expiry, so live rooms stay on backend-issued opaque claim tokens instead of falling back to raw seat secrets after long sessions
- guest sessions now also issue renewable opaque session tokens with expiry, and gateway/bootstrap can resume those sessions from the token path instead of always sending the raw guest session secret back to platform-service
- matchmaking-service now exposes a real local ticket lifecycle with queue join, status fetch, cancel, and simple auto-match
- web app now has a `Queue` page backed by the local matchmaking API for casual/rated ticket flow smoke tests
- matched queue rooms can now bootstrap real authoritative matches by room id and open them in the main board shell
- queue tickets now persist in browser storage and can resume after reloads instead of disappearing from the local matchmaking flow
- matchmaking-service queue tickets now persist to a shared file-backed store and survive local backend restarts
- platform-service now finalizes queued guest-match results idempotently and updates persisted guest ratings
- platform-service now exposes guest rankings and the `Rankings` tab renders a real leaderboard page
- platform-service now exposes recent guest profiles with persisted W/L/D stats and the `Community` tab renders a real guest directory page
- queued match bootstrap now carries guest identity metadata into authoritative match creation and archived history
- `History` now shows which guest profiles played each archived match instead of only anonymous room ids
- `History` and guest-profile recent-match surfaces now also show linked account identity when archived players have claimed accounts
- account profiles now persist direct account-owned rating history entries across file, SQLite, and Postgres backends
- account detail routes now expose derived season summaries from that rating history, and the web `Account` / `Rankings` surfaces now show current season momentum instead of only flat ladder totals
- guest profiles now show recent archived matches and can jump directly into focused `History` detail for those matches
- `History` can now focus on one guest's archived matches and jump back into guest profiles from match detail
- archived snapshots now carry replay frames from the backend position history
- `History` now includes a replay scrubber for stepping through archived board states instead of only viewing the final snapshot
- archived match detail now preserves the full backend event log instead of only the last emitted event batch
- main app now restores the last active authoritative match from browser storage when no explicit room URL is present
- live match websocket clients now auto-reconnect with backoff after socket drops instead of failing permanently on the first disconnect
- starting a fresh game now clears old room-bound URL state instead of accidentally reopening the same matched room again
- backend now resolves authoritative seat ownership from stored guest ids, not only `white` / `black` placeholder player ids
- main app now submits authoritative moves, cards, draw/chat/resign intents using the match seat guest ids when available

Still missing:

- replace the remaining local match authority with server snapshots/events
- isolate animation state from gameplay state more cleanly
- split `apps/web/src/App.tsx` into smaller modules
- shift focus from gameplay-card migration into platform work: persistence, matchmaking, auth, ratings, reconnects, and replay/history storage
- support optimistic UI with rollback from authoritative backend events

### 5. Product Systems

Status: `started`

Completed:

- guest session creation and resume
- guest session resume validation with private session secrets
- casual/rated local queue smoke-test flow
- queue ticket resume across browser reloads
- queued room handoff into real authoritative matches
- guest rating updates on finished queued matches
- rankings surface
- community/profile surface
- archived match history surface
- first guest-linked account claim/session flow
- in-app `Account` page for local account claiming and session resume
- in-app `Account` page now also shows recent archived matches for each claimed account seat
- gateway-owned account-session bootstrap and stale-token recovery through linked guest sessions
- account handle visibility in `Community` through platform account directory/lookups
- `Rankings` now uses an account-first leaderboard derived from linked guest ratings and records instead of only showing guest-first rows with account badges
- rated-result finalization now has an account-first platform path, so queued rated rooms can validate winners against archived account seats instead of only guest ids while still updating the linked rating records underneath
- account persistence now owns direct ladder stats and finalized-match idempotency across file, SQLite, and Postgres backends instead of leaving account standings purely derived from guest rows forever
- guest-result finalization now also backfills linked account ladder stats when both seats already belong to claimed accounts, which keeps older guest-first rated rooms from drifting away from account-facing rankings and profiles

Still missing:

- full account/auth flow beyond guest-linked local accounts
- full custom lobby flow
- richer replay browser timeline and event playback
- reporting
- moderation tools
- richer ratings history and season model

### 6. Scale Hardening

Status: `not started`

Missing:

- load tests
- reconnect storm tests
- queue spike tests
- persistence recovery tests
- replay consistency audits
- backend metrics and tracing

## Biggest Current Risks

### Frontend authority still exists

The current gameplay flow is still largely driven by:

- `apps/web/src/App.tsx`

That means the architecture is not yet truly backend-authoritative even though ordinary moves can now pass through the backend and the shared packages/backend scaffolds now exist.

### `App.tsx` is still too large

The file is still acting as:

- gameplay controller
- UI controller
- animation coordinator
- card state manager
- clock state manager
- replay state manager

This is the next major refactor target.

### Go toolchain is not yet on PATH in this shell

At the time of this update, `go version` still fails in the current terminal, so the install/path needs verification before backend services can be compiled here.

The user has separately confirmed `go version go1.26.2 windows/amd64` in a fresh PowerShell, and backend compilation now works by calling the Go binary directly from `C:\Program Files\Go\bin\go.exe`.

## Recommended Next Work

### Immediate next step

Port the next heavy-mechanics slice from `apps/web/src/App.tsx` into backend-friendly shared logic, with the remaining smaller local-only mechanics and then persistence/product systems as the best next candidates now that `Mirror`, `Fake Piece`, `Black Hole`, and `Gambler` are migrated.

Target pieces:

- card play intent handling beyond the current `Freeze`, `Shield`, `Sniper`, and `Bad Sniper` slice
- target selection flows for multi-step cards
- timed effect state
- move modifiers like fusion, ghost, bomb, lava, fog, and double-move
- authoritative endgame packaging

### After that

Implement first real backend match loop in `services/realtime/cmd/match-service`.

Target outcome:

- create/load a match state
- accept an intent payload
- validate/apply it through shared engine logic
- return updated snapshot

This is now partially complete for:

- create match
- fetch match
- apply move/chat/draw/resign intents in the Go service

### Then

Swap the remaining main-game interaction paths from local-only resolution to backend-approved events while preserving current visual behavior.

## Verification History

Completed successfully:

- `pnpm lint`
- `pnpm build`
- `pnpm --filter @chess404/web build`
- `C:\Program Files\Go\bin\go.exe test ./...` from `services/realtime`
- proxied app path `POST http://localhost:3000/api/realtime/matches`
- proxied sniper flow through the app path:
  - `play_card(sniper)`
  - `select_target`
  - piece removal
  - card consumption
- direct backend promote flow:
  - `play_card(promote)`
  - `select_target(target piece)`
  - backend pending target/options
  - `select_target(selectionId=<pieceType>)`
  - final transform and card consumption
- direct backend promotehim flow:
  - `play_card(promotehim)`
  - `select_target(target piece)`
  - backend pending target/options
  - `select_target(selectionId=<pieceType>)`
  - final enemy-piece transform and card consumption
- backend teleport flow:
  - `play_card(teleport)`
  - `select_target(source piece)`
  - backend pending source square
  - `select_target(destination square)`
  - final move and card consumption
- backend lavaground flow:
  - `play_card(lavaground)`
  - `select_target(empty square)`
  - backend lava square stored with decay counter
  - later move onto lava burns non-king landing piece and removes the triggered trap
- backend invisible flow:
  - `play_card(invisible)`
  - `select_target(own non-king piece)`
  - piece removed from board into backend ghost state
  - later `make_move` from the ghost square updates or materializes the invisible piece authoritatively
- backend unabomber flow:
  - `play_card(unabomber)`
  - `select_target(own non-king piece)`
  - backend bomb tracker attached to that piece
  - later move updates the bomb carrier square and white-turn handoff can trigger a 3×3 explosion
- backend halffuse flow:
  - `play_card(halffuse)`
  - `select_target(first own piece)`
  - backend stores first-piece metadata
  - `select_target(adjacent second own piece)`
  - backend applies fused result or bishop+rook queen upgrade authoritatively

Not yet verified:

- live main-board backend move flow in the browser
- live main-board backend draw/resign/chat flow in the browser
- live main-board backend `Freeze` / `Shield` / `Sniper` / `Bad Sniper` card flow in the browser
- live main-board backend `Promote` / `Demote` card flow in the browser
- live main-board backend `Promote Him` / `Demote Him` card flow in the browser
- live main-board backend `Teleport` card flow in the browser
- multiplayer flow

## Notes

- Latest local platform hardening now in place:
  - authoritative intents can carry per-seat `playerSecret`
  - the Go backend now enforces those secrets when a match was created with them
  - the main web app persists seat secrets in room metadata so refresh/reconnect does not drop seat ownership
  - guest session resume is now backed by a persisted private session secret across file / SQLite / Postgres guest stores, so a forged bare `guestId` no longer silently reclaims that guest profile
  - the main web app and queue room handoff now prefer those backend-issued guest session secrets as the source for seat claims before falling back to browser-generated room secrets
  - queued rooms still reopen normally, but direct/casual rooms no longer affect guest ratings
  - match queue mode is now carried into archive history so `History` can show whether a match was `rated`, `casual`, or direct
  - `platform-service` now also enforces those rating rules server-side:
    - only archived `rated` matches can finalize guest results
    - the archived match must already be `finished`
    - posted winner / white guest / black guest must match the archived room data before ratings can change
  - `match-service` can now lazily rehydrate archived rooms after a backend restart:
    - archived snapshots now persist private seat-secret metadata alongside public room history
    - full internal position history is restored too, so restart recovery does not break cards like `Reverse`
    - existing room ids can be fetched again after restart instead of always forcing a fresh create
  - the stack now exposes lightweight operational status data:
    - `match-service` reports loaded room / active room / subscriber counts
    - `platform-service` reports archive mix and guest/rating counts
    - `matchmaking-service` reports rated/casual queue totals
    - `gateway` now aggregates those checks into a control-plane `system status` and `session bootstrap` surface
    - the web app now has a `Status` tab that reads those service snapshots through Next API routes

- The current implementation is a strong foundation pass, not the full migration.
- The architecture is now pointed in the right direction, but match authority has not fully moved off the frontend yet.
- This file should be updated every time a major milestone is completed.
