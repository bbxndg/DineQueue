# Sprint 2 cancellation and no-show verification (F-08)

Issue [#53](https://github.com/bbxndg/DineQueue/issues/53). Contracts DQ-S2-INTERFACES-2.1 / DQ-S2-STS-2.1. Branch `feat5/sprint2-cancellation-no-show`, base `0f84715abbe480d40b28832bbd691b7e620364bc` (sprint2-staging after PR #60). Tested 27 November 2026, Asia/Bangkok. **Changes were uncommitted during execution.** No hosted CI, release, deployment or reviewer approval is claimed. Checkout, cleaning and activation remain #54.

## Implemented scope

- `POST /api/v1/me/queue-entries/{entryId}/cancel` (empty body, CSRF): the owner's WAITING ticket moves to CANCELLED with `cancelled_at`/`cancelled_by`, ticket version +1 and one `CUSTOMER_CANCEL` event. This works for Ready and Not ready tickets and in a queue closed to new joins. The response is the full owned Ticket. Foreign, unknown or malformed IDs return 404; an owned CALLED/COMPLETED/CANCELLED/NO_SHOW ticket returns 409 INVALID_TRANSITION. Locks are queue → entry only; the floor lock is never taken.
- `POST /api/v1/host/queues/{queueId}/entries/{entryId}/no-show` (ActionVersion + `reasonCode:"CUSTOMER_ABSENT"`): CALLED ticket with a HELD assignment only, SEATING mode, granted queue. Same sequence as Seat: scope checks, keyed replay, independent expiry cleanup, then floor → queue → entry → sorted member locks with every guard rechecked. In one transaction: ticket NO_SHOW with `no_show_by` = Host and CUSTOMER_ABSENT, assignment RELEASED, every member AVAILABLE, every claim released, one `NO_SHOW` event and the idempotency record. Returns `{entry, assignment, tables}`.
- Shared coordinator: `Repository.releaseHold` in `apps/api/internal/queue/seating.go` is used by both #52 expiry and #53 no-show. Expiry's inline release was refactored into it; there is no second expiry implementation.
- Migration `0006_cancel_event_without_assignment.sql` (new; see DEC-039, **needs team review**): `operation_events.assignment_id` is nullable only for a CUSTOMER_CANCEL WAITING→CANCELLED row with an actor. Migrations 0001–0005 are unchanged.
- UI: Customer "Cancel ticket" with confirmation ("Keep my ticket" / "Confirm cancel") through the existing polling coordinator `runMutation`; CANCELLED/NO_SHOW cards show "Join again". The floor held-assignment card adds "No-show" with confirmation ("Keep hold" / "Confirm No-show") through the existing floor mutation and retry path. Exact Next rewrites were added for both routes.

## Executed checks

Go runs in `golang:1.26.10-bookworm` against an isolated container database `dq_go_test`; each fixture uses its own schema. Browser runs use the separate disposable database `dq_f5_test` (migrate, seed, `provision-floor --confirm-database=dq_f5_test`), a local API on port 18087 and `next start` at `http://127.0.0.1:3107`, with Node 22.23.4 and installed Chrome. No shared development or Cloud SQL database was touched.

| Check | Result |
|---|---|
| RED 1: `go vet -tags=integration ./internal/queue/` with the new tests only | FAIL as expected: `f.service.NoShow undefined` |
| RED 2: temporary stubs, `go test -tags=integration -count=1 -run TestF08 -v ./internal/queue/` | FAIL as expected: all 7 F08 tests (routes 404, services unimplemented) |
| `go test -tags=integration -race -count=1 -run 'TestF08\|TestSeating' -v ./internal/queue/` | First GREEN attempt: 6/7 F08 passed. The rejoin step returned 500 because the test fixture inserted sequence 1 without updating the daily counter (fixture defect, fixed in the test). All TestSeating passed |
| `go test -race ./...`, `go vet -tags=integration ./...`, `go test -tags=integration -race -count=1 -v ./...` | PASS: 94 top-level / 55 subtests, 0 fail, 0 skip; vet PASS |
| `npm run lint`, `npm run typecheck`, `npm test` | PASS (14 unit tests) |
| `npm run build` (with `API_INTERNAL_URL` set at build time) | PASS |
| `playwright test e2e/cancel-no-show.spec.ts` | PASS 2/2 |
| Full `npm run test:e2e` after DB reset, API restart and 60 s login-limiter wait | PASS 43/43 (5.7 min), including S1, Lab 7 polling/readiness and S2 floor/matching/seating |

The browser spec was written after the API was GREEN, so it has no recorded RED run.

## STS mapping

| Cases | Actual coverage |
|---|---|
| 020 | `TestF08CustomerCancelOwnWaiting` (not-ready, ready, inactive-queue): CANCELLED, owner actor, one event with null assignment, same code/join time/number, version +1 |
| 021 | `TestF08CancelOwnershipStateAndGates`: foreign/unknown/malformed 404, guest 401, CSRF 403, Host role 403, non-empty body 400, owned CALLED/NO_SHOW/COMPLETED/CANCELLED 409 with no version change |
| 022, 023 | `TestF08HostNoShowReleasesWholeGroup`: ungranted 403, CSRF 403, five invalid bodies 400, unknown 404, stale version 409. Success releases both members and claims with actor and reason. Replay is identical; a changed payload gives IDEMPOTENCY_CONFLICT; a new key gives INVALID_TRANSITION; no extra events or keys |
| 031 (no-show portion) | `TestF08NoShowSeatRace` ×6: exactly one of Seat/No-show wins; tables, ticket and assignment are never mixed |
| 039 | `TestF08CustomerCancelOwnWaiting`, `TestF08HostNoShowReleasesWholeGroup`, browser `cancel-no-show.spec.ts`: owned terminal detail readable, absent from the active list, rejoin gets a new number and Not ready, old row unchanged |
| 040 | `TestF08CancelCallHoldRace` ×6: exactly one of cancel/call+hold wins; no CALLED+CANCELLED, no partial claim or orphan assignment, counter unchanged, one event |
| 041 | `TestF08NoShowRejectsNonHeldTargets`: WAITING and ACTIVE → INVALID_TRANSITION (occupied stays occupied), other queue → NOT_FOUND, MANUAL → SEATING_NOT_ENABLED, expired → HOLD_EXPIRED with system cleanup only |
| 015 (cancel/no-show rollback) | `TestF08RollbackOnEventFailure`: an injected event failure rolls back ticket, tables, claims, assignment and key |
| 056, 057 (subset) | Browser: cancel observed on the Host board and no-show observed on the visible owner detail without reload; cancel in an inactive queue; foreign detail/cancel 404; customer on the Host route 403. Hidden-tab and 401 behaviour rely on the existing Lab 7 polling specs, which passed in the full run |

Not covered here: deployed acceptance, S2-T-055 (cloud cleanup timing), #54 turnover/activation and teammate review.

## Review follow-up (PR review by bbxndg, 27 November 2026)

- `cancel_noshow.go`: comments now state that the unlocked foreign-entry check only rejects early (the locked `entryForUpdate` is the real scope check), and why an entry has at most one assignment (call only from WAITING, terminal states never return; `seating_one_outstanding_entry` enforces one HELD/ACTIVE).
- `seating.go` `releaseHold`: an invalid reason/actor combination now returns an internal error (HTTP 500) instead of `ErrValidation` (400). New test `TestF08ReleaseHoldRejectsInvalidReasonAsInternal`.
- Assignment DTO gains `ticketCode` (ticket code only); the web floor parser requires it and the Host No-show confirmation names the ticket. `TestF08HostNoShowReleasesWholeGroup` and `cancel-no-show.spec.ts` assert it.
- Rerun after the changes (still uncommitted at the time): Go `-race` unit + vet + all integration PASS (95 top-level / 55 subtests); web lint, typecheck, 14 unit tests, build PASS; full Playwright suite after DB reset PASS 43/43.
