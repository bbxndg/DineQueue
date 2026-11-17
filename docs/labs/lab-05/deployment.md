# Lab 5 — Cloud readiness and Docker deployment notes

**Scope:** Episode 2 records the Sprint 1 cloud-readiness review. Episode 3 below records the production Docker plan and Step 3 image checks for Issue #21. Compose integration and full acceptance verification remain pending. GCP deployment and CI/CD belong to later episodes.

## Environment configuration inventory

The names below were compared with `rg -n 'os\.(Getenv|LookupEnv)|process\.env|NEXT_PUBLIC_' apps/api apps/web` and `.env.example`. Keep actual credentials in the local, ignored `.env` or an approved secret manager; never add their values here.

| Variable | Consumer | Purpose | Secret / scope |
|---|---|---|---|
| `APP_ENV` | Go API and database CLI | Selects `development`, `test`, or `production` configuration. | Non-secret |
| `APP_ORIGIN` | Go API | Exact allowed browser origin for authentication/CSRF protections; production requires HTTPS. Local Compose uses `http://localhost:3000`. | Non-secret; deployment-specific |
| `DATABASE_URL` | Go API and database CLI | PostgreSQL connection URL. Local Compose uses service host `db`; production requires verified TLS. | Secret: contains DB credentials |
| `PORT` | Go API | HTTP listener port; defaults to `8080`. The API listens on `:<port>` inside its container. | Non-secret |
| `API_INTERNAL_URL` | Next.js configuration and server-side session helper | API origin used for Next.js rewrites/server requests. Compose sets `http://api:8080`; a host-run web process uses `http://127.0.0.1:8080`. | Non-secret; server-side only, also affects the Next.js build |
| `WEB_HOST_PORT`, `API_HOST_PORT` | Production-image local Compose only | Loopback host ports; defaults 3002 and 8082. Web origin follows `WEB_HOST_PORT`. | Non-secret |
| `WEB_PORT` | Production-image local Compose/Next.js | Web container listener; defaults 3000. API listener uses `PORT`. | Non-secret |
| `NEXT_TELEMETRY_DISABLED` | Next.js/Docker configuration | Disables Next.js telemetry when set to `1`. | Non-secret |
| `POSTGRES_USER` | Docker Compose/PostgreSQL | Local database role name. | Non-secret |
| `POSTGRES_DB` | Docker Compose/PostgreSQL | Local database name. | Non-secret |
| `POSTGRES_PASSWORD` | Docker Compose/PostgreSQL | Local database role password. | Secret; leave blank in `.env.example` |
| `SEED_CUSTOMER_PASSWORD`, `SEED_CUSTOMER2_PASSWORD`, `SEED_HOST_PASSWORD`, `SEED_OTHER_HOST_PASSWORD` | Explicit demo seed CLI only | Operator-supplied passwords for the four local demo users; 12–128 UTF-8 bytes each. Seeding is disabled in production. | Secrets; blank in `.env.example`, never display or commit values |

The source scan also finds test-only variables: `TEST_DATABASE_URL` for isolated Go integration tests; `F01_BASE_URL` and `F01_TEST_PASSWORD` for Playwright E2E tests; and optional `S1_REVIEW_SCREENSHOT_DIR` for review-test output. `EP3_TICKET_FILE` is also a local smoke-script output path for persistence evidence. These are not application runtime configuration and are intentionally not defaulted in `.env.example`. No `NEXT_PUBLIC_` variables are currently used; do not expose server credentials through client-prefixed variables.

`.env.example` already lists the runtime, local Compose, and seed variables above with non-secret defaults/placeholders. Keep `.env` local and ignored. Do not copy real values into this document, Issues, PRs, logs, or screenshots.

## Local run, migration, and seed

From the repository root, with Docker running and local values configured in `.env`:

```bash
docker compose config --quiet
docker compose up --build -d
docker compose run --rm db-tools
```

The `db-tools` command above applies versioned SQL migrations. If the local database needs demo accounts, set all four `SEED_*_PASSWORD` values locally, then run:

```bash
docker compose run --rm db-tools go run -buildvcs=false ./cmd/db seed
```

The seed command inserts missing demo records and preserves existing accounts/passwords on reruns. It is disabled in production. Do not use `docker compose down -v` to reset local data unless deletion is intentional.

