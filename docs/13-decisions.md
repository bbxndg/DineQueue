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

## DEC-031 — F-04 Host queue operation details (11 November 2026) — pending team sign-off

- User authorized F-04 implementation on `feat/F-04-host-queue-operations`. Follow the
  committed F-04 scope and `features/f04-host-queue-implementation-contract.md`.
- Added `GET /api/v1/host/queues` (assigned queues only) so the board can find its
  queue; the documented entries/current/call-next/complete routes are unchanged.
- Customer → 403; Host without a grant → 403, including an unknown well-formed queue
  UUID (no existence oracle); malformed queue/entry ID and an entry from another
  queue → 404. Host mutations take no body (non-empty → 400).
- Call Next and Complete lock the queue row `FOR UPDATE` first, then recheck the grant
  in the same transaction; transitions use conditional `WHERE status=...` updates
  with one-affected-row checks. A `uq_one_called_per_queue` violation maps to 409
  `CALL_IN_PROGRESS`. Any granted Host may complete a ticket another Host called.
- Host operations stay allowed on an inactive queue so existing tickets can drain.
- Board shows ticket code, party size and times only; Complete needs an on-screen
  confirmation. Manual refresh; no polling, no S2 table assignment.

## DEC-032 — EP5 Cloud Run topology and managed database transport (18 November 2026)

- User approved one Cloud Run service with separate Next.js ingress and Go API
  sidecar images, managed PostgreSQL via Cloud SQL, Artifact Registry and Secret
  Manager. This replaces the earlier EP2 proposal of separate web/API services.
  Containers communicate on localhost; only the web container exposes ingress.
- Reuse reviewed production image targets. Add a database-tools image for separate
  migrate/provision jobs. No startup migration/AutoMigrate and no Sprint 2 rules.
- Keep production HTTPS and verified TLS for ordinary TCP. Add explicit opt-in
  Cloud SQL connection-name configuration for a canonical managed Unix socket;
  no arbitrary TCP `sslmode=disable` or environment downgrade is permitted.
- Keep production `seed` disabled. Add a separately guarded provisioning command
  for the dedicated demo/UAT database with exact database-name confirmation and
  environment-supplied passwords. Existing accounts, passwords, active flags and
  revoked grants are preserved; identity conflicts roll back the transaction.
- Only the API container receives the database URL; runtime and job identities
  receive per-secret permissions. Account credentials are supplied only to the
  explicit provisioning job, not the service or migration job.
- Preparation is authorized; live billable resource creation and public exposure
  require the concrete dry-run plan to be reviewed before execution. No live
  deployment, TA approval, cost estimate or test PASS is implied here.
- Contract and tests: `labs/lab-05/ep5-contract.md`; execution results follow actual
  tests. EP6 identity/required gates and EP7 automation remain separate.


## DEC-033 — Lab 7 readiness-aware calling (#44), 25 November 2026

Status: **Contract reviewed on PR #45; readiness local verification recorded; polling pending**. See the
[shared contract](labs/lab-07/readiness-contract.md) and
[STS](labs/lab-07/readiness-sts.md) before implementing either feature branch.

- The instructor announcement supplied by the user removes instructor/TA approval
  for Lab 7 change requests; this does not imply approval of unrelated domain rules.
- One change request combines owner readiness and automatic status updates.
- Proposed BR-003-L7-v1 calls the oldest ready WAITING ticket; original join order
  is retained, and an already CALLED party cannot be displaced.
- Proposed new-ticket default is Not ready; existing rows are backfilled Ready.
  softkoi approved this compatibility choice in PR #45 before implementation.
- Main source at planning: `1524502386065be2eb6025c04bae72a5398c7228`.
  Accepted Lab 6 baseline evidence is still to be confirmed separately.
- Actual readiness implementation/checks are recorded in `labs/lab-07/readiness-results.md`.
  Feature PR review, integrated release and deployment remain pending.


## DEC-034 — Sprint 2 contract baseline and integration process, 27 November 2026

