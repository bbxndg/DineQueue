# 06. Restaurant Operations — Domain Design v2

**Status:** proposed engineering decisions; not implemented/tested. **Topic:** CPE 334 theme 9 (queue and service management). **Revision:** 2026-11-05. This document elaborates the team's chosen domain; the course does not prescribe any of these specific table rules.

## 1. Domain purpose and boundaries

DineQueue solves **walk-in restaurant waiting, calling, seating and table turnover** for a single restaurant. It is not reservation booking, an order-taking POS, a payment application, staff shift manager or automated interior-layout planner. A customer receives a numbered ticket for a party of **1–16** people. An **individual table has 1–8 seats**, so a party of 10 may require a physically valid combination, e.g. table T4 with 6 and adjacent table T5 with 4 seats.

### Roles and scope

- `CUSTOMER`: authenticated ticket join/view/cancel (own tickets only), no floor mutation; seed accounts in S1.
- `HOST`: only assigned restaurant/queue; view operational queue/floor, call, hold, seat, release/clean, mark no-show, limited summary. Every action uses server-side checks.
- `MANAGER` (optional S3): manage floor zones, table metadata and combinability; no default access to private account details beyond the specified operating need. If omitted, seed configuration and review it in PRs instead.

## 2. Example layout, capacity and connected pairs

| Zone | Table | Seats | Approved connections | Initial status |
|---|---|---:|---|---|
| Main | T1 | 2 | T2 | AVAILABLE |
| Main | T2 | 4 | T1, T3 | AVAILABLE |
| Main | T3 | 8 | T2 | AVAILABLE |
| Terrace | T4 | 6 | T5 | AVAILABLE |
| Terrace | T5 | 4 | T4, T6 | AVAILABLE |
| Terrace | T6 | 2 | T5 | AVAILABLE |
| Private | T7 | 8 | none | AVAILABLE |

**Essential 10-person demo:** 10 guests; T4(6)+T5(4) connected, both AVAILABLE and in Terrace → valid. T7(8) alone → insufficient; T3(8)+T5(4) → invalid (different zones, no approved edge). If T5 is HELD or DIRTY, T4+T5 cannot be claimed. Table arrangement is a manager-approved **graph**, never a rule derived from X/Y pixel distance alone.

### Combination algorithm

For a party size `p`, enumerate **single tables** plus connected combinations of **2–3** distinct tables. Reject groups with:

1. any table from different restaurants or floor zones;
2. any table with status other than `AVAILABLE`;
3. an absent/inactive combinability edge or a disconnected induced subgraph;
4. total usable capacity below `p`;
5. invalid edge or table data, size greater than 3, or already held/occupied resources.

A combination is connected if every member table is reachable from every other through **selected** approved edges; 3-table chains are acceptable if approved. Adjacency is symmetric and stored once as ordered `(table_low_id, table_high_id)`.

Candidate ranking (team proposal): **fewest tables → smallest unused capacity → stable lexicographic sorted table IDs**. The guest is chosen using the **oldest eligible WAITING ticket** for the selected set; if FIFO head cannot fit, record explicit `CAPACITY_NOT_ELIGIBLE`, not a secret discretionary skip. Matching is deterministic and explainable; it is not AI or medical-style priority scoring. If multiple sets and parties are eligible, keep a documented and deterministic ordering; do not imply mathematically globally optimal allocation.

**Policy issue for TA:** whether skipping earlier parties for feasible seating is acceptable, max joins/party size, table-moving rules, and how staff override works. Approval required before claiming those decisions as project facts.

## 3. State machines and invariants

### Queue state

| From | Event | To | Preconditions | Sprint |
|---|---|---|---|---|
| — | Join | `WAITING` | signed-in Customer, queue active, size 1–16, no active duplicate | 1 |
| `WAITING` | Call next (S1) | `CALLED` | authorized Host, strict FIFO, no concurrent CALLED ticket | 1 |
| `CALLED` | Complete manually (S1) | `COMPLETED` | authorized Host; means waitlist handled, not meal completed | 1 |
| `WAITING` | Cancel | `CANCELLED` | ticket owner; cancellation window policy | 2 |
| `WAITING` | Match + Call + Hold | `CALLED` | authorized Host, qualifying table set, no other active CALLED in queue | 2 |
| `CALLED` | Seat confirmed | `COMPLETED` | active HELD assignment of same group; all selected tables moved to OCCUPIED | 2 |
| `CALLED` | Mark no-show | `NO_SHOW` | authorized Host; release entire held set | 2 |

