# Pre-Launch Audit Findings

Generated: 2026-05-23

**Total Findings: 118** (18 CRITICAL, 35 HIGH, 40 MEDIUM, 25 LOW)
**Current Launch Readiness Score: 42/100**

---

## State Key

- ✅ FIXED
- 🔧 IN PROGRESS
- ⏳ PENDING

---

## CRITICAL (18)

### Backend — Auth & Security

| # | Status | Finding | File | Lines |
|---|--------|---------|------|-------|
| 1 | ✅ | Auth bypass via string-match fallback in `requireIntentColor` | `state.go` | 2973-3028 |
| 2 | ⏳ | Invisible piece `RoundsLeft` never decrements (turn already switched) | `state.go` | 3459-3465 |
| 3 | ⏳ | Unvalidated promotion allows illegal piece types (e.g., king) | `state.go` | 2541-2546 |
| 4 | ⏳ | Client-controlled RNG seed enables full card-draw prediction | `state.go` | 3037-3042 |
| 5 | ⏳ | GuestID trimming inconsistency — unrecoverable stuck seats | `state.go` | 528-602 |
| 6 | ⏳ | `startBroadcaster` goroutine/ticker leaks forever, no shutdown | `state.go` | 91-93 |
| 7 | ⏳ | Session secrets (`PlayerSecret`, `ClaimToken`) returned in API responses | `gateway/main.go` | 54-69, 1006-1018 |
| 8 | ⏳ | No TLS — all HTTP traffic plaintext | `gateway/main.go` | 199-201 |
| 9 | ⏳ | Internal `.railway.internal` URLs leaked in bootstrap response | `gateway/main.go` | 416-437 |
| 10 | ⏳ | Preview tokens (email verification, password reset) leaked in API responses | `platform-service/main.go` | 879-913, 1019-1021 |
| 11 | ⏳ | Dangerous default: `ACCOUNT_AUTH_EXPOSE_PREVIEW_TOKENS` defaults to `"true"` | `platform-service/main.go` | ~2943 |
| 12 | ⏳ | Unauthenticated `IssueGuestSession(guestID)` — no credential check | `guests_postgres.go` | 120-147 |
| 13 | ⏳ | Zero authorization in moderation system — any caller can ban/suspend | `moderation.go` | 202-516 |
| 14 | ⏳ | Account restriction silently overwritten (map keyed by accountID) | `moderation.go` | 302 |

### Frontend — UX & Stability

| # | Status | Finding | File | Lines |
|---|--------|---------|------|-------|
| 15 | ⏳ | No resign confirmation dialog | `App.tsx` / `useMatchEngine.tsx` | — |
| 16 | ⏳ | No error boundaries around any component | `App.tsx` | — |
| 17 | ⏳ | No touch support on BoardCanvas | `BoardCanvas.tsx` | — |
| 18 | ⏳ | `PlatformContext` typed as `any` — defeats all TypeScript safety | `PlatformContext.tsx` | 7 |

---

## HIGH (35)

### Backend