## Health endpoints

With the current Compose port mapping (`api` on host port `8080`, `web` on host port `3000`):

```bash
curl --fail http://localhost:8080/health/live
curl --fail http://localhost:8080/health/ready
curl --fail http://localhost:3000/api/health/ready
```

Expected healthy responses are HTTP 200 with `{"status":"ok"}` for liveness and `{"status":"ready"}` for both readiness routes. During a database outage, use `curl -i` (without `--fail`) to capture the expected HTTP 503 response.

Observed locally on 17 Nov 2026: all three routes returned HTTP 200 with their expected response bodies after PostgreSQL recovery.

## Six cloud-readiness checks

| # | Result | Evidence / notes |
|---:|---|---|
| 1 | **Not applicable to Sprint 1** | Sprint 1 has no file-upload feature or application data that must persist in a container filesystem. PostgreSQL remains the source of persisted application data. |
| 2 | **Already OK — source review** | `apps/api/internal/auth/seed.go` reads the four passwords from environment variables, hashes them, and uses conflict-safe inserts that preserve existing accounts/passwords. `apps/api/cmd/db/main.go` disables demo seeding in production and prints only a generic completion/error message. |
| 3 | **Already OK — source review** | Authenticated routes use the authentication middleware; no development-mode authentication bypass or default password was found. Production config requires HTTPS for `APP_ORIGIN`, and production seed is disabled. |
| 4 | **Already OK — source review** | Sessions are stored in PostgreSQL `sessions`; the browser cookie contains the opaque token while the database stores its hash. There is no in-process-only session store. |
| 5 | **Already OK — source review** | The API config reads `PORT`, and `app.Listen(fmt.Sprintf(":%d", cfg.Port))` binds the listener on all container interfaces. Compose publishing to `127.0.0.1` restricts host access and does not change the container listener. |
| 6 | **Verified — local runtime check, 17 Nov 2026** | With PostgreSQL stopped using `docker compose stop db`, `curl -sS -i --max-time 8 http://localhost:8080/health/ready` returned HTTP 503 and `{"status":"unavailable"}`. After `docker compose start db`, `docker compose ps -a` showed PostgreSQL healthy; direct API and proxied web readiness both returned HTTP 200 and `{"status":"ready"}`. No volume was removed. |

Checks 1–5 above are source/configuration review; check 6 includes an actual local outage/recovery test. These results do not claim a GCP deployment or TA approval.

## Proposed GCP topology — pending confirmation

**Proposal only; no resources created:** a Next.js web service and Go Fiber API on Cloud Run, with persistent PostgreSQL on a course-approved managed service (Cloud SQL is a candidate), runtime credentials in Secret Manager, and GitHub Actions authentication through short-lived workload identity federation rather than a committed service-account key.

Pending team/TA confirmation: approved GCP services and region; Cloud SQL versus another permitted persistent PostgreSQL option; private connectivity/Cloud SQL connector and service-to-service ingress; billing/budget limits; and the approved GitHub-to-GCP identity setup. Resolve these before creating billable resources or preparing production deployment.

## Episode 2 boundary

Do not add production Dockerfiles, CI workflows, GCP resources, a cloud deployment, or Sprint 2 behavior in this episode. Those are handled by the separate Lab 5 episodes/issues.

## Episode 3 — Production Docker plan (Issue #21)

