# Sprint 2 Call, Hold and Seat verification

Issue [#52](https://github.com/bbxndg/DineQueue/issues/52). Contracts DQ-S2-INTERFACES-2.1 / DQ-S2-STS-2.1. Worktree base `a8168210adcd874f757c9a32af02b126bc8306f9`, tested 27 November 2026, Asia/Bangkok. Changes were uncommitted during execution. No hosted CI, release, cloud deployment or reviewer approval is claimed here.

## Implemented scope

Migration `0005_seating_lifecycle.sql` adds assignment groups, historical table memberships with partial unique live claims, operation events and successful action records. Terminal columns/checks support the shared expiry coordinator and later #53/#54 workflows. Migrations 0001–0004 remain unchanged. Existing queues stay MANUAL; production activation is deferred until the later workflows integrate.

`POST /api/v1/host/queues/{queueId}/match-and-call` rechecks scope, mode, selected-set FIFO, readiness and expected layout/entry/table versions under floor → queue → entry → sorted table locks. It commits CALLED, the complete HELD assignment, member claims, audit event and idempotency response atomically.

`POST /api/v1/host/queues/{queueId}/seating-assignments/{assignmentId}/seat` permits HELD → ACTIVE only strictly before the server deadline, then atomically completes the ticket and marks all members OCCUPIED. Claims remain outstanding after seating for the later group turnover implementation.

The shared expiry coordinator and explicit `POST /expire-holds` release a whole expired group, mark the ticket NO_SHOW/HOLD_EXPIRED and retain membership/history. Authorized mutations invoke expiry in an independent transaction after checking for a successful replay. GET floor and matching stay read-only. Browser countdowns never perform database transitions by themselves.

Successful action keys are scoped to actor, queue and operation. Hashes cover the canonical request and resource path; replay returns the original committed response even after later transitions. Changed payload/path conflicts and authorization is required for replay. JSON validation rejects unknown, duplicate, case-alias, null and malformed fields, invalid versions and mismatched member/version arrays.

Floor snapshots include only the authorized queue's assignments while retaining shared physical table status. The Host UI provides Call & Hold and Seat confirmations, local deadline display, explicit expiry recovery and existing-coordinator write/read coordination. Customer detail understands terminal statuses without exposing floor, member or actor identifiers. Legacy manual call/complete is guarded in SEATING mode and preserved for MANUAL queues.

## Executed checks

| Check | Result | Evidence |
|---|---|---|
| Initial `go test -tags=integration -run TestSeatingCallHoldSeat ./internal/queue` | RED: executable hold assertion failed with INVALID_TRANSITION | `/tmp/dq-seating-red.txt` |
| Targeted `go test -race -tags=integration -count=1 -run 'TestSeating\|TestFloorMigration' ./internal/queue ./db` | PASS before final additional race/rollback cases | `/tmp/dq-seating-targeted.txt` |
| `go test -race -tags=integration -count=1 -json ./...` and `go vet ./...` | PASS, 87 top-level tests; vet PASS | `/tmp/dq-seating-go-final.jsonl` |
| Final `go test -race -tags=integration -count=1 -run TestSeating ./internal/queue` and vet | PASS; includes the last capacity guard | `/tmp/dq-seating-final-targeted.txt` |
| `npm test` | PASS, 14 tests | `/tmp/dq-seating-web-tests.txt` |
| `npm run lint` | PASS | `/tmp/dq-seating-lint.txt` |
| `npm run typecheck` | PASS | `/tmp/dq-seating-types.txt` |
| `npm run build` | PASS, production build | `/tmp/dq-seating-build.txt` |
| `npm run test:e2e --workspace @dinequeue/web -- seating.spec.ts matching.spec.ts floor.spec.ts` | PASS, 6/6 | `/tmp/dq-seating-browser.txt` |

The isolated Go database is `dinequeue_floor_regression_test`; each integration fixture uses its own disposable schema. Browser fixtures use `dinequeue_floor_test`, separate from Go regression. Go 1.26.10 runs in the existing tool image; web uses Node 22.23.4 and installed Chrome. No shared development or Cloud SQL database was migrated or reset.

Initial checks exposed a migration assertion comparing added null columns as if they were old data, a shallow-copy race while sorting request versions, and a test cookie using the wrong configured name. These were corrected; preservation still compares every original field and now also verifies new terminal fields remain null.

## STS mapping

| Cases | Actual coverage |
|---|---|
| 009,011 | `TestSeatingCallHoldSeat`, browser `seating.spec.ts`: connected ten-person call/hold/seat, original ticket and members |
| 012,045,055 | `TestSeatingExpiryBoundaries`: deadline−1µs, exact deadline and +1µs; whole-group release and repeated cleanup |
| 013,014 | `TestSeatingConcurrentSharedTableClaims`: synchronized 20 attempts over two queues on one floor; exactly one assignment/claim group wins |
| 015,047 (seat portion) | `TestSeatingRollbackOnEventFailure`, `TestSeatingRollbackOnSecondMember`: injected SQL failure; ticket/member/version/event/key writes roll back |
| 016,038,043,053,054 | `TestSeatingIdempotencyAndGuards`, `TestSeatingStaleOrderAndMode`: original replay, changed payload, revoked grant, FIFO re-evaluation and legacy-mode guards |
| 028,029,030 | `TestSeatingHTTPStrictValidationAndPermissions`: anonymous/customer/ungranted Host, CSRF and invalid body denial before writes |
| 031 (seat/expiry portion) | `TestSeatingExpirySeatRace`: concurrent seat/cleanup on either side of the deadline; no mixed state |
| 042 | Extended `TestFloorMigrationPreservesLegacyRows`: original row data and credentials preserved, terminal fields null, migration rerun and MANUAL default |
| 032,049,056,058 | Local browser and regression checks; no deployed acceptance is implied |

Cancellation, explicit Host no-show, turnover, activation, their races and deployed acceptance remain #53/#54. Local tests do not establish those later acceptance criteria.

## Browser regression and restart evidence

`polling.spec.ts`, `polling-lifecycle.spec.ts` and `ticket-states.spec.ts` passed all 12 existing session-based browser cases in `/tmp/dq-seating-browser-regression.txt`. That batch's additional seating restart case initially reloaded before API readiness returned. The test now explicitly waits for HTTP 200 readiness after restart.

With `S2_TEST_API_CONTAINER=dq-s2-floor-api`, `seating.spec.ts` then passed 1/1 in `/tmp/dq-seating-restart.txt`. The test restarts the disposable API once while HELD and again while ACTIVE, then reloads and checks the same group. Its Customer owned detail still reports COMPLETED. This is actual local API restart evidence; it does not claim a Cloud Run or database restart.

Subsequent backend-only verification clarifies CAPACITY_INSUFFICIENT for an undersized selected set and reruns all seating integration rules. The browser build and 12 existing cases are the executions above, not additional fresh runs after that error-code clarification.
