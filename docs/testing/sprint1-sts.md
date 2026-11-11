# 08. Software Test Specification (STS) — Sprint 1

**Version:** DQ-STS-S1-1.2. F-01 has local execution evidence in
[`f01-auth-results.md`](f01-auth-results.md); F-02 has local worktree evidence in
[`f02-join-results.md`](f02-join-results.md) (T-009, T-011..017, T-034, T-036 and the
join part of T-010); F-03 has local worktree evidence in
[`f03-my-ticket-results.md`](f03-my-ticket-results.md), covering T-008/T-010/T-018..021/T-035.
F-04, including legacy F-05 completion, has local worktree evidence in
[`f04-host-queue-results.md`](f04-host-queue-results.md) (T-005..007 Host endpoints, T-022..033). DEC-028 groups S1 into four parent features;
all T-001..041 and AC IDs are retained. Completion checks T-028..033 now belong
to F-04's completion workflow (legacy F-05).
T-005 has real join and Host endpoint checks, T-006/T-007 real Host endpoint checks
(F-04) and T-008 a real owner-scoped detail check. Do not count test-only
routes as shipped endpoints.

**Framework plan:** Go `testing`/`httptest` + Fiber test client + real isolated PostgreSQL via GORM; Playwright for selected browser journeys.

**Source:** CPE 334 §8.2 requires test IDs, inputs/preconditions, expected results, test level and AC coverage, including negative/boundary and true DB testing.

## 1. Environments and fixtures

- `TEST_DATABASE_URL` points to an isolated PostgreSQL DB; never a shared production DB.
- Migrations applied freshly; deterministic fixture with restaurant R1, queue Q1 and queue Q2 (Q2 for access-negative tests), Customers C1/C2 (plus 18 more for concurrency), assigned Hosts H1/H3 (Q1) and unassigned-to-Q1 Host H2 (Q2).
- Use a fixed/injected clock for local business date boundaries and predictable FIFO timestamps; reset tables/transactions between cases.
- E2E uses clean seeded demo accounts, no external API/payment services.
- F-03 browser stored-state/read-error tests require psql and TEST_DATABASE_URL
  for the same disposable seeded API database. They create/remove a fixture
  customer, write lifecycle fixtures and temporarily rename/restore queue_entries.
  Use one worker; these tests must not run against a shared or live database.
- For concurrency tests send requests truly in parallel to **real database transactions**, not only mock service calls.

## 2. Test case catalog

Every test must record **actual** command, git commit, result, run date and link once implemented.

| Test ID | Level | Preconditions / inputs | Expected result | AC |
|---|---|---|---|---|
| T-001 | API/DB | Active seeded Customer, correct password | 200 login + cookie, DB session exists | AC-001 |
| T-002 | API/DB | H1 login then GET `/auth/me` | 200 role HOST/id; no password hash/token leak | AC-001 |
| T-003 | API/DB | Wrong password, unknown email | 401, no usable session, no sensitive detail | AC-002 |
| T-004 | API/DB | Login → logout → GET `/auth/me` | logout 204, later 401 | AC-003 |
| T-005 | API | No session, POST join/call-next | 401 | AC-004 |
| T-006 | API | Customer C1 invokes call-next | 403 | AC-004 |
| T-007 | API | H2 has only Q2 grant; calls Q1 | 403 | AC-004, AC-018 |
| T-008 | API | C2 reads C1 ticket id | 404, no private body | AC-005, AC-013 |
| T-009 | API/DB | C1 POST Q1 partySize=4 | 201 `A-001`, WAITING, persisted row | AC-006 |
| T-010 | Browser E2E | C1 login, join, see ticket | Number/status consistent in UI/API/DB | AC-006, AC-011 |
| T-011 | Unit + API | partySize 0,17 | 400, no entry created | AC-007 |
| T-012 | API | partySize 1.5, missing, string | 400, no entry created | AC-007 |
| T-013 | API/DB | C1 joins twice sequentially | second 409 ACTIVE_QUEUE_EXISTS, one active row | AC-008 |
| T-014 | DB concurrency | 10 simultaneous join POST from C1 | exactly one 201; others 409; one row | AC-008 |
| T-015 | DB concurrency | 20 distinct Customers join same Q1 concurrently | 20 successful, unique numbers, no duplicates | AC-009 |
| T-016 | API | Customer joins inactive/unknown queue | defined 404/409; no entry | AC-010 |
| T-017 | API | Host submits customer join | 403 | AC-010 |
| T-018 | API/DB | C1 joins; GET own active and detail | correct ticket/time/size from DB | AC-011 |
| T-019 | Browser E2E | Reload `/my-queue` after join | ticket remains correct and loads from backend | AC-011 |
| T-020 | Browser E2E | C2 has no active ticket | empty state with Join action | AC-012 |
| T-021 | API | Guest accesses private detail | 401; no data | AC-013 |
| T-022 | API/DB | Tickets with fixed ordered timestamps | Host list sorted oldest first | AC-014 |
| T-023 | API/DB | Q1 WAITING A-001,A-002; call-next | A-001 becomes CALLED, timestamps/actor set | AC-015 |
| T-024 | API | No WAITING entries; call-next | 409 QUEUE_EMPTY, no modifications | AC-016 |
| T-025 | API | Q1 has CALLED entry; call-next | 409 CALL_IN_PROGRESS, unchanged entries | AC-016 |
| T-026 | DB concurrency | H1 and another authorized H3 call-next simultaneously | at most one success (BR-005), no duplicate claims | AC-017 |
| T-027 | API | Unassigned Host lists Q1 private entries | 403 | AC-018 |
| T-028 | API/DB | Q1 A-001 CALLED; H1 complete | 200 COMPLETED, persisted actor/time | AC-019 |
| T-029 | Browser E2E | H1 call-next then complete; customer reloads | customer sees COMPLETED | AC-019 |
| T-030 | API | Host completes ticket still WAITING | 409 INVALID_TRANSITION; unchanged | AC-020 |
| T-031 | API | Host completes already COMPLETED ticket | 409 INVALID_TRANSITION; unchanged | AC-020 |
| T-032 | API | Customer C1 attempts complete | 403 | AC-021 |
| T-033 | API/DB | Complete A-001 then call-next | A-002 becomes CALLED | AC-021 |
| T-034 | API/DB | Fixed clock 23:59 vs next local day 00:00 | numbers scoped to local service dates; no collisions | AC-009 |
| T-035 | DB/integration | Stop API, restart without DB reset | previous ticket remains queryable | AC-011 |

