# Sprint 1 requirement completion checklist

Scope: course PDF §§5.2, 6.9 and 7 plus the repository's Sprint 1 SRS, feature
contracts, UI/API/data specifications and STS. Four parent features F-01..F-04;
manual Complete retains legacy F-05. This checklist audits implementation and
local verification separately from human review and release/submission history.

Source: `fix/sprint1-review`, integrated base `d7f27a3` plus the fix worktree.
Exact tested source fingerprints, environments, commands and results are recorded
in [Sprint 1 review results](sprint1-review-results.md). A future commit or merge
does not inherit an execution claim automatically.

## Functional and business requirements

| Requirement | Implemented behavior | Verification |
|---|---|---|
| FR-001; AC-001..003,026 | Login/logout/me; role-directed UI, reload and logout revocation | F-01 API/DB tests; auth browser journeys |
| FR-002; AC-004..005 | Backend role, per-queue grant and ownership enforcement | Guest/Customer/unassigned Host API denial and browser route checks |
| FR-003; AC-006,010 | Choose an active seeded queue and provide party size; closed/unknown queues reject | F-02 positive/negative HTTP+DB tests; join UI |
| FR-004; BR-002/009; AC-006,009 | Persist WAITING, unique daily number in restaurant timezone; reject supplied status/number/date | Counter transaction/index tests, strict JSON validation, local-midnight boundary |
| FR-005; AC-011..012 | Own active status/number/size/issue time, empty state and persisted refresh | F-03 exact persisted-value/read/UI tests |
| FR-006; BR-003; AC-014 | Assigned Host board, WAITING FIFO and current CALLED | Host list/current API and browser tests; timestamp/id tie fixtures |
| FR-007; BR-003/005; AC-015..018 | Strict FIFO Call Next; no skip and no second CALLED | Queue lock, conditional update, partial unique index; parallel Host tests |
| FR-008; BR-004/006; AC-019..021 | Only authorized Host completes CALLED; actor/time/version persist | Positive/negative/repeated/cross-queue tests; customer observes completion |
| FR-009; BR-001/004/008 | Duplicate active ticket and invalid input/transition reject | Same-customer 20-way join, distinct-customer 20-way join, invalid body/range/state tests |
| FR-010; BR-007; AC-005,013 | Only owner reads detail; foreign/missing/malformed returns non-revealing 404 | F-03 role/ownership API tests and account-switch/browser checks |
| FR-011; AC-025 | Explicit repeatable migrations/seeds; ticket/session survives API restart | Fresh isolated setup, migrate/seed idempotency tests and real process restart |
| FR-023; BR-008 and S1 portion of BR-015 | Party 1..16, including 10, joins without table allocation; physical-table limit remains independent | Bounds/raw-number tests, party-10 persisted ticket, large-party UI notice |
| BR-001 | At most one WAITING/CALLED per customer/queue, even across days | Partial unique index plus sequential/parallel tests; completion permits rejoin |
| BR-002 | Unique number scoped to queue/local service date | Unique index, counter lock/upsert, midnight reset and concurrent numbering |
| BR-003 | `(joined_at,id)` ascending without skipping; older-day tickets still eligible | Board and selection tests with reversed fixture insertion and equal-time ID order |
| BR-004 | WAITING → CALLED → COMPLETED only | Conditional SQL and lifecycle CHECK; WAITING/repeated completion rejected |
| BR-005 | One CALLED per queue | Queue-row lock plus DB partial unique index and real concurrent requests |
| BR-006 | Assigned Host only; grant rechecked in mutation transaction | HTTP/service denial tests and revoked-grant checks |
| BR-007 | Customer ownership is in SQL | Queries bind authenticated customer and requested entry; foreign 404 |
| BR-008 | Integer inclusive 1..16 | Client and server validation; database CHECK |
| BR-009 | Backend assigns date/number/status | Strict DTO, server clock/timezone and transaction; supplied extra fields reject |

## UI and nonfunctional requirements