| # | Status | Finding | File | Lines |
|---|--------|---------|------|-------|
| 19 | ✅ | Wide-open WebSocket `CheckOrigin: return true` | `match-service/main.go` | 29 |
| 20 | ✅ | CORS `Access-Control-Allow-Origin` echoes any origin | `match-service/main.go` | 287-306 |
| 21 | ✅ | FIDE-illegal timeout (no insufficient material check) | `state.go` | 3348-3356 |
| 22 | ⏳ | `pushSnapshot` silently drops events when channel full | `state.go` | 1264-1280 |
| 23 | ⏳ | Unauthenticated subscription to any match ID | `state.go` | 947-981 |
| 24 | ⏳ | PlayerSecret echoed in error messages | `state.go` | 3027 |
| 25 | ⏳ | No idempotency key (`clientMoveId`) for intents | `state.go` | ~784 |
| 26 | ⏳ | No proper ELO formula — hardcoded/placeholder | `platform-service` | — |
| 27 | ⏳ | No graceful shutdown (`signal.Notify`) | all `main.go` files | — |
| 28 | ⏳ | Downstream error messages forwarded verbatim to clients | `gateway/main.go` | 1397-1407 |
| 29 | ⏳ | Insufficient path param validation (URL-encoded traversal) | `gateway/main.go` | 190-192 |
| 30 | ⏳ | No request body size limits on 30+ handlers | `platform-service/main.go` | various |
| 31 | ⏳ | Hardcoded DB credentials in source (`postgres:postgres`) | `platform-service/main.go` | 2586-2731 |
| 32 | ⏳ | Session secret returned in every API response | `platform-service/main.go` | various |
| 33 | ⏳ | No server Read/Write timeouts (Slowloris) | `platform-service/main.go` | 81 |
| 34 | ⏳ | Insecure auth fallback chain (token→secret) | `platform-service/main.go` | 203-211 |
| 35 | ⏳ | No CSRF protection on any endpoint | `platform-service/main.go` | all POST |
| 36 | ⏳ | Insecure SMTP auth — `smtp.PlainAuth` without TLS validation | `account_email_delivery.go` | 279-280 |
| 37 | ⏳ | Email outbox exposes email addresses | `platform-service/main.go` | 948-970 |
| 38 | ⏳ | Predictable `crypto/rand` fallback to timestamp | `guests.go:403-409`, `queue.go:456-462` |
| 39 | ⏳ | No rate limiting on any operation | all files | — |
| 40 | ⏳ | `ResumeGuestByToken` does not rotate session token | `guests_postgres.go:183-211` |
| 41 | ⏳ | `queryPostgresGuests` concatenates raw SQL strings | `guests_postgres.go:478-484` |
| 42 | ⏳ | Notification events silently dropped on full channel buffer | `notifications.go:391-402` |
| 43 | ⏳ | Known race window: out-of-order broadcasts | `state.go` | 1286-1360 |
| 44 | ⏳ | `invisible` move consumes turn without progress when shield blocks | `state.go` | 2721-2732 |
| 45 | ⏳ | `abort` move-count check bypassed with double-move | `state.go` | 2898-2907 |
| 46 | ⏳ | `collectBroadcasts` holds write lock while iterating ALL matches | `state.go` | 1310-1360 |

### Frontend

| # | Status | Finding | File | Lines |
|---|--------|---------|------|-------|
| 47 | ✅ | Corrupted emoji strings (🃏, ⬆️, 📡, 💡, 🏰) | `useMatchEngine.tsx` | — |
| 48 | ✅ | `cardUsedBy` not reset on turn change | `useMatchEngine.tsx` | — |
| 49 | ✅ | `finishCardUse` not called in authoritative targeting paths | `useMatchEngine.tsx` | — |
| 50 | ✅ | Animation timer refs not cleaned up on unmount | `useMatchEngine.tsx` | — |
| 51 | ✅ | `replaceLastHistorySnapshot` missing from 16 card effects | `state.go` | — |
| 52 | ✅ | Clock rollback on intent error | `state.go` | — |
| 53 | ⏳ | `App.tsx` and `useMatchEngine.tsx` are near-duplicate files | `App.tsx` / `useMatchEngine.tsx` | — |

---

## MEDIUM (40)

