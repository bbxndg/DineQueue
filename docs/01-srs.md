# 01. Software Requirements Specification (SRS)

**Document:** DQ-SRS-2.0 (current v2 baseline; unresolved rules pending team and instructor/TA resolution)
**Source mapping:** CPE 334 master spec §§5, 6 theme 9, 7, 8.1.
**Product:** DineQueue — walk-in restaurant waitlist and table management.

## 1. Purpose and context

Problem: During peak hours restaurant hosts track walk-in parties with paper, ad hoc chat or spreadsheets. Customers are unsure of queue status; staff may mistakenly call the wrong party or allocate the same table twice.

Goal: Deliver a reliable, auditable waitlist flow for one pilot restaurant: register a **walk-in** party, issue a unique ticket, view waiting status, permit an authorized Host to call and complete entries, and later coordinate table seating.

**Explicitly not** advance appointment booking, table reservation, food orders, POS, payment processing, loyalty programs, SMS/email notifications, external APIs, or medical or sensitive personal data processing.

## 2. Actors

- **Customer:** authenticated individual representing one party; joins a service queue and views only their own ticket details.
- **Host (Operator):** authenticated restaurant operator with authorization to a specific service queue; views/manage its entries and subsequently tables at assigned restaurant.
- **Guest/public display (later, optional):** anonymous viewer of non-private ticket number/status only; never personal/customer details.
- **Manager (optional later, not required):** an extra role only if time permits; core requirements must work with Customer + Host.

## 3. Scope and constraints

### 3.1 Sprint 1 committed scope

One seeded restaurant with one seeded service queue; predefined customer and host accounts; login, logout, current user; customer joins, retrieves and sees status of own ticket; host sees queue, calls FIFO next, completes ticket; unique ticket creation under concurrency; duplicate active-ticket rejection; state and date validation; authorization and persistence; automated evidence; release on `main` with setup guide.

**Party size** is stored from Sprint 1 (integer 1–16 under the v2 BR-008 revision) but capacity matching starts in Sprint 2. The separate physical-table capacity limit is 1–8. Sprint 1 does **not** imply table matching, reservation calendar or public notifications.

### 3.2 Scope for later sprints

Sprint 2: dining table inventory, host table status, party-to-table assignment, cancellation, no-show, queue eligibility policy. Sprint 3: history and summary metrics, restricted public display (if retained), robustness and usability, regression and UAT fixes.

### 3.3 Assumptions requiring confirmation

- Pilot uses one restaurant, one walk-in service queue, and a single operating timezone `Asia/Bangkok`.
- For Sprint 1, ordering is strict FIFO by server-recorded `joined_at` and id tie-breaker.
- `COMPLETED` means **waitlist service successfully completed / party seated**, NOT meal/payment completion.
- Ticket numbering is scoped per (service queue, local business date), starts at `A-001`, resets next local business date. Ticket identifiers remain permanently unique through internal UUID IDs.
- A customer may hold at most one active entry per queue (`WAITING`, `CALLED` in Sprint 1; extend if more active states added).
- One queue has at most one currently `CALLED` entry until it is completed/closed (Sprint 1 simplification).
- v2 party size: 1–16; customer name from seeded account; no public personally identifiable details.
- Any change to the selection policy for table-fit exceptions in Sprint 2 must be documented before implementation and clarified with TA if ambiguous.

## 4. Functional Requirements (FR)

