# F-02 local execution evidence — 6 November 2026

Branch: `feat/F-02-join-waitlist-ticket`. Base HEAD: `4ff1b32`.
All results describe the **uncommitted worktree** at around 22:20–22:50 Asia/Bangkok.
The user commits the changes; no push, Issue, PR, teammate review, CI, release or
deployment had been performed when these checks ran. Contract:
[`f02-join-implementation-contract.md`](../features/f02-join-implementation-contract.md).

## Implemented scope

- Migration `0002_queue_entries.sql`: `queue_day_counters`, `queue_entries`, party
  size 1..16 CHECK, lifecycle CHECK, `uq_queue_day_number`,
  `uq_one_active_ticket_per_customer_queue`, `uq_one_called_per_queue`, FIFO index.
- `internal/queue`: strict join-body parser, service-owned transaction (queue `FOR
  SHARE` → restaurant-local service date → counter upsert → insert), repository,
  HTTP handlers. `GET /api/v1/queues`, `POST /api/v1/queues/{queueId}/entries`.
- Web: `/join` page and form, home Join Waitlist link, same-origin rewrites.
- Not in scope: `/my-queue`, ticket detail endpoints (F-03), Host board (F-04).

## Environment

Windows 11, Go 1.26.10 windows/amd64 (official go.dev archive, SHA-256 verified,
unpacked to a session scratch directory with `GOTOOLCHAIN=local`), PostgreSQL 18.1
local binaries in a fresh ephemeral `initdb` cluster on 127.0.0.1:55432 with a random
password, Node 24.14.0 / npm 11.9.0 (**not** the pinned `.nvmrc` 22.23.4),
Playwright 1.65.0 with the already-installed Chromium build 1243 (v1254 download
failed: system drive full). Docker daemon was not running; no container checks.

## Actual checks

| Command / check | Actual result |
|---|---|
| `go test ./internal/queue/` before implementation | RED: build failed, undefined `ParseJoinRequest`, `ErrValidation`, `TicketCode` |
| `go test -tags=integration -count=1 -v ./internal/queue/` with migration lacking `uq_one_active_ticket_per_customer_queue` | RED: T-013 second join returned 201; T-014 `created=10 conflicts=0 rows=10`; other 8 tests passed |
| Same suite after adding the partial unique index | GREEN |
| `go test -tags=integration -count=1 -v ./...` | PASS: all packages; 8 F-02 integration + 2 F-02 unit tests, F-01 (6 DB + 3 unit), config, postgres, transport regression |
| `go test -race …` | NOT RUN: race detector needs 64-bit cgo; only a 32-bit GCC is installed (`cc1.exe: 64-bit mode not compiled in`) |
| `go vet ./...`, `go vet -tags=integration ./...` | PASS |
| `go mod verify` | PASS (`pgx/v5 v5.12.0` moved from indirect to direct; same version, go.sum unchanged) |
| `go build -buildvcs=false` of `./cmd/api`, `./cmd/db` | PASS |
| Compiled db CLI: migrate ×2, seed ×2 on empty DB | PASS; `schema_migrations` = 0001, 0002 |
| `npm ci --ignore-scripts` | PASS (lockfile install, no install scripts) |
| `npm test`, `npm run lint`, `npm run typecheck`, `npm run build` | PASS; `/join` built as dynamic route |
| `npx playwright test` (production Fiber + `next start`, fresh seeded DB, temporary local config pointing to Chromium 1243, deleted afterwards) | **9 passed**: 7 existing F-01 + 2 new F-02 |
| Post-E2E DB query | one row `A-001`, customer2, party 10, WAITING; counter `last_number = 1` (rejected duplicate left no gap) |
| Seed password search in API/web logs | 0 occurrences |

The first E2E attempt failed all login journeys because the API listened on 18080
while built rewrites target `127.0.0.1:8080`; rerun with the API on 8080 passed.

## Test names → STS

| Test | IDs |
|---|---|
| `TestParsePartySize`, `TestTicketCode` | T-011 (unit), ticket code format |
| `TestF02JoinCreatesWaitingTicket` | T-009, T-036; queue list without private fields |
| `TestF02InvalidPartySizeCreatesNothing` | T-011, T-012 |
| `TestF02DuplicateSequentialJoinConflicts` | T-013; rollback leaves no gap; rejoin after COMPLETED |
| `TestF02ConcurrentDuplicateJoinCreatesOne` | T-014 |
| `TestF02ConcurrentCustomersGetUniqueNumbers` | T-015 |
| `TestF02UnknownAndInactiveQueue` | T-016 |
| `TestF02RoleSessionAndCSRFGates` | T-005 (join), T-017, CSRF, service-level role recheck |
| `TestF02ServiceDateUsesRestaurantTimezone` | T-034 |
| `e2e/join.spec.ts` (2 tests) | T-010 (join part), T-017 browser redirect |

## Remaining limits

- T-010 "see ticket" on `/my-queue` and T-018..021 need F-03; T-006/T-007 need F-04.
- Race detector, Docker image and CI runs are pending on a machine with the pinned toolchain.
- RED/GREEN is a worktree sequence, not a commit sequence. No approval is inferred.
