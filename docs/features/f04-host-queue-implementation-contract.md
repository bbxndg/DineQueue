# F-04 implementation contract — Host Queue Operations (board, Call Next, Complete)

Date: 11 November 2026. User authorized implementation on
`feat/F-04-host-queue-operations`, base `acc51aa` (sprint-1-staging after F-03).
The committed F-04 baseline (including the legacy F-05 completion workflow) is in
`sprint1-contracts.md`. This addendum was written in the worktree before code. It
does not establish an Issue, a pre-implementation addendum commit or TA approval.

## Behavior, API and data

All routes require a valid session cookie, role `HOST` and a grant in
`operator_queue_permissions` for the path queue. Mutations also require exact
Origin and `X-CSRF-Token`. The service rechecks role and grant independently of the
HTTP middleware; mutations recheck the grant inside their own transaction.

| Method / path (`/api/v1` prefix) | Success | Errors |
|---|---|---|
| GET `/host/queues` | 200 `{queues:[{id,name,restaurantName,timezone,isActive}]}` (assigned only) | 401, 403 |
| GET `/host/queues/{queueId}/entries?status=WAITING` | 200 `{entries:[...]}` WAITING, oldest first by `(joined_at,id)` | 400 (other status), 401, 403, 404 |
| GET `/host/queues/{queueId}/current` | 200 `{entry: null \| entry}` (the queue's CALLED entry) | 401, 403, 404 |
| POST `/host/queues/{queueId}/call-next` | 200 updated CALLED entry | 400 (non-empty body), 401, 403, 404, 409 `CALL_IN_PROGRESS` / `QUEUE_EMPTY` |
| POST `/host/queues/{queueId}/entries/{entryId}/complete` | 200 updated COMPLETED entry | 400, 401, 403, 404, 409 `INVALID_TRANSITION` |

- Host entry DTO: `id, queueId, ticketCode, serviceDate, partySize, status,
  joinedAt, calledAt, completedAt`. No customer ID/name/email, no actor IDs,
  no raw sequence number.
- Customer → 403 `FORBIDDEN`. Host without a grant for the queue (including an
  unknown but well-formed queue UUID) → 403. Malformed queue/entry ID → 404.
  Entry that does not exist or belongs to another queue → 404 `NOT_FOUND`.
- **Call Next** (BR-003/005): one transaction locks the queue row `FOR UPDATE`,
  rechecks the grant, rejects 409 `CALL_IN_PROGRESS` if a CALLED entry exists,
  otherwise selects the oldest WAITING entry by `(joined_at,id)` (409
  `QUEUE_EMPTY` if none) and updates it with `WHERE status='WAITING'`, checking
  one affected row. Sets `called_at`, `called_by`, `version+1`. The existing
  `uq_one_called_per_queue` index is the DB backstop; a violation maps to
  `CALL_IN_PROGRESS`.
- **Complete** (BR-004/006, legacy F-05): same queue lock order, grant recheck,
  loads the entry scoped to the path queue `FOR UPDATE`; only CALLED → COMPLETED
  with `WHERE status='CALLED'` and one affected row; sets `completed_at`,
  `completed_by`, `version+1`. WAITING or COMPLETED → 409 `INVALID_TRANSITION`,
  no change. Afterwards the next WAITING entry becomes callable.
- Timestamps come from the server clock (truncated to microseconds) and are
  never earlier than the previous lifecycle timestamp. Host operations remain
  allowed on an inactive queue so existing tickets can be drained.
- No schema change: migration 0002 already contains the lifecycle CHECK and the
  one-CALLED-per-queue partial unique index. No Save-based transitions.

## UI

- `/host/queue` (HOST only; Customer redirected to `/`) keeps the welcome
  heading, lists assigned queues (selector when more than one, `?queue=`), shows
  the current called card and the WAITING table (ticket code, party size, joined
  time in restaurant timezone). No customer identity is displayed.
- Call Next button (disabled while a ticket is called or the list is empty, but
  the server decides). Complete on the current card asks for confirmation text
  before sending. Refresh reloads persisted state. Loading/disabled states and
  mapped 409/403/401 messages; unassigned Host sees "No queue is assigned".

## Acceptance and STS

| AC / tests | Observable acceptance |
|---|---|
| AC-014; T-022 | WAITING list oldest first with fixed timestamps; tie broken by id; no private fields |
| AC-015; T-023 | Call Next promotes A-001 to CALLED with `called_at/called_by`; A-002 stays WAITING |
| AC-016; T-024,T-025 | Empty queue 409 `QUEUE_EMPTY`; existing CALLED 409 `CALL_IN_PROGRESS`; rows unchanged |
| AC-017; T-026 | Parallel Call Next from two authorized Hosts (H1/H3): exactly one 200, one CALLED row |
| AC-004/018; T-005..007,T-027 | Guest 401; Customer 403; Host without grant 403 on board/call/complete; no writes |
| AC-019; T-028,T-029 | Complete persists COMPLETED with `completed_at/completed_by`; customer sees COMPLETED in browser |
| AC-020; T-030,T-031 | WAITING or COMPLETED completion 409 `INVALID_TRANSITION`, unchanged |
| AC-021; T-032,T-033 | Customer complete 403; after completing A-001, Call Next calls A-002 |
| Regression | CSRF/Origin on mutations; entry from another queue 404; service-level role/grant checks; parallel complete succeeds once |

Tests run against isolated real PostgreSQL, fail-first for the transition and
locking rules. Browser test drives a real Host Call Next / Complete. Record actual
commands and results in `../testing/f04-host-queue-results.md`.

DoD: AC-014..021 and T-005..007/T-022..033 executed, persisted DB checks, E2E
Host call/complete, synchronized API/traceability docs. Teammate review,
integration, CI and release remain separate workflow steps.
