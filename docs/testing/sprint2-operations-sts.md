# Sprint 2 Software Test Specification (STS) — Operations v2

**Status:** planned only, NOT RUN. `S2-T-*` IDs are reserved for implementation; update exact file paths and commands in evidence index once tests exist. Use a real isolated PostgreSQL database for transaction and concurrency verification. No mock-only persistence evidence.

## Fixture plan

- Restaurant R1; zones Main, Terrace, Private; tables T1(2),T2(4),T3(8) Main; T4(6),T5(4),T6(2) Terrace; T7(8) Private.
- Approved symmetric edges T1–T2, T2–T3, **T4–T5**, T5–T6. No edge T4–T7.
- Customers C1(size10), C2(size4), C3(size2); Hosts H1/H3 authorized; H2 unassigned. Deterministic time and queue order; all endpoints use authenticated sessions and anti-CSRF values in browser/API test clients.
- Before each case, reset fixtures safely in dedicated test DB; prefer transactional teardown, never point at production. Concurrency tests use distinct DB connections with a synchronization barrier.

## Cases: table graph, ranking and floor

| Test ID | Level | Setup / action | Expected | AC |
|---|---|---|---|---|
| S2-T-001 | API/DB | H1 GET floor | Accurate zones, capacity, version and table states | S2-AC-001 |
| S2-T-002 | API | H2 unauthorized GET floor/mutate table | 403, no private operating data or mutation | S2-AC-001 |
| S2-T-003 | Unit | T4(6), T5(4) approved adjacency; p=10 | Valid connected candidate of 10 seats | S2-AC-002 |
| S2-T-004 | Unit/API | T4 + T7, different zones or not adjacent | Rejected `TABLES_NOT_CONNECTED` | S2-AC-002 |
| S2-T-005 | Unit | 3 members connected by chain T4–T5–T6 | Accepted only if all chosen edges approved and same zone | S2-AC-002 |
| S2-T-006 | Unit | p=10, single T7(8) | Rejected `CAPACITY_INSUFFICIENT` | S2-AC-003 |
| S2-T-007 | Unit | Candidate sets of 1/2/3 with varied unused seats | Rank table count, spare seats, IDs deterministically | S2-AC-003 |
| S2-T-008 | API/DB | T5 DIRTY or HELD during preview | Exclude T4+T5; no recommendation of unavailable set | S2-AC-003 |

## Cases: atomic hold, seat and concurrency

| Test ID | Level | Setup / action | Expected | AC |
|---|---|---|---|---|
| S2-T-009 | API/DB | C1 size10 WAITING; H1 call+hold T4+T5 | One HELD assignment with two mappings; ticket CALLED; both tables HELD | S2-AC-004 |
| S2-T-010 | API/DB | Invalid table ID in request; try to hold T4+unknown | Entire transaction rollback; T4 AVAILABLE and ticket WAITING | S2-AC-004 |
| S2-T-011 | API/DB | Valid T4+T5 HELD then Seat | ticket COMPLETED, assignment ACTIVE, T4/T5 OCCUPIED | S2-AC-005 |
| S2-T-012 | API/DB | Attempt Seat with expired/released hold | 409, no partial occupancy or queue completion | S2-AC-005 |
| S2-T-013 | DB concurrency | H1/H3 concurrently hold sets T4+T5 and T5+T6 | Exactly one wins overlapping T5; other 409; no duplicate held mapping | S2-AC-006 |
| S2-T-014 | DB concurrency | 20 parallel attempts to reserve overlapping sets | At most one claim per table, zero partial assignments | S2-AC-006 |
| S2-T-015 | DB rollback | Inject SQL failure midway after one table update | All rows unchanged after rollback; no assignment/member orphan | S2-AC-006 |

## Cases: cleanup and authorized changes

| Test ID | Level | Setup / action | Expected | AC |
|---|---|---|---|---|
| S2-T-016 | API/DB | Active 10-person seating T4+T5; checkout | Both DIRTY, assignment CLOSED, audit event | S2-AC-007 |
| S2-T-017 | API/DB | One table DIRTY; attempt direct DIRTY→AVAILABLE | `INVALID_TRANSITION`; only DIRTY→CLEANING→AVAILABLE | S2-AC-007 |
| S2-T-018 | Unit/API | Earlier 6-person waiting cannot fit chosen T2(4), later 4-person can | Recommend oldest *eligible*, show capacity skip rationale | S2-AC-008 |
| S2-T-019 | API/DB | H1 override rank with logged reason, then invalid overcapacity override | Valid audited permitted action; invalid overcapacity still rejected | S2-AC-008 |
| S2-T-020 | API/DB | C1 cancels own WAITING ticket | CANCELLED, becomes inactive, event recorded | S2-AC-009 |
| S2-T-021 | API | C2 tries C1 cancel; C1 tries cancel CALLED | 404 for foreign, 409 invalid transition | S2-AC-009 |
| S2-T-022 | API/DB | CALLED C1 with HELD T4+T5 marked no-show | ticket NO_SHOW, both tables AVAILABLE, assignment RELEASED | S2-AC-010 |
| S2-T-023 | API/DB | NO_SHOW attempted twice / unauthorized Host | conflict or forbidden, no leftover hold | S2-AC-010 |
| S2-T-024 | API/DB | C1 seated T4+T5 then checkout | all member tables DIRTY, no table AVAILABLE early | S2-AC-011 |
| S2-T-025 | Browser E2E | H1 reviews floor, seats 10 people then checkouts | visible all-table transitions match backend, refresh persists | S2-AC-011 |
| S2-T-026 | API/DB | Duplicate checkout of CLOSED assignment | conflict, no duplicate event/state | S2-AC-012 |
| S2-T-027 | DB integration | Try next group on T4 while DIRTY | reject even if sum capacity would fit | S2-AC-012 |

## Extra security and boundary cases

| Test ID | Level | Inputs | Expected |
|---|---|---|---|
| S2-T-028 | API | Customer directly invokes `/host/queues/{id}/match-and-call` | 403; no side effect |
| S2-T-029 | API | Forged browser cookie with missing/bad CSRF token | 403; no side effect |
| S2-T-030 | API | duplicate table IDs / more than 3 IDs / altered capacity in JSON | reject; capacity comes from DB |
| S2-T-031 | DB concurrency | Seat and server hold-expiry attempt race | only one terminal hold outcome, no contradictory states |
| S2-T-032 | DB integration | Simulate restart with active T4+T5 assignment | assignment and both states persist |
| S2-T-033 | API | Host H2 attempts to seat H1's authorized restaurant table | 403 and no mutation |

## Execution and evidence policy

- Suggested commands (after implementation): `go test ./...`, `go test -tags=integration ./...`, `go test -race ./...`, frontend `npx playwright test`; no tests have been run now.
- Preserve TDD evidence for connectedness rules (`S2-T-004`) and race exclusion (`S2-T-013`) with a real **RED** test, implementation, **GREEN** test and reviewed PR. Use transaction error injection only on isolated test DB.
- Test every API mutation with own role/scope/ownership checks. Return stable business errors, not raw SQL error strings.
- Record command, test name/file, inputs/preconditions, expected vs actual, date, SHA, result, test report/CI link. Screenshots alone are insufficient.

| Test | Actual file | Command | Commit | Result | Evidence |
|---|---|---|---|---|---|
| S2-T-009 | [fill] | [fill] | [fill] | NOT RUN | [fill] |
| S2-T-013 | [fill] | [fill] | [fill] | NOT RUN | [fill] |
| S2-T-015 | [fill] | [fill] | [fill] | NOT RUN | [fill] |
