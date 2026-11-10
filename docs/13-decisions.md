# 13. Decision Register / ADRs / Open Stakeholder Questions

**Instruction basis:** The PDF says themes are stakeholder briefs, NOT finished SRS. Identify ambiguities, resolve with instructor/TA when necessary and record decisions. This file explicitly separates **proposed** decisions from confirmed ones.

## Proposed architecture / domain decisions

| ID | Decision | Rationale | Status / owner |
|---|---|---|---|
| DEC-001 | Restaurant walk-in waitlist, not advance reservations | Keeps Topic 9 queue central | PROPOSED / team |
| DEC-002 | S1 one seeded restaurant and queue | Feasible working end-to-end slice | PROPOSED / team |
| DEC-003 | Roles CUSTOMER and HOST; Host scoped by queue | Meets two-role/backend-authorization rule | PROPOSED / team |
| DEC-004 | Sprint1 FIFO by server join time/id | Deterministic and testable | PROPOSED / team |
| DEC-005 | S1 at most one CALLED in queue | Avoid double calling and simplifies station behavior | PROPOSED / team |
| DEC-006 | Local service date in Asia/Bangkok; per-queue daily number A-001 | Clear human number and uniqueness scope | PROPOSED / team |
| DEC-007 | Party size inclusive 1–16; individual table max 8 | Allows 10-person party via adjacent table combination | PROPOSED / team |
| DEC-008 | S1 COMPLETED = waitlist processed / manually seated | Fits queue domain without table subsystem S1 | PROPOSED / clarify semantics |
| DEC-009 | S2 select oldest fitting party for a given table | Makes restaurant seating feasible but changes strict FIFO | PENDING TA clarification |
| DEC-010 | Once table feature active, completing queue requires valid assignment | Eliminates ghost completion in final product | PROPOSED / version rule |
| DEC-011 | Next.js + Go Fiber + PostgreSQL, modular monolith | Go backend practice, pgx/sqlc transaction control; team must confirm toolchain | PROPOSED / team |
| DEC-012 | Opaque DB-backed HttpOnly sessions with server logout | Secure simple role-session design | PROPOSED / team |
| DEC-013 | Sprint 3 non-private public display only if time allows | Core queue + tests first | PROPOSED / team |
| DEC-014 | No real payment, SMS, external API | Out of course scope | SUPPORTED by master PDF |

## Required stakeholder clarifications

| Question | Proposed default | Why it matters | Follow-up |
|---|---|---|---|
| Q-01: What is 'service complete' in restaurant context? | Party successfully seated, not meal complete | status semantics | Ask TA/instructor if contested |
| Q-02: Can a 2-person group be served before 6-person group when only 2-seat table frees up? | Earliest **eligible** party | priority/fairness + acceptance tests | Record explicit policy approval |
| Q-03: Does a restaurant business day reset at midnight? | Local midnight Asia/Bangkok | ticket number uniqueness | Confirm and update BR-002 |
| Q-04: Are no-shows terminal, can they rejoin? | Terminal; may rejoin as new ticket | duplicate active constraint | Resolve in S2 contract |
| Q-05: Can a host serve several queues? | Only explicitly granted queues | access matrix | Confirm if adding F-10 |
| Q-06: Is DineQueue eligible as assigned theme for the 2-person team? | Must match last digit of one teammate ID | course eligibility | Verify using actual team IDs |
| Q-07: Are course Lab5 cloud resources known yet? | Not until Lab5 instructions | deployment choice | Follow handout |

## ADR template

**ADR-XXX: Title**
- Date: `[fill]`
- Context / ambiguity: `[fill]`
- Options: `[fill]`
- Decision and rationale: `[fill]`
- Authorized by / consulted TA: `[fill]`
- Consequences for SRS/BR/SDS/AC/STS/tests: `[fill]`
- Related Issue/PR: `[fill]`

## Change control

Decisions are append-only in the versioned repo. If DEC-009 or any BR changes, preserve previous version and show rationale; do not silently rewrite specs after implementation and pretend they predated code.


## v2 decision proposals (5 Nov 2026; pending team/TA sign-off)

