# Sprint 1 foundation execution evidence

**Historical access-layer record:** these runs describe the original pgx/sqlc
foundation. User-authorized DEC-026 later replaces that layer with GORM; see
`sprint1-gorm-results.md` for the separate current implementation verification.
Do not interpret successful sqlc checks below as tests of the ORM revision.

Date: 6 November 2026 (client timezone Asia/Bangkok).
Scope: FD-AC-01..07 in `../features/sprint1-foundation-contract.md`.
Original foundation execution had no Git SHA / branch / PR / CI URL because Git was
not initialized then. Git has since been initialized on `main`; local import commits
are recorded separately below. No release, deployment or teammate review is claimed.

## Environment

- Host Node v22.17.0, npm 10.9.2; target `.nvmrc`/container Node 22.23.4 not executed.
- Verified official Go 1.26.10 toolchain unpacked only under `/tmp/dineq-toolchain`.
- sqlc 1.31.1 official release archive SHA-256 verified against release asset metadata;
  binary under `/tmp/dineq-sqlc`.
- Real isolated PostgreSQL 18.1 (installed host binaries); new cluster under
  `/tmp/dineq-foundation-pg`, random password, temporary local ports, no existing DB touched.
- Go test cache/module cache under `/tmp/dineq-go-cache` and `/tmp/dineq-go-mod`.
- Docker CLI present, daemon unavailable. Compose config validation works with elevated execution.

## Actual results

| Command / check | Actual result |
|---|---|
| `npm install --ignore-scripts --no-audit --no-fund` | Completed; lockfile generated, package lifecycle scripts disabled |
| `npm test` | PASS; Node test runner for API upstream configuration |
| `npm run lint` | PASS; final ESLint run has no warnings/errors |
| `npm run typecheck` | PASS |
| `tsc --noEmit --incremental false -p apps/web/tsconfig.json` with `.next` temporarily removed | PASS; typecheck does not depend on a previous build |
| `npm run build` | PASS outside sandbox; Next.js production compilation, TypeScript and static page generation completed |
| `go test -race ./...` from `apps/api` | PASS; config and HTTP tests, no Go race report |
| `go vet ./...` from `apps/api` | PASS |
| `go build -buildvcs=false -o /tmp/dineq-api ./cmd/api` | PASS |
| `sqlc generate` | PASS; generated pgx/v5 query bindings checked into source directory |
| `sqlc diff` | PASS; checked-in bindings match SQL inputs |
| `go mod verify` | PASS; all modules verified |
| `go test -tags=integration -race ./...` with isolated `TEST_DATABASE_URL` | PASS against real PostgreSQL; includes generated SELECT 1 readiness query |
| `docker compose config --quiet` with non-secret validation-only environment values | PASS |
| `docker compose --env-file .env.example config --quiet` without a password | Expected rejection; required password enforced, no repository `.env` created |
| Production Next server + API + real DB smoke | PASS: landing page HTTP 200, direct readiness 200, proxied `/api/health/ready` 200 |
| Controlled DB outage with the same running API/web | PASS: liveness 200; direct and proxied readiness 503, generic response body |
| `npm audit --omit=dev --json` | Completed; registry reported 0 production dependency vulnerabilities at execution time |
| Docker image build / `docker compose up` | NOT RUN successfully: daemon cannot be reached |
| F-01..05 / T-001..036, S2/S3 tests | NOT RUN; product workflows are not implemented |

The Go commands above used `/tmp/dineq-toolchain/go/bin/go` and the cache environment
variables listed above; sqlc used `/tmp/dineq-sqlc/sqlc`. The real DB integration
suite and runtime checks were launched by a temporary Python subprocess harness.
It started the isolated DB, ran the integration command, started the compiled API
and production Next server, checked HTTP responses, stopped DB to check failure
behavior, and terminated all temporary server processes in `finally` cleanup.
The temporary PostgreSQL cluster and its random credential file were removed after checks.

## Test names / source mapping

- `apps/web/tests/api-origin.test.mjs`: allowed host/container origins and rejection
  of invalid protocols, credential-bearing URLs, paths, queries and fragments without echoing secrets.
- `apps/api/internal/platform/config/config_test.go`:
  `TestLoadValidConfig`, `TestLoadRejectsInvalidConfigWithoutSecrets`,
  `TestProductionRequiresHTTPSAndVerifiedDatabaseTLS`.
