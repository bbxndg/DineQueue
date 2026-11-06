# Sprint 2 Feature Engineering Contracts — DineQueue Operations v2

**Draft version:** DQ-S2-CONTRACT-2.0 | **Status:** PROPOSED, not implemented or tested. **Dependencies:** released S1 vertical slice and Lab5 environment; changes to S1 state semantics must be versioned/reviewed.

## Cross-feature interface and Definition of Done

All API requests require cookie-authenticated, CSRF-protected sessions and scoped Host/Manager permissions. Every mutation is idempotent or returns a stable conflict; errors use documented `error.code`. The UI never assumes an API write succeeded until confirmed. Migrations, unique constraints, realistic seeds, negative/boundary/race tests, PR review and traceability updates are part of feature completion. Every feature should be tracked as a GitHub Issue and reviewed before implementation.

| Error code | Meaning | HTTP |
|---|---|---:|
| `PARTY_TOO_LARGE` | Party outside 1–16 limit | 400 |
| `INVALID_TABLE_SET` | Missing/duplicate/too many table IDs | 400 |
| `TABLES_NOT_CONNECTED` | Selected tables not physically combinable | 409 |
| `CAPACITY_INSUFFICIENT` | Sum usable capacity too small | 409 |
| `TABLE_NOT_AVAILABLE` | One or more tables HELD/OCCUPIED/DIRTY/etc. | 409 |
| `STALE_PREVIEW` | Client preview/layout version changed | 409 |
| `CALL_IN_PROGRESS` | Queue already has CALLED ticket | 409 |
| `INVALID_TRANSITION` | Queue/assignment/table status transition not allowed | 409 |
| `HOLD_EXPIRED` | Server hold no longer valid | 409 |
| `SCOPE_FORBIDDEN` | Host not allowed for this restaurant/queue | 403 |

### F-06/F-16: Interactive floor and table lifecycle

**Feature SDS:** Render floor zones and status of each table (code, capacity, position for display, version). Staff selects one or more tables. Available state, current assignment and authorized scope come from API. Staff may checkout a seated group then mark DIRTY → CLEANING → AVAILABLE. Do not allow a table not in the selected restaurant. No automatic drag-based adjacency inference.

**UI:** `/host/floor`; zone tabs, accessible table list alternative to visual grid, readable statuses, confirm dangerous transitions, detail panel and conflict toast. **API:** `GET /host/restaurants/{id}/floor`, `POST /host/seating-assignments/{id}/checkout`, `POST /host/seating-assignments/{id}/start-cleaning`, `POST /host/seating-assignments/{id}/finish-cleaning`. **Data:** `floor_zones`, `dining_tables`, events/assignment members.

| AC | Observable outcome | Tests |
|---|---|---|
| S2-AC-001 | Assigned Host sees accurate zone/table inventory and statuses; unauthorized Host gets 403. | S2-T-001,002 |
| S2-AC-007 | Checkout closes whole group, makes every member DIRTY; cleaning progresses to AVAILABLE with guards. | S2-T-016,017 |

**DoD:** real DB persists transitions; zone scoping enforced; keyboard accessible list and interactive plan agree; commits/tests linked.

### F-14: Explainable matching recommendations

**Feature SDS:** Given waiting party and table availability snapshot, enumerate individual tables and connected sets up to 3, same zone, approved edges, ALL AVAILABLE and adequate capacity. Recommend sorted sets by minimum tables, minimum spare capacity then stable IDs. Given a selected table set, choose oldest eligible WAITING party by `(joined_at,id)` and record why earlier unfitting parties could not be selected. Preview is advisory, not reservation.

**UI:** show required seats, selected tables, excess seats, connection rationale, earlier queue constraint and confirm action. **API:** `GET /host/queues/{queueId}/matching?partyId=...` and/or `POST /host/queues/{queueId}/match-preview`; no side effects. **Data:** same restaurant/zone table graph, statuses and waiting list.

| AC | Observable outcome | Tests |
|---|---|---|
| S2-AC-002 | Reject disconnected/cross-zone/inactive connections; accept connected T4+T5. | S2-T-003..005 |
| S2-AC-003 | Party 10 receives connected 6+4 candidate; single 8-seat option is excluded, deterministic ranking. | S2-T-006..008 |
| S2-AC-008 | Oldest fitting party chosen; permitted manual override has explicit auditable actor/reason and cannot break invariants. | S2-T-018,019 |

**DoD:** pure function unit tests for graph/heuristic and integration tests with authoritative state; rationale visible in UI; policy confirmation recorded.

### F-07/F-15: Atomic multi-table hold, call and seating