**Status:** Step 3 production image targets implemented and build/start checks executed; full Episode 3 acceptance remains pending (see results below).
**Tracking:** [Issue #21](https://github.com/bbxndg/DineQueue/issues/21), branch `feat/lab5-ep3-production-docker`, PR target `gcp-staging`.

### Packaging and startup design

| Component | Planned change | Acceptance evidence |
|---|---|---|
| Go Fiber API | Add a multi-stage production target that runs a compiled API binary as a non-root user. Retain the existing development target. | Inspect runtime user/start command; start successfully on the configured `PORT` and all container interfaces. No runtime `go run`. |
| Next.js web | Add a production target containing the built app and required runtime files, running as a non-root user with `NODE_ENV=production`. | Production server starts without source bind mounts or `next dev`; configured `PORT` works. |
| API routing | Verify Next.js server requests and browser-facing rewrites reach the same intended API. Rewrites currently depend on `API_INTERNAL_URL` at build time. | Login, health proxy, Customer and Host flows work from the built image. Document any build-time origin and rebuild requirement; do not imply runtime changes rewrite an existing build. |
| Database tools | Package the compiled database CLI and its required SQL migrations separately from API startup. | Migration succeeds before API startup, reruns safely, and startup does not invoke GORM AutoMigrate. |
| PostgreSQL | Retain PostgreSQL data in a named volume and isolate the EP3 verification project from the developer database. | The same ticket code and status remain after down/up without removing the volume. |
| Compose | Add a standalone `compose.production.yaml` for production-image verification; keep existing local development commands usable. | Config validation succeeds; database becomes healthy, migration exits 0, API and web start successfully. |
| Build context | Review `.dockerignore`, pinned base images, lockfile installation and runtime contents. | Local secrets and generated host dependencies are excluded; no real credential is baked into image configuration or layers. |

Use separate web and API images, consistent with the EP2 topology proposal. Cloud services, connectivity and deployment identity remain decisions for later episodes. Implementation must use official framework/container references and preserve the current dependency versions unless a change is justified and recorded.

### Configuration, credentials and local verification

- Supply database credentials and seed passwords at runtime through an ignored local environment file. Never pass credentials through Docker build arguments or copy that file into an image.
- The API must continue enforcing `APP_ORIGIN`, backend RBAC, CSRF and PostgreSQL-backed sessions. `DATABASE_URL` must address the intended database service; do not log it.
- Production API configuration currently requires an HTTPS `APP_ORIGIN` and `sslmode=verify-full`. Keep those validations. A local HTTP/non-TLS test may run production-built images with explicitly documented local API `APP_ENV=development`; this is packaging verification, not evidence of production HTTPS/TLS behavior. Next.js still uses its production server.
- Demo seed remains an explicit local setup action, disabled when `APP_ENV=production`. Seed before the smoke test with operator-supplied local passwords; application startup must not silently seed or reset accounts. Secure cloud account provisioning belongs to the deployment episode.
- Document the final Compose commands, host ports, environment-file handling and service names after implementation. Use a separate Compose project, database volume and available ports for EP3 tests; apply the same project/file options to every command, including migration and down/up.

### Verification sequence and acceptance criteria

All items below are pending until actual execution evidence is recorded:

- [ ] Review the Docker/Compose diff and confirm no source or configuration change expands Sprint 1 business scope.
- [ ] Validate the production Compose configuration without printing expanded secrets; build the production web, API and database-tool targets.
- [ ] Start PostgreSQL, run versioned migrations, explicitly seed local demo accounts, and start the production-built API/web images in the documented order.
- [ ] Verify API `/health/live` returns 200/`ok`, API `/health/ready` and web `/api/health/ready` return 200/`ready`. Verify a database outage returns readiness 503 and recovery restores 200; always restore the test database afterward.
- [ ] Verify configured listener ports and non-root runtime users. Confirm application startup uses compiled/built artifacts and does not depend on host source mounts.
- [ ] Smoke-test Customer login/join/own-ticket and Host FIFO Call Next/Complete, preserving role and ticket privacy checks.
- [ ] Create a separate ticket for persistence evidence. Record its code/status, stop the verification Compose project with `down` (never `down -v`), restart with `up -d`, and confirm the same ticket and status through the app without resetting or reseeding the database.
- [ ] Commit the implementation and verify a clean clone of that exact commit can build/run using a fresh local environment file populated from `.env.example`, with no old `.env`, host `node_modules` or `.next` artifacts.
- [ ] Run web tests, lint, typecheck and build; run Go tests with `-tags=integration -race -count=1` against isolated real PostgreSQL and run Go vet. Record actual results and any skipped checks.
- [ ] Review tracked files, build context, image contents/configuration and captured logs for accidental credentials; redact secrets from all evidence.
- [ ] Replace planned instructions with verified commands and record dates, tested SHA, container/health results, clean-clone outcome and before/after persistence evidence here or in a linked results record/PR. Do not reuse EP2 results as an EP3 execution claim.
- [ ] Open a PR into `gcp-staging` with `Part of #21`; obtain teammate review and merge before closing the Issue.

Evidence to capture during execution: build completion, container status (including migration exit 0), health responses, ticket before/after restart, clean-clone command output and tested SHA, plus the reviewed PR link. Store only sanitized evidence. Episode 3 does not deploy GCP resources or add CI/CD or Sprint 2 features.

### Step 3 image checks — 17 November 2026

Snapshot: uncommitted Dockerfile changes based on `5b8617bdb4728c6a54afb49a3b6d8928f970e74b`; this is not a claim that the base commit contains these changes. Repeat final acceptance on the committed EP3 snapshot.

Executed from the repository root, all three builds exited 0:

```bash
docker build --target production -f apps/api/Dockerfile -t dinequeue-ep3-api:step3 .
docker build --target db-tools -f apps/api/Dockerfile -t dinequeue-ep3-db-tools:step3 .
docker build --target production -f apps/web/Dockerfile -t dinequeue-ep3-web:step3 .
```

The API and database-tool images run compiled static binaries as UID/GID `10001:10001`. SQL migrations are embedded in the database binary; timezone data is included with the `timetzdata` build tag, and the runtime retains CA trust roots. The web image runs `next start` as user `node` (observed UID 1000), contains production dependencies and the built app, and accepts `PORT` (tested with 3100).

`API_INTERNAL_URL` is a non-secret web build argument, defaulting to `http://api:8080`. Use the same origin at runtime; rebuild the web image when the upstream origin changes because its rewrites are generated during build. Never pass credentials through this argument.

A temporary Docker network and two containers, with no host ports or database, were used for the startup checks. Requests were made using `docker exec` with Node `fetch` in the web container. Results:

| Check | Actual result |
|---|---|
| API `http://api:8080/health/live` | HTTP 200, `{"status":"ok"}` |
| API `http://api:8080/health/ready`, database unavailable | HTTP 503, `{"status":"unavailable"}` |
| Web `http://127.0.0.1:3100/api/health/live` | HTTP 200, `{"status":"ok"}` |
| Web `http://127.0.0.1:3100/api/health/ready`, database unavailable | HTTP 503, `{"status":"unavailable"}` |
| Runtime inspection | API uses `/app/api`; web uses `next start --hostname 0.0.0.0`; both non-root |

API `APP_ENV=development` was explicitly used for this local HTTP/non-TLS packaging check. Production configuration enforcement was not relaxed. Temporary containers/network were removed afterward; images remain available locally. `git diff --check` passed.

Not yet executed for EP3: production Compose integration, database-tool execution/migration, healthy-database recovery, login/queue smoke journey, persistence, clean-clone verification, full regression and final-commit verification. These belong to the remaining steps and must not be marked passed from the image checks above.


## Episode 3 — Reproducible local production-image run

This is local verification of production-built artifacts using `APP_ENV=development` for HTTP and non-TLS local PostgreSQL. Next.js uses `NODE_ENV=production` and `next start`; the API and migration CLI are compiled binaries. Production API HTTPS/TLS requirements and disabled demo seed remain enforced by the application. This Compose file is not a Cloud Run deployment configuration.

Use Docker Engine/Desktop with Compose v2, and Node 22 if running the optional smoke script. Commands below work in Linux or Windows WSL. Native PowerShell can use `Copy-Item .env.example .env.docker` instead of `cp`, omit `chmod`, and use `curl.exe` instead of `curl`. Docker itself runs the same pinned Linux images on both developers' machines.

### Fresh local configuration

From the repository root:

```bash
cp .env.example .env.docker
chmod 600 .env.docker
```

Edit the ignored `.env.docker` locally: fill `POSTGRES_PASSWORD`, set `DATABASE_URL` to `postgres://<user>:<URL-encoded-password>@db:5432/<database>?sslmode=disable` matching the `POSTGRES_*` values, and supply all four seed passwords (12–128 bytes). Use a random URL-safe password or URL-encode special characters in the URL. Do not copy a teammate's secrets. Keep the default host ports `WEB_HOST_PORT=3002`, `API_HOST_PORT=8082`, web container `WEB_PORT=3000`, API container `PORT=8080`, or select free ports before building.

The production-image Compose derives `APP_ORIGIN` from `WEB_HOST_PORT` and passes the same API upstream `http://api:<PORT>` to the web build and runtime. The existing development Compose stays on host 3000/8080. For this verification, always open `http://localhost:3002` (or your chosen web host port), not the development URL or `127.0.0.1`, because Origin checks are exact. Rebuild web if changing the API's internal `PORT`.

### Build, migrate, seed and start

Use the same environment file, project name and Compose file on every command:

```bash
docker compose --env-file .env.docker -p dinequeue-ep3 -f compose.production.yaml config --quiet
docker compose --env-file .env.docker -p dinequeue-ep3 -f compose.production.yaml build api web migrate seed
docker compose --env-file .env.docker -p dinequeue-ep3 -f compose.production.yaml up -d --wait
docker compose --env-file .env.docker -p dinequeue-ep3 -f compose.production.yaml run --rm seed
docker compose --env-file .env.docker -p dinequeue-ep3 -f compose.production.yaml ps -a
```

`db` becomes healthy, `migrate` exits 0, then API and web start; web has a readiness healthcheck. Seed is explicit and is not run by normal startup. Its profile keeps credentials out of API/web services. Migration reruns are checksum-checked and safe; seed preserves existing passwords, so editing a seed password does not change an account already stored in the volume. Use a new verification project for a genuinely fresh database instead of deleting a developer's data.

The database is not published to the host by this Compose file. Containers have no source bind mounts; the only persistent volume is PostgreSQL's project-scoped `postgres_data`.

With default host ports:

```bash
curl --fail http://localhost:8082/health/live
curl --fail http://localhost:8082/health/ready
curl --fail http://localhost:3002/api/health/ready
node --env-file=.env.docker scripts/lab5/docker-smoke.mjs journey
```

The optional `journey` script must target a dedicated seeded verification database with no existing active ticket for Customer One, and no older waiting ticket. It creates and completes a 10-person ticket, checks privacy/RBAC and CSRF/Origin rejection, reads the server-rendered Host page, and verifies logout revocation. It never prints credentials or raw session cookies. It does not replace the browser or concurrency regression suites.

For outage evidence, stop only the verification project's DB, use `curl -i --max-time 15` on direct and proxied readiness (expected 503), and always restore it with the same Compose options and `up -d --wait db`; health should return 200 again.

### Persistence evidence

After the journey, create a separate WAITING ticket. Choose a new private output path; `create` refuses to overwrite an existing file:

```bash
EP3_TICKET_FILE=/tmp/dinequeue-ep3-ticket.json node --env-file=.env.docker scripts/lab5/docker-smoke.mjs create
docker compose --env-file .env.docker -p dinequeue-ep3 -f compose.production.yaml down
docker compose --env-file .env.docker -p dinequeue-ep3 -f compose.production.yaml up -d --wait
EP3_TICKET_FILE=/tmp/dinequeue-ep3-ticket.json node --env-file=.env.docker scripts/lab5/docker-smoke.mjs verify
```

On native PowerShell set `$env:EP3_TICKET_FILE` to a local path before the Node commands. Keep this file out of Git/evidence: it contains a private ticket identifier. The script prints only ticket code/status. For screenshots, use a Customer browser window before and after restart; keep the same ticket and status. `verify` logs in again and reads the saved ticket through the app API. Do not seed/reset the database between these two reads, and never add `-v` to the persistence-test `down` command.

### Clean-clone verification

After pushing the implementation commit, clone that branch into a new folder, record `git rev-parse HEAD`, create a fresh `.env.docker` from its `.env.example`, and repeat the commands above with a new Compose project name (for example `dinequeue-ep3-clean`). Stop the previous verification project first or select other free host ports. Do not copy old `.env`, `node_modules`, `.next`, or data volumes into the clone. Docker's build cache may be reused; a clean checkout is independent of a cold image download.

Capture the build result, migration exit 0, container status, 200/503/recovery responses, smoke result, ticket code/status before and after down/up, and tested SHA. The teammate review and merged PR remain separate evidence. After verification, `down` retains the local volume for inspection; remove only an explicitly disposable audit volume when finished, outside the persistence test.

Official packaging references: [Next.js deployment](https://nextjs.org/docs/app/getting-started/deploying), [Docker Go guide](https://docs.docker.com/guides/golang/), and [Compose startup order](https://docs.docker.com/compose/how-tos/startup-order/).