| T-036 | API/DB | C1 joins with partySize=10 | 201 WAITING; value 10 preserved, even though seeded physical tables max 8 (S2 pairing) | AC-007 extended |

`T-034/035/036` give further boundary/persistence evidence. Add them to traceability after code implementation.

| T-037 | API/DB | Missing/foreign Origin; invalid/missing CSRF on logout | 403 and session remains active | AC-022 |
| T-038 | API/DB | Expired/revoked session; disabled user; HTTPS cookie | 401; HttpOnly/Secure/Lax/Path=/; hash only in DB | AC-023 |
| T-039 | API/DB | Repeat login; unknown JSON field; >10 attempts/minute | token rotation, strict 400, bounded 429; insert failure rolls back revocation | AC-024 |
| T-040 | DB | Migrate and seed twice; invalid role/FK/duplicate; seed conflict | idempotency, constraints, transaction rollback | AC-025 |
| T-041 | Browser E2E | Customer/Host login, reload, wrong role route, logout, invalid session | role shell and login redirect; no browser auth storage | AC-026 |

## 3. Real TDD evidence plan

### BR-001 red → green → refactor (sample)

- **RED commit:** Add T-013 (duplicate join) and T-014 (race) to baseline; record expected 409/one-row invariant and a failing test log.
- **GREEN commit:** Add transaction + partial unique index; tests pass on real Postgres.
- **REFACTOR commit:** Improve validation/mapping without removing strong tests. Retain original ID and captured logs/commit references.

### BR-003/005 red → green (sample)

- RED: T-026 simultaneous Host calls currently both claim same entry / or both proceed.
- GREEN: queue-row `FOR UPDATE` lock + re-evaluate called state in transaction.
- Never fake failing logs, deliberately damage released app or rewrite test expectations to match incorrect code.

## 4. Suggested execution command matrix

| Check | Proposed command after implementation | Status now |
|---|---|---|
| API build | `go build ./...` | NOT RUN |
| Go unit tests | `go test ./...` | NOT RUN |
| API/Postgres integration | `go test -tags=integration ./...` | NOT RUN |
| Browser E2E | `npx playwright test` (web project) | NOT RUN |
| Go vet | `go vet ./...` | NOT RUN |
| Docker smoke test (Lab 5) | `docker compose up --build` | NOT RUN |

The table above describes the remaining whole-Sprint product verification plan.
Actual F-01 commands/results, including foundation regression, are recorded in
`f01-auth-results.md`, `f02-join-results.md` and `f03-my-ticket-results.md`;
do not interpret this table as a status for all repository tests.

## 5. Test-result evidence template

| Test IDs | Actual test file/name | Command | Date/time | Git SHA | Result | CI link/screenshot |
|---|---|---|---|---|---|---|
| T-013,T-014 | `[fill]` | `[fill]` | `[fill]` | `[fill]` | NOT RUN | `[fill]` |
| T-026 | `[fill]` | `[fill]` | `[fill]` | `[fill]` | NOT RUN | `[fill]` |

**Never enter PASS before actually running tests.**