**Feature SDS:** Host selects eligible ticket and 1–3 table IDs; backend re-checks queue rule and table topology under DB locks, creates HELD assignment, changes every table to HELD, and updates ticket WAITING→CALLED in one transaction. Seat confirms active hold, updates every table OCCUPIED, assignment ACTIVE and ticket CALLED→COMPLETED in one transaction. Idempotency is scoped to actor and request key. Overlapping actions must conflict deterministically without partial writes.

**UI:** preview modal → Call & Hold confirmation → Active Call panel with hold expiry → Seat party confirmation; on 409 refresh and explain stale allocation. **API:** `POST /host/queues/{id}/match-and-call` `{entryId,tableIds,expectedLayoutVersion,idempotencyKey}`, `POST /host/seating-assignments/{id}/seat`. **Data:** queue entries, dining tables, assignment and membership, events, optional idempotency keys. **Dependencies:** F-06, F-14, F-16.

| AC | Observable outcome | Tests |
|---|---|---|
| S2-AC-004 | One successful call claims both T4 and T5 and CALLED ticket atomically; no unrelated modifications. | S2-T-009,010 |
| S2-AC-005 | Valid hold seats party, queue COMPLETED and all tables OCCUPIED in same committed transaction. | S2-T-011,012 |
| S2-AC-006 | Two Hosts racing for overlapping table sets cannot both succeed; injected failure leaves no partial group. | S2-T-013..015 |

**DoD:** PostgreSQL concurrency tests, partial unique active table-index, version/hold checks, rollback tests, documented lock order and PR review. Go Fiber handler must not bypass service/repository guards.

### F-08: Customer cancellation and staff no-show

**Feature SDS:** Customer can cancel only own WAITING ticket; Host can mark CALLED NO_SHOW and releases any HELD table group transactionally. Terminal tickets cannot rejoin same state; user may create new ticket after prior terminal status (business day policy to confirm).

**UI:** customer Cancel with confirmation; Host No-show with confirmation and reason. **API:** `POST /me/queue-entries/{entryId}/cancel`, `POST /host/queue-entries/{entryId}/no-show`. **Data:** queue status, events, assignment changes.

| AC | Observable outcome | Tests |
|---|---|---|
| S2-AC-009 | Customer cancels own WAITING ticket; other-ticket cancel returns 404; CALLED cannot be canceled by Customer. | S2-T-020,021 |
| S2-AC-010 | No-show releases every HELD table and marks CALLED→NO_SHOW, never leaves orphan hold. | S2-T-022,023 |

**DoD:** negative tests and backfilled traceability, idempotent release, no staff bypass.

### F-09: Group checkout and cleaning

**Feature SDS:** A Host can close one ACTIVE seating assignment, atomically marking *all* tables DIRTY and assignment CLOSED. Next, explicit staff operations progress DIRTY→CLEANING→AVAILABLE. Whole-set release is a v2 product rule. Record event time and actor.

| AC | Observable outcome | Tests |
|---|---|---|
| S2-AC-011 | A 10-person 6+4 group checks out, both tables remain unavailable until cleaned. | S2-T-024,025 |
| S2-AC-012 | Double checkout / early AVAILABLE fails, and a second customer cannot use DIRTY table. | S2-T-026,027 |

**DoD:** database transaction proof, no duplicate release, UI reports correct state, events persisted.

## Planned UI/API/data handoff

| Feature | Main screen | API shapes | Migrations | Test type |
|---|---|---|---|---|
| F-06/F-16 | `/host/floor` | typed zone/table snapshot | zones/tables/edges | API, DB, browser |
| F-14 | matching side panel | candidate sets with reason/capacity/versions | none beyond graph | pure unit, API |
| F-07/F-15 | call/seat confirmation | POST with explicit `tableIds`; response assignment/ticket | assignments/members/active unique index | transaction concurrency |
| F-08 | own ticket, host call card | cancel/no-show mutations | event append | authorization, transitions |
| F-09 | floor and assignment detail | checkout/clean transitions | event/history | API/DB, E2E |

## Global Sprint 2 DoD

- Every S2-AC-001..012 has a recorded real test, expected/actual, commit, result, and review link.
- Sprint 1 auth/ticket/ownership/concurrency tests still pass after revised completion semantics.
- A seeded 10-person party moves join → matched group → call/hold → seat → group checkout → cleaned; negatives demonstrate disconnected and occupied-group rejection.
- All DB constraints/migrations and API contracts are committed before implementation; deployment from main remains repeatable after Lab 5.
- **Never mark this specification as implemented or tests as PASS until actual execution.**
