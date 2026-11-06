# 07. Sprint 1 Feature-Level Engineering Contracts

**Version:** DQ-S1-Contracts-1.2, grouping revision 6 Nov 2026 (DEC-028, user authorized).
Four parent features F-01..F-04; F-04 includes completion under retained legacy ID
F-05. The original 1.1 contract/history remains in Git. This grouping revision
retains scope and all FR/BR/AC/test IDs; it does not establish earlier approval or
pre-implementation history. Every contract includes behavior/workflow, relevant
UI/API/data, validation/error/permissions, AC, STS linkage, dependencies and DoD as
required by CPE 334 §8.2.

## F-01: Authentication, Session Lifecycle and RBAC

**Depends on:** database users/sessions seed. **FR:** FR-001, FR-002. **NFR:** NFR-003, NFR-004, NFR-015.

Implementation details and added AC-022..026 / T-037..041:
[`f01-auth-implementation-contract.md`](f01-auth-implementation-contract.md).
Origin/CSRF defenses apply from S1; S2/S3 repeat these controls as regression tests.
Existing F-01 pre-implementation baseline is preserved. The 6 Nov addendum was
written before code but committed afterward with user authorization; this does
not establish a pre-implementation addendum commit.

**Feature SDS / workflow:** Seed 2 Customer and >=1 Host accounts. `/login` submits email+password; server verifies password hash, issues secure opaque cookie session. Guard resolves current user on protected requests; `/auth/me` identifies role. `/auth/logout` revokes session. Host actions and queue scoping denied at backend. Session expiration: 12h (proposal). Invalid/disabled account generic failure; never store plaintext password.

**UI:** login screen, logout action, clear expired-session prompt. **API:** `/auth/login`, `/auth/me`, `/auth/logout`. **Data:** `users`, `sessions`, `operator_queue_permissions`.

| AC | Observable outcome | Test IDs |
|---|---|---|
| AC-001 | Valid demo credentials login; current user returns correct role/id, no password hash. | T-001,T-002 |
| AC-002 | Invalid email/password fails without issuing a usable session. | T-003 |
| AC-003 | Logout invalidates existing session; subsequent protected request fails. | T-004 |
| AC-004 | Customer and unassigned Host receive 403 for Host queue actions; anonymous receives 401. | T-005,T-006,T-007 |
| AC-005 | Foreign customer ticket detail cannot be retrieved. | T-008 |

**DoD:** tests above executed against submitted commit; passwords hashed; environment template excludes secrets; reviews verify role/ownership checks; docs match actual cookie lifecycle.

## F-02: Join Waitlist and Generate Ticket

**Depends on:** F-01, seeded queue, allocator and constraints. **FR:** FR-003, FR-004, FR-009. **BR:** BR-001, BR-002, BR-008, BR-009.

Implementation details and error-code choices (DEC-029):
[`f02-join-implementation-contract.md`](f02-join-implementation-contract.md).

**Feature SDS / workflow:** authenticated Customer opens join form; selects seeded active service queue, party size 1–16; server computes business date, atomically increments queue-day counter and inserts `WAITING` ticket with unique code; duplicate active customer ticket in same queue is forbidden by a partial unique DB index. Return entry summary and navigate to My Ticket. No client-supplied `ticketNumber` or status.

**UI:** `/join`; invalid range error, loading submit, duplicate conflict banner. **API:** `POST /queues/{queueId}/entries`. **Data:** queue day counters, queue entries, indexes.

| AC | Observable outcome | Test IDs |
|---|---|---|
| AC-006 | Logged-in customer submits party size 4; receives 201, correct queue/id/code, WAITING; row exists in DB. | T-009,T-010 |
| AC-007 | Party size 0, 17, noninteger, absent rejected with no DB entry; party size 10 accepted with a persisted WAITING ticket. | T-011,T-012,T-036 |
| AC-008 | Rejoining same active queue rejected 409; parallel duplicate requests result in one entry. | T-013,T-014 |
| AC-009 | Concurrent different customers get unique sequences on the same queue/day. | T-015 |
| AC-010 | Inactive/nonexistent queue or unauthorized role cannot create valid entry. | T-016,T-017 |

**Additional S1 acceptance:** a 10-person party can join and see a WAITING ticket (T-036); no table allocation is implied before Sprint 2.

**DoD:** API+DB integration and concurrency tests prove atomicity; migration includes both unique-index guards; negative tests and UI feedback documented; reviewed PR.

## F-03: Customer My Ticket and Privacy

**Depends on:** F-01, F-02. **FR:** FR-005, FR-010. **BR:** BR-007.

**Feature SDS / workflow:** Customer opens `/my-queue` and sees own active queue ticket; if no active entry, show Join button. Direct ticket endpoint checks owner before return. After completed, entry moves to history (S1 ticket detail still accessible to its owner). No email/identity in public status.

**UI:** status card + empty/error states. **API:** `GET /me/queue-entries/active`, `GET /me/queue-entries/{entryId}`. **Data:** queue_entries customer relation.

