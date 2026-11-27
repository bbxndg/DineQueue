# Sprint 2 checkout, cleaning and seating activation verification (F-09/F-16)

Issue [#54](https://github.com/bbxndg/DineQueue/issues/54). Contracts DQ-S2-INTERFACES-2.1 / DQ-S2-STS-2.1. Branch `feat6/sprint2-checkout-cleaning`, base `a51781cd7e88834cd851e42a668abcf7577a454f` (sprint2-staging after PR #61, which contains Feature 5 cancellation/no-show). Tested 27 November 2026, Asia/Bangkok. **Changes were uncommitted during execution.** No hosted CI, release, deployment, production activation or reviewer approval is claimed.

## Implemented scope

- `POST /api/v1/host/queues/{queueId}/seating-assignments/{assignmentId}/checkout`, `/start-cleaning` and `/finish-cleaning` (ActionVersion, CSRF, granted queue). One shared service path (`turnover.go`) follows Seat's sequence: scope checks, keyed replay, independent expiry housekeeping, then floor → queue → entry → sorted member locks, with state, version and claim ownership rechecked after locking.
  - Checkout: ACTIVE → CLOSED with turnover DIRTY and `closed_at`; every member OCCUPIED → DIRTY. The ticket stays COMPLETED.
  - Start Cleaning: CLOSED/DIRTY → CLOSED/CLEANING with `cleaning_started_at`; every member DIRTY → CLEANING.
  - Finish Cleaning: CLOSED/CLEANING → CLOSED/CLEANED with `cleaned_at`, `released_at` and `release_reason=CLEANED`; every member CLEANING → AVAILABLE; every outstanding claim released. Membership rows are kept as history.
  - Claims stay outstanding through DIRTY and CLEANING, so the existing partial unique claim index and table state block early allocation. A CLEANED or RELEASED group matches no rule, so an old group can never change tables allocated again later. Wrong state → 409 INVALID_TRANSITION, stale version → STALE_PREVIEW, foreign → 404, changed payload with a used key → IDEMPOTENCY_CONFLICT.
  - Events `CHECKOUT`, `START_CLEANING`, `FINISH_CLEANING` with the Host actor and member table IDs, committed with the state change and the idempotency record.
- Assignment results for CLOSED groups now resolve the COMPLETED ticket (`seatingResult`).
- `db enable-seating --queue-id=<uuid> --confirm-database=<name>` (`EnableSeating`): requires the exact current database name, a floor-state row and at least one table in an active zone, and no CALLED ticket in the queue. Locks floor → queue and changes only `operations_mode` MANUAL → SEATING. A rerun on a SEATING queue is a no-op. WAITING tickets, readiness, history, counters and credentials are untouched and no assignment is created. The CLI prints the fixed refusal reason; other errors remain generic. API startup does not migrate, provision or activate (no such calls in `cmd/api` or the HTTP transport).
- No new migration: 0005 already holds the turnover phase, timestamps, release reason and claim index. Migrations 0001–0006 are unchanged.
- Host UI: the floor assignment card shows the current state (Held, Occupied, Checked out · Dirty, Cleaning) and ticket code, and offers only the next valid whole-group step: Checkout, Start Cleaning or Finish Cleaning, each with its own confirmation listing every member table ("Keep current state" backs out). Exact Next rewrites for the three routes. The floor parser accepts `turnoverPhase`.

## Executed checks

Go: `golang:1.26.10-bookworm` against the isolated container database `dq_go_test` (one schema per fixture). Browser: disposable `dq_f5_test` (migrate, seed, `provision-floor`), local API :18087, `next start` at `http://127.0.0.1:3107`, Node 22.23.4, installed Chrome. CLI: separate disposable `dq_f6cli_test`, dropped afterwards. No shared development or Cloud SQL database was touched and no production queue was activated.

| Check | Result |
|---|---|
| RED 1: `go vet -tags=integration ./internal/queue/` with the new tests only | FAIL as expected: `undefined: EnableSeating` |
| RED 2: temporary stubs, `go test -tags=integration -count=1 -run 'TestF09\|TestF16' ./internal/queue/` | FAIL as expected: 9/9 |
| GREEN: same selection with `-race -v` | PASS 9/9 on first run |
| `go test -race ./...`, `go vet -tags=integration ./...`, `go test -tags=integration -race -count=1 -v ./...` | PASS: 104 top-level / 73 subtests, 0 fail, 0 skip |
| `npm run lint`, `npm run typecheck`, `npm test` | PASS (14 unit tests) |
| `npm run build` with `API_INTERNAL_URL` | PASS |
| `playwright test e2e/turnover.spec.ts` | PASS 1/1 |
| Full `npm run test:e2e` after DB reset, API restart and 60 s limiter wait | PASS 44/44 (5.9 min) |
| Real CLI on `dq_f6cli_test` | Refused without a floor, with a wrong confirmation and with a legacy CALLED ticket (mode stayed MANUAL each time); after the ticket was completed, activation succeeded and a rerun was a no-op; ticket/readiness/password hashes unchanged; 0 assignments |

The browser spec was written after the API was GREEN, so it has no recorded RED run.

## STS mapping

| Cases | Actual coverage |
|---|---|
| 016, 024, 027 | `TestF09FullGroupTurnover`: both members DIRTY → CLEANING → AVAILABLE in separate transactions; claims retained until finish; another queue's hold on the same tables fails while DIRTY/CLEANING and succeeds after; timestamps preserved; three events; cleaned group leaves the floor list |
| 017, 026, 054 | `TestF09TransitionGuardsAndIdempotency`: every shortcut from HELD/ACTIVE/DIRTY → INVALID_TRANSITION with no change; stale version; replay returns the committed response; changed payload IDEMPOTENCY_CONFLICT; fresh-key duplicate checkout rejected with one event; ungranted replay 403; unknown/cross-queue 404 |
| 028 (turnover routes) | `TestF09TurnoverHTTPGates`: ungranted Host 403, customer 403, CSRF 403, four invalid bodies 400, wrong-state 409, success response shape |
| 048 | `TestF09StaleHistoricalCleaningCannotTouchLaterGroup`: after A is CLEANED and B holds the same tables, every A step with fresh keys is rejected; B's tables and claims unchanged |
| 047 (turnover portion) | `TestF09RollbackOnMemberOrEventFailure`: second-member and event failure for each of the three steps roll back tables, assignment, claims, event and key |
| Cross-queue races | `TestF09TurnoverRacesNewHoldAcrossQueues` ×4 per step: a hold from another queue never succeeds before finish-cleaning commits; state never mixed |
| 053 (checkout portion) | `TestF09ConcurrentIdenticalCheckoutReplay`: six concurrent identical requests, one transition and event |
| 043, 044 | `TestF16EnableSeatingRefusesUnsafeActivation`, `TestF16EnableSeatingPreservesLegacyData` and the real CLI run: refusal cases, inactive-queue activation, history/readiness/counters/credentials preserved, legacy bodyless call guarded after activation, other queues untouched |
| 025 | Browser `turnover.spec.ts`: join 10 → Ready → T4+T5 → Call & Hold → Seat → Checkout → Start Cleaning → Finish Cleaning, reload at each boundary, dirty/cleaning tables disabled, 360 px without overflow, customer sees only COMPLETED and gets 403 on Host routes |
| Regression | Full browser run includes cancellation/no-show, seating, matching, floor, Lab 7 polling/readiness and S1 specs; expiry is covered by the Go seating tests |

Not covered: S2-T-032 restart for CLOSED/CLEANING groups (no API restart was performed in this run), deployed acceptance, Cloud SQL activation, and teammate review.

## Review follow-up (PR #63 review by bbxndg, 27 November 2026)

- `turnover.go`: a DIRTY/CLEANING group that no longer owns every member claim now fails with an internal error (HTTP 500) instead of 409 INVALID_TRANSITION, and changes nothing. New test `TestF09LostClaimIsInternalError` (start-cleaning and finish-cleaning; service and HTTP). It failed first with INVALID_TRANSITION, then passed after the change.
- `enable-seating`: returns an `ActivationResult` and the CLI logs `database, queue name (ID), mode before -> after`. With `APP_ENV=production` it refuses unless `--i-understand-production` is given (the flag is rejected for other commands). New test `TestF16EnableSeatingReportsChangeAndGuardsProduction`. The production path is verified at function level only: a local `APP_ENV=production` CLI run is not possible because production config requires Cloud SQL settings.
- Rerun (uncommitted at the time): Go `-race` unit + vet + all integration PASS (106 top-level / 75 subtests); the real CLI on a throwaway `dq_f6cli_test` again refused the three unsafe cases, then logged `mode MANUAL -> SEATING` and on rerun `SEATING -> SEATING`, with tickets/readiness/password hashes unchanged. Web code did not change, so lint/build/browser were not rerun after this follow-up.
