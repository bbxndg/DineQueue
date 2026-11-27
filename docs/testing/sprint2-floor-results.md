# Sprint 2 floor inventory — Issue #50

## Scope and tested source

Date: 27 November 2026. Branch: `feat2/sprint2-floor-inventory`.
Tests were run on base commit `65bdbd9ca9a719bace80d54644f1d9e3f2f950bd` plus the inventory changes that were uncommitted at execution time. This record describes local execution; it is not a tested commit SHA for a future PR, review approval or cloud deployment.

The Host floor at `/host/floor?queueId=<granted-queue-id>` displays zones, capacities, table states and explicitly approved connections. Map and List views share the same selection, limited to three AVAILABLE tables in one active zone. Selection is inspection only. Existing queues remain MANUAL and retain their queue-board controls.

The implementation uses the existing queue module for its handler, service and repository, reusing queue-scoped authorization. Reads run in a read-only repeatable-read transaction. There is no GET-triggered provisioning, hold expiry or other business write. `assignments: []` reserves the response field until #52 supplies allocation storage and scoped assignment projection.

## Implementation

| Area | Source |
|---|---|
| Explicit schema | `apps/api/db/migrations/0004_floor_inventory.sql` |
| Handler and snapshot service | `apps/api/internal/queue/floor.go`, `host_http.go` |
| Snapshot queries | `apps/api/internal/queue/floor_repository.go` |
| Guarded synthetic fixture | `apps/api/internal/queue/floor_provision.go`, `apps/api/cmd/db/main.go` |
| Host page and interactive view | `apps/web/src/app/(main)/host/floor/page.tsx`, `apps/web/src/components/floor-board.tsx` |
| Same-origin read and validation | `apps/web/src/lib/floor.ts`, exact floor rewrite in `next.config.mjs` |

Migration 0004 preserves migrations 0001–0003. It adds MANUAL policy defaults, restaurant floor versioning, zones, tables and canonical connections. Composite foreign keys prevent cross-zone/cross-restaurant edges, including direct invalid database writes. CHECK constraints reject capacities outside 1–8, unknown states, nonpositive versions, and coordinates outside 0–100, including NaN/infinity.

Fixture IDs derive deterministically from the restaurant. The layout is Main T1(2)–T2(4)–T3(8), Terrace T4(6)–T5(4)–T6(2), and isolated Private T7(8). Provisioning locks floor before queue, confirms the connected database, and rejects an existing conflicting layout. Repeating it preserves states, IDs, versions, passwords and queue data; it does not reactivate disabled graph edges or enable SEATING.

## Local verification

Runtime: Go 1.26.10, PostgreSQL 18.6 and Node 22.23.4. Go checks used the existing `dinequeue-ep4-go-check` image with current source mounted; no image source, Dockerfile or CI workflow was changed. Browser checks used installed Chrome and a Next production build.

The disposable regression database has an empty public schema; individual Go tests create isolated schemas. Browser tests use a separate seeded database. An initial combined setup failed the existing empty-database foundation precondition; the final regression run uses the separate databases.

| Command | Actual final result |
|---|---|
| `npm test` | PASS — 14 test cases |
| `npm run lint` | PASS |
| `npm run typecheck` | PASS |
| `npm run build` with test `API_INTERNAL_URL` | PASS; `/host/floor` included |
| From `apps/api`: `go test -race -tags=integration -count=1 -json ./...` | PASS — 71 top-level tests, including integration tests |
| From `apps/api`: `go vet ./...` | PASS |
| From `apps/api`: `go build -buildvcs=false -o /evidence/api ./cmd/api` and `-o /evidence/db ./cmd/db` | PASS; binaries used for API/CLI checks |
| Targeted final rerun: `go test -race -tags=integration -count=1 -run TestFloor -v ./internal/queue ./db` | PASS — all 5 inventory/migration cases, including added service-authorization and disabled-edge preservation assertions |
| `npm run test:e2e --workspace @dinequeue/web` | PASS — 39/39 browser cases (S1/Lab 7 plus 4 floor cases) |
| PostgreSQL container restart; compare persisted table rows and reopen Host floor | PASS — table snapshot identical; readiness HTTP 200 and Terrace floor readable |
| `db migrate`, `db seed`, `db provision-floor --queue-id=10000000-0000-4000-8000-000000000001 --confirm-database=dinequeue_floor_test` | PASS on disposable database |

