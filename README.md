# DineQueue v2 — Restaurant Waitlist, Floor & Seating Operations

> **Engineering Package v2.0 — Current project baseline, clarified 6 November 2026.**
> **Product workflows remain unimplemented.** A local Sprint 1 repository foundation now exists;
> foundation checks are recorded separately below. No instructor approval, product acceptance,
> deployment or release is claimed.
> Grounded on CPE 334 `group_work_specs_v1.pdf` (15 pages, issued 6 Oct 2026, itself marked *Draft for instructor review*); Domain-specific table algorithms, seat limits, screens, schemas and Go Fiber are **proposed project decisions**, not requirements imposed by the PDF.

## Current v2 baseline and implementation status

- Party size: **1–16**; a physical table seats **1–8**.
- Sprint 1: F-01..F-05, two core roles (`CUSTOMER`, `HOST`), manual waitlist completion.
- Sprint 2 design: approved connected sets of **1–3 tables**, atomic hold/call/seat,
  no-show, whole-group checkout and cleaning. These workflows are not implemented.
- Present source: repository foundation only. **F-01 has not started.**
- Git repository initialized on `main`, with `origin` connected to
  `https://github.com/bbxndg/DineQueue.git`. Commits import existing work as of
  6 November; they do not establish historical test-first or teammate-review evidence.
- Selecting v2 does not imply TA approval of unresolved policies in the decision register.

## Product vision

A walk-in restaurant operations platform: **Customer Waitlist Management**, **Interactive Floor & Table Management**, **Smart Queue/Table Matching**, **Combined Adjacent Tables** and **Restaurant Operations Analytics**. The central workflow remains a waiting line (CPE 334 theme 9), not advance appointment/reservation booking or POS/payment.

**The ten-person requirement:** each physical dining table seats at most **8**; a group may be **1–16** people. For party size 10, two approved adjacent tables T4(6)+T5(4), both AVAILABLE, same zone and restaurant, can be claimed/held/seated as **one atomic group assignment**. Arbitrary tables nearby on a screen cannot automatically be combined. A staff member must confirm physical compatibility. In v2 up to 3 connected tables may be joined, pending team/TA approval.

## Read these files first

1. [Thai overview](docs/00-thai-overview.md) — สรุปไทยและผลกระทบต่อ Sprint.
2. [v2 change log](docs/15-v2-change-log.md) — what was revised from original baseline.
3. [SRS](docs/01-srs.md) — actors, functional, business, nonfunctional requirements.
4. [Feature inventory](docs/02-features.md) — F-01..F-19, prioritization, planned sprint.
5. [System SDS](docs/03-system-sds.md) — modular architecture, Go proposal, permissions, transaction design.
6. [Database and API Contract](docs/04-data-and-api.md) — schema, joins, uniqueness, v1/v2 routes.
7. [Restaurant Operations Domain Design](docs/06-restaurant-operations.md) — most important new document: topology, 10-person matching, holds, checkout, concurrency and fairness.
8. [UI Specification](docs/05-ui-spec.md) and [detailed interaction specification](docs/ui/operations-interaction-spec.md).
9. [Three-sprint plan](docs/sprints/plan.md).
10. [S1 Engineering Contracts](docs/features/sprint1-contracts.md) and [S1 STS](docs/testing/sprint1-sts.md).
11. [S2 Engineering Contracts](docs/features/sprint2-restaurant-operations-contracts.md) and [S2 STS](docs/testing/sprint2-operations-sts.md).
12. [Traceability](docs/09-traceability.md).
13. [Decisions / open questions](docs/13-decisions.md).
14. [Security and quality review](docs/16-security-quality-review.md).
15. [Workflow](docs/10-workflow.md), [Issue backlog](docs/14-issue-backlog.md), [Lab 5–7](docs/labs/labs5-7.md) and [submission requirements](docs/release/submissions.md).

## Proposed stack (team to confirm)