| ID | Priority | Requirement | Sprint |
|---|---|---|---|
| FR-001 | Must | Authenticated Customer and Host can login and logout; current-user endpoint reports identity/role. | S1 |
| FR-002 | Must | Host/Customer permissions are enforced in API according to role and queue ownership. | S1 |
| FR-003 | Must | Customer selects the active seeded queue, provides valid party size, and joins. | S1 |
| FR-004 | Must | Successful joining persists a unique human-readable ticket number and `WAITING` status. | S1 |
| FR-005 | Must | Customer views own active queue entry, status, issue time, and number. | S1 |
| FR-006 | Must | Host views waiting entries and the current called ticket in an authorized queue. | S1 |
| FR-007 | Must | Host invokes `Call Next` for earliest eligible WAITING entry in authorized queue. | S1 |
| FR-008 | Must | Host marks currently called ticket as COMPLETED and records completion time. | S1 |
| FR-009 | Must | System rejects duplicate active tickets, invalid party sizes, invalid/out-of-order transitions and attempts that bypass queue ordering. | S1 |
| FR-010 | Must | Protected customer ticket details are inaccessible to any other customer/anonymous user. | S1 |
| FR-011 | Must | Data persists across API restart and supports repeatable setup/migrations/seeding. | S1 |
| FR-012 | Must | Host can view an interactive floor plan with table capacities/status/positions and seat an eligible CALLED party at one available table or an approved connected combination. | S2 |
| FR-013 | Must | Allocation/hold/occupancy of **every table** in an assignment is exclusive and recorded as one atomic transaction. | S2 |
| FR-014 | Must | Customer can cancel own eligible waiting entry; canceled ticket loses active status. | S2 |
| FR-015 | Must | Host can mark called entry NO_SHOW, with controlled permissions. | S2 |
| FR-016 | Must | Host can check out a seated party, mark all assigned tables DIRTY, then confirm CLEANING → AVAILABLE; release must apply to the entire combined set. | S2 |
| FR-017 | Should | Host can manage multiple counters/queues if approved, while scoping access to the assigned queue. | S2/S3 |
| FR-018 | Must | Authorized staff can inspect historical queue events, seating/turnover, counts, measured wait times and utilization metrics with date/zone filters. | S3 |
| FR-019 | Should | Public display can show current ticket number and limited non-private status without any user details. | S3 |
| FR-020 | Must | Deployed app uses documented configuration, guarded CI and repeatable delivery/release procedures. | Lab5+ |
| FR-021 | Must | Team records independent UAT, defects, fixes and re-test evidence. | Lab6 |
| FR-022 | Must | Team implements one TA-approved change with impact analysis and regression tests. | Lab7 |


### Extended requirements for Restaurant Operations (v2 design revision)

These IDs are **additional proposals** expanding Topic 9; they are not prescribed by the course PDF. The team must confirm them and revise contracts before implementation.

| ID | Priority | Requirement | Planned |
|---|---|---|---|
| FR-023 | Must | Customer can enter a party size 1–16, including a party larger than any single dining table, and still receive a valid waitlist ticket. | S1 |
| FR-024 | Must | Host sees floor zones, each table's seat capacity (1–8), stable coordinates and current availability, including HELD/occupied/dirty/cleaning states. | S2 |
| FR-025 | Must | Store manager-approved undirected combinability edges between tables; only connected tables within the same zone can form a seating set. | S2 |
| FR-026 | Must | For a party of 10, the system can suggest 6+4 adjacent seats and allocate **both** tables atomically; reject nonadjacent, already-held, undercapacity, or out-of-service groups. | S2 |
| FR-027 | Must | Host confirms a suggested set; system creates an exclusive short-lived hold and calls the selected customer atomically. | S2 |
| FR-028 | Must | Host seats the CALLED party, converting the hold into an ACTIVE seating assignment and all its tables to OCCUPIED in one transaction. | S2 |
| FR-029 | Must | Host closes the seating session, marks all member tables DIRTY, records cleaning and returns each table to AVAILABLE only after confirmation. | S2 |
| FR-030 | Should | Host sees why the matching algorithm recommended a table set, and can apply a permissioned override with an explicit reason/audit record. | S2/S3 |
| FR-031 | Must | Customer-facing status shows current ticket, own position where meaningful, and accurate non-private waitlist updates; approximate wait estimates must be clearly labelled or withheld when data is insufficient. | S3 |
| FR-032 | Must | Operations dashboard calculates wait time, seating turnaround, occupancy, no-show/cancellation counts using persisted timestamps and selected filters. | S3 |
| FR-033 | Should | Manager configures table capacities, zones, combinability edges and layout safely; seed-only management is acceptable until S3. | S3 |
| FR-034 | Must | Concurrent match/hold/seat/checkout requests cannot allocate overlapping tables, duplicate a seating assignment or leave a partially updated group. | S2 |
| FR-035 | Must | Operator sees both an actionable queue list and interactive floor surface, with conflict recovery, loading/empty/denied/error states and accessible labels. | S2 |