| Item | Implementation / local verification |
|---|---|
| UI P-01 Login | Associated email/password labels, pending/error states, role routing and expired-session feedback; keyboard focus order checked |
| UI P-02 Customer home | Restaurant/queue context, identity/logout, My Queue navigation, Join or View My Queue based on persisted active state |
| UI P-03 Join | Queue label/selector, party validation and 9+ notice, pending/error/conflict feedback, successful ticket navigation; long queue selector fits mobile |
| UI P-04 My Ticket/detail | Persisted fields, timezone-labelled joined time, text WAITING/CALLED/COMPLETED messages, refresh, empty/error/retry, owner-only completed bookmark |
| UI P-05 Host | Assigned selector, waiting FIFO, current joined/called times, Call/Complete/Cancel/Refresh, keyed confirmation and conflict/read-error recovery |
| NFR-001 | Both identical-customer and distinct-customer 20-way joins checked against real PostgreSQL |
| NFR-002 | Parallel Call Next and Complete, plus mixed Complete/Call Next, preserve lifecycle and one CALLED |
| NFR-003 | Server-side roles and resource scope, independently of UI controls |
| NFR-004 | Argon2id with random salt; opaque hashed DB sessions, expiry/rotation/revocation; local secrets excluded from Git |
| NFR-005 | PostgreSQL persistence, migration/seed idempotency, actual API process restart |
| NFR-006 | Labels, keyboard activation, status text, success/empty/error/loading states, local visual/layout checks at 360px and 1280px; see final completion checkpoint in results |
| NFR-009 | Unit + real DB integration/concurrency/rollback + selected browser tests mapped to existing AC/T IDs |
| NFR-010 | Public display is later scope; shipped private DTO projections and Host UI omit customer identity |
| NFR-015 | Exact Origin and per-session CSRF on mutations; strict validation; backend role/resource checks |
| SDS auth failure logging | Fixed redacted event messages; automated log test checks submitted email/password/token do not appear |
| Data/transactions | Explicit SQL migrations; unique/FK/lifecycle constraints; service-owned transaction, deterministic queue-first locks and rollback tests |
| SDS API contract | [Implemented S1 OpenAPI export](../api/sprint1.openapi.json), 14 paths/operations, JSON/local schema-reference checks; matches documented route/DTO/error contracts |
| Local setup | Docker build/start/health verified with local port override; no new migration/seed needed for fixes |

NFR-007 performance load target is scheduled for S3; NFR-008 CI/deployment is Lab 5.
Floor-related NFR-011..014 and table constraints/algorithms belong to S2/S3.
Signup, password recovery and email verification are not required by course §5.2
for Sprint 1. No later-sprint feature is needed to complete this local queue slice.

## Release and human evidence boundary

The course and Sprint 1 DoD also require the team's real Git/review/release evidence.
Code changes cannot manufacture prior Issues, a pre-implementation commit, a
teammate approval, contributions by another student or an assessment submission.

| Item | Observed evidence / next release action |
|---|---|
| Existing feature PR history | [F-01 #1](https://github.com/bbxndg/DineQueue/pull/1), [F-02 #3](https://github.com/bbxndg/DineQueue/pull/3), [F-03 #4](https://github.com/bbxndg/DineQueue/pull/4), [F-04 #5](https://github.com/bbxndg/DineQueue/pull/5) exist; approval is not inferred from existence |
| Issues / board / contributions | GitHub read-only issue search on 13 Nov returned no Issues. Team must provide actual process/contribution evidence; this checklist does not backdate any |
| Current fixes | Local worktree; commit/push and peer-reviewed fix PR into sprint staging still required |
| Release | Reviewed sprint-staging → main PR, actual submitted SHA/tag, and checks tied to that source remain release actions |
| Submission | Use the actual main release, setup/demo, executed test results and direct evidence links per `../release/submissions.md` |

Human cross-team UAT is a separate Lab 6 activity. Agent-run keyboard/layout/visual
verification is identified as such and does not claim another student's acceptance.