| Layer | Technology |
|---|---|
| Frontend | Next.js + TypeScript + Tailwind CSS |
| Backend | **Go Fiber v3** + Go modules, handlers → services → repositories |
| Database | PostgreSQL |
| DB access | pgx + sqlc, SQL migration tool |
| Authentication | server-backed opaque session cookie, password hashing, CSRF protection |
| Testing | Go `testing`, PostgreSQL integration tests, Playwright E2E |
| DevOps | Docker, GitHub Actions, GCP (Lab 5) |

Official Fiber v3 docs currently say Go 1.26+ is required. Framework selection is a **proposal** following team discussion; the course encourages reusing the Lab 1–4 stack where practical. Do not install an unknown dependency, run external scripts, or commit secrets to make a prototype work.

## Three-sprint scope

| Milestone | Target features | Deadline |
|---|---|---|
| **Sprint 1** | Authentication/RBAC; customer joins party 1–16; unique ticket, own status; strict FIFO call/complete; persistence, negative/race tests, reviewed Git workflow; no digital table allocation yet | **8 Nov 2026** |
| **Sprint 2** | Floor zones; table graph; matching; atomic single/combined hold/seat; cancellation/no-show; checkout/cleaning; operations UI; negative+concurrency integration tests | **29 Nov 2026** |
| **Sprint 3** | Customer status improvements/guarded ETA; queue history; measured analytics; optional floor config/public display; UAT fixes, regression and final release | **Progress by 29 Nov; final 13 Dec** |

**Course labs also mandatory:** Lab 5 Docker/CI/GCP by **15 Nov**, Lab 6 cross-team UAT and defect fix by **22 Nov**, Lab 7 approved change + regression/metrics by **29 Nov**. Submission deadline 23:59 on the specified date.

## Primary engineering acceptance scenario

1. Customer signs in and joins party of 10, receives A-001 WAITING (valid despite no single 10-seat table).
2. Host sees T4(6) and T5(4) connected in Terrace; T7(8) alone is not eligible.
3. Host previews match, calls and **holds both** T4/T5; queue CALLED.
4. Second Host tries an overlapping table group and is rejected without a partial allocation.
5. Host confirms seating; ticket COMPLETED, assignment ACTIVE, both tables OCCUPIED.
6. Guest leaves: group checkout makes both DIRTY; cleaning returns each to AVAILABLE.
7. API rejects cross-tenant/unauthorized, wrong-state, disconnected, occupied and undersized table groups.

## Important caveats

- F-01..F-19 remain proposed product specifications, not completed features. Product tests T-001..036 and S2 tests are **NOT RUN**. Foundation source/checks are recorded separately; decision statuses remain **PROPOSED / PENDING** unless team/TA signs off.
- Sprint 1 minimum is deliberately preserved, with a versioned boundary change from max party 8 to 16. Do not silently backdate commits or claim previous tests had already validated 10 guests.
- Matching is a deterministic heuristic, not an ML model and not guaranteed globally optimal. Floor adjacency must be manager-approved, not inferred from CSS coordinates.
- If advanced scope threatens deadlines, prioritize strict S1 flow then S2 atomic combined-table seating with true DB tests; defer decorative maps, ML ETA, public display or floor editing before dropping engineering correctness.

## Source and technical reading (no execution required)

Course: `group_work_specs_v1.pdf`, Sections 5, 6.9, 7–11. Official references in `docs/16-security-quality-review.md` include PostgreSQL locking/partial indexes, OWASP session/CSRF/authorization, and Fiber v3 API docs.

## Local repository foundation

Scope: [foundation contract](docs/features/sprint1-foundation-contract.md).
Results and limitations: [foundation evidence](docs/testing/sprint1-foundation-results.md).
This is a web shell and API health/database wiring, not the Sprint 1 queue product.