## 5. Business Rules (BR)

| ID | Rule and rejection behavior | Sprint |
|---|---|---|
| BR-001 | One active queue entry per (customer, service queue); concurrent attempts may create only one. Conflict: `ACTIVE_QUEUE_EXISTS`. | S1 |
| BR-002 | Unique ticket number per (queue, local service date) allocated atomically; concurrent joins cannot duplicate a sequence. | S1 |
| BR-003 | First eligible `WAITING` ticket is the oldest by `(joined_at ASC, id ASC)` in that queue; manual skip disallowed in S1. | S1 |
| BR-004 | Valid Sprint 1 transitions: `WAITING -> CALLED -> COMPLETED` only. `WAITING -> COMPLETED` rejected. | S1 |
| BR-005 | No new `Call Next` while the same service queue already has a `CALLED` ticket; rejection `CALL_IN_PROGRESS`. | S1 |
| BR-006 | Only authorized Host can call/complete tickets in the assigned queue. Customers cannot do staff transitions. | S1 |
| BR-007 | Customers can read only their own tickets; use non-revealing `404` for inaccessible foreign tickets. | S1 |
| BR-008 | Party size is integer in inclusive range [1,16] (REVISED proposal; per-table capacity is independently capped at 8, multi-table seating in Sprint 2). | S1 |
| BR-009 | Ticket dates/numbers come from the backend in restaurant-local timezone; never trust a supplied sequence/status. | S1 |
| BR-010 | Seating requires a `CALLED` ticket and one or more physically combinable, connected, AVAILABLE tables from the same restaurant/zone whose summed usable capacity is >= party size; assignments/holds are exclusive and transactional. | S2 |
| BR-011 | In S2, for a selected single/connected table set, recommend the oldest WAITING party who fits (joined_at,id). Skipping an earlier party is allowed only under an explicit capacity/availability policy; surface reason and log operator override. This revises S1 strict FIFO and needs TA/team confirmation. | S2 |
| BR-012 | Only WAITING may be customer-canceled; only CALLED may be marked NO_SHOW; terminal statuses cannot be reactivated. | S2 |
| BR-013 | In S2, `COMPLETED` means seated and requires an ACTIVE seating assignment and OCCUPIED table set, committed atomically. Sprint 1 `COMPLETED` is manual waitlist service completion only (contract-versioned change). | S2 |
| BR-014 | Wait duration = called_at - joined_at; completed counts are based on queue records, not simulated meal/payment events. | S3 |


### Extended business rules — table combination and operations

| ID | Rule | Phase |
|---|---|---|
| BR-015 | Each physical table has capacity 1–8; party size may be 1–16. A party of 10 therefore cannot use a single capacity-8 table. | S1/S2 |
| BR-016 | A combined seating set must consist of 2–3 distinct tables in one restaurant and zone; all edges must form a connected graph using explicitly manager-approved `table_connections` (not screen proximity). | S2 |
| BR-017 | The sum of **usable seats** must cover the party size; single-table and multiple-table candidates are ranked deterministically: minimum table count, then minimum unused seats, then stable table IDs, subject to oldest-eligible-party fairness policy. | S2 |
| BR-018 | Only AVAILABLE tables may be HELD; HELD tables cannot be used by any other ticket. An unexpired HELD set belongs to exactly one CALLED entry. | S2 |
| BR-019 | CALL+HOLD and SEAT transitions atomically mutate the queue entry, seating assignment, and **all** involved table rows/mappings. One failed check rolls back the entire operation. | S2 |
| BR-020 | Completing service after S2 seating enables requires ACTIVE assignment; seats must reflect OCCUPIED; a repeated seating request is rejected, never silently duplicated. | S2 |
| BR-021 | No-show or explicit hold expiry releases all held tables transactionally and records who/when/why. Define 5-minute configurable hold as proposal; do not use frontend timers as authority. | S2 |
| BR-022 | After seated party departs, all member tables become DIRTY; only authorized staff may set CLEANING then AVAILABLE. Partial release of a combined group is forbidden unless a separate reviewed business rule is approved. | S2 |
| BR-023 | Manual overrides must not bypass safety invariants; actor, reason, original suggestion and final selection are auditable. | S2/S3 |
| BR-024 | ETA must not imply a guaranteed seating time. Use measured historical durations and show 'Estimate unavailable' without adequate data. | S3 |
| BR-025 | Floor-plan positions are display metadata; server validates explicit table connections, capacity, role and state regardless of client-supplied coordinates. | S2 |

