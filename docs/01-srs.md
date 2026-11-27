# 01. Software Requirements Specification (SRS)

> Sprint 2 review draft for [#49](https://github.com/bbxndg/DineQueue/issues/49): [feature contract](features/sprint2-restaurant-operations-contracts.md), [UI/API/data](features/sprint2-ui-api-data.md), [STS](testing/sprint2-operations-sts.md) and [traceability](testing/sprint2-traceability.md).
> The source baseline b4d1606 includes Lab 7 readiness/polling. Its [reviewed change contract](labs/lab-07/readiness-contract.md) remains the maintenance history; the S2 rules below are not implementation or a new test result.

**Document:** DQ-SRS-2.1 (Sprint 2 review draft; decision/review status is recorded in DEC-034–038)
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

Sprint 1 has four parent features (DEC-028): F-01 Authentication/Session/RBAC,
F-02 Join Waitlist and Ticket Issuance, F-03 My Ticket and Privacy, and F-04 Host
Queue Operations (board, FIFO Call Next, Complete). F-05 remains the legacy ID
for completion within F-04. Existing requirements, rules and acceptance scope are retained.

One seeded restaurant with one seeded service queue; predefined customer and host accounts; login, logout, current user; customer joins, retrieves and sees status of own ticket; host sees queue, calls FIFO next, completes ticket; unique ticket creation under concurrency; duplicate active-ticket rejection; state and date validation; authorization and persistence; automated evidence; release on `main` with setup guide.

**Party size** is stored from Sprint 1 (integer 1–16 under the v2 BR-008 revision) but capacity matching starts in Sprint 2. The separate physical-table capacity limit is 1–8. Sprint 1 does **not** imply table matching, reservation calendar or public notifications.

### 3.2 Scope for later sprints

Sprint 2: dining table inventory, host table status, party-to-table assignment, cancellation, no-show, queue eligibility policy. Sprint 3: history and summary metrics, restricted public display (if retained), robustness and usability, regression and UAT fixes.

### 3.3 Assumptions requiring confirmation

- Pilot uses one restaurant, one walk-in service queue, and a single operating timezone `Asia/Bangkok`.
- For the maintained Sprint 1 baseline, ordering is FIFO among Ready WAITING parties by original server-recorded `joined_at` and id; the pre-Lab 7 strict-WAITING rule remains in version history.
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
| FR-012 | Must | Host can view the floor and select a valid available table set, call/hold the oldest fitting Ready party, then seat that CALLED party from its HELD assignment. | S2 |
| FR-013 | Must | Allocation/hold/occupancy of **every table** in an assignment is exclusive and recorded as one atomic transaction. | S2 |
| FR-014 | Must | Customer can cancel own eligible waiting entry; canceled ticket loses active status. | S2 |
| FR-015 | Must | Host can mark called entry NO_SHOW, with controlled permissions. | S2 |
| FR-016 | Must | Host can check out a seated party, mark all assigned tables DIRTY, then confirm CLEANING → AVAILABLE; release must apply to the entire combined set. | S2 |
| FR-017 | Should | Host can manage multiple counters/queues if approved, while scoping access to the assigned queue. | S2/S3 |
| FR-018 | Must | Authorized staff can inspect historical queue events, seating/turnover, counts, measured wait times and utilization metrics with date/zone filters. | S3 |
| FR-019 | Should | Public display can show current ticket number and limited non-private status without any user details. | S3 |
| FR-020 | Must | Deployed app uses documented configuration, guarded CI and repeatable delivery/release procedures. | Lab5+ |
| FR-021 | Must | Team records independent UAT, defects, fixes and re-test evidence. | Lab6 |
| FR-022 | Must | Team implements a reviewed maintenance change with impact analysis and regression; the instructor announcement of 25 Nov removes TA approval for Lab 7 change requests. | Lab7 |


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
| FR-030 | Should | Host sees why a table set is recommended and may select another valid set with an audited selection reason; the oldest fitting Ready party for the final set cannot be overridden. | S2/S3 |
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
| BR-003 | Maintained S1 uses BR-003-L7-v1: the oldest `WAITING AND is_ready=true` by `(joined_at ASC, id ASC)`; returning Ready preserves original order and cannot displace CALLED. | S1 |
| BR-004 | Valid Sprint 1 transitions: `WAITING -> CALLED -> COMPLETED` only. `WAITING -> COMPLETED` rejected. | S1 |
| BR-005 | No new `Call Next` while the same service queue already has a `CALLED` ticket; rejection `CALL_IN_PROGRESS`. | S1 |
| BR-006 | Only authorized Host can call/complete tickets in the assigned queue. Customers cannot do staff transitions. | S1 |
| BR-007 | Customers can read only their own tickets; use non-revealing `404` for inaccessible foreign tickets. | S1 |
| BR-008 | BR-008-S2-v1 retains the implemented integer boundary [1,16]; physical table capacity remains 1–8. Join is allowed without a current fitting table; S2 does not revise this boundary again. | S1 |
| BR-009 | Ticket dates/numbers come from the backend in restaurant-local timezone; never trust a supplied sequence/status. | S1 |
| BR-010 | BR-010-S2-v1: Call & Hold requires 1–3 distinct AVAILABLE, active approved connected tables in one restaurant/zone with enough capacity for the selected eligible party. Seat requires that same CALLED party and its unexpired HELD group; all changes are atomic. | S2 |
| BR-011 | BR-011-S2-v1: Host selects the table set first; choose the oldest Ready WAITING party who fits by (joined_at,id). Earlier Not-ready or oversized parties retain order and receive a Host-visible skip reason, even if another free set elsewhere could fit them. User selected this policy on 27 Nov; teammate contract review remains pending. | S2 |
| BR-012 | Only WAITING may be customer-canceled; only CALLED may be marked NO_SHOW; terminal statuses cannot be reactivated. | S2 |
| BR-013 | BR-013-S2-v1: in explicitly activated SEATING queues, completion requires the Seat transaction with an ACTIVE assignment and all members OCCUPIED. MANUAL queues retain Lab 7 manual completion; historical completed rows without assignments remain valid. COMPLETED never means meal completion. | S2 |
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
| BR-021 | BR-021-S2-v1: a hold expires at server time >= its five-minute deadline. Cleanup atomically records NO_SHOW/HOLD_EXPIRED and releases every member; never seat an expired hold. Cleanup timing and its separate transaction are defined in the S2 contract; browser countdowns are not authoritative. | S2 |
| BR-022 | BR-022-S2-v1: whole-group checkout sets every member DIRTY; authorized staff confirm whole-group CLEANING then AVAILABLE. Keep claims until cleaning finishes; old assignment actions cannot affect a later group. | S2 |
| BR-023 | BR-023-S2-v1: alternate valid table-set selection records actor, selection reason, original suggestion and final set. No arbitrary party-order override; all topology/capacity/state/readiness/scope rules still apply. | S2/S3 |
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
- Selected-set-first oldest fitting Ready policy was chosen by the user on 27 Nov (DEC-035); teammate engineering-contract review is still required.
- Does the business date reset at midnight or restaurant opening hour?
- Are host permissions assigned per queue or per restaurant? (Proposed: per queue.)
- v2 uses party size up to 16 independently of the largest single table; confirm restaurant/TA acceptance of that operating limit.
- Is Sprint 2 table assignment acceptable within Topic 9? (It is a queue-specific extension, but confirm decisions if needed.)

All answers must be documented in `13-decisions.md`. Do not present draft assumptions as stakeholder-approved facts.

## Sprint 3 proposed requirement refinement — DQ-S3-1.0 (#55)

The [Sprint 3 contracts](features/sprint3-contracts.md) and [interface definitions](features/sprint3-ui-api-data.md) refine remaining requirements before implementation. Existing FR/BR IDs remain; these proposals await teammate review under DEC-041–044.

| Existing requirement / feature | Proposed refinement and retained boundary |
|---|---|
| FR-010 / F-03 and customer extension of F-11 | CUSTOMER can list only their own COMPLETED/CANCELLED/NO_SHOW tickets with bounded stable pagination and owned detail. This explicitly adds Customer history; FR-018 still covers staff history. |
| FR-017 / F-10 | Additional queue uses explicit grants and independent numbering; shared restaurant floor claims remain exclusive. |
| FR-018/032, BR-014 / F-11/F-18 | Scoped staff event history and measured counts/durations; configuration coverage and shared-floor limits determine whether utilization can be calculated. Null results explain missing evidence. |
| FR-019, NFR-010/013 / F-12 | Explicitly enabled public slug exposes only called code/status, with no private identifiers. |
| FR-031, BR-024 / F-17 | MANUAL Ready position uses eligible joined_at/id order; SEATING and non-WAITING positions are unavailable with reason. Numeric ETA remains unavailable until an independently reviewed model is supported by data. |
| FR-033-S3-v1 / F-19 | Proposed actor revision: HOST with explicit restaurant configuration grant, rather than a new Manager role. Safe versioned web floor setup and separate seating activation; queue grants alone are insufficient. |
| NFR-006/007/009 / F-13 | Accessible states, meaningful linked tests and measured p95 reads under 20 documented local concurrent users; actual execution/review/release required. |

FR-033's earlier Manager wording is retained above as the prior design; the proposed S3 actor refinement must be reviewed before coding. No matching/lifecycle BR is changed by this checkpoint. The [S3 traceability matrix](testing/sprint3-traceability.md) records missing ACs, dependencies, accountable owners and dated targets.