No transition out of `COMPLETED`, `CANCELLED`, `NO_SHOW`. No user-provided status or ticket number is authoritative. The S1 `Complete` endpoint's meaning is **revised in S2**; contract/test revisions must precede code changes. For S2, disable bare `complete` without a table assignment (or make it a guarded alias for Seat), documenting compatibility explicitly.

### Table state

`AVAILABLE → HELD → OCCUPIED → DIRTY → CLEANING → AVAILABLE`

- `AVAILABLE`: no active/held assignment; eligible for recommendations.
- `HELD`: reserved for a *single* CALLED group; hold contains server expiry and owner.
- `OCCUPIED`: seated party is currently using resource; may be one member of a combined group.
- `DIRTY`: guest departed but table not ready; cannot be allocated.
- `CLEANING`: staff cleaning; cannot be allocated.
- `OUT_OF_SERVICE`: administrative exception permitted only from allowed states; not part of normal cycle.

The `seating_assignments` record moves `HELD → ACTIVE → CLOSED` or `HELD → RELEASED` (expiry/no-show). **Do not infer table occupancy solely from a queue entry**: `COMPLETED` means the waiting journey ended when seated, while the table remains OCCUPIED until checkout.

### Invariants

1. One active ticket/customer/queue; ticket number unique per (queue, local service date, sequence).
2. A `HELD` or `ACTIVE` seating assignment may claim 1–3 tables; **no table belongs to two open assignments**.
3. All assignment members are same restaurant/zone, connected, usable combined capacity ≥ party size.
4. `CALLED` + held assignment are mutually consistent (S2); `COMPLETED` + active assignment are mutually consistent at seating time.
5. Every transition and actor is persisted and auditable; all timestamps issued server-side, UTC storage with restaurant-local service-day semantics.
6. No incomplete group allocation: transaction commits all table rows, mapping rows, assignment status, and queue transition together or none.
7. Checkout of a combined group releases **all** its tables into DIRTY atomically; cleaning availability remains separate staff-confirmed action.

## 4. Critical transactional workflows

### A. Customer joins with 10 guests (S1)

1. Client submits `{partySize:10}` to an authenticated queue.
2. Backend validates integer 1–16, queue open and no active ticket.
3. PostgreSQL transaction allocates next daily sequence and inserts `WAITING` ticket with race-safe partial unique index.
4. Client receives number `A-014`, status WAITING and size 10. **No table match is required in Sprint 1.**

### B. Host previews matching (S2)

1. UI displays zone/floor, waiting parties, table statuses and explanatory candidate groups.
2. Read endpoint computes deterministic candidates using snapshot, approved graph and rules.
3. Preview is **non-binding**, never claims capacity. Stale previews must be revalidated on commit.
4. A 10-person group may show `T4 + T5 · 10 seats · 0 unused`, with `2 tables combined` warning and staff physical confirmation.

### C. Host calls and holds a table set (S2)

1. `POST /host/queues/{queueId}/match-and-call` with opaque ticket ID, sorted `tableIds`, `layoutVersion`/optional `expectedVersion`, idempotency key.
2. Backend checks scoped permission, candidate eligibility and no outstanding CALLED ticket; locks the queue scope and selected table rows in a deterministic order; validates all rows are AVAILABLE and approved/connected.
3. Transaction inserts HELD assignment + member mappings, changes **all** selected table statuses to HELD, updates ticket to CALLED and appends an event. If any step fails, roll back all changes.
4. Return explicit ticket, hold deadline, selected table IDs, authoritative updated versions. Other Hosts see updated HELD states after refresh.
5. On conflict (409), UI discards preview and reloads. Do not retry blindly with stale IDs.

**Lock order policy:** queue scope → queue entry → selected tables sorted by stable ID → assignment/mapping inserts. Every write path uses same ordering. Transaction-level PostgreSQL row locks and unique constraints protect against concurrent claim; code must catch deadlock/serialization conflicts and retry only a bounded number of safe attempts where idempotency is defined.

### D. Seat held party (S2)

1. Host confirms the group arrived and tables are physically ready.
2. Transaction locks relevant scope/assignment/member tables, verifies CALLED, HELD, not expired, and all member tables still HELD for that assignment.
3. Update all tables to OCCUPIED; assignment HELD → ACTIVE, queue CALLED → COMPLETED, persist timestamps/events atomically.
4. Reject repeated/expired/unauthorized seats with 409/403 and leave data unchanged.

