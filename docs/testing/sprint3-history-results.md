# Sprint 3 Customer Ticket History verification (Feat 2, F-11 customer extension)

Issue [#56](https://github.com/bbxndg/DineQueue/issues/56). Contract DQ-S3-1.0 ([contracts](../features/sprint3-contracts.md), [UI/API/data](../features/sprint3-ui-api-data.md), [STS](sprint3-sts.md)). Branch `feat2/sprint3-customer-ticket-history`, baseline `a70055cc457e065d99f713978000bf8226319ab7` (sprint3-staging after PR #67). Tested 27 November 2026, Asia/Bangkok. **All results are for the baseline SHA plus the uncommitted working-tree changes**, not for the unchanged baseline commit. No peer approval, deployment or product acceptance is claimed.

## Implemented scope

- `GET /api/v1/me/queue-entries/history` (`apps/api/internal/queue/history.go`), registered before `/queue-entries/:entryId`. CUSTOMER session only (anonymous/expired 401, Host 403). Returns `{tickets: Ticket[], nextCursor: string|null}` using the existing owned Ticket DTO, with `Cache-Control: no-store`.
  - Repository predicate on every page: `customer_id = <session user>` AND status IN (COMPLETED, CANCELLED, NO_SHOW) AND, with a cursor, `(joined_at, id) < (cursor)`; ordered `joined_at DESC, id DESC`; `LIMIT limit+1`. Parameterized SQL; each page is an independent read with no transaction across requests.
  - Query: only `limit` (digits, 1–50, default 20) and `cursor`, each at most once; any other key (including owner selectors) → 400 VALIDATION_ERROR with a generic message.
  - Cursor: unpadded base64url of `{"v":1,"joinedAt":"<UTC, exactly 6 fractional digits>","id":"<lowercase canonical UUID>"}`, at most 512 bytes. Duplicate/unknown/missing properties, other versions, other timestamp forms, non-canonical UUIDs and padded/invalid encoding are rejected. The cursor only moves the boundary; it never names or widens the owner.
  - Legacy MANUAL tickets are returned as stored; no event, terminal timestamp or other backfill. COMPLETED is shown as a completed waitlist turn, never as dining checkout.
- Migration `0007_customer_history_index.sql`: partial index `(customer_id, joined_at DESC, id DESC) WHERE status IN terminal`. Migrations 0001–0006 unchanged. See the plan evidence below.
- Web: `/my-queue/history` (static segment, resolved before `[entryId]`) with heading, Back to My Ticket, items (code, final status, party size, queue · restaurant, stored service date, joined time in the restaurant timezone, link to the existing owned detail), Load more, empty, initial failure + Retry, later-page failure keeping loaded rows + Retry of the same cursor, 401 → clear and go to login, 403 → clear. One page request at a time; responses after unmount or a session change (component keyed by session) are discarded. No history polling. Navigation links on the Customer home and `/my-queue`. One exact rewrite for the history path, placed before the `:entryId` rewrite.

## Index decision (isolated populated-data plan)

Database `dq_s3plan_test` (disposable, dropped afterwards): 2,000 synthetic customers × 100 COMPLETED tickets (200,000 rows), `ANALYZE`, then `EXPLAIN (ANALYZE, BUFFERS)` of the history query for one customer with `LIMIT 21`.

| | Plan | Buffers | Execution |
|---|---|---|---|
| Existing schema | Parallel Seq Scan on `queue_entries` + top-N sort | 4,327 | 18.3 ms |
| With the 0007 index, first page | Index Scan on `idx_queue_entries_customer_history`, no sort | ~78 | 0.34 ms |
| With the index, keyset page | Same index; `(joined_at,id) < (…)` inside the Index Cond | ~32 | 0.36 ms |

Without the index each request scans every ticket in the system; with it, cost follows the requested page. Deployment note: plain `CREATE INDEX` inside the migration transaction briefly blocks writes to `queue_entries` while it builds.

## Executed checks

Go: `golang:1.26.10-bookworm` container against isolated `dq_go_test` (one schema per fixture). Browser: disposable `dq_f5_test` (migrate, seed, provision-floor), local API :18087, production `next start` at `http://127.0.0.1:3107`, Node 22.23.4, installed Chrome. No shared developer or Cloud SQL database was touched; all accounts are synthetic.

| Time (ICT) | Check | Result |
|---|---|---|
| 22:51 | RED 1: `go vet -tags=integration ./internal/queue/` with new tests only | FAIL as expected: `undefined: encodeHistoryCursor` |
| 22:52 | RED 2: compile-only stubs, `go test -run 'TestHistory\|TestParseHistory'` and `go test -tags=integration -run TestS3History` | FAIL as expected: 2/2 unit and 7/7 integration (route absent → `/history` fell to `:entryId` 404; stub cursor/query logic) |
| 22:53 | GREEN: same, `-race` | 2/2 unit; 6/7 integration — the expired-session fixture violated `sessions` CHECK (test data, fixed in the test) |
| 22:54 | Rerun integration | 7/7 PASS |
| 22:55 | `go test -race ./...`, `go vet ./...`, `go vet -tags=integration ./...`, `go test -tags=integration -race -count=1 -v ./...` (after adding 0007 and its test) | PASS: 116 top-level / 75 subtests, 0 fail, 0 skip |
| 23:04 | `playwright test e2e/history.spec.ts` | 4/5: H09 asserted "no role=alert", but Next's route announcer also has role=alert (test assertion, fixed by filtering text) |
| 23:05 | Rerun `history.spec.ts` | 5/5 PASS (H12 executed a real API restart; not skipped) |
| 23:19 | Root `npm test`, `npm run lint`, `npm run typecheck` | PASS (16 unit tests) |
| 23:20–23:26 | Full `npm run test:e2e` after DB reset, API restart and 60 s limiter wait | **49/49 PASS** (6.2 min): S1, Lab 7 polling/readiness, S2 floor/matching/seating/cancel/no-show/turnover and the 5 history cases |
| 23:27 | Root `npm run build` | PASS; route `/my-queue/history` listed |
| — | `git diff --check` | clean |

The browser spec was written after the API was GREEN, so it has no recorded RED run. An earlier full browser run was interrupted when the editor session ended; it produced no result and is not counted.

## STS mapping (H01–H12)

| Case | Coverage |
|---|---|
| H01 | `TestS3HistoryOwnerAndRoleAccess`: A sees only A's terminal rows; anonymous and expired session 401; Host 403; no ticket/event writes; `no-store`; no identity fields |
| H02 | `TestS3HistoryForeignDetailSelectorsAndCursorOwnership`: foreign detail 404; `ownerId/userId/customerId` 400; a cursor built from B's row returns only A's older rows |
| H03 | `TestS3HistoryTerminalFilterAndPreservation`: legacy MANUAL COMPLETED, seated COMPLETED with ACTIVE assignment, CANCELLED, NO_SHOW returned; WAITING and CALLED excluded; assignment stays ACTIVE; no checkout field |
| H04 | Same test: code/party/service date/joinedAt (µs) equal to stored rows; full Ticket DTO; ticket and event snapshot unchanged |
| H05 | `TestS3HistoryKeysetPagination`: 60 rows incl. 4 at one instant and ±1 µs neighbours; limits 1/7/20/50/default traverse exactly the DB order without duplicates or gaps; cursor keeps 6 fractional digits; final `nextCursor` null; `TestHistoryCursorRoundTripKeepsMicroseconds` |
| H06 | `TestS3HistoryRejectsInvalidQueries` (26 inputs) and `TestParseHistoryQuery`: all 400 VALIDATION_ERROR, message without SQL/JSON/row details |
| H07 | `TestS3HistoryNewTerminalRowBetweenPages`: row cancelled above the boundary is absent from the ongoing traversal and first on a fresh first page |
| H08 | Browser: home navigation → history, 20 then Load more to 23 in DB order, keyboard Enter opens legacy owned read-only detail, Back to My Ticket shows the active WAITING ticket, 360 px without overflow |
| H09 | Browser: empty state distinct from failure; initial 503 → Retry; later-page 500 keeps 20 rows, double-click sends one request, Retry repeats the same cursor once |
| H10 | Browser in America/Los_Angeles: stored service date `2026-11-21` shown unchanged; joined time rendered in Asia/Bangkok (21 Nov 00:30) |
| H11 | Browser/API: `no-store`; A's delayed response released after the tab switched to B is not rendered; expired session → `/login` with data cleared |
| H12 | Browser: real API process restart on the same database; same history rows and owned detail. `TestS3HistoryRouteDoesNotBreakOwnedReads` plus the 44 existing browser cases cover ownership, join, readiness and active polling regressions |

Not covered: hosted CI, teammate review, deployed smoke of a released SHA, and a Cloud SQL plan check.