| AC | Observable outcome | Test IDs |
|---|---|---|
| AC-011 | Customer sees exact persisted ticket number, size, status and time after join. | T-018,T-019 |
| AC-012 | Customer with no active entry sees correct empty state. | T-020 |
| AC-013 | Another customer's ticket is not exposed even by direct URL/API call. | T-008,T-021 |

**DoD:** read is DB-backed, no cross-user data exposed, one browser E2E demonstrates status after join.

## F-04: Host Queue Operations — Board, FIFO Call Next and Complete

**Depends on:** F-01, F-02. **FR:** FR-006, FR-007, FR-008, FR-009. **BR:** BR-003, BR-004, BR-005, BR-006.

This is one parent feature with two workflows. Complete retains legacy ID F-05
for traceability; it has no separate parent feature or feature branch. Both
workflows must satisfy their existing acceptance criteria for F-04 to be complete.

### Queue board and FIFO Call Next

**Feature SDS / workflow:** authorized Host enters `/host/queue`; backend lists WAITING entries FIFO, current CALLED entry; clicking Call Next obtains queue lock, rechecks active called state and selects earliest waiting ticket atomically; if no waiting ticket, return business error; if already called, reject. Concurrent Hosts never claim same ticket. Other queue's Host denied.

**UI:** waiting table, current called card, action loading/error; no personal email. **API:** GET Host queue endpoints and `POST /host/queues/{queueId}/call-next`. **Data:** queue row lock, queue entry timestamps, permission join.

| AC | Observable outcome | Test IDs |
|---|---|---|
| AC-014 | Host views authorized WAITING entries in consistent FIFO order. | T-022 |
| AC-015 | Call Next selects oldest eligible WAITING entry and persists CALLED/actor/time. | T-023 |
| AC-016 | Empty queue and already-called queue reject with defined 409 code. | T-024,T-025 |
| AC-017 | Concurrent Call Next requests cannot double claim, and one CALLED/queue rule survives. | T-026 |
| AC-018 | Unauthorized/other-queue Host cannot view/manage queue. | T-007,T-027 |

**Workflow DoD:** real SQL locking, DB integration/race tests, visible UI transitions and reviewed code. Mock-only tests don't count as proof of persistence.

### Complete Waitlist Service (retained legacy ID F-05)

**Prerequisite within F-04:** an entry successfully moved to CALLED by Call Next.
**FR:** FR-008, FR-009. **BR:** BR-004, BR-006.

**Feature SDS / workflow:** Host marks current CALLED entry COMPLETED, records `completed_at`, `completed_by`. Completion means the queue/waitlist service finished, not meal completion. Reject status WAITING or terminal entry. Oldest waiting party can then be called. Sprint 2 will require table assignment before completion, through a versioned rule revision.

**UI:** Complete button on current called card; confirmation text; queue refresh. **API:** POST `/host/queues/{queueId}/entries/{entryId}/complete`. **Data:** queue_entries state and timestamps.

| AC | Observable outcome | Test IDs |
|---|---|---|
| AC-019 | Authorized Host completes CALLED entry; status and completion metadata persist. | T-028,T-029 |
| AC-020 | WAITING/COMPLETED ticket cannot be directly/repeatedly completed. | T-030,T-031 |
| AC-021 | Only authorized Host can complete and the next waiting entry becomes callable. | T-032,T-033 |

**Workflow DoD:** positive/negative paths + scoped access + persistent DB check + E2E Host call/complete; documented transition rule.

**Combined F-04 DoD:** AC-014..021 and their existing T-007/T-022..033 checks cover
the board, call, completion, permissions, concurrency and invalid transitions.
The Host can complete the current call and then call the next waiting entry.
Keep the two workflow APIs and all existing AC/test IDs; review and integrate
them as one parent feature. S1 completion remains manual waitlist handling.

## Global Sprint 1 acceptance and DoD

- All four P0 parent features F-01..F-04 are present in web/API/DB and function without stubbed persistence; F-04 includes the entire legacy F-05 completion workflow and AC-019..021.
- At least one normal end-to-end journey, business-rule rejection and forbidden-role/cross-user journey automated and demonstrated.
- Tests show meaningful TDD evidence: original failing test at earlier commit/log → code change → passing test on release commit.
- `main` release tag/commit can be freshly installed from README instructions; migrations and seed reproducible.
- Each feature has an Issue, branch, PR, substantive teammate review and integrated test evidence; both members understand design and tests.
- Document actual results and evidence links; **this draft by itself is not evidence of execution**.


## v2 revision note — preserve S1 baseline contract history

- `BR-008` changed from party size 1–8 to **1–16** to support 10+ guests; retain old contract in Git history, do not backdate it.
- AC-007/T-011 boundary changes from invalid 9 to invalid 17; T-036 is the accepted 10-person positive case.
- Sprint 1 remains `WAITING → CALLED → COMPLETED` **without table allocation**; `BR-010..025` require a separate pre-implementation S2 contract.
- If this specification is being introduced after actual implementation, record a change request/ADR rather than rewriting historical Sprint 1 evidence.