| # | Status | Finding | File | Lines |
|---|--------|---------|------|-------|
| 54 | ⏳ | `markMatchFinished` does not clear temporary game state | `state.go` | 920-931 |
| 55 | ⏳ | `DrawOfferedBy` persists across multi-step card selection | `state.go` | 1686-2445 |
| 56 | ⏳ | `gambler` card uses weak deterministic randomness | `state.go` | 3240-3255 |
| 57 | ⏳ | `cloneEvents` shallow-copies payload maps | `state.go` | 4158-4174 |
| 58 | ⏳ | `Service.subs` map grows unbounded if unsubscribe not called | `state.go` | 947-981 |
| 59 | ⏳ | `persistSnapshot` silently swallows archive errors | `state.go` | 933-945 |
| 60 | ⏳ | 1-second ticker creates latency for time-sensitive broadcasts | `state.go` | 1282-1303 |
| 61 | ⏳ | `cardTemplateByMechanic` panics on unknown mechanic | `state.go` | 508-515 |
| 62 | ⏳ | No request body size limit on most handlers | `gateway/main.go` | various |
| 63 | ⏳ | No rate limiting on any gateway endpoint | `gateway/main.go` | all |
| 64 | ⏳ | No request ID or tracing headers | `gateway/main.go` | — |
| 65 | ⏳ | No security response headers | `gateway/main.go` | — |
| 66 | ⏳ | `healthz` is a no-op, does not check backends | `gateway/main.go` | 222-228 |
| 67 | ⏳ | No Content-Type validation on incoming requests | `platform-service/main.go` | all POST |
| 68 | ⏳ | Error messages leak account existence (enumeration) | `platform-service/main.go` | various |
| 69 | ⏳ | X-Forwarded-For spoofable for rate limit bypass | `auth_rate_limit.go` | 175-192 |
| 70 | ⏳ | Unbounded loop in match-claims/active | `platform-service/main.go` | 374-396 |
| 71 | ⏳ | Health endpoint leaks internal state | `platform-service/main.go` | 146-181 |
| 72 | ⏳ | Unauthenticated account/guest listing | `platform-service/main.go` | 1932-2216 |
| 73 | ⏳ | Plaintext secrets in secondary stores (match claims) | `platform-service/main.go` | 2767 |
| 74 | ⏳ | Handle-based admin auth (mutable identifier) | `platform-service/main.go` | 3292-3306 |
| 75 | ⏳ | SetAccountRestriction overwrites without warning | `moderation.go` | 268-307 |
| 76 | ⏳ | Block create vs. update not distinguishable | `moderation.go` | 202-237 |
| 77 | ⏳ | Duplicate `randomToken` functions with divergent fallbacks | `guests.go:403`, `queue.go:456` |
| 78 | ⏳ | Weak deterministic guest name generation | `guests.go:391-401` |
| 79 | ⏳ | `EnsureGuest` does not indicate created vs. updated | `guests_postgres.go:47-118` |
| 80 | ⏳ | Content-Type mismatch on 404 responses | `platform-service/main.go` | 88-93 |
| 81 | ⏳ | `useMatchTimer` — setInterval cleanup race on re-render | `useMatchTimer.tsx` | 81-112 |
| 82 | ⏳ | `useAuthoritativeMatch` — stale closure in useCallback deps | `useAuthoritativeMatch.ts` | 36 |
| 83 | ⏳ | `cardPool.ts` — module-level mutable state | `cardPool.ts` | — |
| 84 | ⏳ | Duplicate storage key constants across 3 files | `App.tsx`, `AuthPage.tsx`, `AccountPage.tsx` | — |
| 85 | ⏳ | No shared fetch error handling utility | all service files | — |
| 86 | ⏳ | `formatDateTime` duplicated in 4 files | multiple | — |
| 87 | ⏳ | Board canvas has no ARIA fallback | `BoardCanvas.tsx` | — |
| 88 | ⏳ | Hardcoded emoji icons for cards | `CardsPage.tsx` | — |
| 89 | ⏳ | `connectToMatchStream` handler not wrapped in error boundary | `match-service.ts` | 176 |
| 90 | ⏳ | `gateway/main.go`: JSON encode error silently discarded | `gateway/main.go` | 1490-1494 |
| 91 | ⏳ | `gateway/main.go`: No CORS headers | `gateway/main.go` | 1490-1494 |
| 92 | ⏳ | `gateway/main.go`: No per-request context deadline on outbound calls | `gateway/main.go` | 1346-1395 |
| 93 | ⏳ | `gateway/main.go`: No field-level length limits | `gateway/main.go` | all handlers |