The four floor browser cases cover selection and keyboard equivalence, 360px/1280px layouts, failed/malformed reads and recovery, denial clearing protected content, empty configured floor, selection invalidation, hidden-page pause and unmount cleanup. They reuse the existing Lab 7 polling coordinator. Screenshot review confirmed readable states and no page-level horizontal overflow.

Actual new Go test names:

1. `TestFloorAccessAndEmptySnapshot`
2. `TestFloorInventoryAndReadOnlySnapshot`
3. `TestFloorProvisionPreservesLiveDataAndRejectsConflicts`
4. `TestFloorConcurrentProvisionAndConstraints`
5. `TestFloorMigrationPreservesLegacyRows`

The migration test builds the actual 0001–0003 schema, inserts WAITING/CALLED/COMPLETED tickets, passwords and a day counter, then applies migration 0004 twice. Persisted legacy row contents remain identical and the queue defaults to MANUAL.

The pre-implementation floor test failed because the missing endpoint returned 404 instead of the expected queue-scope denial. Local RED/GREEN logs, final regression output, source manifest and UI captures are stored outside the repository at `/tmp/dq-s2-floor-evidence/`. The task-specific API/DB servers were stopped after verification. No real credentials are part of this evidence or fixture.

## STS coverage and remaining work

| STS | Inventory result and boundary |
|---|---|
| S2-T-001 | Local inventory snapshot, capacities, approved edges, modes, versions and empty floor verified; real assignments await #52 |
| S2-T-002/033 | Guest/Customer/ungranted Host and unknown/invalid queue GET rejection verified; new lifecycle mutation boundaries remain with their feature issues |
| S2-T-034 | Shared physical table state and exact queue grant verified; assignment privacy cannot be claimed before #52 |
| S2-T-042/052 | Migration 0004 preservation, idempotent/concurrent provisioning, busy state/password preservation and configuration-conflict rejection verified |
| S2-T-049 | Floor keyboard/list selection, status text and responsive layout verified; future lifecycle dialogs pending |
| S2-T-050/057 | Read failures, malformed payloads, stale snapshot recovery, hidden/unmounted views and access denial verified; preview/write-conflict handling pending #51/#52 |
| S2-T-058/059 | Existing local regression and frontend/API builds recorded; integrated seating regression and clean-clone release remain later work |
| S2-T-060 | NOT RUN — no Issue #50 cloud deployment claimed |

Matching, call/hold, seating, no-show, cancellation, checkout, cleaning and explicit seating activation remain in #51–#54. Full S2 acceptance, teammate review, PR/CI URLs and accepted release commit must be recorded during integration.

## Local setup for manual review

Use a dedicated local synthetic database whose name ends `_demo`, `_uat` or `_test`. Set `DATABASE_URL`, `APP_ENV` and `APP_ORIGIN` in your shell for that database; the CLI does not load `.env` automatically. Keep credentials local. Existing `SEED_*_PASSWORD` variables are needed only when creating demo accounts with the existing seed command.

From `apps/api`, with Go 1.26.10 available:

```bash
go run ./cmd/db migrate
# Only if this dedicated local database has no demo accounts yet:
go run ./cmd/db seed
go run ./cmd/db provision-floor \
  --queue-id=10000000-0000-4000-8000-000000000001 \
  --confirm-database=dinequeue_demo
```

Replace `dinequeue_demo` with the exact connected synthetic database name. A mismatch or a general production database name is refused. For demo/UAT production-mode account provisioning, keep using the separately guarded `provision-demo` process; `seed` remains disabled in production. Floor provisioning is a separate explicit step.

Start the API and web using the existing local setup, sign in as a granted Host, then follow **View restaurant floor** from the queue board. Inspect Terrace T4+T5, switch to List view, test selection using keyboard, and return to the queue board to confirm the existing MANUAL workflow remains available.