```text
apps/web/          Next.js + TypeScript + Tailwind; minimal landing shell
apps/api/cmd/api/  Go Fiber entrypoint and graceful shutdown
apps/api/internal/health/           bounded readiness service
apps/api/internal/transport/http/   health handlers and generic errors
apps/api/internal/platform/         validated config and pgx/sqlc adapter
apps/api/internal/db/               sqlc-generated queries
apps/api/db/                        schema placeholder and SELECT 1 query
```

### Run the local Docker development environment

Requires a running Docker daemon and Compose. It does not require host Go/Node.

1. Copy `.env.example` to `.env` and restrict permissions (`chmod 600 .env`).
2. Generate a local password, for example with `openssl rand -hex 24`, and put it in
   `POSTGRES_PASSWORD`. There is deliberately no default password.
3. Set `DATABASE_URL` to `postgres://dinequeue:<your-password>@db:5432/dinequeue?sslmode=disable`.
   Replace the placeholder; if using a non-hex password, URL-encode its credential component.
   Match `POSTGRES_USER` and `POSTGRES_DB` if changing them. Never commit this file or print it in CI.
4. Run `docker compose config --quiet`, then `docker compose up --build`.
5. Open http://localhost:3000. Check `curl --fail http://localhost:8080/health/live`
   and `curl --fail http://localhost:3000/api/health/ready`.

PostgreSQL is private to the Compose network. Web and API published ports bind only
to loopback. The database named volume survives `docker compose down`; do not use
`down --volumes` unless intentionally deleting local data. Changing PostgreSQL environment
passwords does not update credentials in an already initialized volume.

Web source changes reload through the read-only source mount. For backend/config/dependency
changes rerun `docker compose up --build`. These Dockerfiles are development targets;
production image hardening and CI/GCP delivery remain Lab 5 work.

### Host tools and checks

Pinned runtime targets: Node 22.23.4 (`.nvmrc`), Go 1.26.10, sqlc 1.31.1, PostgreSQL 18.6.
The web also supports the existing host Node 22.17.0 used for foundation validation.
Direct dependencies and transitive resolution are recorded in `package-lock.json`,
`go.mod` and `go.sum`. No external install script is required.

```bash
npm ci --ignore-scripts
npm test
npm run lint
npm run typecheck
npm run build
npm run dev
```

`API_INTERNAL_URL` is server-only and defaults to `http://127.0.0.1:8080` for host web
development; Compose sets `http://api:8080`. Production builds bake Next.js rewrites
using the build-time value, so rebuild when changing it. No database/session secret
belongs in `NEXT_PUBLIC_*`. No general API proxy, authentication or CORS policy is implemented yet.

From `apps/api`, with Go on PATH:

```bash
go test -race ./...
go vet ./...
go build -buildvcs=false -o /tmp/dinequeue-api ./cmd/api
sqlc diff
```

Run `sqlc generate` when changing query/schema inputs. Versioned domain migrations and
demo seeds are deferred to the database/auth/queue Issues; the current SELECT 1 has no domain tables.

For a host API process, supply `APP_ENV`, `APP_ORIGIN`, `DATABASE_URL` and optional `PORT`
through the process environment, then run `go run -buildvcs=false ./cmd/api` in `apps/api`.
Go does not automatically load `.env`. A host API needs a reachable PostgreSQL instance;
the Compose-only hostname `db` will not resolve on the host.

Production config rejects HTTP origins and database URLs without `sslmode=verify-full`.
TLS termination and trusted-proxy/session policies still require implementation before release.

For the opt-in integration test, set `TEST_DATABASE_URL` to a dedicated isolated PostgreSQL
database and run `go test -tags=integration -race ./...` from `apps/api`. Missing configuration
fails explicitly instead of silently reporting a skipped integration check as success.

The `-buildvcs=false` flag supports source archives without Git metadata; it does not create
release provenance. Foundation validation originally ran before Git initialization. Keep
that execution record separate from subsequent commits, reviews, releases and CI results.
