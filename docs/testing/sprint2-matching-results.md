# Sprint 2 table matching — local verification

Issue: [#51](https://github.com/bbxndg/DineQueue/issues/51). Contract: DQ-S2-INTERFACES-2.1 and DQ-S2-STS-2.1. Tested 27 November 2026 (Asia/Bangkok), on uncommitted changes based on `28dfc6c41a9e0a112c2bac38201d398c6af04ea0`. No future commit or hosted CI result is claimed.

## Delivered behavior

Host floor selection reads `GET /api/v1/host/queues/{queueId}/matching?tableId=...` for one to three distinct canonical table IDs. The server validates scope, active same-zone connectedness and availability from PostgreSQL, then selects the oldest Ready WAITING party that fits that final set. Not-ready and too-large earlier parties receive explicit skip reasons; their original priority is preserved.

The `entryId` selector returns advisory connected-set recommendations for a Ready WAITING entry, ordered by table count, spare seats and sorted IDs. It cannot force that party ahead of an earlier fitting party when a set is selected. A CALLED ticket blocks actionable preview with CALL_IN_PROGRESS. Empty/no-ready/no-fitting/no-set outcomes are explicit.

Preview uses a read-only repeatable-read transaction including queue authorization. It creates no holds, leases, expiry work or table/ticket updates. DTOs contain Host ticket projections, versions and table data, without customer identity. Matching does not require SEATING activation for advisory reads; MANUAL calls retain their existing behavior. Allocation, call confirmation, expiry and seating remain owned by #52.

## Executed checks

| Command | Actual result | Local log |
|---|---|---|
| `go test ./internal/matching` against the initial empty implementation | RED: connected ten-person and chain assertions failed | `/tmp/dq-matching-red.txt` |
| `go test -race -tags=integration -count=1 ./...` | PASS on isolated PostgreSQL | `/tmp/dq-matching-go.txt` |
| `go vet ./...` | PASS | same Go log; shell exit 0 |
| `npm test` | PASS, 14 tests | `/tmp/dq-matching-web-tests.txt` |
| `npm run lint` | PASS | `/tmp/dq-matching-lint.txt` |
| `npm run typecheck` | PASS | `/tmp/dq-matching-types.txt` |
| `npm run build` | PASS, production Next build | `/tmp/dq-matching-build.txt` |
| `npm run test:e2e --workspace @dinequeue/web -- matching.spec.ts floor.spec.ts` | PASS, 5/5 browser tests using installed Chrome | `/tmp/dq-matching-browser.txt` |

Go commands ran in the existing Go 1.26.10 tool image, using the dedicated `dinequeue_floor_regression_test` database. Web checks used pinned Node 22.23.4 where subprocess isolation was required. Initial restricted-environment build/test subprocess failures were rerun successfully outside that restriction. Browser startup first required selecting the installed Chrome channel and supplying the isolated API origin to server-rendered reads; the corrected browser run passed. After verification, one explanatory UI sentence was shortened without changing behavior. One integration run exposed an unpopulated embedded GORM projection; the projection was corrected before the passing regression.

## Test mapping and boundaries

`internal/matching/matching_test.go`: connected 6+4 for ten people, unavailable exclusions, disconnected sets, three-table chain, deterministic ranking, party boundaries 1/16 and 0/17 rejection, cross-zone exclusions and an unselected middle vertex (S2-T-003–007,035, partial).

`internal/queue/matching_integration_test.go`: oldest-fitting Ready selection, restored original priority, scoped role/grant denial, invalid selectors, unavailable states, empty/no-fit outcomes and unchanged inventory snapshots (S2-T-002,008,018,030,036,051, partial). Later mutation races, forced-party rejection, audit records and hold claims belong to #52; these tests do not claim that coverage.

`apps/web/e2e/matching.spec.ts`: selected pair, Ready ticket preview, disconnected-set error and stale-floor invalidation. The existing floor suite checks accessible Map/List selection and responsive layouts.

No new migration, dependency, Docker configuration, workflow or cloud resource is included. Local results do not establish deployed acceptance or teammate approval.