## 6. Measurable non-functional requirements (NFR)

These numerical targets are **team proposals**, not values mandated in the PDF.

| ID | Requirement | Verification |
|---|---|---|
| NFR-001 | Correctness: no duplicate numbers or active entries after 20 concurrent join attempts on isolated test DB. | Integration concurrency test |
| NFR-002 | Correctness: two simultaneous `Call Next` requests never claim the same entry or violate BR-005. | Integration concurrency test |
| NFR-003 | Security: server-side authorization applies to every protected endpoint, not just hidden UI buttons. | Role/ownership API tests |
| NFR-004 | Security: hash managed passwords with secure algorithm; sessions expire and logout invalidates them; do not commit secrets. | Auth tests/config review |
| NFR-005 | Persistence: tickets survive service restart and schema can be recreated by migrations/seeds. | Integration/restart check |
| NFR-006 | Usability: success, empty, error and loading states; forms have accessible text labels; clear status for color-blind users. | Manual accessibility/UAT cases |
| NFR-007 | Performance target: p95 API read response < 1s for 20 local synthetic concurrent users in documented hardware/test conditions. | Repeatable load test (S3) |
| NFR-008 | Reliability: normal release requires passing required CI build/tests; deployment identifies exact commit/tag. | Lab 5 CI pipeline record |
| NFR-009 | Maintainability: meaningful automated unit + DB integration + selected E2E tests with IDs mapped to ACs. | STS and CI results |
| NFR-010 | Privacy: anonymous display excludes customer names, emails, identifiers and private notes. | API/view review + tests |


### Extended NFRs for the revised scope

| ID | Measurable target | Evidence |
|---|---|---|
| NFR-011 | No overlapping ACTIVE/HELD table claims across at least 20 concurrent seating/hold requests in a real PostgreSQL test DB. | DB concurrency tests |
| NFR-012 | Table/queue/allocation state is all-or-nothing after injected SQL failure or concurrent update; zero partial table-group allocations. | Rollback integration tests |
| NFR-013 | Public status response contains ticket codes only and no user identifiers, emails or private notes. | API schema + negative privacy test |
| NFR-014 | Keyboard-usable floor interaction, accessible table statuses with text labels, clear mobile screens; check at 360px and desktop at 1280px. | UAT/accessibility checklist |
| NFR-015 | Logged-in state-changing requests use origin restriction and CSRF defense; sessions are securely invalidated; API validates all values and enforces scoped roles. | Security integration tests/review |

## 7. Acceptance boundaries

Acceptance requires demonstrable working UI → API → persistent DB, meaningful negative and boundary behavior, correct authorization, tests tied to AC IDs and reproducible developer setup. Screenshots or a front-end-only demonstration are **not** sufficient.

## 8. Open issues and stakeholder questions

- Does TA approve `COMPLETED = party seated` semantics, given the stakeholder's wording 'serving/completing'?
- Does the restaurant need strict FIFO even when table capacity would force a delay, or may the first fitting party be selected?
- Does the business date reset at midnight or restaurant opening hour?
- Are host permissions assigned per queue or per restaurant? (Proposed: per queue.)
- v2 uses party size up to 16 independently of the largest single table; confirm restaurant/TA acceptance of that operating limit.
- Is Sprint 2 table assignment acceptable within Topic 9? (It is a queue-specific extension, but confirm decisions if needed.)

All answers must be documented in `13-decisions.md`. Do not present draft assumptions as stakeholder-approved facts.
