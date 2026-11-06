# GORM foundation revision — execution evidence

Date: 6 November 2026 (Asia/Bangkok).
Decision: DEC-026, explicitly authorized by the user in chat.
Base commit: `9b4e832`; these checks ran against the modified, uncommitted worktree.
At execution time no new commit, push, PR, CI run or deployment had been performed.
Subsequent user-authorized commits capture this work; they do not retroactively turn
the earlier worktree test runs into checks executed on a Git SHA that did not exist yet.

## Scope

- Replace direct pgx pool and sqlc-generated access with GORM 1.31.2 and the
  PostgreSQL driver 1.6.3. pgx 5.12.0 remains an indirect driver dependency.
- Keep bounded lazy startup, health endpoints, generic errors, request deadlines,
  shutdown pool cleanup and explicit SQL migration policy.
- Remove sqlc config/queries/generated code and the Makefile generator check.
- No application models, domain migrations or F-01..05 functionality added.

## Actual commands and results

Go commands used `/tmp/dineq-toolchain/go/bin/go` (Go 1.26.10), with
`GOCACHE=/tmp/dineq-go-cache` and `GOMODCACHE=/tmp/dineq-go-mod`, from `apps/api`.

| Command / check | Actual result |
|---|---|
| Initial `go test ./internal/platform/postgres` after adding the adapter tests | RED: old pgx pool had no `DB` method required by the new ORM adapter contract |
| `go get gorm.io/gorm@v1.31.2 gorm.io/driver/postgres@v1.6.3` and `go mod tidy` | Completed; direct/indirect module requirements and checksums updated |
| `go test -race ./...` | PASS; adapter tests ran, unchanged config/HTTP package results were cached |
| `go vet ./...` | PASS |
| `go build -buildvcs=false -o /tmp/dineq-gorm-api ./cmd/api` | PASS |
| `go test -tags=integration -race -count=1 ./...` with temporary TEST_DATABASE_URL | PASS; fresh run against isolated real PostgreSQL 18.1 |
| `go mod verify` | PASS; all modules verified |
| `git diff --check` | PASS |
| Docker daemon availability check | Unavailable: Docker reported it could not connect to the daemon; no image build/run claimed |

The initial RED is a compile-time API mismatch demonstrating the access-layer replacement,
not evidence that queue/seating business rules were implemented or tested.

## Verified behavior

- `TestOpenIsLazyWithBoundedPool`: GORM initialization succeeds without a running DB;
  underlying pool initially has zero connections and a maximum of 10.
- `TestOpenRejectsMalformedDSNWithoutSecrets`: malformed PostgreSQL URL returns a generic
  error without its credential value.
- `TestPostgresReadiness`: SELECT 1 through GORM succeeds on real PostgreSQL and honors
  a canceled operation context.
- `TestOpenDoesNotCreateApplicationTables`: empty `public` schema stays empty;
  application startup performs no AutoMigrate or schema creation.
- `TestGORMTransactionCommitAndRollback`: GORM Create on a connection-local temporary
  fixture table is rolled back on error and persisted on successful transaction commit.
  This is a test fixture, not an implemented application model.
- Existing config and Fiber HTTP tests remain part of the unit/integration runs.

A temporary Python subprocess harness initialized a separate PostgreSQL cluster using
the installed PostgreSQL 18.1 binaries. It generated a random password and selected
temporary ports, then exercised the compiled API:

1. Before DB start: liveness 200, readiness 503 (lazy startup).
2. After DB start and successful integration suite: readiness 200.
3. After controlled DB stop: liveness stays 200, readiness returns generic 503.
4. API log inspection: generated DB password and ORM SELECT 1 text absent.

All runtime checks passed. The API/DB processes were stopped and the cluster, temporary
fixture tables and credential file removed in cleanup. Existing databases were not used.

## Limitations and remaining work

- Docker daemon was unavailable; the updated dependency set was not built/run in its image.
- PostgreSQL 18.6 container and target Node image were not revalidated here.
- Frontend source was unchanged; its tests/build were not rerun for this backend-only revision.
- SQL migration tooling and domain models remain future S1 work. No startup AutoMigrate.
- Auth/RBAC/sessions/CSRF, queue concurrency and all S2/S3 behavior remain unimplemented.
- Original pgx/sqlc results are retained in `sprint1-foundation-results.md` as historical evidence.

## Official implementation references

GORM PostgreSQL setup and its underlying driver are documented in
[Connecting to PostgreSQL](https://gorm.io/docs/connecting_to_the_database.html).
The shared business transaction policy follows
[GORM transactions](https://gorm.io/docs/transactions.html); pooling and cleanup use
the [generic sql.DB interface](https://gorm.io/docs/generic_interface.html).
Schema evolution remains explicit SQL migrations rather than automatic startup
[migration](https://gorm.io/docs/migration.html).
