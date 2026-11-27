# IPL Auction Arena — Persistent Project Context

Baseline: 2026-11-27 | Source: `ipl_auction_db-main (3).zip` | Requirements: `README.md`

## 1. Working agreement and status

This file records the inspected archive, not a deployed environment. README is authoritative for MVP requirements. Preserve Go 1.26, Chi, pgx/raw SQL, handler/service/repository boundaries and existing transactional workflows. No GORM, SQLBoiler, mandatory Redis or unrelated feature additions. Single backend instance initially. All progress must be labelled source-present, integrated, verified or blocked; never equate source presence with passing tests. Update this file after each significant change with affected files, migrations, contract changes, tests and next steps. Commit it alongside the code. For another conversation, use the latest repository snapshot plus this file; no claim of automatic access to changes on the user's machine.

**Original audit baseline (historical):** 86 original files, 69 Go files, 4 migration pairs (initial down empty), four CSVs, no frontend, no Go tests, no CI, no infrastructure definition or `.env.example`. Source only; no application logic modified in this audit. No production readiness claim.

### Current checkpoint — Milestone 1B.1 (2026-11-27): player administration authorization

- Migration `000005_add_user_admin_permission` adds `public.users.is_admin` (boolean, NOT NULL, default false); rollback drops the column. Apply before running the updated backend. Existing users and newly registered users are non-admin unless explicitly promoted through trusted database operations. No public promotion endpoint. Back up database before migration; do not reset existing data.
- `UserRepository` includes `IsAdmin` in reads and returns the DB default on Create (does not accept caller-provided privilege); `UserService.IsAdmin` obtains live permission. Login/registration and `/auth/me` expose read-only `is_admin` for frontend display. Registration DTO unchanged.
- `middleware/identity.go` extracts existing context helper without changing session auth. `middleware/admin.go` checks authenticated context identity against live database authorization, returning 401 anonymous, 403 non-admin, 500 lookup failure, or forwarding admin requests. Permission is rechecked every request. No client header/body or room-host status grants admin rights.
- `api/router.go` and `cmd/server/main.go` wire middleware and user service. All `/admin/players` routes are admin-gated; legacy POST/PUT/DELETE `/players` routes have the same gate to avoid bypass; authenticated player GET endpoints remain compatible. No change to room-specific host privileges.
- Verified: `GOTOOLCHAIN=local GO111MODULE=off go test -race -v backend/internal/middleware/{identity,admin,admin_test}.go` passed 5 table subtests plus permission revocation test; gofmt clean. Full build/tests still blocked by Go 1.23.2 vs required 1.26; PostgreSQL migration and real HTTP/session integration not executed here. User reported previous health/schema/build checks done but did not provide output; not independently verified.
- Design choice: shared existing participant session with separate admin authorization and route namespace; dedicated admin authentication/session lifecycle (README Security) remains open, as do CSRF/production cookie hardening and full route integration tests. Admin role does not grant room host permissions. Dataset edit validation remains for Milestone 2.
- Patch ZIP contains only 12 changed/new code or migration files plus this context, not `.env`/config or unrelated files. Apply to Milestone 1A baseline. Next: verify migration, auth and admin route HTTP status on local machine, then invitation-code enforcement and remaining room lifecycle security before automatic queue.

### Previous checkpoint — Milestone 1A (2026-11-27)

First implementation batch, not yet integration-verified. The sections below describing original code and defects are historical audit findings; this checkpoint supersedes those specific findings. All other audit gaps remain open.

- `auction/validation.go`: canonical approved rules; exact decimal comparison, reject custom nonzero/nonempty overrides, normalize values in `RoomService.CreateRoom`. 13 unit cases in `validation_test.go` pass, including `-race` in standalone stdlib-only package. Existing room rows not changed; database-level rule constraints/reconciliation remain pending. Explicit numeric zero in existing JSON contract still means default for integer settings.
- `RoomRepository.GetByIDForUpdate`: transaction-only row lock; `MarkInProgress` updates WAITING to IN_PROGRESS in the same transaction as auction status update. `AuctionService.StartAuction` now requires host participation, 4–room-capacity participants, exact team count (unique/FK-backed), and atomically transitions room/auction. Auction create, joins, team creation, manual player add, room updates/deletes use the same room row lock with existing transaction manager. Constructor wiring changed in `main.go`. No automatic first reveal or start event/outbox yet; pool readiness and legacy inconsistent room repair pending. Host still explicitly joins using existing API (idempotent join). Existing room deletion FK restrictions retained.
- `RoomRepository.GetForUser` / `GetByIDForUser` plus room handler/service: room list and ID detail are restricted to host or existing member. Code lookup remains invitation discovery; raw UUID join API unchanged for compatibility. Explicit invitation code enforcement, rate limiting and admin auth still pending.
- Fixed duplicate WHERE in `GetByRoomAndStatus`; canonical player role `BATTER` instead of `BATSMAN`.
- `AuctionWorkflowRepository.AcceptBid`: checks persisted timer deadline against PostgreSQL `clock_timestamp()` after acquiring auction row lock; expired or null deadline rejects new bids. Seven-second reset uses `statement_timestamp()` rather than transaction-start `NOW()`. Accepted request replay remains before deadline check; request fingerprint, exact bid precision, unsold eligibility, actor and timeout execution still pending. Database test of timer boundaries required.
- `lobby_integration_test.go`: opt-in real PostgreSQL test covers simultaneous joins for 4 slots, complete franchise selection, nonmember room access, nonhost start, atomic status results, locked join, existing host rejoin and duplicate start. Requires migrated disposable database. Not compiled or executed here because project requires Go 1.26 and no PostgreSQL available.
- `gofmt -l backend` returned no output. `GOTOOLCHAIN=local GO111MODULE=off go test -race ./backend/internal/auction` passed. `GOTOOLCHAIN=local go test ./...`, `go build ./backend/cmd/server`, and `go vet ./...` blocked at go.mod version check (local Go 1.23.2). No dependency, migration, dataset, README, or frontend changes. No end-to-end success claim.
- Next: run full build/tests with Go 1.26 and disposable PostgreSQL; finish Milestone 1 (admin authorization, invitation enforcement, old-state/DB rule reconciliation, smoke checks), then dataset eligibility/automatic queue (Milestone 2). Do not start frontend integration before REST/WS contracts stabilize.

Local validation from repo root (use a disposable migrated DB; never production):

```bash
go version
go build ./...
go vet ./...
go test -race ./...
IPL_TEST_DATABASE_URL='postgresql://USER:PASSWORD@localhost:5432/ipl_auction_test?sslmode=disable' go test -race -v ./backend/internal/services -run TestLobbyLifecycleIntegration
```

Remaining milestones: 1 foundation/security, 2 dataset/queue, 3 engine/timers/settlement, 4 actors/WS, 5 host/unsold/completion, 6 outbox/reconnect/recovery, 7 frontend foundation/lobby, 8 live sports broadcast UI, 9 squads/results/admin UI, 10 multiplayer acceptance/security/performance/deployment. Keep README's approved 12 phases as the product traceability baseline.