---

## LOW (25)

| # | Status | Finding | File | Lines |
|---|--------|---------|------|-------|
| 94 | ⏳ | Redundant `hasPartialSeats` logic when both seats set | `state.go` | 545-554 |
| 95 | ⏳ | `buildSnapshot` passes nil presence, `persistSnapshot` clears it anyway | `state.go` | 801, 933 |
| 96 | ⏳ | `starterHandCardsForMode` returns 30+ cards exceeding maxHandSize | `state.go` | 517-526 |
| 97 | ⏳ | MatchID auto-gen uses `now.UnixMilli()` with no randomness | `state.go` | 532-535 |
| 98 | ⏳ | `insertCardToHand` can exceed maxHandSize | `state.go` | 4046-4063 |
| 99 | ⏳ | `cloneCardsWithOwner` weak ID uniqueness | `state.go` | 4005-4013 |
| 100 | ⏳ | `gambler` lose-branch can return itself | `state.go` | 3253-3263 |
| 101 | ⏳ | `undo` card effect persists across turns | `state.go` | 1411-1460 |
| 102 | ⏳ | `log.Printf` used instead of structured logging | everywhere | — |
| 103 | ⏳ | `parseSquareOptions` reuses `parseParasiteSquare` | `state.go` | 4094-4102 |
| 104 | ⏳ | `loadArchivedMatchLocked` shallow-copies events | `state.go` | 1021-1030 |
| 105 | ⏳ | `recoveredPresenceSeedTime` can return zero time | `state.go` | 1078-1089 |
| 106 | ⏳ | `ensureRemovalDoesNotCreateCheck` blocks beneficial attacks | `state.go` | 3761-3776 |
| 107 | ⏳ | Gateway root handler accepts all HTTP methods | `gateway/main.go` | 210-220 |
| 108 | ⏳ | No HEAD/OPTIONS support anywhere | `gateway/main.go` | all |
| 109 | ⏳ | Magic numbers without named constants | `gateway/main.go` | various |
| 110 | ⏳ | Repeated `time.Now().UTC()` in same request | `gateway/main.go` | multiple |
| 111 | ⏳ | Unchecked json.Encode errors (many locations) | `platform-service/main.go` | many |
| 112 | ⏳ | No strict HTTP method whitelisting | `platform-service/main.go` | various |
| 113 | ⏳ | Password reset confirmation no second factor | `platform-service/main.go` | 1157-1196 |
| 114 | ⏳ | Potential body close issues on nil r.Body | `platform-service/main.go` | various |
| 115 | ⏳ | Error swallowing in Stats() methods | `guests_postgres.go:326-328` |
| 116 | ⏳ | In-memory maps not guarded against nil receiver | many store files |
| 117 | ⏳ | App.css contains Create React App boilerplate | `App.css` | — |
| 118 | ⏳ | DROP_RATES comment not enforced (no runtime assertion) | `CardsPage.tsx` | — |

---

## Fix Plan

### Phase 1: Critical Backend (Priority Order)
1. Invisible piece RoundsLeft never decrements
2. Unvalidated promotion
3. Client-controlled RNG seed
4. GuestID trimming inconsistency
5. startBroadcaster goroutine leak
6. Session secrets in gateway responses
7. Internal URLs leaked
8. Preview tokens leaked
9. IssueGuestSession unauthenticated
10. Moderation auth
11. Account restriction overwrite

### Phase 2: Critical Frontend
12. Resign confirmation
13. Error boundaries
14. Touch support
15. PlatformContext typing

### Phase 3: HIGH Backend
16-45. All HIGH items

### Phase 4: HIGH Frontend + MEDIUM/LOW
46-118. Remaining items