| Decision | Proposed choice | Why | Status |
|---|---|---|---|
| DEC-015 | Every physical table capacity <=8; party size <=16 | Allows concrete 10-person adjacent-table case | PROPOSED |
| DEC-016 | Combined table sets up to 3 distinct physically connected tables in one floor zone | Limits combinatorial search while permitting larger groups | PROPOSED; confirm with staff/TA |
| DEC-017 | Explicit manager-approved `table_connections`; visual proximity alone is insufficient | Prevents impossible physical combinations and keeps matching testable | PROPOSED |
| DEC-018 | `AVAILABLE → HELD → OCCUPIED → DIRTY → CLEANING → AVAILABLE` | Captures call/seat/checkout/cleaning as real resource workflow | PROPOSED |
| DEC-019 | Hold time 5 minutes by default, server-controlled expiry | Prevent indefinite reservation; policy adjustable | PROPOSED; TA confirmation |
| DEC-020 | Oldest eligible party on selected table set; exact FIFO skipped only when capacity impossible | Fairness + feasibility trade-off | PENDING TA decision |
| DEC-021 | Rank candidate connected sets by minimum table count, then unused seats, then stable IDs | Deterministic explanation and tests, not opaque 'AI matching' | PROPOSED |
| DEC-022 | Release *whole* combined group into DIRTY, then CLEANING → AVAILABLE | No partially freed combined set in MVP | PROPOSED |
| DEC-023 | Switching from NestJS/Prisma proposal to Go Fiber v3 + pgx/sqlc | Last user tech exploration; not a mandated technology | PROPOSED until 2 developers agree |
| DEC-024 | Operate one restaurant and floor with seeded zones and table edges; layout-editing UI deferred to S3 | Keep S2 implementable | PROPOSED |
| DEC-025 | No POS, payments, future reservations, ML forecasts, or personal details on public display | Topic 9 and privacy/scope | PROPOSED |

**Critical open questions:** maximum group size (16?), number of joined tables (3?), whether all connected tables can be physically pushed together, which tables cannot be combined (fire exits/accessibility), acceptable skip policy, hold expiry behavior, whether manual override requires a Manager role, and whether the course approves the broader S2 scope. Never infer physical joinability from X/Y coordinates alone.

## Foundation implementation scope — 6 November 2026

The user authorized Sprint 1 repository foundation in chat: Next.js/TypeScript/Tailwind,
Go Fiber, PostgreSQL, modular monolith, Docker-ready local development and environment configuration.
When asked about database access the user requested a recommendation. The implementation agent
recommended retaining pgx/sqlc from the current documents and proceeds on that basis; GORM was
discussed but has not been approved as a replacement. This records the implementation assumption,
not team/TA sign-off on DEC-023 or all domain decisions. See `features/sprint1-foundation-contract.md`.

## DEC-026 — GORM database access (6 November 2026)

- Authorization: the user explicitly requested switching to an ORM in chat.
- Decision: use Go Fiber + GORM + the GORM PostgreSQL driver. This supersedes the
  pgx/sqlc access-layer proposal in DEC-011/DEC-023 and the original foundation assumption above.
- Scope: replace foundation database wiring only; F-01 and all queue/seating features remain unimplemented.
- Keep explicit versioned SQL migrations, PostgreSQL constraints and deterministic row locks.
  Do not call AutoMigrate during application startup.
- Services coordinate one GORM transaction and pass its `tx` to every participating repository.
  Per-write default ORM transactions do not replace the business transaction.
- Keep DTOs/domain objects separate from persistence models; parameterize values and
  allowlist dynamic SQL identifiers. Avoid unconditional Save for state transitions;
  use explicit conditional updates and check affected rows, including deliberate zero-value updates.
- Preserve lazy startup, bounded readiness, generic errors, finite pooling and shutdown cleanup.
  Disable ORM SQL logging in the foundation so connection details/parameters are not exposed.
- pgx may remain as an indirect dependency of the official PostgreSQL driver; the app
  must not retain a separate pgx pool, sqlc generator or generated-query layer.
- Team/TA approval of unrelated domain rules is not implied. Original test evidence and
  earlier dependency proposals remain historical records, not GORM verification results.