## 2. Approved rules and scope

- Private rooms: 4–10 participants, one unique franchise per participant. Creator is host, with equal bidding rights. Room locked at start; completed results read-only. Host disconnect does not pause or transfer control; participant must reconnect before bidding. Host may cancel only pre-start.
- ₹120 crore starting purse, 18–25 players, max 8 overseas, min increment ₹0.10 crore. First bid exactly base price; subsequent bids at least increment higher. Dynamic reserve must preserve minimum squad feasibility. No deadline-expired bids.
- Reveal 15-second first-bid window; no bid means UNSOLD. Each accepted bid resets deadline to 7 seconds; expiry with bid means SOLD. Server clock authoritative. Host chooses category order, shuffle, start, pause/resume, skip, end and unsold-round control.
- Unsold rounds: up to six nominations per team per round; duplicate nominations by different teams trigger mini-auction restricted to nominators; no bid leaves unsold. Completion only when all squads satisfy constraints.
- Persist bid/state/event and atomic sale/purse/squad/outbox. All room mutations go through one ordered room command processor. Publish only after commit. Snapshot before live events; gap recovery; restart enters RECOVERING, never auto-resolves stale deadlines, waits for host resume.
- Frontend: Next.js/React/TypeScript, Tailwind, shadcn/ui, Motion; landing/lobby/live/team/unsold/results/admin screens; responsive, reduced-motion, authorized graphics. Separate admin permissions. No licensed IPL assets unless permitted.
- Reference-only records separate from auction-eligible records per README. Existing generated data conflicts; resolve explicitly before seeding. Retention mode not required for MVP; do not conflate that mode with dataset eligibility.

## 3. Original audit architecture and execution paths (see current checkpoint)