Status: **PROPOSED engineering contract; teammate review NOT RECORDED**. Related issue [#49](https://github.com/bbxndg/DineQueue/issues/49); source main b4d160675695328c2a73d098a185c4be05022e02. User authorized the documentation branch and five implementation groups, not unreviewed application changes or passing-test claims.

The version 2.1 [contract](features/sprint2-restaurant-operations-contracts.md), [interfaces](features/sprint2-ui-api-data.md), [STS](testing/sprint2-operations-sts.md) and [traceability](testing/sprint2-traceability.md) supersede conflicting older S2 sketches. Keep original IDs and version BR-008/010/011/013 explicitly. Review and commit this design before RED/implementation. Feature branches start from updated sprint2-staging after dependencies merge. F-06..09/F-14..16 all remain in Sprint 2; separate S3 issues #55/#56 remain necessary for Deliverable 2.

## DEC-035 — Selected table set and readiness-aware FIFO, 27 November 2026

Status: **USER SELECTED; teammate engineering review pending; no TA approval claimed**. The user explicitly chose Host-selected table-set-first matching over global-floor oldest-feasible matching.

For the final valid set choose the oldest Ready WAITING party that fits, ordered by original joined_at/id. Earlier Not-ready or oversized parties keep their original position; a larger free table elsewhere does not forbid the capacity skip for the selected set. No arbitrary party override. Rank table suggestions by count, unused seats and stable IDs; a valid alternative set is audited and requires recomputing/reconfirming its selected party. Versions: BR-011-S2-v1 and BR-023-S2-v1; STS 018/019/036–038.

This resolves the team's selection default in DEC-009/020 for this draft, without inventing stakeholder/TA sign-off. It preserves Lab 7 readiness, original order and CALLED non-preemption.

## DEC-036 — Five-minute hold expiry and full release, 27 November 2026

Status: **USER SELECTED terminal outcome; processing mechanism proposed for review**. The user selected NO_SHOW and release of the whole set at expiry, rather than returning the ticket to WAITING.

At server time >= the stored deadline, Seat is forbidden. The common release coordinator records NO_SHOW/HOLD_EXPIRED, HELD→RELEASED and every member AVAILABLE atomically. Explicit expire-holds and pre-mutation housekeeping use this service; reads never write. Housekeeping is its own audited transaction and survives a later requested action's rejection. Do not promise execution at an exact wall-clock instant on idle Cloud Run; the next cleanup invocation completes the persisted transition. No new scheduled/cloud resource is required by this design. Version BR-021-S2-v1; STS 012/031/045/055.

## DEC-037 — Legacy queue compatibility and additive activation, 27 November 2026

Status: **PROPOSED for teammate review**. Preserve MANUAL default on existing queues and historical completed tickets without fabricated seating records. SEATING is explicitly activated after all lifecycle handlers integrate, with valid floor data and no legacy CALLED ticket. Readiness, counters and historical IDs/times survive migration/provisioning.

MANUAL retains existing Call Next/Complete tests. SEATING rejects bare legacy call/complete and requires selected-set call/hold/Seat; no bypass through old endpoints. Own cancellation/new terminal detail behavior is added through the reviewed S2 routes. Version BR-013-S2-v1; STS 042–044/052/058. Old application binaries that do not understand the new lifecycle are not a safe rollback after activation; preserve data and use a reviewed compatible version/forward fix.

## DEC-038 — Shared resource claims turnover and ownership, 27 November 2026

Status: **PROPOSED technical protocol; feature ownership explicitly deferred by the user**.

Queue-scoped Host routes reuse existing grants. Seating transactions lock restaurant floor-state before queue/entry/sorted tables/assignment, so two queues cannot claim overlapping tables. Existing readiness/cancel keep queue→entry and never acquire a later floor lock. Persist scoped request-key results with successful mutations.

Keep membership claims through CLOSED/DIRTY and CLOSED/CLEANING, releasing on finish-cleaning; an explicit CLEANED turnover phase stops an old group changing a later allocation. This replaces the older release-at-checkout sketch while preserving its historical design in Git. The interface contract defines the additive schema, strict DTOs, provisioning/activation and error semantics; STS 014/046–048/053–054 test these invariants.

Benjamin Garforth (bbxndg) and Kawinpop Churari (softkoi) will assign an implementation owner and independent reviewer before each feature starts. The user's response was to decide the split later; this draft does not assign work on their behalf. Record actual review PR/SHA/date before freezing this protocol; none is recorded yet.

## DEC-039 — F-08 cancellation/no-show details (#53), 27 November 2026 — pending team review

Status: **PROPOSED; implemented locally on `feat5/sprint2-cancellation-no-show`, uncommitted and not reviewed**.

- New migration `0006_cancel_event_without_assignment.sql` makes `operation_events.assignment_id` nullable only for a `CUSTOMER_CANCEL` WAITING→CANCELLED row with an actor (CHECK). Reason: S2-T-020 requires one owner event, a never-held ticket has no assignment, and 0005 declared `assignment_id NOT NULL`. 0001–0005 are unchanged. Alternative if rejected: record no event for cancellation and revise S2-T-020.
- Customer Cancel takes an empty body (non-empty → 400 VALIDATION_ERROR) and no idempotency key; a repeat is 409 INVALID_TRANSITION. It locks queue→entry and never the floor.
- Host No-show `expectedVersion` is the HELD assignment version. Request keys use the operation name `no-show`, hashed with the entry path. An entry without a HELD assignment (WAITING, seated/COMPLETED, manual CALLED) returns 409 INVALID_TRANSITION; a MANUAL queue returns SEATING_NOT_ENABLED; an already-expired hold returns HOLD_EXPIRED after the independent system cleanup.
- Expiry and no-show share one repository coordinator (`releaseHold`); only reason (HOLD_EXPIRED/CUSTOMER_ABSENT), actor (system/Host) and event action (EXPIRE_HOLD/NO_SHOW) differ.

## DEC-040 — F-09/F-16 turnover and activation details (#54), 27 November 2026 — pending team review

Status: **PROPOSED; implemented locally on `feat6/sprint2-checkout-cleaning`, uncommitted and not reviewed**.

- No new migration: 0005's turnover phase, timestamps, `release_reason=CLEANED` and the live-claim index are sufficient.
- Checkout, Start Cleaning and Finish Cleaning share one service path. `expectedVersion` is the assignment version; request-key operation names are `checkout`, `start-cleaning`, `finish-cleaning`, hashed with the assignment path. Events are `CHECKOUT` (ACTIVE→DIRTY), `START_CLEANING` (DIRTY→CLEANING) and `FINISH_CLEANING` (CLEANING→CLEANED). Any other state, including a CLEANED historical group, returns 409 INVALID_TRANSITION.
- `db enable-seating` requires the exact `current_database()` name (no `_demo/_uat/_test` suffix restriction, because activation is ultimately meant for a real queue after authorization). It needs a floor-state row plus at least one table in an active zone, and no CALLED ticket. A rerun on a SEATING queue is a no-op. There is no deactivation command. The CLI prints only the three fixed refusal messages; other errors stay generic.
