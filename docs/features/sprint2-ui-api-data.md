# Sprint 2 UI API and data definitions

Version DQ-S2-INTERFACES-2.1, 27 November 2026. Pre-implementation draft under [#49](https://github.com/bbxndg/DineQueue/issues/49); governed by the [feature contract](sprint2-restaurant-operations-contracts.md). None of the new routes, commands, DTOs or migrations below is implemented by this documentation change.

## 1 Scope and shared transport

Keep Go Fiber v3, GORM/PostgreSQL, explicit SQL migrations and Next.js same-origin routing. Handler → business service/transaction coordinator → repository. Existing authentication, HttpOnly sessions, secure production cookies, exact Origin, X-CSRF-Token, error envelope and request IDs are unchanged. All protected reads use `Cache-Control: no-store`; GETs have no domain or expiry side effects.

All routes below have prefix `/api/v1`. Customer routes require CUSTOMER; Host routes require HOST and the exact `operator_queue_permissions` grant for the path queue, rechecked in mutation transactions. The queue's restaurant is looked up server-side. A grant for another queue in the same restaurant does not authorize this queue's assignments. A granted Host may see shared physical table state, but not foreign-queue assignment/ticket identifiers.

Use canonical 36-character UUIDs. Missing/malformed/foreign private Customer entry IDs are indistinguishable 404 NOT_FOUND. Host malformed path IDs are 404; unknown or ungranted queue scope remains 403 FORBIDDEN, matching S1. Unknown/foreign assignment IDs inside an authorized queue are generic 404. Do not return SQL errors, customer identities, secrets or staff actor IDs in ordinary Customer/Host DTOs.

Validate JSON strictly: 4 KiB maximum for these small mutation bodies; reject unknown fields, duplicate keys, null/missing required fields, non-integral versions, duplicate table IDs and inconsistent version/table arrays. Read query keys are allowlisted. Server-issued versions must be positive integers. Normalize parsed UUIDs before duplicate detection, key hashing and version matching, so case variation cannot identify the same table twice. All 1–3 member IDs and their expected versions must match exactly; the server supplies capacities and connections.

## 2 Screens and interaction states

| Surface | Required behavior |
|---|---|
| `/host/queue` | Retain queue selector, all WAITING parties in original order, readiness and current call. MANUAL retains Call Next/Complete. SEATING links to floor and displays held tables, deadline, Seat/No-show, and explicit expired-hold recovery. |
| `/host/floor?queueId=...` | Validate granted queue on initial server render; show restaurant/zone, mode, available/held/occupied/dirty/cleaning/out-of-service text labels, capacities and approved edges. Interactive cards and keyboard/list alternative expose the same selection. No coordinate-based adjacency or arbitrary status editor. |
| Selected-set preview | Select 1–3 tables, show total capacity and oldest fitting Ready party for that final set, with skip explanations. Changing tables invalidates the confirmation. An optional party recommendation drawer ranks valid sets, but cannot force that party ahead of an earlier eligible party. |
| Call and Hold confirmation | Show ticket code, party size, exact table codes/capacities, zone and five-minute policy. Confirm must use a fresh preview's entry/set/versions. On changed party or set, require a new confirmation. |
| Assignment panel | Distinguish HELD from ACTIVE and CLOSED turnover. Seat confirms arrival before deadline; No-show requires confirmation. Checkout lists all member tables; Start/Finish Cleaning are distinct confirmations. Hide unsafe shortcuts even though backend guards remain decisive. |
| `/my-queue` and owned detail | Retain Ready/Not ready only for WAITING, add Cancel confirmation for WAITING. Explain CANCELLED/NO_SHOW and offer a new join if permitted. Completed waitlist service does not imply dining/checkout. No floor or other customer's data is exposed. |

Every surface needs loading, empty, denied/session-expired, network-error, conflict and success behavior. Failed reads retain the last successful snapshot with a stale notice, never a fabricated empty state. Disable mutations when the relevant snapshot is stale or a write is pending. On 409 refetch and clear table selection/confirmation if entry, table versions, layout or assignment version changed. On 401 clear protected state and route to login; forbidden scope clears that view; initial foreign detail remains actual HTTP 404.

Reuse Lab 7's visible-only five-second-after-settle coordinator, immediate visible return, one in-flight refresh per view, abort/invalidation on unmount/queue change, and write/read coordination. Add floor/assignment reads to that coordinator rather than adding independent overlapping timers. Preserve focus and input during polls. A countdown reaching zero disables Seat but does not mutate the database or assert that cleanup already ran. Expire holds is an explicit POST or server housekeeping, never a polling GET write. Mobile 360px and desktop 1280px must remain usable; color alone is insufficient.

## 3 Read interfaces

| Method and path | Success | Validation and absence |
|---|---|---|
| GET `/host/queues/{queueId}/floor` | 200 FloorSnapshot | Exact queue grant; empty configured floor is `zones:[]`, not fake tables |
| GET `/host/queues/{queueId}/matching?tableId={id}&tableId={id}` | 200 MatchPreview for that selected set | 1–3 distinct IDs; selected-set form; no `entryId` in this form |
| GET `/host/queues/{queueId}/matching?entryId={id}` | 200 recommendations for that queue's Ready WAITING entry | Exactly one selector form. Foreign/missing queue entry is 404; non-WAITING/not-ready is 409 ENTRY_NOT_ELIGIBLE |

For a valid selected set with no eligible party, return `entry:null`, `candidates:[]`, and `availabilityReason` of QUEUE_EMPTY, NO_READY_PARTIES or NO_FITTING_PARTY. Party-based recommendations with no feasible set return an empty candidate list and NO_MATCH_AVAILABLE. An invalid/disconnected/unavailable selected set is a documented 400/409, not an empty success. `CALL_IN_PROGRESS` prevents presenting a new call as actionable.

| DTO | Fields and constraints |
|---|---|
| FloorSnapshot | `queueId, restaurantId, operationsMode, layoutVersion, serverNow, holdTtlSeconds, zones, assignments`; one consistent DB snapshot. `assignments` contains this queue's HELD/ACTIVE and CLOSED not-yet-cleaned groups only |
| Zone | `id, name, isActive, tables, connections`; zones ordered by stable ID; no inactive zone in matching |
| Table | `id, code, capacity, status, positionX, positionY, version`; tables ordered by code then ID. Coordinates finite display metadata, not permission/connection proof |
| Connection | `tableIds:[lowId,highId]`; active, approved undirected edges only |
| MatchPreview | `queueId, layoutVersion, serverNow, entry, entryVersion, candidates, skipped, availabilityReason`; entryVersion is null when entry is null; missing selection is represented explicitly, not silently replaced with a different requested entry |
| Candidate | `tableIds` sorted, `tableVersions:[{tableId,version}]`, `zoneId, totalSeats, unusedSeats, rank, selectionReason`; capacities come from persisted tables |
| Selected entry | Existing HostEntry fields with `isReady`; preview adds `entryVersion` separately without renaming existing Customer/Host fields |
| Skipped | Host-only `ticketCode, reasonCode` where reason is NOT_READY or CAPACITY_NOT_ELIGIBLE. No owner identity/free-text notes |
| Assignment | `id, queueId, entryId, zoneId, status, turnoverPhase, version, tableIds, holdExpiresAt, heldAt, seatedAt, closedAt, cleaningStartedAt, cleanedAt, releasedAt, releaseReason`; nullable instants are explicit null |

Existing Customer Ticket fields/envelopes remain, including `isReady`, queue/restaurant/timezone context and owner-scoped detail. New CANCELLED/NO_SHOW statuses extend the status union; never coerce them to WAITING/COMPLETED. Do not add table/member/actor/customer identity fields to the Customer response. Existing active-list filtering remains WAITING/CALLED, including earlier service dates and inactive queues.

## 4 Mutation interfaces

`ActionVersion` is `{ "expectedVersion": <positive assignment version>, "idempotencyKey": "<canonical UUID>" }`. Host successes return 200 `{entry:HostEntry, assignment:Assignment, tables:[Table]}` from committed state. Customer Cancel returns the full owned Ticket, matching readiness PATCH's response family. Empty lists are `[]`, not null.

| Method and path | Body | Atomic outcome |
|---|---|---|
| POST `/host/queues/{queueId}/match-and-call` | `entryId, tableIds, expectedLayoutVersion, expectedEntryVersion, expectedTableVersions:[{tableId,version}], idempotencyKey, selectionReason` | WAITING/Ready→CALLED; new HELD assignment; every selected table HELD; member claims/event/action result |
| POST `/host/queues/{queueId}/seating-assignments/{assignmentId}/seat` | ActionVersion | Valid HELD→ACTIVE, all members OCCUPIED, ticket COMPLETED |
| POST `/host/queues/{queueId}/seating-assignments/{assignmentId}/checkout` | ActionVersion | ACTIVE→CLOSED with turnover DIRTY; all members DIRTY; ticket stays COMPLETED |
| POST `/host/queues/{queueId}/seating-assignments/{assignmentId}/start-cleaning` | ActionVersion | CLOSED/DIRTY→CLOSED/CLEANING and all members CLEANING |
| POST `/host/queues/{queueId}/seating-assignments/{assignmentId}/finish-cleaning` | ActionVersion | CLOSED/CLEANING→CLOSED/CLEANED, every member AVAILABLE, release all outstanding membership claims |
| POST `/host/queues/{queueId}/entries/{entryId}/no-show` | ActionVersion plus `reasonCode:"CUSTOMER_ABSENT"` | CALLED→NO_SHOW, HELD→RELEASED, all members AVAILABLE and claims released |
| POST `/host/queues/{queueId}/expire-holds` | `{ "idempotencyKey":"<canonical UUID>" }` | Bounded server cleanup of that queue's expired HELD group; 200 `{expiredAssignments:[id],serverNow}`; none expired gives `[]` |
| POST `/me/queue-entries/{entryId}/cancel` | Empty body, as documented | Own WAITING→CANCELLED, audit event; no table/assignment mutation |

`selectionReason` is RECOMMENDED for the highest-ranked valid set or ALTERNATIVE_SET for a different valid set. A reason never authorizes a skipped eligible party, disconnected group or unavailable table. A changed selection must obtain a new preview before confirmation.

Host keyed actions scope a key to authenticated actor, queue and operation. The canonical request hash includes HTTP method, canonical path/resource IDs (including entry/assignment ID) and normalized JSON; sort member/version arrays so mere JSON field/member order is not a new intent. Store that hash and original success response in the business transaction. Same key/same canonical request returns the original success without duplicate events even if versions have since advanced; the UI then refetches current state. Same key/different path resource or payload returns 409 IDEMPOTENCY_CONFLICT. Authorization is rechecked before replay. New keys for repeated terminal operations conflict. Customer Cancel has no key: a repeat returns INVALID_TRANSITION rather than a second event.

Housekeeping expiry is a distinct committed action before an authorized seating mutation; never run it for malformed/unauthenticated/forbidden input. It may release an expired hold even when the later requested action fails. A retry of a previously successful keyed action replays before housekeeping, so transport retries cannot unexpectedly start unrelated work. Invalid non-expiry operations leave target business rows, events and action records unchanged.

## 5 Errors and precedence

Retain `{error:{code,message,requestId}}`. Authenticate and enforce role/Origin/CSRF before business work; parse/validate scope and payload; authorize before querying private targets or replaying results. After successful-key replay and independent expiry cleanup, validate mode/current state/deadline, optimistic versions, table set and selected-set FIFO eligibility under locks. In call+hold, an existing CALLED takes precedence over empty/no-ready selection. Do not expose whether a forbidden table or foreign ticket exists.

| HTTP/code | Meaning |
|---|---|
| 400 VALIDATION_ERROR | JSON/query/type/UUID body/duplicate fields or inconsistent versions; use existing generic validation convention |
| 400 INVALID_TABLE_SET | Empty, duplicate, more than three or inconsistent selected member IDs |
| 401 UNAUTHENTICATED; 403 FORBIDDEN; 404 NOT_FOUND | Existing auth, grant and privacy conventions; no new SCOPE_FORBIDDEN alias |
| 409 CALL_IN_PROGRESS; QUEUE_EMPTY; NO_READY_PARTIES | Existing call constraints retained |
| 409 ENTRY_NOT_ELIGIBLE; QUEUE_ORDER_CHANGED | Requested entry not Ready WAITING, or another earlier Ready party fits the final set |
| 409 TABLES_NOT_CONNECTED; CAPACITY_INSUFFICIENT; TABLE_NOT_AVAILABLE | Invalid physical grouping, insufficient summed capacity, or non-AVAILABLE member |
| 409 STALE_PREVIEW | Layout, expected entry/table or assignment version mismatch; refetch/reconfirm |
| 409 HOLD_EXPIRED | An expired hold cannot be seated; releaseReason preserves expiry when cleanup already released it |
| 409 INVALID_TRANSITION | Wrong assignment/ticket/turnover state, direct-to-AVAILABLE or repeated terminal action; cleanup with no expired hold succeeds with an empty list |
| 409 IDEMPOTENCY_CONFLICT | Previously used key with a different canonical payload |
| 409 SEATING_NOT_ENABLED; TABLE_SELECTION_REQUIRED; SEATING_ASSIGNMENT_REQUIRED | Respect MANUAL/SEATING mode and prevent legacy endpoint bypass |

Selected-set validation checks scope first, active zone/connectedness next, availability next, then total capacity. A stale snapshot may return STALE_PREVIEW before those set-state errors. Concurrent tests assert the documented conflict category and persisted invariants; they must not depend on thread timing to choose one winner.

## 6 Persistent model and invariants

These are additive designs; implementations must inspect actual constraint names and migration order. Migrations 0001–0003 remain immutable. All instant fields are UTC `timestamptz`; service-day labels retain restaurant timezone rules.

| Entity | New fields and constraints |
|---|---|
| `service_queues` | `operations_mode` MANUAL/SEATING NOT NULL default MANUAL; `hold_ttl_seconds` integer default 300, validated 60–1800. No global environment switch silently changes historical queues |
| `restaurant_floor_state` | One row per restaurant; positive `layout_version`; the shared serialization row for seating/configuration writes across queues |
| `floor_zones` | UUID, restaurant FK, name, active flag; unique scoped identity for composite table FKs |
| `dining_tables` | UUID, restaurant/zone composite FK, unique restaurant/code, capacity CHECK 1–8, validated status, finite position_x/position_y in normalized range 0–100 inclusive, positive version, updated timestamp |
| `table_connections` | Ordered low/high UUID PK with low<high, active flag and configuration provenance; reject duplicate/self/cross-zone/cross-restaurant edges transactionally |
| `seating_assignments` | UUID, queue/entry/restaurant/zone FKs, HELD/ACTIVE/CLOSED/RELEASED, version, hold/seat/checkout/release timestamps, release reason, capacity snapshot; CLOSED adds DIRTY/CLEANING/CLEANED turnover phase and cleaning timestamps |
| `seating_assignment_tables` | Assignment/table composite PK, capacity snapshot, `released_at` nullable. Unique `table_id WHERE released_at IS NULL`; historical membership is never deleted |
| `queue_entries` | Extend status and lifecycle CHECKs with `cancelled_at`, `cancelled_by`, `no_show_at`, `no_show_by`, `no_show_reason` (CUSTOMER_ABSENT/HOLD_EXPIRED). Preserve is_ready, joined_at, numbers, existing lifecycle times and all three uniqueness indexes |
| `operation_events` | Append-only action, actor or system actor, queue/entry/assignment, selected members, from/to state, server timestamp and restricted reason/skip metadata; no credentials or customer free text |
| `operation_requests` | Actor/queue/operation/key unique, canonical request hash and committed response for Host keyed mutations; no session token or CSRF value |

Only one HELD/ACTIVE assignment may exist per queue entry, enforced with a partial unique index; retain historical released/closed assignments. Membership claims remain outstanding through CLOSED/DIRTY and CLOSED/CLEANING, and release only on finish-cleaning or no-show/expiry. This intentionally strengthens the old v2 sketch that released claims at checkout: a dirty table must not be claimable, and an old closed assignment must never clean a newly allocated table.

DB CHECKs enforce local state/timestamp consistency; foreign keys enforce row ownership relationships. Same-zone graph connectivity, sum capacity, membership count and full-group changes require service validation under the shared floor lock plus constraints and tests, not a cross-row CHECK claim. Table state/assignment/claim checks must agree at each commit. Do not use `Save` upsert behavior to create missing lifecycle rows.

For WAITING, called/completed/terminal fields are null. CALLED has call actor/time and no terminal/completion. COMPLETED retains its S1 call/completion fields; new seated entries also have an ACTIVE/CLOSED assignment, while historical manual completions remain valid without one. CANCELLED has no call/completion/no-show fields and requires cancelled_at plus cancelled_by equal to the owner. NO_SHOW retains call fields, has no completion/cancellation, and requires no_show_at and no_show_reason; no_show_by is the authorized Host for CUSTOMER_ABSENT and null only for system HOLD_EXPIRED. Terminal instants cannot precede earlier lifecycle instants. Readiness remains a separate boolean.

## 7 Transaction algorithm and cleanup ownership

First perform non-mutating role/scope checks. For a seating business transaction acquire floor-state → queue → entry → member tables sorted by ID → assignment/action rows. Recheck grants after locking the queue. Cancellation/readiness acquire queue → entry and never subsequently acquire floor-state. Configuration/activation also starts with floor-state before any queue locks. This prevents cross-queue double claims and inverse-order deadlocks.

Compute server time after locks, not at transaction start before a possible lock wait. Re-read selected-set eligibility and versions inside call+hold. Guard each conditional write and verify affected-row counts. Insert the event and idempotency result in the same transaction. Roll back all of that business operation if any check/update/event insert fails. Do not catch an SQL error and continue a failed PostgreSQL transaction.

The shared expiry coordinator (#52) uses the same lock order, verifies HELD and deadline, changes the complete group to RELEASED/AVAILABLE, records NO_SHOW with HOLD_EXPIRED and releases every claim exactly once. #53 calls this coordinator rather than adding a competing implementation. Seat-versus-expiry and no-show-versus-seat must have exactly one valid outcome. An optional future scheduler may invoke it; correctness cannot depend on a browser timer or guaranteed background execution on Cloud Run.

## 8 Migration provisioning and activation sequence

| Stage | Owning issue | Required behavior |
|---|---|---|
| Floor/configuration migration | #50 | Next free numbered migration, expected 0004: floor/zone/table/edge data and MANUAL policy columns. No existing entry rewritten |
| Shared seating/lifecycle migration | #52 | Next free migration, expected 0005: assignments, membership claims, events, action keys, turnover and new terminal-state CHECKs/fields. Inspect actual applied names; never replace migration 0002 |
| Cancel/no-show and turnover | #53/#54 | Reuse shared schema; add a new migration only for a separately reviewed schema revision |
| Synthetic floor provisioning | #50 | Planned explicit `db provision-floor --queue-id=<uuid> --confirm-database=<name>` for dedicated demo/UAT data; local/test seed may reuse fixtures. Preserve table IDs, states and credentials on rerun; conflicting configuration fails, never silently resets live tables |
| Seating activation | #54 | Planned explicit `db enable-seating --queue-id=<uuid> --confirm-database=<name>` after all handlers integrate; verify exact database, valid configured floor and no legacy CALLED. Change mode atomically under floor/queue locks; preserve WAITING/readiness/history |

Those CLI names are proposed interfaces to implement/test in their owning issues, not runnable instructions today. Normal API startup never migrates, provisions or activates seating. Existing production synthetic seed remains forbidden. Use the existing db-tools image/job and secret handling where applicable; actual deployment/provisioning authorization and execution are separate from this documentation PR. No secret values belong in examples or logs.

Migration acceptance includes populated S1/Lab 7 fixtures, repeated migration/provisioning, constraint inspection, restart persistence, explicit activation refusal and immutable old rows. Until seating activation the deployed queue remains MANUAL. Do not roll an activated database/application back to old binaries that do not understand terminal states or assignments; use a reviewed forward fix or compatible application version, preserving live data.
