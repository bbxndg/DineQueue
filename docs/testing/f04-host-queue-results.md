# F-04 local execution evidence — 11 November 2026

Branch: `feat/F-04-host-queue-operations`. Base HEAD: `acc51aa` (sprint-1-staging
after the F-03 merge). Results below describe the **uncommitted worktree**, not a
release commit or CI run. The implementation addendum
(`../features/f04-host-queue-implementation-contract.md`) was written in the
worktree before code, without a separate commit. No Issue, teammate review,
integration, deployment, commit or push had been performed when these checks ran.

## Scope and environment

- `internal/queue`: Host board reads (assigned queues, FIFO WAITING list, current
  CALLED), Call Next and Complete (legacy F-05) with queue row lock `FOR UPDATE`,
  grant recheck inside the transaction, conditional updates and affected-row checks.
- Web: `/host/queue` board (current card, waiting table, Call next, Complete with
  confirmation, refresh, error/notice messages); Next rewrites for Host routes.
- No new migration or dependency: migration 0002 already has the lifecycle CHECK
  and `uq_one_called_per_queue`.
- Windows 11 amd64; Go 1.26.10 (go.dev archive, SHA-256 verified); Node 24.14.0
  (runtime target remains Node 22); PostgreSQL 18.1 local binaries.
- PostgreSQL ran as a disposable local cluster (port 55432, random credentials).
  Go integration fixtures migrate/seed independent schemas and drop them.
- Browser: Playwright 1.65.0 with locally installed Chromium revision 1243 via a
  temporary config outside the repository; production `next start` (port 3100)
  and compiled Fiber API (port 8080) against a freshly migrated/seeded database.
- `-race` was not run: only a 32-bit GCC is available on this machine (cgo).
  Docker/CI were not run.

## Actual checks

| Command / check | Actual result |
|---|---|
| `go test -tags=integration -count=1 -run TestF04 ./internal/queue` with stub service and no routes (RED) | FAIL: all 8 F-04 tests failed (Host endpoints returned 404) |
| Same, after implementation (first run) | 7 PASS, 1 FAIL: test helper built a doubled My Ticket path; fixed the test call, no production change |
| `go test -tags=integration -count=1 -v ./...` (final, 22:23 +07) | **PASS: 46 top-level tests, 0 failures**, including 8 F-04 tests plus F-01/F-02/F-03 regression |
| `go test -count=1 ./...` | PASS |
| `go vet ./...` and `go vet -tags=integration ./...` | PASS |
| `go build ./...`, `go mod verify` | PASS, all modules verified |
| `gofmt -l` on new files | Clean after formatting the test fixture struct |
| `npm run lint`, `npm run typecheck`, `npm test`, `npm run build` | PASS; `/host/queue` builds as dynamic route |
| Browser run 1 | 12 PASS, 3 FAIL: new spec hit the 10 logins/minute limiter (`Too many attempts`). Added a 61 s wait before the F-04 spec |
| Browser run 2 | 14 PASS, 1 FAIL: test opened `/join` before customer sign-in finished. Test now waits for the signed-in home page |
| Browser run 3, fresh seeded DB (22:21 +07) | **15 PASS** (7 F-01, 2 F-02, 3 F-03 incl. stored-state test, 3 F-04) |
| DB check after browser run | A-003 (party 3) COMPLETED, `called_by`/`completed_by` = seeded Host, version 3; no CALLED rows remain |
| API log scan | No 5xx, no panic, generated seed password absent |

## Test mapping

| Test IDs | Test | Notes |
|---|---|---|
| T-022 | `TestF04WaitingListIsFIFO` | Fixed out-of-order `joined_at`; status filter default/400; assigned-queue list; no private fields |
| T-023 | `TestF04CallNextCallsOldestWaiting` | A-001 CALLED with `called_at/called_by`, version 2; A-002 WAITING; current endpoint |
| T-024, T-025 | `TestF04CallNextBusinessConflicts` | `QUEUE_EMPTY` writes nothing; `CALL_IN_PROGRESS` leaves versions unchanged |
| T-026 | `TestF04ConcurrentCallNextClaimsOnce` | 10 parallel requests from H1 + H3: exactly one 200, nine `CALL_IN_PROGRESS`, oldest entry called |
| T-005..007, T-027, T-032 | `TestF04AccessControl` | Guest 401; Customer 403; unassigned Host 403 on list/current/call/complete; unknown queue 403, malformed 404; CSRF/Origin; non-empty body 400; service-level role/grant recheck; revoked grant denied |
| T-028, T-033 | `TestF04CompleteThenCallNext` | H3 completes H1's call; `completed_by` = H3; owner reads COMPLETED via F-03; A-002 called next |
| T-030, T-031 | `TestF04InvalidCompletion` | WAITING and repeated completion 409 `INVALID_TRANSITION`, unchanged; unknown/malformed 404; other queue's entry 404 |
| Regression | `TestF04ConcurrentCompleteSucceedsOnce` | 6 parallel completes: one 200, five 409 |
| T-029 | `e2e/queue-operations.spec.ts` | Real browser Host call/complete; customer sees CALLED then COMPLETED; board shows no customer identity; Cancel keeps CALLED |
| T-027 (browser) | `e2e/queue-operations.spec.ts` | Unassigned Host sees "No queue is assigned"; API 403 |
| T-006 (browser) | `e2e/queue-operations.spec.ts` | Customer redirected from `/host/queue`; call-next 403 |

## Limits

- RED was shown against stubs (endpoints absent), not by removing the queue lock;
  the partial unique index `uq_one_called_per_queue` is a second DB guard.
- No `-race`, Docker smoke test, CI run or Node 22 run on this machine.
- The F-04 browser spec waits 61 s for the login rate limit window and drains
  earlier specs' active tickets through real Host transitions; run with one worker
  against a disposable seeded database only.
- Logs were kept in the session scratchpad; the temporary cluster, servers and
  Playwright config were removed after the run.

## PR review follow-up — 12 November 2026

Base: committed F-04 branch HEAD `953a925` plus uncommitted review fixes. Same local
environment as above (fresh disposable PostgreSQL 18.1 cluster and seeded database).

| Review point | Change | Fail-first evidence |
|---|---|---|
| Malformed `entryId` was validated after the queue lock | `Complete` checks role and both IDs before opening the transaction | `TestF04MalformedEntryDoesNotTakeQueueLock` (queue row held by another transaction): before fix waited 2 s and failed with `context deadline exceeded`; after fix returns `ErrNotFound` immediately |
| Host handlers ignored `auth.Identity` `ok` | All five Host handlers return 401 `UNAUTHENTICATED` without an identity, like the other queue handlers | `TestHostHandlersRequireIdentity` (unit, no DB): before fix panicked when an empty identity reached the service; after fix 401 on all five routes |
| Test fixture writes ignored errors | Second-queue insert, its grant and the grant revocation now fail fast (revocation also requires one affected row); `t.Fatal(fmt.Sprintf(...))` → `t.Fatalf` | Test-only change |
| Board showed time only | Host board shows date and time in the restaurant timezone | Covered by browser run below (no assertion on time text) |

| Command / check | Actual result |
|---|---|
| `go test -tags=integration -count=1 -v ./...` (14:56 +07) | **PASS: 48 top-level tests, 0 failures** |
| `go test -count=1 ./...`, `go vet` (with and without tag), `go build ./...` | PASS |
| `npm run lint`, `typecheck`, `test`, `build` | PASS |
| Playwright on fresh seeded DB (14:57 +07) | **15 PASS** |
