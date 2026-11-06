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