### E. No-show, expiry, checkout and cleaning (S2)

- **No-show:** only CALLED; release the entire HELD group → AVAILABLE, assignment → RELEASED, ticket → NO_SHOW, append event in one transaction.
- **Expiry:** server-controlled expiry check inside each relevant operation, optionally a scheduled sweeper with idempotent cleanup. Expiry must not race with seat; only one winning transaction. On expiry, define whether the ticket becomes NO_SHOW or requires explicit staff decision (**TA/team decision**).
- **Checkout:** after guests leave, mark assignment CLOSED, all member tables DIRTY and create turnover event atomically. Ticket stays COMPLETED.
- **Cleaning:** Host sets all group members DIRTY → CLEANING → AVAILABLE in group-level transactions, records actor/time; an OCCUPIED or HELD table can never be directly marked AVAILABLE.

## 5. Fairness and operational exceptions

- S1: strict FIFO, no skipping.
- S2: oldest fitting waiting group **for a chosen valid connected table set**; show `skipped 6-person group: selected table has only 4 seats` only in staff UI. Do not share customer identities in public read models.
- Manual selection outside recommendation requires documented permission, reason and event. It cannot bypass graph, capacity, state or ownership constraints.
- Capacity-efficiency vs fairness tension: ranked alternatives are suggestions; preserve reason and human confirmation. Do not misrepresent a heuristic as an optimum.
- Do not accept special seating requests as executable logic unless independently specified and tested. Keep free-text notes private and length-limited.

## 6. Customer and host experience

**Customer mobile:** view current waitlist, choose party size 1–16, review before joining, ticket with status/position, cancel waiting ticket in S2, history in S3. For party size >8, explicitly say “We may seat your group at adjacent tables; staff will confirm availability.” No guaranteed time, no promise that two tables can always be joined.

**Host workspace:** left queue rail, center active call/selected party, right or tabbed floor zone with table cards. Actions include Recommend, Call & Hold, Confirm Seating, No-show, Checkout, Start Cleaning, Mark Available. Buttons reflect status but every server mutation still checks authorization and the latest persisted state. Use server-provided rationale and conflict recovery, never silent optimistic allocation.

**Manager/reports:** configure zones/edges (S3 optional) and compare real waitlist/turnover metrics. Show metric definition, window, count of records and exclusions. No made-up ETA or “live” claims without a real refresh source.

## 7. Example acceptance walkthrough — 10 guests, 2 tables

1. Seed T4(6) ↔ T5(4) in Terrace. Seed nonconnected T7(8).
2. Customer C1 joins party of 10 and gets a ticket; C1 cannot join again while active.
3. Host sees recommendation `T4+T5` and selects Call & Hold.
4. Confirm **both** change AVAILABLE→HELD; another Host attempting either table receives conflict and no partial hold.
5. Confirm Seat: C1 queue becomes COMPLETED; T4/T5 OCCUPIED; one ACTIVE group assignment with two membership records.
6. Checkout: T4/T5 DIRTY and assignment CLOSED; neither may be allocated.
7. Start Cleaning → Mark Available: only then may either table be used again.
8. Negative: T4 + T7 rejected (not same zone); T7 alone rejected (8 < 10); T4 + already HELD T5 rejected; failure injection proves rollback.

## 8. Nonfunctional/security boundaries

- All mutations must enforce authentication, queue/restaurant scope, ownership and CSRF for cookie-based browser requests.
- Do not trust client-supplied capacity, adjacency, table state, timestamps, ranking, role, ticket number or price-like values.
- Prefer strict JSON schemas and field allowlists, parameterized GORM queries / explicit SQL (DEC-026), explicit transactions, minimal error details, limited request body sizes and rate limits on login/join.
- Public display only exposes ticket code + generic status, never customer details or staff-only notes.
- No CDNs or third-party scripts necessary for product execution; pin versions and review dependencies before use. No tool can guarantee an app will never be hacked: perform tests, review and deployment security checks.

## 9. Required decisions before accepting v2 baseline

- [ ] Team and TA agree the queue-first scope remains Topic 9.
- [ ] Team confirms party max 16, 2–3 joined tables, and per-table max 8.
- [ ] Restaurant staff/TA confirm which physical table pairs can actually be moved/combined.
- [ ] Team/TA approves oldest-eligible skip behavior and override audit requirements.
- [ ] Team agrees server hold expiry semantics and service-day/timezone handling.
- [ ] Sprint 2 feature contracts and STS are committed **before** corresponding implementation.
