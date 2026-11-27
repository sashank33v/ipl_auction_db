# IPL Auction Arena

A real-time, multiplayer IPL fantasy auction platform where **4--10
friends** create a private room, select different IPL franchises, and
compete in a live player auction.

The application combines a reliable, server-authoritative auction engine
with a polished sports-broadcast-style interface. Participants can bid
in real time, manage their franchise purse and squad, nominate unsold
players, reconnect after network interruptions, and review final auction
results.

> **Fan-project disclaimer:** IPL Auction Arena is an independent,
> unofficial fan project. It is not affiliated with, endorsed by, or
> sponsored by the BCCI, IPL, or any IPL franchise. Franchise names and
> player information are used for identification and simulation
> purposes. Do not use official team logos, player photographs, or other
> protected assets without the necessary permissions.

------------------------------------------------------------------------

## Contents

-   [Overview](#overview)
-   [Product goals](#product-goals)
-   [Core features](#core-features)
-   [Auction rulebook](#auction-rulebook)
-   [Room and participant rules](#room-and-participant-rules)
-   [Host permissions](#host-permissions)
-   [Technology architecture](#technology-architecture)
-   [System architecture](#system-architecture)
-   [Backend design](#backend-design)
-   [Frontend and visual design](#frontend-and-visual-design)
-   [Database design](#database-design)
-   [Auction lifecycle and state
    machine](#auction-lifecycle-and-state-machine)
-   [Bidding and timer flow](#bidding-and-timer-flow)
-   [WebSocket protocol](#websocket-protocol)
-   [Persistence and transaction
    safety](#persistence-and-transaction-safety)
-   [Reconnection and recovery](#reconnection-and-recovery)
-   [Security](#security)
-   [API design](#api-design)
-   [Project structure](#project-structure)
-   [Development phases](#development-phases)
-   [Local development setup](#local-development-setup)
-   [Environment configuration](#environment-configuration)
-   [Database migrations and seed
    data](#database-migrations-and-seed-data)
-   [Testing strategy](#testing-strategy)
-   [Deployment](#deployment)
-   [MVP boundaries](#mvp-boundaries)
-   [Future enhancements](#future-enhancements)
-   [Design principles](#design-principles)
-   [License](#license)

------------------------------------------------------------------------

## Overview

IPL Auction Arena simulates a private IPL-style player auction.

A room contains 4--10 participants. Each participant selects one unique
IPL franchise. The host controls the auction flow, but **the host has
exactly the same bidding rights as every other participant**.

The backend is authoritative for:

-   Bid validation and ordering.
-   Auction timers and deadlines.
-   Team purse balances.
-   Squad size and overseas-player limits.
-   Player availability and sale status.
-   Auction events and recovery state.

The browser is a presentation and interaction client. It sends commands
and renders server-confirmed state; it never decides whether a bid or
sale is valid.

## Product goals

1.  Deliver a fun, polished, multiplayer auction experience.
2.  Preserve auction correctness under simultaneous bids and network
    interruptions.
3.  Make room state and auction outcomes consistent for every
    participant.
4.  Keep the system understandable, testable, and maintainable.
5.  Use animation and graphics where they improve feedback, without
    making the interface slow or distracting.

## Core features

### Private multiplayer rooms

-   Create a private room with a unique, hard-to-guess room code and
    shareable link.
-   Support 4--10 participants per room.
-   Assign one distinct IPL franchise to each participant.
-   Only the host can start the auction.
-   Lock room membership and franchise selection after the auction
    starts.
-   Restore a participant's existing session after refresh or reconnect.

### Live auction

-   Reveal players one at a time.
-   Accept bids from every participant, including the host.
-   Display the current highest bid, highest-bidding franchise, and
    countdown.
-   Update purse and squad information after a confirmed sale.
-   Mark players sold or unsold according to the approved timer rules.
-   Run unsold nomination rounds after the main pool.

### Team management

-   Starting purse: ₹120 Cr.
-   Squad size: minimum 18 and maximum 25 players.
-   Maximum overseas players: 8.
-   Display remaining purse, squad composition, and overseas-player
    count.
-   Prevent purchases that violate squad, purse, or overseas
    constraints.

### Results

-   Display final squads and purchase prices.
-   Display remaining purse and squad summaries.
-   Display unsold players and auction outcomes.
-   Make completed-room results read-only.

### Admin player-data management

-   Provide a separate authenticated admin interface for managing the
    player dataset.
-   Keep administrative dataset management separate from participant
    room permissions.
-   Validate imported and edited player records before they are used in
    new rooms.

------------------------------------------------------------------------

## Auction rulebook

The following rules are the approved product rules and must be treated
as the source of truth for implementation.

### Purse and squad constraints

  Rule                         Approved value
  -------------------------- ----------------
  Starting purse per team             ₹120 Cr
  Minimum squad size               18 players
  Maximum squad size               25 players
  Maximum overseas players                  8
  Minimum bid increment              ₹0.10 Cr

The backend must validate affordability and squad constraints before
accepting a bid. It must also apply dynamic purse protection so a team
cannot spend so much that it becomes impossible for that team to
complete the minimum 18-player squad.

### Player reveal and bidding timer

1.  When a player is revealed, the first-bid window is **15 seconds**.
2.  If no valid bid is received within those 15 seconds, the player
    becomes **UNSOLD**.
3.  Once the first valid bid is accepted, the bidding timer becomes **7
    seconds**.
4.  Every subsequent valid higher bid resets the timer to **7 seconds**.
5.  If no higher valid bid is received before the 7-second deadline, the
    player is **SOLD** to the current highest bidder.
6.  The browser may render a countdown, but only the server decides when
    the deadline has expired.

### Opening bid and bid validation

-   The opening bid starts at the player's base price.
-   A subsequent bid must satisfy the minimum increment.
-   The bid must be affordable for the participant's team.
-   The team must have room under the maximum squad size.
-   The team must remain within the overseas-player limit.
-   Dynamic purse protection must preserve the team's ability to reach
    the minimum squad size.
-   Every bid is validated against the latest authoritative room state.

### Simultaneous bids

-   Bids are processed sequentially by the server within the room.
-   The first valid bid processed by the room's serialized event
    processor is accepted.
-   Later bids are validated against the updated state.
-   Every bid request receives an explicit accepted or rejected
    response.

### Sale

When a player is sold:

-   The winning amount is deducted from the winning team's purse.
-   The player is assigned to the winning team's squad.
-   The player is marked as sold and cannot return to the available
    pool.
-   The sale, purse deduction, squad assignment, and corresponding
    persisted event must be committed atomically.
-   A failed database transaction must never be broadcast as a
    successful sale.

### Unsold players and nomination rounds

-   The host decides when to start an unsold round.
-   Each team may nominate up to **6 unsold players per round**.
-   Duplicate nominations create a mini-auction between the teams that
    nominated that player.
-   If nobody bids, the player remains unsold.
-   Unsold rounds continue under host control until the host finishes
    them.

### Auction completion

The host finishes the main player pool and the unsold rounds. Completion
must not be allowed while a team violates the minimum squad size or
other squad constraints.

At completion:

-   No team may exceed 25 players.
-   No team may have fewer than 18 players.
-   No team may exceed 8 overseas players.
-   No team may spend more than ₹120 Cr.
-   Final squads and results become read-only.

------------------------------------------------------------------------

## Room and participant rules

-   Rooms are private and use unique, hard-to-guess room codes.
-   A room supports 4--10 participants.
-   Each participant selects one unique IPL franchise.
-   A franchise cannot be selected by more than one participant in the
    same room.
-   The room creator becomes the host.
-   Only the host can start the auction.
-   Once the auction starts, the room is locked:
    -   No new participants may join.
    -   Participants cannot change their selected franchise.
-   Refreshing or reconnecting restores the participant's existing
    session and team state.
-   A completed room is read-only.

## Host permissions

The host controls the auction flow, not the bidding privileges.

The host can:

-   Choose the category order.
-   Shuffle players within a category.
-   Start the auction.
-   Pause and resume the auction.
-   Skip the current player.
-   End the auction.
-   Start unsold rounds.
-   Finish unsold rounds.

The host cannot receive special bidding priority or extra bidding
rights.

### Host disconnect behavior

-   If the host disconnects during an auction, the auction continues
    uninterrupted.
-   Host controls are unavailable while the host is disconnected.
-   Host controls are restored when the host reconnects.
-   There is no automatic host transfer.
-   The host can cancel a room only before the auction starts.

## Technology architecture

The architecture is selected for the product's real-time correctness and
visual requirements. The frontend is intentionally component-based
because the auction room contains many interactive, independently
updating UI regions.

  -----------------------------------------------------------------------
  Layer                   Technology              Responsibility
  ----------------------- ----------------------- -----------------------
  Frontend framework      Next.js + React +       Application structure,
                          TypeScript              routes, typed UI

  Styling                 Tailwind CSS            Responsive styling and
                                                  design system

  UI components           shadcn/ui               Accessible reusable
                                                  interface components

  Animation               Motion for React +      Transitions and
                          CSS/SVG                 interaction feedback

  Backend                 Go                      Business logic and
                                                  auction engine

  HTTP router             Chi                     REST API routing and
                                                  middleware

  Real-time transport     WebSocket               Live auction
                                                  communication

  Database                PostgreSQL              Durable application
                                                  state

  PostgreSQL driver       pgx / `database/sql`    Database connectivity
                                                  and SQL execution

  Cross-instance          Redis (optional)        Fan-out and
  coordination                                    coordination when
                                                  scaling

  Deployment              Frontend, backend, and  Production hosting
                          managed PostgreSQL      
                          services                
  -----------------------------------------------------------------------

### Database access policy

-   Use raw SQL for database operations.
-   Do not use GORM.
-   Do not use SQLBoiler.
-   Keep SQL in repository/data-access code.
-   Use PostgreSQL transactions for consistency-critical operations.

### Why this stack?

-   React components make complex live screens easier to organize and
    update.
-   TypeScript helps keep frontend event payloads and API contracts
    consistent.
-   Motion supports purposeful UI animations without requiring a 3D
    rendering engine.
-   Go provides a strong foundation for the room event processor and
    auction rules.
-   PostgreSQL provides transactional guarantees and durable event
    history.
-   Redis is optional and is not the source of truth for auction state.

------------------------------------------------------------------------

## System architecture

``` mermaid
flowchart TB
    U["4–10 Participants"]
    F["Next.js + React + TypeScript"]
    UI["Tailwind + shadcn/ui + Motion"]
    API["Go HTTP API - Chi"]
    WS["Go WebSocket Gateway"]
    AUTH["Session and Authorization"]
    ROOM["Room Manager"]
    ACTOR["Room Actor / Serialized Event Loop"]
    ENGINE["Auction Engine"]
    DB[("PostgreSQL")]
    OUTBOX["Transactional Outbox"]
    REDIS[("Optional Redis")]

    U --> F
    F --> UI
    F --> API
    F <--> WS
    API --> AUTH
    WS --> AUTH
    AUTH --> ROOM
    ROOM --> ACTOR
    ACTOR --> ENGINE
    ENGINE --> DB
    ENGINE --> OUTBOX
    OUTBOX --> WS
    WS -. "Cross-instance fan-out when required" .-> REDIS
```

### Authority boundaries

  Component           Owns
  ------------------- -------------------------------------------------
  Frontend            Rendering, user input, local presentation state
  HTTP handlers       Request parsing and response mapping
  Services            Application use cases and authorization checks
  Room manager        Room lifecycle and room actor lookup
  Room actor          Serialized processing of room commands
  Auction engine      Auction rules and state transitions
  Repository          SQL queries and transactions
  PostgreSQL          Durable authoritative state
  WebSocket gateway   Connection lifecycle and event delivery

The browser is never authoritative for purse, bid acceptance, timer
expiry, squad composition, player status, or sale outcome.

## Backend design

### Room actor

Each active room has one serialized event-processing loop.

All state-changing commands for that room are processed in order by that
room's actor. Different rooms can progress independently.

Example:

1.  Team A and Team B submit bids close together.
2.  Both commands enter the same room's queue.
3.  The room actor processes one command.
4.  The auction engine validates it against current state.
5.  If valid, the new state is persisted.
6.  The next command is checked against the updated state.

This provides a clear ordering boundary for simultaneous bids and host
actions.

### Auction engine

The auction engine owns:

-   Current auction phase.
-   Current player.
-   Current highest bid.
-   Current highest bidder.
-   Base price and bid increment validation.
-   First-bid and subsequent-bid deadlines.
-   Team purse validation.
-   Squad size validation.
-   Overseas-player validation.
-   Player lifecycle transitions.
-   Sold and unsold outcomes.
-   Auction completion validation.

The engine should be testable independently of HTTP handlers and
WebSocket connection code.

### Repository layer

Repositories own database access and transaction boundaries. They must
not contain frontend concerns or WebSocket presentation logic.

## Frontend and visual design

The product should feel like a premium sports broadcast combined with a
live auction platform.

### Visual direction

-   Dark stadium-inspired interface.
-   Strong contrast and readable typography.
-   Franchise-specific accent colors.
-   High-quality player cards.
-   Clear bid and countdown hierarchy.
-   Responsive layouts for desktop and smaller screens.

### Main screens

  Screen           Main elements
  ---------------- -------------------------------------------------------
  Landing page     Product introduction, create room, join room
  Room lobby       Participants, franchise selection, host controls
  Live auction     Player card, current bid, countdown, bidding controls
  Team dashboard   Squad, purse, overseas count, player categories
  Unsold round     Nomination controls and mini-auction state
  Results          Final squads, spending, unsold players
  Admin panel      Authenticated player-data management

### Live auction screen layout

``` text
+--------------------------------------------------------------+
| ROOM CODE      AUCTION STATUS             ROUND / CATEGORY   |
+--------------------------------------------------------------+
| TEAM A / PURSE       CURRENT PLAYER CARD       TEAM B / PURSE |
|                      PLAYER IMAGE                              |
|                      PLAYER NAME                               |
|                      ROLE / COUNTRY                            |
|                      BASE PRICE                                 |
|                                                                |
+--------------------------------------------------------------+
| RECENT BIDS          CURRENT BID / COUNTDOWN     BID CONTROLS |
+--------------------------------------------------------------+
| TEAM SQUADS / PURSE SUMMARY / OVERSEAS COUNT                   |
+--------------------------------------------------------------+
```

### Animation policy

Animations should improve feedback, hierarchy, and atmosphere. They must
never delay or block auction interaction.

  Interaction             Visual treatment
  ----------------------- ------------------------------------
  Page navigation         Short, smooth transition
  Player reveal           Player-card entrance and spotlight
  New bid                 Price counter transition
  Highest bidder change   Franchise highlight
  Countdown               Animated timer ring
  Player sold             SOLD stamp and brief celebration
  Player unsold           Subtle UNSOLD transition
  Purse update            Animated balance change
  Squad update            New player-card entrance
  Auction completion      Final-results reveal

### Graphics policy

-   Prefer optimized player images where usage rights permit.
-   Use SVG for icons, vector graphics, and decorative accents.
-   Use CSS gradients and lighting for stadium atmosphere.
-   Use Motion for UI transitions.
-   Avoid unnecessary 3D rendering in the MVP.
-   Avoid constantly moving backgrounds, excessive particles, and
    effects that compete with bidding information.
-   Respect reduced-motion preferences.
-   Keep important text and controls readable over decorative
    backgrounds.

No official IPL or franchise logos or player photographs should be used
unless the required rights or permissions are available. Use neutral
placeholders or original graphics otherwise.

## Database design

The following is the proposed logical schema. Exact column definitions,
indexes, constraints, and migration order are to be finalized during the
database-design phase.

  -----------------------------------------------------------------------
  Table                               Purpose
  ----------------------------------- -----------------------------------
  `users`                             Participant identity

  `rooms`                             Private room information and host

  `room_participants`                 Room membership, session
                                      association, participant role

  `teams`                             Franchise assignment and purse

  `players`                           Master player dataset

  `room_players`                      Room-specific player availability
                                      and lifecycle

  `auctions`                          Auction session and current state

  `bids`                              Accepted bid history

  `squads`                            Purchased players assigned to teams

  `auction_events`                    Ordered room event history

  `unsold_nominations`                Unsold-round nominations

  `outbox_events`                     Persisted events awaiting
                                      publication
  -----------------------------------------------------------------------

### Entity relationship diagram

``` mermaid
erDiagram
    USERS ||--o{ ROOM_PARTICIPANTS : joins
    ROOMS ||--o{ ROOM_PARTICIPANTS : contains
    ROOM_PARTICIPANTS ||--o| TEAMS : controls
    ROOMS ||--|| AUCTIONS : owns
    PLAYERS ||--o{ ROOM_PLAYERS : included_in
    ROOMS ||--o{ ROOM_PLAYERS : tracks
    AUCTIONS ||--o{ BIDS : records
    AUCTIONS ||--o{ AUCTION_EVENTS : emits
    TEAMS ||--o{ SQUADS : owns
    PLAYERS ||--o{ SQUADS : assigned_to
    ROOM_PARTICIPANTS ||--o{ UNSOLD_NOMINATIONS : nominates
```

### Important data constraints

-   A franchise can be assigned to only one participant per room.
-   A room supports no more than 10 participants and no fewer than 4
    when starting.
-   A player can be sold only once in a room.
-   A team cannot exceed its purse or squad limits.
-   A team cannot exceed the overseas-player limit.
-   Accepted bids and auction events must be persisted.
-   Room event sequence numbers must be monotonically increasing.
-   State-changing operations must be transactionally safe.

### Player lifecycle

``` mermaid
stateDiagram-v2
    [*] --> AVAILABLE
    AVAILABLE --> IN_AUCTION
    IN_AUCTION --> SOLD
    IN_AUCTION --> UNSOLD
    UNSOLD --> IN_AUCTION: Unsold round
    SOLD --> [*]
```

## Auction lifecycle and state machine

``` mermaid
stateDiagram-v2
    [*] --> WAITING
    WAITING --> READY: Host starts auction
    READY --> PLAYER_REVEALED: Reveal player
    PLAYER_REVEALED --> UNSOLD: No bid within 15 seconds
    PLAYER_REVEALED --> BIDDING: First valid bid
    BIDDING --> BIDDING: Valid higher bid
    BIDDING --> SOLD: No higher bid within 7 seconds
    SOLD --> READY: Next player
    UNSOLD --> READY: Next player
    READY --> UNSOLD_ROUND: Main pool completed
    UNSOLD_ROUND --> COMPLETED: Host finishes rounds
    COMPLETED --> [*]
```

`RECOVERING` is an operational recovery state entered after an
unexpected server restart. It is not an automatic sale or unsold
outcome.

## Bidding and timer flow

### First bid

1.  Server reveals the player.
2.  Server establishes the 15-second first-bid deadline.
3.  Participants may submit valid bids.
4.  If the deadline expires without a valid bid, the player becomes
    unsold.
5.  If a valid bid is accepted, the room enters the bidding phase and
    the deadline becomes 7 seconds.

### Subsequent bids

1.  Participant submits a bid request with a unique request ID.
2.  Room actor processes the request.
3.  Auction engine validates increment, purse, squad size, overseas
    limit, and dynamic purse protection.
4.  If valid, the bid is persisted and the deadline resets to 7 seconds.
5.  If invalid, the client receives a rejection response.
6.  If no higher valid bid arrives before the deadline, the player is
    sold to the current highest bidder.

### Server-authoritative timer

-   The server owns the deadline and decides when it expires.
-   Clients receive timing information and render countdowns.
-   Client countdowns are not used to authorize sales.
-   Timer state and deadlines are persisted sufficiently to support
    restart recovery.

Main-queue MVP implementation (Milestone 3A):

- A server worker checks persisted deadlines at 250 ms intervals. Row locks and rechecks, rather than ticker precision, determine bid/settlement ordering.
- A three-second transition interval before each automatic reveal (including the first) gives the interface time for result presentation. This is an implementation default, not an extension of the bidding window.
- Starting the auction automatically schedules the first player; manual nomination HTTP route is superseded. Main pool exhaustion leaves the auction active and idle until host-controlled unsold rounds are implemented.
- On process startup active auctions enter `RECOVERING` before HTTP starts. `POST /auctions/{auctionID}/recover/resume` is host-only for synchronized MAIN rooms. It preserves the current player and accepted bid and opens a fresh full 15-second (no bid) or 7-second (accepted bid) window because downtime is not bidding time. Exact pre-crash remaining time restoration and multi-instance coordination are future work.
- Single backend instance only for this MVP. WebSocket broadcast, pause/skip, unsold mini-auctions and final completion remain future milestones.


## WebSocket protocol

One WebSocket connection is maintained per participant per room.

### Communication model

``` mermaid
sequenceDiagram
    participant C as Participant Browser
    participant H as WebSocket Gateway
    participant R as Room Actor
    participant E as Auction Engine
    participant DB as PostgreSQL

    C->>H: Connect and authenticate
    H->>R: Register participant
    R-->>H: Current snapshot
    H-->>C: AUCTION_STATE snapshot

    C->>H: BID_PLACE
    H->>R: Submit bid command
    R->>E: Validate bid
    E->>DB: Persist accepted bid and state
    DB-->>E: Commit success
    E-->>R: Accepted result
    R-->>H: Publish room event
    H-->>C: BID_ACCEPTED / room event
```

### Event envelope

Every room event should carry enough metadata to identify its room,
request or event, and ordering position.

Illustrative payload:

``` json
{
  "event_id": "unique-event-id",
  "room_id": "room-id",
  "sequence": 42,
  "type": "BID_ACCEPTED",
  "timestamp": "2026-01-01T12:00:00Z",
  "payload": {
    "player_id": "player-id",
    "team_id": "team-id",
    "amount": 2.5
  }
}
```

The values above are examples, not production IDs or a fixed final
schema.

### Client-to-server event categories

  Event                        Purpose
  ---------------------------- -----------------------------
  `BID_PLACE`                  Submit a bid
  `HOST_START_AUCTION`         Start the auction
  `HOST_PAUSE`                 Pause the auction
  `HOST_RESUME`                Resume the auction
  `HOST_SKIP`                  Skip the current player
  `HOST_END`                   End the auction
  `REQUEST_SNAPSHOT`           Request authoritative state
  `UNSOLD_NOMINATE`            Nominate an unsold player
  `HOST_START_UNSOLD_ROUND`    Start an unsold round
  `HOST_FINISH_UNSOLD_ROUND`   Finish an unsold round

### Server-to-client event categories

  Event                 Purpose
  --------------------- ------------------------------------
  `BID_ACCEPTED`        Confirm an accepted bid
  `BID_REJECTED`        Explain a rejected bid
  `PLAYER_REVEALED`     Announce the current player
  `PLAYER_SOLD`         Confirm a sale
  `PLAYER_UNSOLD`       Confirm an unsold outcome
  `AUCTION_STATE`       Send a full authoritative snapshot
  `AUCTION_PAUSED`      Announce pause
  `AUCTION_RESUMED`     Announce resume
  `SQUAD_UPDATED`       Update team squad information
  `PURSE_UPDATED`       Update purse information
  `AUCTION_COMPLETED`   Announce completion

Event names and payloads should be finalized in the API/WebSocket
contract before implementation. The categories above describe the agreed
behavior.

### Ordering and missed events

-   Every room event has a monotonically increasing sequence number.
-   The server broadcasts accepted events to connected participants.
-   On reconnect, the participant receives a complete state snapshot
    before live updates resume.
-   If a client detects a sequence gap, it requests a fresh snapshot.
-   Every bid request receives an explicit accepted or rejected
    response.
-   Bid requests use unique IDs. Duplicate requests are processed once
    and return the original result.

## Persistence and transaction safety

PostgreSQL is the durable source of truth.

### Atomic sale transaction

The following operations must be committed together:

1.  Confirm the current player and winning bid.
2.  Persist the accepted bid or final winning amount as required by the
    transaction design.
3.  Deduct the winning amount from the team's purse.
4.  Assign the player to the winning team's squad.
5.  Mark the player as sold.
6.  Persist the corresponding auction event and outbox event.
7.  Commit the transaction.

If the transaction fails:

-   No sale is broadcast.
-   No purse update is broadcast.
-   No squad update is broadcast.
-   The in-memory room state must not be treated as committed success.

### Event publication

-   Persist events before publishing them.
-   Publish only after the database transaction commits.
-   Use the transactional outbox to recover events that were committed
    but not yet published.
-   Consumers and publishers must tolerate duplicate delivery where
    applicable.
-   Event sequence numbers and idempotency keys must be enforced.

## Reconnection and recovery

### Participant disconnect

-   The auction continues uninterrupted.
-   The participant's team, squad, and purse remain unchanged.
-   The disconnected participant cannot bid until reconnected.
-   Reconnection restores the existing participant session.
-   A full authoritative snapshot is sent before live updates resume.

### Host disconnect

-   The auction continues uninterrupted.
-   Host controls are unavailable while disconnected.
-   Host controls return after reconnect.
-   No automatic host transfer occurs.

### Browser refresh

-   Restore the participant's existing session.
-   Rejoin the same room and franchise.
-   Fetch the authoritative room snapshot.
-   Resume live event delivery after snapshot synchronization.

### Missed WebSocket events

-   Clients track the last processed sequence number.
-   If a gap is detected, request a fresh snapshot.
-   The server returns the complete current state.
-   Live updates resume after synchronization.

### Unexpected server restart

1.  Load persisted room and auction state.
2.  Reconstruct the latest committed state from durable data.
3.  Enter `RECOVERING`.
4.  Do not automatically sell or mark the current player unsold because
    of the restart.
5.  Restore the persisted timer/deadline information.
6.  Wait for the host to resume.
7.  Continue from the recovered state.

## Security

-   Use unique, hard-to-guess room codes.
-   Validate room membership for every room action.
-   Authenticate participant sessions.
-   Enforce host-only permissions on host controls.
-   Allow participants to bid only for their own assigned teams.
-   Validate all WebSocket event types and payloads on the server.
-   Never trust client-supplied purse, squad, player status, timer, or
    sale state.
-   Validate bid amount and all squad constraints in the backend.
-   Protect admin routes with separate authentication and authorization.
-   Keep secrets and database credentials in environment variables.
-   Use HTTPS/WSS in production.
-   Apply request limits and sensible connection limits.
-   Avoid logging session tokens or other secrets.

## API design

The following is the proposed REST API surface. Request and response
schemas, status codes, and authorization details must be specified in
the API contract before implementation.

  Method   Endpoint                           Purpose
  -------- ---------------------------------- -------------------------
  `POST`   `/api/rooms`                       Create a room
  `POST`   `/api/rooms/{code}/join`           Join a room
  `GET`    `/api/rooms/{code}`                Get room details
  `GET`    `/api/rooms/{code}/participants`   List participants
  `POST`   `/api/rooms/{code}/teams`          Select a franchise
  `POST`   `/api/rooms/{code}/start`          Host starts the auction
  `GET`    `/api/rooms/{code}/auction`        Get auction state
  `GET`    `/api/rooms/{code}/players`        Get room player pool
  `GET`    `/api/rooms/{code}/results`        Get final results
  `GET`    `/api/health`                      Health check

Bidding and live auction controls use WebSocket events. REST endpoints
must not create a second, competing path for authoritative live bidding.

## Project structure

The following is the proposed repository layout. Exact files may evolve
as implementation decisions are made, but the responsibility boundaries
should remain.

``` text
ipl-auction-arena/
│
├── backend/
│   ├── cmd/
│   │   └── server/
│   │       └── main.go
│   ├── internal/
│   │   ├── api/
│   │   ├── auth/
│   │   ├── config/
│   │   ├── database/
│   │   ├── rooms/
│   │   │   ├── manager.go
│   │   │   ├── actor.go
│   │   │   └── commands.go
│   │   ├── auction/
│   │   │   ├── engine.go
│   │   │   ├── bidding.go
│   │   │   ├── timer.go
│   │   │   ├── validation.go
│   │   │   └── state.go
│   │   ├── websocket/
│   │   ├── handlers/
│   │   ├── services/
│   │   ├── repository/
│   │   ├── middleware/
│   │   ├── models/
│   │   └── dto/
│   ├── migrations/
│   ├── seed/
│   ├── go.mod
│   └── .env.example
│
├── frontend/
│   ├── src/
│   │   ├── app/
│   │   ├── components/
│   │   │   ├── ui/
│   │   │   ├── auction/
│   │   │   ├── players/
│   │   │   ├── teams/
│   │   │   └── room/
│   │   ├── hooks/
│   │   ├── services/
│   │   ├── store/
│   │   ├── animations/
│   │   ├── types/
│   │   └── lib/
│   ├── public/
│   │   ├── players/
│   │   ├── teams/
│   │   └── assets/
│   └── package.json
│
├── docs/
│   ├── architecture/
│   ├── database/
│   ├── api/
│   └── websocket/
│
├── docker-compose.yml
├── README.md
└── .gitignore
```

## Development phases

Implementation should proceed in controlled phases. Auction correctness
and contracts should be established before full UI integration.

  Phase   Scope
  ------- -------------------------------------------
  1       Final architecture and database design
  2       Project setup and local infrastructure
  3       Authentication, rooms, and participants
  4       Franchise selection and player management
  5       Auction engine and validation
  6       Timer and bidding
  7       WebSocket events and room actor
  8       Persistence, outbox, and recovery
  9       Frontend visual system and animations
  10      Live auction integration
  11      Testing, security, and performance
  12      Deployment and documentation

The frontend visual system may be developed alongside backend work after
API and WebSocket contracts are agreed.

## Local development setup

### Prerequisites

Install the following before starting development:

-   Go (version selected and pinned by the repository).
-   Node.js LTS and npm.
-   PostgreSQL.
-   Git.
-   Docker and Docker Compose (recommended for local infrastructure).

### Clone the repository

``` bash
git clone <repository-url>
cd ipl-auction-arena
```

### Configure the backend

``` bash
cd backend
cp .env.example .env
```

Fill in the local PostgreSQL connection details and other required
environment values.

### Configure the frontend

``` bash
cd ../frontend
npm install
```

### Start local infrastructure

If the project uses the provided Docker Compose configuration:

``` bash
docker compose up -d
```

### Run database migrations

Use the migration command selected by the project. The migration tool
and exact command should be documented when the migration setup is
implemented.

### Start the backend

``` bash
cd backend
go run ./cmd/server
```

### Start the frontend

In another terminal:

``` bash
cd frontend
npm run dev
```

Open the local frontend URL printed by the development server.

> These are the intended development steps. Exact scripts, migration
> commands, port numbers, and environment variable names must match the
> implemented repository configuration.

## Environment configuration

Keep secrets out of source control. The following names are illustrative
and should be finalized with the implementation.

### Backend

``` env
APP_ENV=development
HTTP_ADDR=:8080
DATABASE_URL=postgres://USER:PASSWORD@localhost:5432/ipl_auction_arena?sslmode=disable
SESSION_SECRET=replace-with-a-long-random-secret
FRONTEND_ORIGIN=http://localhost:3000
REDIS_URL=
```

### Frontend

``` env
NEXT_PUBLIC_API_BASE_URL=http://localhost:8080
NEXT_PUBLIC_WS_BASE_URL=ws://localhost:8080
```

Never commit real credentials, production connection strings, or session
secrets.

## Database migrations and seed data

-   Use version-controlled SQL migrations.
-   Keep schema changes reproducible across development, testing, and
    production.
-   Seed the player master dataset through a controlled import process.
-   Validate player identity, role, base price, nationality/overseas
    classification, and eligibility before import.
-   Approved fantasy pool: all 543 unique players (413 original auction candidates and 130 retained-source records) are eligible. Preserve the original source cohort as provenance, not an eligibility restriction.
-   Retained-source base prices follow the approved fantasy classification (capped ₹2 Cr, uncapped ₹0.50 Cr; ₹2 Cr fallback if a game price is unavailable).
-   Each waiting room receives a persisted 1–543 category-wise queue. Host can reorder categories and shuffle selected categories before start; a default queue is generated at start if not already prepared. The order is frozen once started. Empty categories are skipped.
-   Preserve source and verification metadata for imported player
    records.
-   Do not silently change approved auction rules through seed data.

The exact dataset snapshot and its source provenance should be
documented alongside the seed/import files.

## Testing strategy

### Auction engine unit tests

Test at least:

-   Opening bid at base price.
-   Minimum increment validation.
-   Rejection of unaffordable bids.
-   Squad maximum of 25.
-   Minimum squad feasibility and dynamic purse protection.
-   Overseas-player limit of 8.
-   First-bid timeout after 15 seconds.
-   Subsequent-bid timeout after 7 seconds.
-   Valid bid resetting the timer.
-   Sale to the current highest bidder.
-   No-bid player becoming unsold.
-   Duplicate unsold nominations.
-   Maximum 6 nominations per team per unsold round.
-   Auction completion validation.

### Concurrency tests

-   Simultaneous bids in one room.
-   Commands processed in serialized room order.
-   Independent activity across separate rooms.
-   Duplicate bid request IDs.
-   Race conditions around timer expiry and incoming bids.

### Database integration tests

-   Sale transaction commits all related updates.
-   Failed transaction changes no authoritative state.
-   Player cannot be sold twice in one room.
-   Event sequence ordering is maintained.
-   Outbox publication can recover after interruption.
-   Room state can be restored after process restart.

### WebSocket tests

-   Participant connection and authentication.
-   Host-only command authorization.
-   Accepted and rejected bid responses.
-   Broadcast to connected room participants.
-   Reconnect snapshot before live updates.
-   Sequence gap detection and snapshot recovery.
-   Duplicate request handling.

### Frontend tests

-   Room creation and joining.
-   Franchise selection and locking.
-   Bid controls and rejected-bid feedback.
-   Countdown rendering from server timing.
-   Purse and squad updates.
-   Reduced-motion behavior.
-   Responsive layout.
-   Disconnect and reconnect experience.

### End-to-end acceptance scenarios

1.  Create a room and join with 4--10 participants.
2.  Assign unique franchises.
3.  Start the auction as host.
4.  Run bidding with multiple participants.
5.  Verify purse and squad updates after a sale.
6.  Verify an unbid player becomes unsold after 15 seconds.
7.  Verify a bid resets the timer to 7 seconds.
8.  Run an unsold nomination round.
9.  Disconnect a participant and reconnect.
10. Disconnect the host and verify the auction continues.
11. Restart the server and verify `RECOVERING` behavior.
12. Complete the auction and verify final squad constraints.

## Deployment

The production deployment consists of:

-   Frontend application hosting.
-   Go backend hosting.
-   Managed PostgreSQL.
-   Optional Redis if multi-instance coordination is required.
-   HTTPS and secure WebSocket connectivity.

### Deployment requirements

-   Configure production environment variables and secrets.
-   Apply database migrations before serving traffic that depends on
    them.
-   Configure frontend origin and CORS correctly.
-   Configure WebSocket upgrade support and connection timeouts.
-   Use persistent database storage and backups.
-   Add health and readiness checks.
-   Protect admin functionality with authentication.
-   Verify reconnect and recovery behavior in the deployed environment.
-   Monitor application errors, database failures, WebSocket
    disconnects, and outbox backlog.

### Scaling approach

**Initial MVP**

-   One Go backend instance.
-   One PostgreSQL database.
-   In-memory room actor registry.
-   Persisted auction state and event history.
-   No Redis requirement.

**When multiple backend instances are introduced**

-   Introduce cross-instance room routing and coordination.
-   Use Redis or another suitable messaging layer for cross-instance
    fan-out.
-   Ensure a room has one authoritative command processor at a time.
-   Do not rely on Redis Pub/Sub as durable event storage.
-   Continue to use PostgreSQL and the persisted event/outbox design for
    correctness and recovery.

## MVP boundaries

### Included in MVP

-   Private rooms for 4--10 participants.
-   Unique franchise assignment.
-   Host-controlled auction lifecycle.
-   Equal bidding rights for all participants.
-   Server-authoritative bidding and timers.
-   Purse, squad, and overseas constraints.
-   Unsold nomination rounds.
-   WebSocket real-time updates.
-   Reconnection and snapshot recovery.
-   Persistence and restart recovery.
-   Final squad and results views.
-   Authenticated admin player-data management.
-   Polished responsive UI with purposeful animations.

### Not required for MVP

-   Retention mode (future feature; disabled by default).
-   Automatic host transfer.
-   Mandatory email/password accounts for every participant.
-   3D stadium rendering or a WebGL-based interface.
-   Multi-region deployment.
-   Complex distributed event streaming infrastructure.
-   Official IPL branding or licensed assets.

## Future enhancements

Potential additions after the MVP is stable:

-   Optional retention mode.
-   Historical auction summaries.
-   Advanced player statistics and analytics.
-   Additional auction formats.
-   More visual themes.
-   Multi-instance backend deployment.
-   Spectator mode, if room permissions and product scope are defined.
-   Exportable auction results.

Future enhancements must not weaken the approved MVP auction rules or
recovery guarantees.

## Design principles

1.  **Server authority:** The backend decides all auction outcomes.
2.  **Transactional correctness:** A sale is never partially committed.
3.  **Serialized room commands:** A room processes state-changing
    commands in order.
4.  **Recoverability:** Durable state allows safe recovery after
    failures.
5.  **Explicit contracts:** REST and WebSocket payloads are documented
    and versioned.
6.  **Separation of concerns:** UI, transport, application logic,
    auction rules, and persistence have clear boundaries.
7.  **Purposeful visuals:** Animation supports comprehension and
    feedback, not distraction.
8.  **Performance first:** No visual effect may delay bidding or
    authoritative updates.
9.  **Security by validation:** Every participant and host action is
    authorized on the server.
10. **Incremental delivery:** Build and test one reliable phase at a
    time.

------------------------------------------------------------------------

## License

Choose and add a license before public distribution. Until a license is
added, all rights remain with the project owner by default. Also review
the use of IPL, franchise, and player-related names, images, and other
third-party intellectual property before publishing or deploying the
project.

------------------------------------------------------------------------

**IPL Auction Arena --- Private rooms. Live bidding. One authoritative
auction.**