- `apps/api/internal/transport/http/server_test.go`:
  `TestLivenessDoesNotRequireDatabase`,
  `TestReadinessReportsDatabaseStateWithoutLeakingErrors`,
  `TestUnknownRouteHasGenericJSONError`, `TestPanicReturnsGenericError`,
  `TestMethodNotAllowedPreservesClientErrorStatus`.
- `apps/api/internal/platform/postgres/pool_integration_test.go`:
  `TestPostgresReadiness` (explicit integration build tag; missing DB config fails).

## Failed attempts and corrections

- Before config implementation, `go test ./internal/platform/config` failed with
  `undefined: Load`. This is an initial compile-time RED record, not proof of any
  completed business-rule TDD cycle or Git commit history.
- Initial frontend lint reported anonymous default-export warnings; named config exports fixed them.
- Initial npm dependency resolution reported ESLint 9 deprecated; final manifest/lock pins ESLint 10.14.0.
- Sandboxed Next build failed parsing its TypeScript CLI subprocess output; the unchanged
  compiler checks/build succeeded outside sandbox. Typechecking was not disabled.
- Initial Go build failed obtaining VCS status. This unversioned directory uses
  `-buildvcs=false`; no release SHA is invented.
- The first runtime harness attempt completed DB integration tests but could not launch
  the API because the binary was not yet produced by that failed build. After building
  successfully, the complete runtime/outage harness passed and cleaned up its processes.
- Docker discovery initially hit sandbox permissions; outside sandbox the daemon was
  still unavailable. No Docker runtime success is inferred from Compose validation.

## Remaining verification / risks

- Actual Compose image builds/runtime, PostgreSQL 18.6 image and Node 22.23.4 image are unverified.
- Browser visual/keyboard checks, domain migrations/seeds, authentication/RBAC/CSRF,
  and all queue business rules remain outside this foundation task.
- Container versions are patch-pinned tags, not immutable digests. They still require
  image review and refresh before a release.
- npm audit covered production dependencies only; zero reported findings is not a security guarantee.
- The original foundation task left SRS party-size/AC-007 inconsistencies unchanged.
  The subsequent v2 baseline import explicitly corrects those two documentation issues;
  unresolved domain decisions remain pending, and no product tests are claimed.

## Initial import checks — 6 November 2026

The user requested at least five focused local commits. Existing artifacts are grouped
by specifications, web, API, Docker/local configuration, then README/evidence.
This is an import of work already performed, not a reconstructed TDD history.

Source references:

- `1eb4880`: v2 requirements and contracts, including party-size and AC-007 corrections.
- `6905726`: frontend foundation and tests.
- `1ef505d`: backend foundation and tests.
- `105d7eae07b35fe23f6b04721ff21cadda37beb5`: Docker/local configuration.

| Actual command/check during import | Result |
|---|---|
| `npm test` | PASS; 2 tests |
| `npm run lint` | PASS; no reported warnings/errors |
| `npm run typecheck` | PASS |
| `npm run build` | PASS; production compilation, TypeScript and static generation completed |
| `go test -race ./...` from `apps/api` | PASS; cached package results, no reported races |
| `go vet ./...` from `apps/api` | PASS |
| `go build -buildvcs=false -o /tmp/dinequeue-api ./cmd/api` | PASS |
| `sqlc diff` from `apps/api` | PASS |
| `docker compose config --quiet` with non-secret validation-only values | PASS |
| v2 README/SRS/AC consistency assertions | PASS |
| Recognizable credential-pattern scan of import candidates | No matches for scanned private-key/AWS-key/GitHub-token patterns |
| Staged scope review and `git diff --cached --check` | Checked before each commit; whitespace cleanup performed in initial documents |

Web/API checks above ran on the worktree sources before their import commits;
those commits capture the same tested code. This is not a CI run or a claim that
tests executed against a Git SHA that did not yet exist. Go used the same explicit
toolchain/cache locations as the original run. The local Compose check ran again
after staging its configuration. No Git hooks were bypassed and no files were pushed.

Real-DB integration, outage smoke checks, dependency audit and Docker runtime checks
were not repeated during this import. Their original results/limitations remain
separate above; F-01..05 product tests remain NOT RUN.