## DEC-027 — F-01 implementation details (6 November 2026)

- Authorization: user requested implementation of F-01 on its feature branch.
- Keep the existing 12-hour session proposal for this implementation: random 32-byte
  cookie, SHA-256 hash in sessions, active account/role checked from DB on each request.
- Argon2id v19 (64 MiB, t=3, p=2), 16-byte random salt; no hardcoded seed credentials.
- Exact APP_ORIGIN on login and unsafe authenticated requests, per-session HMAC-SHA256
  CSRF token on authenticated mutations. This brings NFR-015 initial verification into
  S1; S2/S3 retain regression. No global SESSION_SECRET is needed by this design.
- Explicit embedded SQL migrations via cmd/db, transaction/advisory lock/checksum;
  opt-in demo seed disabled in production, preserving existing accounts/passwords/grants.
- A local per-process login limiter and two concurrent password-verification slots
  bound resource use. Shared proxy/rate-limit policy and expired-session retention
  are deployment decisions before multi-instance operation.
- Scope: authentication UI/API plus reusable guards; no queue/ticket operations.
  Guard tests do not establish actual endpoint privacy before F-02..04 are integrated.
- This records implementation choices under user authorization. No teammate/TA
  approval, pre-implementation addendum commit, PR or CI is asserted.

## DEC-028 — Four Sprint 1 parent features (6 November 2026)

- Authorization: user requested updating repository documents to four S1 features.
- Current grouping: F-01 Authentication/Session/RBAC; F-02 Join Waitlist and Ticket
  Issuance; F-03 My Ticket and Privacy; F-04 Host Queue Operations (board, strict
  FIFO Call Next and Complete).
- F-05 is retained as the legacy ID for Complete inside F-04, without a fifth
  parent feature or separate feature branch. F-06..F-19 are not renumbered.
- F-04 includes FR-006/007/008/009, BR-003/004/005/006 and all AC-014..021 with
  their original test IDs, including completion T-028..033. Other scopes and IDs
  are retained. No requirement, negative/concurrency check or DoD item is removed.
- This changes feature organization only. S1 still uses manual waitlist completion;
  the API/state model and S2 seating preconditions retain their versioned rules.
- Contracts, inventory, sprint/backlog, traceability, workflow, release template,
  README and AGENTS reflect the grouping. Historical execution records keep their
  original context, with a later grouping note where needed.
- No new implementation, test execution, teammate/TA approval, commit or release
  is implied by this documentation change.

## DEC-029 — F-02 implementation details (6 November 2026) — pending team sign-off

- Authorization: user requested implementation of F-02 on its feature branch.
- Unknown/malformed queue ID → 404 `NOT_FOUND`; inactive queue → 409 `QUEUE_INACTIVE`
  (T-016 allowed either; this fixes the choice).
- Party size must be a plain JSON integer literal 1..16; `4.0`, `1e1`, strings and
  unknown fields are 400. Client-supplied number/status/date fields are rejected.
- Counter allocated by `INSERT … ON CONFLICT DO UPDATE … RETURNING` inside the join
  transaction; a rejected duplicate rolls back and leaves no number gap.
- Ticket code is derived (`prefix-%03d`), not stored. `GET /api/v1/queues` returns
  active queues to any authenticated role.
- `/join` shows the created ticket in place until F-03 adds `/my-queue`.

## DEC-030 — F-03 read and UI details (10 November 2026) — pending team sign-off

- User authorized F-03 implementation on its existing branch. Follow the committed
  F-03 scope and `features/f03-my-ticket-implementation-contract.md` addendum.
- My Ticket endpoints are CUSTOMER-only (Host 403); detail queries include owner
  ID and return generic 404 for foreign, absent or malformed ticket IDs.
- Active reads include WAITING/CALLED across service dates and inactive queues;
  COMPLETED remains available by owned detail URL without a history-list feature.
- Owned read summaries add queue/restaurant names and restaurant timezone; dates
  use YYYY-MM-DD. Manual refresh loads persisted status; no ETA or polling.
- Join now redirects to `/my-queue`; no F-04/S2 mutations are introduced.