`backend/cmd/server/main.go` loads config → connects pgx pool → constructs repositories, services and handlers → builds Chi router → starts HTTP server → graceful shutdown (10s). Router uses request ID, CORS, logger, recovery, 30s HTTP timeout; cookie auth for protected routes. Config reads `DATABASE_URL` (required), `SERVER_PORT` (8000), `APP_ENV` (development), `FRONTEND_URL` (http://localhost:3000). `go.mod` at repository root. DB pool max 10/min 2, connect/ping timeout 10s. `DBTX` permits pool or transaction; `TransactionManager.WithinTransaction` begins, invokes callback, commits or rolls back.

Current request flow: client → Chi → middleware → handler → service → repository → PostgreSQL. Target flow adds authenticated WS gateway → room manager/actor → engine → existing transaction services/repositories → durable event/outbox → gateway. REST mutations and WebSocket commands must share that same authority; no competing state paths.

- Auth: register bcrypt-hashes password; login compares hash, generates opaque token, persists SHA-256 token hash with 30-day expiry; middleware checks session expiry/revocation and updates last-used; logout revokes. Cookies HttpOnly, SameSite=Lax, Secure depends on request TLS. No guest identity/admin roles yet.
- Lobby: `CreateRoom` generates 12-character uppercase hex random code, stores host user ID and rule values. Join checks existing user membership first, then WAITING and capacity. Team handler resolves participant from authenticated user, team service validates franchise and copies room purse. DB enforces unique room/franchise and participant/team. Host adds individual active master players; base price copied into room_players.
- Start: host + WAITING room/auction + at least 4 participants and 4 teams. `AuctionRepository.StartAuction` changes only auction to ACTIVE, not room; critical inconsistency. No automatic reveal.
- Main nomination: POST → `MainNominationWorkflowService` → transaction → locks auction, checks ACTIVE/MAIN and host, no active player, locks available room player → sets IN_AUCTION and 15s deadline → persists event and outbox → commits. Queue ordering/eligibility not enforced here.
- Bid: handler authenticates, resolves participant/team from membership, validates request body, constructs payload → `BidWorkflowService` → `AuctionWorkflowRepository.AcceptBid`. Checks request ID twice (before and after auction lock), ACTIVE/current player/team, player status, squad/overseas, opening amount/increment/purse using PostgreSQL numeric, calculates cheapest remaining-player reserve respecting overseas slots, inserts bid sequence, updates current bid and 7s deadline, inserts event + outbox in transaction. No expiry validation or actor yet. Replay checks do not bind returned bid to matching team/player/amount.
- Sale: `AuctionSaleService` / repository lock auction, team, player, validate expiry/affordability/squad limits; deduct purse, insert squad, mark SOLD, event/outbox, clear active fields atomically. Not constructed/called by runtime. Unsold analogous (requires no bid, deadline passed). No scheduler invokes either. `NOW()` is transaction-start time; use post-lock clock and consistent command ordering when implementing timer logic.
- Unsold: nomination workflow locks auction, validates ACTIVE/UNSOLD round, same-room team and UNSOLD player, enforces max six and unique nomination, persists event/outbox. No mutation route, no round start/end/mini-auction orchestration; service does not set required EventPayload.
- Completion: transaction locks auction/room/teams, checks no active player, counts team squads and overseas; updates both states and event/outbox. Not wired, requires IN_PROGRESS room (start currently leaves WAITING), does not fully reconcile purse/spend or require all main pool/rounds done.
- Events: read endpoints by sequence and after sequence; workflows allocate monotonic sequence under auction lock; no publisher or WS gateway. Outbox helper methods exist; no retry/reclaim loop or startup recovery. No coherent snapshot endpoint.

## 4. Database and dataset

12 domain tables from README + `sessions` + `schema_migrations`. UUID PKs, NUMERIC(6,2) crore purse/price, foreign keys (many composite room-scoped), unique room code, one auction per room, unique room/user membership, unique room/master player, unique franchise per room, unique team/participant, unique squad/room_player (prevents double sale), unique auction bid sequence/request ID/event sequence, unique outbox event ID, unique nomination per team/player/round. States: room WAITING/IN_PROGRESS/COMPLETED/CANCELLED; auction WAITING/ACTIVE/PAUSED/RECOVERING/COMPLETED; round MAIN/UNSOLD; player AVAILABLE/IN_AUCTION/SOLD/UNSOLD; outbox PENDING/PROCESSING/PUBLISHED. State combination invariants not fully enforced.

Migration 1 is a PostgreSQL 17.11 pg_dump-style schema, includes migration tracking table, changes session search_path and contains `transaction_timeout` (PG17 compatibility concern); initial down is empty. Migration 2 adds users.updated_at index. Migration 3 adds nullable category and queue sequence, eligibility defaults false and indexes. Migration 4 extends categories to reference-only types; rollback can fail if such records remain. No migration runner or import command included. No unique queue position; no explicit event foreign key from outbox to event. Global player classification is mutable while live auctions join master rows; snapshot relevant attributes or protect edits. Review migration tool ownership of schema_migrations and fresh apply/rollback on PG17 before release.

Dataset counts verified by CSV parsing:

| File | Rows | Notes |
|---|---:|---|
| datasets/players_master_roles_updated.csv | 543 | 413 AUCTION_CANDIDATE (eligible), 130 RETENTION_ONLY_REFERENCE (ineligible); source/provenance columns present |
| datasets/ipl_auction_players_seed.csv | 413 | 295 domestic, 118 overseas; base prices ₹0.30–2.00 Cr; roles BATTER/BOWLER/ALL_ROUNDER/WICKET_KEEPER |
| datasets/ipl_auction_retention_130_seed.csv | 130 | All domestic flags false for overseas; ₹2.00 Cr each |
| datasets/player_auction_metadata_543.csv | 543 | All marked eligible, including 130 references; conflicts with master and README |

`generate_metadata.py` validates record counts/categories and duplicate normalized names; incorrectly emits `is_auction_eligible: true` for both record types. Re-run in a temporary copy reproduced byte-identical metadata and reported 543 eligible. No DB import, provenance verification or live dataset inspected. Player API omits category/eligibility; service accepts BATSMAN while SQL and seed require BATTER. Room pool addition checks active only; category query filters eligible but nullable auction_sequence scanned into int and no assignment path. Need transactional seed/import, stable IDs, eligibility validation, pool snapshot and deterministic queue order.

## 5. README traceability (static, unverified unless noted)

| Requirement area | State | Evidence / gap |
|---|---|---|
| Private code / sharing | Partial | random code & unique constraint; all rooms/code exposed via authenticated list; join uses room_id; no UI share links |
| Capacity / membership / host | Partial | service checks; no host auto-membership; count-then-insert race; start doesn't lock room |
| Franchise selection | Partial | ten allowed codes + uniqueness; no safe pre-start change path; start accepts missing teams; race with start |
| Fixed rules | Incorrect | client-configurable purse, minimum squad, overseas limit, increment |
| Start & lock | Incorrect | only auction ACTIVE, room remains WAITING; no atomic start event or pool feasibility |
| Category order/shuffle | Partial | schema/read query only; no write or host control |
| Reveal | Partial | host transaction sets deadline; manual nomination; no queue enforcement |
| Opening/increment/purse/squad/overseas | Present | locked bid transaction; numeric precision/replay/deadline risks; not tested |
| Dynamic purse protection | Partial | cheapest available/unsold per team under overseas slots; no collective feasibility across teams; eligibility/ordering gaps |
| Sequential commands | Partial | auction row locks serialize selected workflows, not all room mutations; actor empty |
| 15s/7s deadlines | Partial | persisted deadlines; no scheduler/expiry reject; no pause/resume |
| Sold/unsold atomic transaction | Present but disconnected | service/repository code not wired, no runtime tests |
| Host pause/resume/skip/end | Missing | no routes/commands |
| Unsold nomination/rounds | Partial | transaction exists, not reachable; payload missing; no mini-auction/restricted bidders |
| Completion/read-only results | Partial | unused transaction + squad read APIs; no completed result projection or full invariant |
| Admin identity/data | Incorrect | player writes accessible to any authenticated user; no role or admin UI |
| Reconnect & refresh | Partial | persisted sessions/membership, client-set connection status; no snapshot or WS |
| WS protocol & sequence gaps | Missing | persisted event reads only; no gateway, envelopes, ordering contract or snapshots |
| Durable state/outbox | Partial | bid/reveal/sale/etc persist events; no worker/retry or publish-after-commit integration |
| Restart RECOVERING | Missing | enum only; no recovery manager |
| Security | Partial/incorrect | bcrypt/hash/cookie/CORS/auth + some ownership; privacy/admin/CSRF/rate limits/production secure-cookie gaps |
| REST API contract | Partial | route surface exists; differs from proposed /api/code-based paths; no versioned contract, do not add duplicate authority |
| Frontend screens/design/animation | Missing | no frontend folder |
| Setup/config/migrations/import | Partial | Go/config/SQL/CSVs exist, no reproducible environment scripts or importer |
| Unit/concurrency/DB/WS/UI/E2E tests | Missing | zero tests; smoke blocked |
| Deployment/monitoring | Partial | health and graceful shutdown, no readiness/HTTP server timeouts/CI/backup/monitoring infrastructure |
| License | Missing | README requests decision, no license file |

## 6. Original defect register and priorities (see current checkpoint)

P0 = blocks correctness/privacy or foundational implementation; P1 = required MVP hardening; P2 = maintainability/deployment follow-up. Static findings require regression tests.

| ID | Priority | Evidence and required action |
|---|---|---|
| B01 | P0 | `auction_repository.go:StartAuction` updates auctions only; `auction_service.go` prechecks nontransactionally. Atomically lock room + auction, enforce exact participant/team match and pool readiness, set both statuses, event/outbox. Reconcile existing inconsistent rows deliberately. |
| B02 | P0 | `auction_workflow_repository.go:AcceptBid` does not read/check deadline. Reject expired bids under lock using authoritative clock; reset 7s after acceptance, test exact boundary and stale timer command. |
| B03 | P0 | `auction/` and `rooms/` eight package-only files; sale/unsold/completion not wired. Implement actor/engine/scheduler and generation/deadline identity for stale timeout protection; no automatic stale resolution after restart. |
| B04 | P0 | `room_handler.go:GetAllRooms/GetRoomByID` returns private rooms/codes to all logged-in users. `JoinRoom` uses raw room UUID; protect list/details and require code for new member, retain rejoin. |
| B05 | P0 | `router.go` registers global player POST/PUT/DELETE under normal Auth, not admin. Add explicit admin authorization and secure provisioning; prevent live master edits altering room rules. |
| B06 | P0 | `room_player_repository.go:GetByRoomAndStatus` contains two WHERE clauses consecutively: SQL syntax failure. Remove duplicate and test all player statuses. |
| B07 | P0 | `player_service.go:validatePlayer` accepts BATSMAN; initial schema and CSV accept BATTER. Align canonical role (optionally explicit legacy normalization) and test. |
| B08 | P0 | `generate_metadata.py` hardcodes true, 130 reference-only records mislabelled. Align with README and master data; room add/nominate also enforce eligibility. Do not silently change data policy. |
| B09 | P0 | `room_service.go:CreateRoom` accepts caller-supplied core rule settings. Enforce approved immutable constants and DB checks/migration; do not silently coerce old rooms. |
| B10 | P0 | `room_participant_service.go:JoinRoom`, `team_service.go:CreateTeam`, room update/delete, player add check state separately from mutation. Serialize lobby writes on room lock and share lock order with start; capacity race and post-start changes possible. |
| B11 | P1 | `AcceptBid` duplicate request returns stored bid without comparing team/player/amount; bind retry identity and canonical request; mismatches conflict. UUID validation and exact <=2 decimals required; DB NUMERIC rounds extra precision, payload may differ from persisted bid. |
| B12 | P1 | `room_player_repository.go` category scan nullable `auction_sequence` into int; insert doesn't assign it. Build/backfill deterministic unique room queue; null-safe read until populated. |
| B13 | P1 | `unsold_nomination_service.go:NominatePlayer` omits EventPayload required by transactional repository. Add authenticated mutation command and round phases; restrict mini-auction bidders; verify max six and duplicates. |
| B14 | P1 | `auction_event_repository.go:Create`, `outbox_event_repository.go:Create`, `unsold_nomination_repository.go:Create` omit required room_id (legacy helpers); inspect bid Create too. Consolidate unsafe direct writes around transaction workflows or remove after caller audit. |
| B15 | P1 | `auction_completion_repository.go` only checks purse nonempty (DB nonnegative), lacks full spend reconciliation/main-round completion; no host command. Validate final squads/purse/rounds, read-only invariant. |
| B16 | P1 | `outbox_event_repository.go` transitions pending→processing→published only, no publisher, claim timeout/retry/recovery. Persist canonical envelope/sequence and implement crash-tolerant publication. |
| B17 | P1 | `auth_handler.go` Secure depends on r.TLS, potentially false behind TLS proxy; no rate limiting or CSRF/origin enforcement beyond CORS; main nomination body not bounded. Validate production cookie config and all request boundaries. |
| B18 | P1 | `main.go` HTTP server has no explicit header/idle timeouts or DB readiness; WS needs dedicated lifetime outside 30s REST middleware. Add config, readiness, observability and graceful WS shutdown. |
| B19 | P1 | Initial migration is PG17 dump with schema_migrations + search_path setting, empty down; validate fresh migrations using chosen runner on PG17, avoid changing already-applied migration checksums without plan. |
| B20 | P1 | `room_player_service.go`/bid/sale use live master player status/overseas, mutable classification affects active room and squad history. Snapshot required attributes or version immutable records; protect existing rooms. |
| B21 | P1 | Client can set own CONNECTED/DISCONNECTED via REST, bid doesn't check actual connection; derive from gateway membership and enforce reconnect requirement. Host controls same. |
| B22 | P2 | Auth middleware writes session last_used every request; repeated participant lists and squad/CTE queries inside locks; measure DB contention, optimize indexed membership and throttled activity writes before scaling. No benchmark claims. |
| B23 | P2 | Error handling matches strings, sometimes returns wrapped DB errors; use typed domain errors, stable HTTP/WS codes, generic internal response; no secrets in logs. |
| B24 | P1 | `DeleteRoom` uses hard DELETE and existing FKs without cascades; populated WAITING room cancellation may fail. Implement guarded cancellation or transactionally defined cleanup, preserve history. |
| B25 | P1 | No automated tests/CI; no verified build. Establish test matrix and smoke before production changes. |

## 7. Existing REST routes (not contract promises)

Public: GET `/health`; POST `/auth/register`, `/auth/login`, `/auth/logout`. Protected: GET `/auth/me`.

- `/rooms`: POST/GET `/`; GET/PUT/DELETE `/{id}`; GET `/code/{code}`; POST `/join`; GET `/{id}/participants`; GET `/{roomID}/teams`, `/{roomID}/participants/{participantID}/team`; POST `/players`; GET `/{roomID}/players`, `/status/{status}`, `/category/{category}` (under players); POST/GET `/{roomID}/auction`; POST `/{roomID}/auction/start`; GET `/{roomID}/squads`.
- `/participants`: GET `/{id}`, PUT `/{id}/connection-status`. `/teams`: POST `/`, GET `/{id}`, GET `/{teamID}/squad`. `/players`: POST/GET `/`, GET/PUT/DELETE `/{id}`. `/room-players`: GET `/{id}`.
- `/auctions`: GET `/{id}`, POST `/{roomID}` (duplicate creation alias); GET/POST `/{auctionID}/bids`, GET `/{auctionID}/bids/request/{requestID}`; GET `/{auctionID}/unsold/rounds/{roundNumber}` and `/teams/{teamID}`; GET `/{auctionID}/events` and `/{sequenceNumber}`; POST `/{auctionID}/nominate`. GET `/bids/{id}`; GET `/squads/{id}`.
- No WebSocket, snapshot, host pause/resume/skip/end, unsold mutation, final results/admin route. Existing HTTP bid request uses room_player_id, bid_amount (decimal string), request_id (DB UUID). Do not change contracts without documenting migration and ensuring one authority.

## 8. Verification and environment

- ZIP extraction succeeded. Original source inventory checked, README requirement sections reviewed, Go source and SQL examined statically. `gofmt -l backend` returned no deviations (format/parse only, not compile).
- `GOTOOLCHAIN=local go version` = go1.23.2. `go test ./...`, `go build ./...`, `go vet ./...` with local toolchain rejected because go.mod requires >=1.26.0. Auto-toolchain download failed DNS/network to proxy.golang.org. No dependency downgrade or go.mod changes. No PostgreSQL CLI/server or Docker available in this environment; no DATABASE_URL provided. Runtime build, migrations, API tests and E2E NOT verified. No claim of dependency/CVE scan.
- Python CSV counts and isolated metadata generation succeeded; regenerated bytes match existing CSV and reproduce all-eligible defect.
- On provisioned dev environment: `go version`, `go mod download`, `go build ./...`, `go vet ./...`, `go test ./...`, `go test -race ./...`; use PostgreSQL 17 disposable test database, verify migration up/down policy, seed validated dataset, start `go run ./backend/cmd/server` from repository root. `curl -i http://localhost:8000/health` only proves HTTP liveness. DB integration and multiplayer smoke required before declaring success.

## 9. Incremental development sequence and acceptance gates

1. **Foundation (next):** make reproducible local setup and smoke harness; repair start/room locking, fixed rules, SQL/role/eligibility defects, private room and admin permissions; tests for concurrent joins, start, all participants assigned, nonmember access, admin denial and status/category queries. No frontend dependency.
2. **Auction correctness:** exact decimals and request replay binding, shared event persistence, deadline check under lock, actor and timer integration, atomic sale/unsold; deterministic clock tests + real PG rollback, simultaneous bids, duplicate IDs, late bids, stale timer, no duplicate sale, 15s→UNSOLD and 7s→SOLD, purse/squad invariants.
3. **Host lifecycle/unsold:** pause/resume/skip, queue order and shuffle, round transitions, six nominations, mini-auction eligibility, complete only valid squads, read-only results. Test disconnected host behavior.
4. **Real time and recovery:** authenticated WS gateway, snapshot sequence barrier, ordered outbox and retries, duplicate delivery, reconnect/host disconnect, restart RECOVERING and resume; no stale deadline auto-sale. Single per-room authority across REST and WS.
5. **Frontend:** all README screens and responsive visual design; server-confirmed state, countdown and bid feedback, reduced motion, reconnect, admin. Use neutral/authorized assets.
6. **Production gate:** end-to-end 4–10 participant acceptance cases, race tests, security checks, migrations/backups, performance measurement, readiness, TLS/WSS, deployment/CI and docs. Redis only if scaling requires it.

Before each task: read current files and this context; state impact/acceptance criteria; make smallest coherent change; run relevant verification; update context with results and unresolved issues. No speculative claims of completion.

## 10. Change log

- 2026-11-27: Milestone 1A implementation: transaction-scoped lobby operations, atomic auction start, fixed rule validation, scoped room reads, deadline check, SQL/role corrections, tests. See current checkpoint for verified versus pending work.

- 2026-11-27: Baseline audit, documented architecture/requirements/defects/verification and next steps. Added this file only; original project code and datasets unchanged. Full runtime verification blocked by toolchain/network/database availability.

## 11. File responsibility index

The following inventory is generated from original files; function names identify entry points. Empty packages intentionally marked.
- `backend/cmd/server/main.go` — main.
- `backend/internal/api/router.go` — NewRouter.
- `backend/internal/auction/bidding.go` — package-only placeholder.
- `backend/internal/auction/engine.go` — package-only placeholder.
- `backend/internal/auction/state.go` — package-only placeholder.
- `backend/internal/auction/timer.go` — package-only placeholder.
- `backend/internal/auction/validation.go` — package-only placeholder.
- `backend/internal/config/config.go` — Load, getEnv.
- `backend/internal/database/database.go` — ConnectDB.
- `backend/internal/database/transaction.go` — NewTransactionManager, WithinTransaction.
- `backend/internal/handlers/auction_event_handler.go` — NewAuctionEventHandler, GetEventBySequence, GetEventsAfterSequence.
- `backend/internal/handlers/auction_handler.go` — NewAuctionHandler, GetAuctionByRoom, GetAuctionByID, CreateAuction, StartAuction.
- `backend/internal/handlers/auth_handler.go` — NewAuthHandler, Register, Login, Logout, writeAuthResponse, writeMessage.
- `backend/internal/handlers/bid_handler.go` — NewBidHandler, PlaceBid, GetBidByID, GetBidByRequestID, GetAuctionBids.
- `backend/internal/handlers/main_nomination_handler.go` — NewMainNominationHandler, NominatePlayer.
- `backend/internal/handlers/me_handler.go` — NewMeHandler, GetMe.
- `backend/internal/handlers/player_handler.go` — NewPlayerHandler, CreatePlayer, GetPlayerByID, GetAllPlayers, UpdatePlayer, DeletePlayer.
- `backend/internal/handlers/room_handler.go` — NewRoomHandler, generateRoomCode, CreateRoom, GetRoomByID, GetRoomByCode, GetAllRooms, UpdateRoom, DeleteRoom, writeJSON.
- `backend/internal/handlers/room_participant_handler.go` — NewRoomParticipantHandler, JoinRoom, GetRoomParticipants, GetParticipantByID, UpdateConnectionStatus.
- `backend/internal/handlers/room_player_handler.go` — NewRoomPlayerHandler, AddPlayerToRoom, GetRoomPlayerByID, GetPlayersByRoom, GetPlayersByStatus, GetAvailablePlayersByCategory.
- `backend/internal/handlers/squad_handler.go` — NewSquadHandler, GetSquadByID, GetSquadByTeam, GetSquadsByRoom.
- `backend/internal/handlers/team_handler.go` — NewTeamHandler, CreateTeam, GetTeamByID, GetTeamByParticipant, GetTeamsByRoom.
- `backend/internal/handlers/unsold_nomination_handler.go` — NewUnsoldNominationHandler, GetNominationsByRound, GetNominationsByTeamAndRound.
- `backend/internal/middleware/auth.go` — Auth, UserIDFromContext.
- `backend/internal/middleware/cors.go` — CORS.
- `backend/internal/middleware/logger.go` — Logger.
- `backend/internal/middleware/recovery.go` — Recovery.
- `backend/internal/repository/auction_completion_repository.go` — NewAuctionCompletionRepository, CompleteAuction.
- `backend/internal/repository/auction_event_repository.go` — NewAuctionEventRepository, Create, GetBySequence, GetAfterSequence.
- `backend/internal/repository/auction_repository.go` — NewAuctionRepository, Create, GetByID, GetByRoom, scanAuction, UpdateStatus, StartAuction.
- `backend/internal/repository/auction_sale_repository.go` — NewAuctionSaleRepository, CompleteSale.
- `backend/internal/repository/auction_unsold_repository.go` — NewAuctionUnsoldRepository, MarkUnsold.
- `backend/internal/repository/auction_workflow_repository.go` — NewAuctionWorkflowRepository, AcceptBid, getExistingBid.
- `backend/internal/repository/bid_repository.go` — NewBidRepository, Create, GetByID, GetByRequestID, GetByAuction.
- `backend/internal/repository/db.go` — shared types/interfaces.
- `backend/internal/repository/main_nomination_workflow_repository.go` — NewMainNominationWorkflowRepository, NominatePlayer.
- `backend/internal/repository/outbox_event_repository.go` — NewOutboxEventRepository, Create, GetByID, GetPending, MarkProcessing, MarkPublished.
- `backend/internal/repository/player_repository.go` — NewPlayerRepository, Create, GetByID, GetAll, Update, Delete.
- `backend/internal/repository/room_participant_repository.go` — NewRoomParticipantRepository, Create, GetByID, GetByRoomAndUser, GetByRoom, UpdateConnectionStatus.
- `backend/internal/repository/room_player_repository.go` — NewRoomPlayerRepository, Create, GetByID, GetByRoomAndPlayer, GetByRoom, GetByRoomAndStatus, getPlayers, UpdateStatus, GetAvailablePlayersByCategory.
- `backend/internal/repository/room_repository.go` — NewRoomRepository, Create, GetByID, GetByCode, GetAll, Update, Delete.
- `backend/internal/repository/session_repository.go` — NewSessionRepository, Create, GetByTokenHash, GetActiveByUser, UpdateLastUsed, Revoke.
- `backend/internal/repository/squad_repository.go` — NewSquadRepository, Create, GetByID, GetByTeam, GetByRoom.
- `backend/internal/repository/team_repository.go` — NewTeamRepository, Create, GetByID, GetByParticipant, GetByRoom, UpdateRemainingPurse.
- `backend/internal/repository/unsold_nomination_repository.go` — NewUnsoldNominationRepository, Create, GetByID, GetByRound, GetByTeamAndRound.
- `backend/internal/repository/unsold_nomination_workflow_repository.go` — NewUnsoldNominationWorkflowRepository, CreateNomination.
- `backend/internal/repository/user_repository.go` — NewUserRepository, Create, GetByID, GetByEmail, GetAll, Delete.
- `backend/internal/rooms/actor.go` — package-only placeholder.
- `backend/internal/rooms/commands.go` — package-only placeholder.
- `backend/internal/rooms/manager.go` — package-only placeholder.
- `backend/internal/services/auction_completion_service.go` — NewAuctionCompletionService, CompleteAuction.
- `backend/internal/services/auction_event_service.go` — NewAuctionEventService, GetEventBySequence, GetEventsAfterSequence, ValidateEventPayload.
- `backend/internal/services/auction_sale_service.go` — NewAuctionSaleService, CompleteSale.
- `backend/internal/services/auction_service.go` — NewAuctionService, CreateAuction, GetAuctionByID, GetAuctionByRoom, StartAuction.
- `backend/internal/services/auction_unsold_service.go` — NewAuctionUnsoldService, MarkUnsold.
- `backend/internal/services/auction_workflow_service.go` — NewBidWorkflowService, AcceptBid.
- `backend/internal/services/bid_service.go` — NewBidService, GetBidByID, GetBidByRequestID, GetAuctionBids.
- `backend/internal/services/main_nomination_workflow_service.go` — NewMainNominationWorkflowService, NominatePlayer.
- `backend/internal/services/outbox_event_service.go` — NewOutboxEventService, GetOutboxEventByID, GetPendingEvents.
- `backend/internal/services/player_service.go` — NewPlayerService, validatePlayer, CreatePlayer, GetPlayerByID, GetAllPlayers, UpdatePlayer, DeletePlayer.
- `backend/internal/services/room_participant_service.go` — NewRoomParticipantService, JoinRoom, GetParticipantByID, GetRoomParticipants, UpdateConnectionStatus.
- `backend/internal/services/room_player_service.go` — NewRoomPlayerService, AddPlayerToRoom, GetRoomPlayerByID, GetPlayersByRoom, GetPlayersByStatus, GetAvailablePlayersByCategory.
- `backend/internal/services/room_service.go` — NewRoomService, CreateRoom, GetRoomByID, GetRoomByCode, GetAllRooms, UpdateRoom, DeleteRoom.
- `backend/internal/services/session_service.go` — NewSessionService, CreateSession, GetSessionByTokenHash, GetActiveSessionsByUser, UpdateLastUsed, RevokeSession.
- `backend/internal/services/squad_service.go` — NewSquadService, GetSquadByID, GetSquadByTeam, GetSquadsByRoom.
- `backend/internal/services/team_service.go` — NewTeamService, CreateTeam, GetTeamByID, GetTeamByParticipant, GetTeamsByRoom.
- `backend/internal/services/unsold_nomination_service.go` — NewUnsoldNominationService, NominatePlayer, GetNominationsByRound.
- `backend/internal/services/unsold_nomination_workflow_service.go` — NewUnsoldNominationWorkflowService, CreateNomination.
- `backend/internal/services/user_service.go` — NewUserService, CreateUser, GetUserByID, GetUserByEmail, GetAllUsers, DeleteUser.

Other files: `README.md` approved spec; `.gitignore` local exclusions; `go.mod`/`go.sum` dependency manifest/checksums; `backend/migrations/000001`–`000004` SQL changes described above; `datasets/*.csv` data inventory above; `generate_metadata.py` metadata generator.


## Local integration correction — Milestones 1A + 1B
- User compiler output showed five old service constructor signatures with the newer transaction-aware main.go. Cause consistent with an incomplete 1A application before the 1B patch.
- Recovery patch supplies all 13 changed/new 1A Go files not superseded by 1B. Retains 1B main.go, router, admin migration and auth changes; does not touch .env or config.go.
- Local build and runtime verification pending; do not remove transaction manager arguments to bypass compile errors.

## Milestone 1B — Invitation-code join enforcement
- User reported synchronization done; local build/test output not provided.
- Existing POST /rooms/join now requires JSON room_code instead of room_id. Retained existing route prefix/body convention instead of proposed README code-in-path route to avoid logging invitations through the current path logger. Removed GET /rooms/code/{code} preview route; members can still retrieve rooms by ID.
- RoomParticipantService resolves normalized uppercase room code via GetByCodeForUpdate within the existing transaction; existing membership is returned before status/capacity checks, while new participants must find a WAITING room with available slots. Typed errors map to 404/409; unexpected errors map to generic 500.
- Existing cryptographic 12-character uppercase hex code and database uniqueness constraint preserved; no migration required. Join rate limits and shareable frontend links remain pending.
- Updated lobby integration fixture and assertions for code-based joins, UUID-only rejection, locked-room rejection, concurrent capacity and existing-member rejoin. gofmt completed; full tests blocked by installed Go 1.23.2 versus required Go 1.26. Changes staged in working reference copy and delivered as inline instructions for manual local application; local validation pending.

## Milestone 2A — Dataset metadata correction and pre-import audit
- User manually applied invitation route changes and explicitly left lobby_integration_test.go pending. Delivery now combines file-by-file explanation with a changed-files ZIP; local build/test results not yet supplied.
- Existing POST /rooms/join route retained; old GET /rooms/code/{code} route removed. Updated lobby integration test included in this patch to align the code-based API and cover concurrent capacity, UUID-only rejection, host start, and rejoin.
- Fixed generate_metadata.py to propagate source eligibility instead of marking every record true. Regenerated datasets/player_auction_metadata_543.csv: 413 candidates and 130 reference-only records, preserving categories, names and prices. README requires reference-only records remain distinct; no decision made to expand the fantasy pool beyond the source eligibility labels.
- Added validate_player_dataset.py (read-only): checks identity uniqueness, file coverage, candidate/reference partition, source eligibility, metadata, candidate nationality/overseas flag, role and price. Exits 1 when import readiness fails. Current result: 44 candidates have UNVERIFIED nationality (including several retained stars already classified as candidates by the master); their default false overseas flag is not safe for live auction limits. Do not import or activate this pool until these records are verified and the intended fantasy pool policy is reconciled. Reference data is retained, not sold by default.
- Verified 543 unique metadata records, 413 true/130 false, reference-category consistency and deterministic generation. Audit correctly exits 1 with 44 nationality findings. Go test formatting passed; full build/integration tests blocked by Go 1.26 and unavailable PostgreSQL. No live database changes, no migrations, no automatic queue implementation yet.
- Next: resolve candidate nationality and approved pool policy, controlled database seed/metadata reconciliation, then transaction-safe room queue generation and host category ordering.


### Milestone 2B — Retention data enrichment (2026-11-27)
- Enriched 130 reference rows and corrected 44 previously UNVERIFIED candidate countries; official IPL retention roster, Sky Sports country lists and national-board/player sources. Country uses cricket representation (e.g. West Indies), not legal citizenship.
- Retention price restored from user game classification already present in master: 72 capped at 2.00 Cr and 58 uncapped at 0.50 Cr. These are not official reserve prices.
- Primary role corrections: Angkrish Raghuvanshi (BATTER), Jayant Yadav (ALL_ROUNDER), Sherfane Rutherford (BATTER). Other dual-role disagreements documented and existing role kept. Existing name keys preserved.
- Eligibility remains 413 candidates / 130 reference records pending explicit confirmation of 543-player fantasy pool. Source ledger provides proposed activation categories, not applied.
- SQL patch updates existing UUIDs in one guarded transaction; rejects missing/duplicate names or active auctions; no DB connection from this environment. Existing room-specific prices not rewritten.
- No invented image URLs or missing official auction-list fields. Local PostgreSQL execution pending.
- User clarified the 130 must use the same data structure as the 413, correctly sorted by cricket country, with a 2 Cr fallback where no confirmed game price exists. Both seed schemas match; `datasets/ipl_players_unified_543.csv` joins the six common seed columns with auction category/eligibility and sorts by country/name. 58 existing user-designated uncapped reference prices remain 0.50 Cr, rather than replacing already documented game prices. No official retention auction reserve price is implied.
- Unified data has 543 unique names, no missing/unverified country and all overseas flags consistent. 174 SQL update tuples match the unified data. SQL guards both room and auction status; stop the backend before applying, back up existing DB, and inspect any old room-specific player snapshots separately. Reference eligibility unchanged per README; the user requested common formatting, not explicit activation. No live database execution or role-by-role independent profile verification claimed; source ledger documents cross-checks and retained role decisions.

## Milestone 2C — Approved 543-player fantasy pool and category queue (2026-11-27)

- User explicitly approved all 543 players for the fantasy mega auction. Supersedes earlier 413-only eligibility. Preserve `dataset_record_type` as source provenance (413 original candidates, 130 retained-source records), not as eligibility. No new nationality research in this step; reuse corrected Milestone 2B data and source ledger. Existing approved capped/uncapped price values unchanged.
- `generate_metadata.py` derives categories from effective game capped status and normalized role; all metadata records eligible. Unified seed and master updated; validator checks identity, eligibility, category, price, country, and overseas classification. Categories: capped batter 38, bowler 89, all-rounder 68, wicketkeeper 29; uncapped batter 52, bowler 111, all-rounder 124, wicketkeeper 31; associate all-rounder 1; marquee 0. No arbitrary marquee designation.
- `datasets/update_fantasy_pool_543.sql` updates existing records by unique exact name in a single transaction; checks exact 543 active existing records and no active/paused/recovering auctions; preserves UUIDs, image URLs, active flags and historical room prices. Old 174 update filename contains the same revised script to avoid restoring obsolete eligibility. Apply with server stopped and backup first. No schema migration required beyond existing migrations 3/4.
- `auction/queue.go`: fixed approved snapshot size; complete category permutation validation and optional subset shuffle validation. Default order marquee (empty), capped batter/bowler/all-rounder/wicketkeeper, uncapped batter/bowler/all-rounder/wicketkeeper, associate all-rounder. Default within-category alphabetical; shuffle only selected categories. Future dataset expansion requires changing the explicit pool size policy.
- `repository/auction_queue_repository.go`: must execute under room lock and transaction; locks master for consistent generation; uses row_number, category rank and optional random sorting, upserts unique room-player rows with persistent 1..543 sequence. Rejects missing/ineligible pool, unexpected existing room rows, stale or noncontiguous prepared queue at start. Explicit prepare regenerates waiting queue. Existing manual room rows reused, never deletes players. `room_player_repository.go` returns category and sequence and ordered list; category filter excludes unsequenced rows.
- `services/auction_queue_service.go`, `handlers/auction_queue_handler.go`, `api/router.go`: authenticated host-only POST /rooms/{roomID}/players/queue, JSON {} defaults or category_order complete 10-item permutation + shuffle_categories optional list. Returns count and players. Existing GET /rooms/{roomID}/players returns persisted order to members. No constructor signature change. `auction_service.go` ensures queue before atomic room/auction start, preserves prepared order. `auction_handler.go` reports conflicts. `main_nomination_workflow_repository.go` enforces next available sequenced player; existing legacy active unsequenced auctions retain manual nomination compatibility. Automatic reveal/timer/sale orchestration not yet implemented. Category and role still read from master, so master edits mid-auction remain a future hardening item; do not edit master during live play.
- `auction/queue_test.go` passed under local Go 1.23.2 in isolated module-disabled pure package race run, alongside 13 existing rule cases. Python generator and dataset validator passed, 543 eligible and zero issues. Lobby integration test extended (requires migrated disposable DB seeded with approved 543 players), not run; full build blocked by Go 1.26 toolchain/network, PostgreSQL unavailable. Local user must run build, tests and HTTP/DB smoke checks before marking milestone verified. Previous local code edits assumed applied through 1B invitation and admin work; patch includes changed files and refreshed 2B datasets, excludes env/config/main/migrations. README updated to reflect user approval.
- Next milestone: automatic next-player reveal, authoritative deadline worker and transactionally resolved SOLD/UNSOLD state. Do not claim live auction engine completed.

### Milestone 2C — Local database verification (user-reported)
- User reported old Nomination Test Room cleanup succeeded: only one WAITING auction remained. Guarded `datasets/update_fantasy_pool_543.sql` completed with COMMIT, UPDATE 174, and subsequent independent SELECT returned 543 eligible records. Country counts total 543: India 375 and 168 overseas across 11 other cricket countries/teams. Master dataset update verified by supplied terminal output; no direct database access by assistant.
- Next local gate: build and race tests, create a fresh WAITING room, host POST /rooms/{roomID}/players/queue with {}, verify 543 unique players and contiguous sequences 1..543, then test host shuffle and authorization. Full queue runtime and automatic auction engine remain unverified/unimplemented respectively.


## Milestone 3A — Server-authoritative main auction engine (implementation, integration verification pending)

- User supplied DB output: 543 eligible, 375 India and 168 overseas, committed nationality/pool update; user reported queue verification done. Exact queue output not supplied.
- Added `backend/internal/auction/timer.go` and timer tests; ticker checks at 250 ms, cancellation-aware. `AuctionEngineService` uses bounded DB contexts and logs only committed transitions. `AuctionEngineRepository` selects due active MAIN auctions in IN_PROGRESS rooms, obtains auction row lock (`FOR UPDATE OF a SKIP LOCKED`), rechecks due condition, and performs one transaction-scoped nomination or settlement. Reuses nomination/sale/unsold repository methods, so event/outbox writes and purse/squad changes remain atomic. Auto nomination uses persisted auction sequence and room host identity from DB. No client request required. Main queue exhaustion leaves active idle state until host controls/unsold phase.
- Implementation decision: 3-second reveal/transition interval using auction updated_at, including first reveal; no change to 15/7-second bid windows. Nomination and bid events now contain authoritative persisted `timer_deadline_at`; check deadline again immediately before bid insert; sale/unsold compare database wall clock. Sale/unsold clear state with statement time to support transition interval. No schema or dependency changes.
- Server startup marks previously ACTIVE auctions RECOVERING before serving; new host-only POST `/auctions/{auctionID}/recover/resume` accepts synchronized MAIN auctions, verifies host participant, preserves current player/bid and gives full new 15s/7s window (decision: downtime excluded, exact remaining duration not yet persisted). Resume writes durable event/outbox. Legacy inconsistent rooms and UNSOLD recovery require later reconciliation; no automatic settlement on restart. Run only one backend process; multi-instance startup coordination not implemented. Manual `POST /auctions/{auctionID}/nominate` removed from router, service retained for reuse and tests.
- `lobby_integration_test.go` extends disposable DB fixture with forced deadlines, sequence order, opening/rolling timers, no early settlement, unsold, higher bid, recovery, host authorization, late bid rejection, concurrent sale attempts, single squad insertion, purse and sale event/outbox checks. Cleanup order updated for bids/squads/events. Do not point tests at working database. Execute with `IPL_TEST_DATABASE_URL` only for isolated migrated/seeded 543-player test DB.
- Verified locally in available Go 1.23.2 via `GOTOOLCHAIN=local GO111MODULE=off go test -race -v ./backend/internal/auction`: timer, queue, rule tests pass. Full `go build ./...` blocked by module Go 1.26.0 requirement; download unavailable, no PostgreSQL binary. Full compile, DB integration, HTTP smoke and actual concurrent transaction execution not verified. No claim of production readiness.
- Next: verify actual engine locally, then host controls/room actors & real-time delivery per roadmap. Pending: unsold mini-auction scheduling, main exhaustion event, pause/resume/skip, completion, true recovery state restoration and multi-instance coordination, WebSockets, outbox publisher, master-data edit protection during active auction, frontend, load/security testing.


## Milestone 3A — Local smoke verification (user-provided terminal output)
- Local backend port 8888. Room aaed78a0-0488-4455-a4db-f774e989189d, auction 7b961b71-ae3d-45ae-a01b-db71691f0285. Four franchises CSK, RR, RCB, PBKS started at 120 Cr. Room join, franchise selection, automatic nomination and UNSOLD verified. Opening timer exactly 15s. CSK bid 2.00 then RR 2.10 for Jake Fraser-McGurk; accepted events showed 7s, sale 7.15s after last bid, RR purse 117.90. Restart RECOVERING preserved player and froze settlement; non-host resume 403, host 200. Accepted bid 2.00 CSK on Travis Head preserved across shutdown/restart, then exactly one observed SOLD squad row and CSK purse 116.00 after resume. User-provided output validates this scenario, not comprehensive load or concurrency proof. Bid response returned zero created_at, addressed in 4A.

## Milestone 4A — Room actors and live WebSocket stream (implementation)
- `rooms/manager.go`, `actor.go`, `commands.go`: single FIFO channel per active auction, 64 queued command capacity, context cancellation, panic containment, idle cleanup; one auction per room. REST and socket bids share `BidHandler.placeAuthenticatedBid` and `BidWorkflowService.AcceptBid`; engine Advance and recovery resume use same manager. PostgreSQL transaction locks/deadlines remain authoritative. Lobby start still transaction guarded; future host/unsold commands not implemented. `database/transaction.go` rollback uses independent bounded cleanup context.
- `handlers/realtime_handler.go`: authenticated `/ws/auctions/{auctionID}`, session revalidation, origin restriction, membership guard, one live socket per room/user (new replaces old), snapshot then ordered committed events, ping/pong, bounded reads/writes, cancellation-aware shutdown. Client commands BID_PLACE and REQUEST_SNAPSHOT; other control commands pending. Snapshot carries server time and persisted deadlines; no per-second server countdown messages.
- `repository/realtime_repository.go`: repeatable-read snapshot with sequence watermark, player, teams, squads, latest event; 500ms per-socket polling, 2s refresh, max 100 contiguous replay; gaps fall back to full snapshot. Not shared push fan-out; no presence writes, no lobby socket, no distributed coordination. Outbox broadcaster/advanced recovery deferred. `go.mod` adds Gorilla WebSocket v1.5.3; go.sum not regenerated here, user must run go mod tidy. No DB migration.
- `repository/auction_workflow_repository.go`: retries must match original team, room player and numerical amount; both INSERT RETURNING and existing request SELECT now include created_at, scanned into Bid.CreatedAt (fix observed zero timestamp). Added integration assertions for timestamp/retry identity, snapshot membership and event continuity.
- Race tests for pure rooms and auction packages pass under available Go 1.23.2; full module requires Go 1.26.0, unavailable here due to network/toolchain limits. WebSocket handler/repository tests and PostgreSQL integration tests written but not executed. Run full go mod tidy/build/vet/test and local two-client smoke tests before commit. Keep local .env, config.go, .gitignore and database untouched by patch.

## Milestone 4A — User-reported local smoke tests and Git checkpoint
- Commit `4934bcc` on main/origin/main, clean working tree. Four sessions, snapshot synchronization, nomination/UNSOLD broadcasts, accepted socket bid sequence 5 and correct timestamp, concurrent REST equal-bid test (RR 200, CSK 409, sequence 6), identical retry returns original bid, cross-team/changed amount retry 409, one row per request ID, reconnect snapshot matches REST after observer receives events. DB confirmed RR SOLD event sequence 156. All-client SOLD delivery, simultaneous WS concurrency and full load testing remain open. Full local build/vet/race output not supplied in chat.

## Milestone 5A — Host controls (implementation, local verification pending)
- Added `repository/auction_control_repository.go`, `services/auction_control_service.go`, `handlers/auction_control_handler.go`; router and WebSocket gateway expose normal pause/resume. Both transports share AuctionService, the existing per-auction actor and PostgreSQL row lock. DB host identity and room participant membership checked inside transaction. ACTIVE MAIN/IN_PROGRESS only for pause; PAUSED MAIN/IN_PROGRESS only for resume. No revival of expired player. Existing schema used. Pause retains bid, player, old timer timestamps and stores paused_at; resume shifts both timestamps by pause duration. If idle, resume starts a fresh 3-second transition delay. Paused state persists on restart. Recovery route unchanged. No host disconnect auto-pause. REST requires authenticated host but not an active WebSocket connection (presence enforcement remains deferred).
- Transaction writes sequenced AUCTION_PAUSED/AUCTION_RESUMED and matching outbox entry with reason host_control. WebSocket HOST_PAUSE/HOST_RESUME return CONTROL_RESULT or ERROR; event stream continues to send committed events and snapshots. For paused countdown, use deadline minus paused_at.
- Extended disposable DB integration fixture for unauthorized/repeated controls, expired deadline rejection, long pause with original deadline in past, pause survival on restart, exact remaining time, blocked new bids, accepted bid preservation and event/outbox counts. `scripts/smoke_host_controls.sh` is an optional real-server test for no-bid or accepted-bid pause/resume; never use production auction. No .env/config/go.mod/go.sum/migration modifications.
- Patch reconstructed from prior uploaded repository plus Milestone 4A patch, not a fresh clone of commit 4934bcc. Apply only after git apply --check, preserving any newer local edits. Existing pure auction and rooms race tests passed with Go 1.23.2; full build/DB/HTTP/WS tests blocked here by unavailable Go 1.26 toolchain and PostgreSQL. Local verification required. Skip, unsold execution, completion and shared fan-out remain subsequent work.
