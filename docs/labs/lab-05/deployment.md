# Lab 5 — Cloud readiness and Docker deployment notes

**Scope:** Episode 2 records cloud readiness; Episode 3 records production-image packaging and [its local verification results](ep3-results.md). Episode 4 below adds pull-request CI for Issue #15. Episode 5 adds first-deployment preparation below; actual GCP execution is recorded separately. Release automation belongs to EP7. See each episode's evidence for actual execution/review status.

## Environment configuration inventory

The names below were compared with `rg -n 'os\.(Getenv|LookupEnv)|process\.env|NEXT_PUBLIC_' apps/api apps/web` and `.env.example`. Keep actual credentials in the local, ignored `.env` or an approved secret manager; never add their values here.

| Variable | Consumer | Purpose | Secret / scope |
|---|---|---|---|
| `APP_ENV` | Go API and database CLI | Selects `development`, `test`, or `production` configuration. | Non-secret |
| `APP_ORIGIN` | Go API | Exact allowed browser origin for authentication/CSRF protections; production requires HTTPS. Local Compose uses `http://localhost:3000`. | Non-secret; deployment-specific |
| `DATABASE_URL` | Go API and database CLI | PostgreSQL connection URL. Local Compose uses service host `db`; production TCP requires verified TLS; explicit managed Cloud SQL sockets follow DEC-032. | Secret: contains DB credentials |
| `API_PORT` | Go API sidecar | Optional listener override for a non-ingress API container; otherwise uses `PORT`. Cloud plan uses `8081`. | Non-secret |
| `CLOUD_SQL_INSTANCE` | Go API/database CLI | Explicit managed instance connection name; permits only its exact `/cloudsql/...` Unix socket in production. Blank for ordinary TCP/local use. | Non-secret |
| `ENABLE_DEMO_PROVISION`, `CONFIRM_DEMO_DATABASE` | Explicit `provision-demo` CLI only | Requires `true` and exact connected DB name ending in `_demo`/`_uat`; does not enable production `seed`. | Non-secret; operator controlled |
| `DEMO_CUSTOMER_PASSWORD`, `DEMO_CUSTOMER2_PASSWORD`, `DEMO_HOST_PASSWORD`, `DEMO_OTHER_HOST_PASSWORD` | Explicit provisioning job only | Four credentials supplied from named Secret Manager versions, not the web/API service. | Secrets |
| `PORT` | Go API | HTTP listener port; defaults to `8080`. The API listens on `:<port>` inside its container. | Non-secret |
| `API_INTERNAL_URL` | Next.js configuration and server-side session helper | API origin used for Next.js rewrites/server requests. Compose sets `http://api:8080`; a host-run web process uses `http://127.0.0.1:8080`. | Non-secret; server-side only, also affects the Next.js build |
| `WEB_HOST_PORT`, `API_HOST_PORT` | Production-image local Compose only | Loopback host ports; defaults 3002 and 8082. Web origin follows `WEB_HOST_PORT`. | Non-secret |
| `WEB_PORT` | Production-image local Compose/Next.js | Web container listener; defaults 3000. API listener uses `PORT`. | Non-secret |
| `NEXT_TELEMETRY_DISABLED` | Next.js/Docker configuration | Disables Next.js telemetry when set to `1`. | Non-secret |
| `POSTGRES_USER` | Docker Compose/PostgreSQL | Local database role name. | Non-secret |
| `POSTGRES_DB` | Docker Compose/PostgreSQL | Local database name. | Non-secret |
| `POSTGRES_PASSWORD` | Docker Compose/PostgreSQL | Local database role password. | Secret; leave blank in `.env.example` |
| `SEED_CUSTOMER_PASSWORD`, `SEED_CUSTOMER2_PASSWORD`, `SEED_HOST_PASSWORD`, `SEED_OTHER_HOST_PASSWORD` | Explicit demo seed CLI only | Operator-supplied passwords for the four local demo users; 12–128 UTF-8 bytes each. Seeding is disabled in production. | Secrets; blank in `.env.example`, never display or commit values |

The source scan also finds test-only variables: `TEST_DATABASE_URL` for isolated Go integration tests; `F01_BASE_URL` and `F01_TEST_PASSWORD` for Playwright E2E tests; optional `F01_BROWSER_CHANNEL` to select an installed browser locally (CI defaults to its matching bundled Chromium); and optional `S1_REVIEW_SCREENSHOT_DIR` for review-test output. `EP3_TICKET_FILE` is also a local smoke-script output path for persistence evidence. These are not application runtime configuration and are intentionally not defaulted in `.env.example`. No `NEXT_PUBLIC_` variables are currently used; do not expose server credentials through client-prefixed variables.

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

## EP2 topology proposal — superseded for EP5 by DEC-032

**Historical EP2 proposal, not the selected EP5 topology; no resources created:** a Next.js web service and Go Fiber API on Cloud Run, with persistent PostgreSQL on a course-approved managed service (Cloud SQL is a candidate), runtime credentials in Secret Manager, and GitHub Actions authentication through short-lived workload identity federation rather than a committed service-account key.

Pending team/TA confirmation: approved GCP services and region; Cloud SQL versus another permitted persistent PostgreSQL option; private connectivity/Cloud SQL connector and service-to-service ingress; billing/budget limits; and the approved GitHub-to-GCP identity setup. The user selected the EP5 topology in DEC-032. Confirm the actual project/region/budget and inspect the dry run before billable resource creation. GitHub federation is EP6 work.

## Episode 2 boundary

Do not add production Dockerfiles, CI workflows, GCP resources, a cloud deployment, or Sprint 2 behavior in this episode. Those are handled by the separate Lab 5 episodes/issues.

## Episode 3 — Production Docker plan (Issue #21)

**Status:** Production images and local Compose implemented and verified; see [EP3 execution results](ep3-results.md) for the tested SHA, commands, logs and limits. Teammate review/merge remains pending.
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
- Production API configuration requires an HTTPS `APP_ORIGIN` and `sslmode=verify-full` for ordinary TCP. EP5 adds only the explicit managed Cloud SQL socket path described in DEC-032; retain the original TCP validations. A local HTTP/non-TLS test may run production-built images with explicitly documented local API `APP_ENV=development`; this is packaging verification, not evidence of production HTTPS/TLS behavior. Next.js still uses its production server.
- Demo seed remains an explicit local setup action, disabled when `APP_ENV=production`. Seed before the smoke test with operator-supplied local passwords; application startup must not silently seed or reset accounts. Secure cloud account provisioning belongs to the deployment episode.
- Document the final Compose commands, host ports, environment-file handling and service names after implementation. Use a separate Compose project, database volume and available ports for EP3 tests; apply the same project/file options to every command, including migration and down/up.

### Verification sequence and acceptance criteria

The local verification items below have execution evidence in [EP3 results](ep3-results.md). Review/merge is tracked separately:

- [x] Review the Docker/Compose diff and confirm no source or configuration change expands Sprint 1 business scope.
- [x] Validate the production Compose configuration without printing expanded secrets; build the production web, API and database-tool targets.
- [x] Start PostgreSQL, run versioned migrations, explicitly seed local demo accounts, and start the production-built API/web images in the documented order.
- [x] Verify API `/health/live` returns 200/`ok`, API `/health/ready` and web `/api/health/ready` return 200/`ready`. Verify a database outage returns readiness 503 and recovery restores 200; always restore the test database afterward.
- [x] Verify configured listener ports and non-root runtime users. Confirm application startup uses compiled/built artifacts and does not depend on host source mounts.
- [x] Smoke-test Customer login/join/own-ticket and Host FIFO Call Next/Complete, preserving role and ticket privacy checks.
- [x] Create a separate ticket for persistence evidence. Record its code/status, stop the verification Compose project with `down` (never `down -v`), restart with `up -d`, and confirm the same ticket and status through the app without resetting or reseeding the database.
- [x] Commit the implementation and verify a clean clone of that exact commit can build/run using a fresh local environment file populated from `.env.example`, with no old `.env`, host `node_modules` or `.next` artifacts.
- [x] Run web tests, lint, typecheck and build; run Go tests with `-tags=integration -race -count=1` against isolated real PostgreSQL and run Go vet. Record actual results and any skipped checks.
- [x] Review tracked files, build context, image contents/configuration and captured logs for accidental credentials; redact secrets from all evidence.
- [x] Replace planned instructions with verified commands and record dates, tested SHA, container/health results, clean-clone outcome and before/after persistence evidence here or in a linked results record/PR. Do not reuse EP2 results as an EP3 execution claim.
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

At the end of Step 3, Compose integration, migration execution, recovery, login/queue smoke, persistence, clean-clone and regression checks were still pending. They were subsequently executed; their separate results are recorded in [EP3 results](ep3-results.md), not inferred from the Step 3 image checks.


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

### Windows guide — PowerShell or WSL

**Verification status:** these Windows instructions are based on the repository configuration and official Docker/Microsoft documentation. They have not been executed on a Windows host. Record the Windows tester, date, commit SHA and actual results after following them; the Linux results above do not establish a Windows PASS.

#### 1. Prepare Docker Desktop and choose a terminal

Install Git and [Docker Desktop for Windows](https://docs.docker.com/desktop/setup/install/windows-install/) using their official installers. Start Docker Desktop and wait for its engine to be running. These images require **Linux containers**, not Windows containers. If using the WSL 2 backend, follow Docker's [WSL setup and integration instructions](https://docs.docker.com/desktop/features/wsl/). For a WSL terminal, enable integration for that Linux distribution in Docker Desktop.

Choose one terminal workflow for the steps below:

- **Native PowerShell:** use the numbered PowerShell steps in this section, from the repository folder containing `compose.production.yaml`.
- **WSL (for example Ubuntu):** use a checkout inside the Linux distribution, enable its Docker Desktop integration, and follow the Bash commands in the sections above. Run `chmod` and the `EP3_TICKET_FILE=... node ...` syntax inside WSL, not PowerShell. Host Node 22 is needed inside WSL only for the optional helper.

Go and Node do not need to be installed on Windows to build/start the containers or test them through a browser. The optional Node smoke helper requires host Node 22 (matching the repository's Node engine range). No host `npm install` or `npm run dev` is needed for the Docker app.

#### 2. Check the checkout and create local configuration (PowerShell)

Use the branch/commit being reviewed for EP3, or the accepted integration branch after merge. Open PowerShell at that checkout's root and run each command separately. Stop and resolve any error before continuing:

```powershell
git status
git rev-parse HEAD
docker --version
docker compose version
docker info --format '{{.OSType}}'
```

The final command must report `linux` and confirms that the daemon is reachable. If Docker cannot connect, open Docker Desktop and wait for the engine; do not reset the database to fix an engine connection error.

Create the environment file only if it does not already exist, then edit it:

```powershell
if (-not (Test-Path .env.docker)) { Copy-Item .env.example .env.docker }
notepad.exe .env.docker
```

Fill `POSTGRES_PASSWORD` and `DATABASE_URL` with matching credentials and host `db`, as described under Fresh local configuration. Fill all four `SEED_*_PASSWORD` values (12–128 bytes) for this machine's demo accounts. Leave the example host ports 3002/8082 unless they are occupied. Save the file as `.env.docker`, not `.env.docker.txt`. PowerShell does not use `chmod`; keep this local file in your own account's folder and out of Git/screenshots. `git check-ignore .env.docker` should print the filename, confirming the existing ignore rule.

#### 3. Build, start, migrate and seed (PowerShell)

```powershell
docker compose --env-file .env.docker -p dinequeue-ep3 -f compose.production.yaml config --quiet
docker compose --env-file .env.docker -p dinequeue-ep3 -f compose.production.yaml build api web migrate seed
docker compose --env-file .env.docker -p dinequeue-ep3 -f compose.production.yaml up -d --wait
docker compose --env-file .env.docker -p dinequeue-ep3 -f compose.production.yaml run --rm seed
docker compose --env-file .env.docker -p dinequeue-ep3 -f compose.production.yaml ps -a
```

Expected: API running, DB and web healthy, migration `Exited (0)`. Seed runs explicitly for the new EP3 database and does not change passwords of existing accounts on reruns. This project has a different database volume from the development Compose project, even if both use the same database name or local passwords.

#### 4. Check health and use the app (PowerShell)

Use `curl.exe` explicitly so Windows PowerShell does not substitute its `curl` alias:

```powershell
curl.exe --fail -i http://localhost:8082/health/live
curl.exe --fail -i http://localhost:8082/health/ready
curl.exe --fail -i http://localhost:3002/api/health/ready
```

Expected: HTTP 200, `ok` for liveness and `ready` for readiness. If you changed host ports, use those values instead. Open `http://localhost:3002`: Customer `customer1@example.test` uses this machine's `SEED_CUSTOMER_PASSWORD`; Host `host@example.test` uses `SEED_HOST_PASSWORD`. Use separate browser profiles or normal/Incognito windows for the two roles. Cookies on `localhost` can also be shared with a development app on another port; use a separate browser context for EP3 to avoid session conflicts.

With host Node 22 installed, optionally run the HTTP journey on a dedicated seeded database with no existing active ticket for Customer One and no older waiting ticket:

```powershell
node.exe --env-file=.env.docker scripts/lab5/docker-smoke.mjs journey
```

#### 5. Verify persistence and stop safely (PowerShell)

For a manual check, create a new WAITING ticket through the browser, note its code/status and capture the card. Run:

```powershell
docker compose --env-file .env.docker -p dinequeue-ep3 -f compose.production.yaml down
docker compose --env-file .env.docker -p dinequeue-ep3 -f compose.production.yaml up -d --wait
```

Refresh My Ticket (sign in again with the same Customer if required). The same code/status must remain. Capture the result; do not seed/reset between reads and do not use `down -v`.

Alternatively, after the optional journey completes, use the Node helper instead of creating another ticket manually. PowerShell environment-variable syntax differs from Bash; keep the same terminal open so the file path is retained:

```powershell
$env:EP3_TICKET_FILE = Join-Path $env:TEMP ("dinequeue-ep3-" + [guid]::NewGuid().ToString() + ".json")
node.exe --env-file=.env.docker scripts/lab5/docker-smoke.mjs create
docker compose --env-file .env.docker -p dinequeue-ep3 -f compose.production.yaml down
docker compose --env-file .env.docker -p dinequeue-ep3 -f compose.production.yaml up -d --wait
node.exe --env-file=.env.docker scripts/lab5/docker-smoke.mjs verify
Remove-Item Env:\EP3_TICKET_FILE
```

This creates a unique private file under Windows TEMP and avoids overwriting earlier evidence. Do not share the JSON file (it contains a private ticket identifier). See [Microsoft's environment-variable reference](https://learn.microsoft.com/en-us/powershell/module/microsoft.powershell.core/about/about_Environment_Variables) for `$env:` syntax.

When finished, use the same `down` command to stop the EP3 containers while retaining their database volume. For clean-clone evidence, use a new folder, fresh `.env.docker` and a new project name such as `dinequeue-ep3-clean` consistently on every command; stop the first project before reusing its host ports.

#### Troubleshooting and Windows evidence

| Symptom | Action |
|---|---|
| `docker` is not found / daemon cannot connect | Check Docker Desktop installation, start its engine, reopen the terminal; for WSL check distribution integration. |
| Image platform/OS mismatch | Ensure Docker Desktop is using Linux containers. |
| Cannot find Compose/environment file | Return to the repo root and check the environment filename is exactly `.env.docker`. |
| Host port already in use | Stop the conflicting app or change `WEB_HOST_PORT`/`API_HOST_PORT` in `.env.docker` and rerun `up -d --wait`; use the new web port in your browser. |
| Migration fails or web stays unhealthy | Inspect the EP3 `api`, `web` and `migrate` logs with the command below. Confirm database host `db` and matching credentials; redact logs before sharing. |
| Login fails after editing a seed password | Existing database passwords are preserved; use the password originally seeded for this project. Check that you opened the EP3 URL. Do not delete a volume to troubleshoot login. |
| `node.exe` is not found | Use the browser journey, or install the supported Node 22 version for the optional helper. |

```powershell
docker compose --env-file .env.docker -p dinequeue-ep3 -f compose.production.yaml logs --tail 100 api web migrate
```

Keep actual build completion, container/health output, ticket before/after restart, tested SHA and teammate review evidence. Report failures as observed; only mark Windows verification passed after the Windows developer has executed the procedure successfully. Do not capture passwords, cookies or credential-bearing database URLs.

Official packaging references: [Next.js deployment](https://nextjs.org/docs/app/getting-started/deploying), [Docker Go guide](https://docs.docker.com/guides/golang/), and [Compose startup order](https://docs.docker.com/compose/how-tos/startup-order/).

## Episode 4 — Pull-request CI

Tracking: [Issue #15](https://github.com/bbxndg/DineQueue/issues/15); [pre-implementation contract](ep4-contract.md); [local execution results and pending GitHub evidence](ep4-results.md). Status: local checks passed; hosted CI, intentional-failure PR and reviewed release evidence remain pending. No hosted PASS or merge is implied by the workflow's presence.

`.github/workflows/ci-docker.yml` is named **CI & Docker Build** and retains the stable job ID/name **test-and-docker**. PRs targeting `gcp-staging` or `main` run it; `workflow_call` allows EP7 reuse and `workflow_dispatch` permits manual runs once GitHub recognizes the workflow on the default branch. There are no path filters that could skip a required check. EP6 must select the actual `test-and-docker` check from a completed run when configuring required checks; this episode does not change branch protection.

The single job runs on Ubuntu 24.04, with a 30-minute timeout and cancellation of superseded runs on the same workflow/ref. It uses only `contents: read`, checkout without persisted credentials, official Actions pinned to full verified commit SHAs, Node from `.nvmrc` and Go 1.26.10. It has no GCP credentials, OIDC write permission, deployment environment or `pull_request_target` trigger.

Checks run in this order:

1. Record the checked-out revision and PR head SHA. GitHub normally tests the synthetic PR merge commit; retain both revisions in the evidence.
2. Validate the workflow with actionlint 1.7.12 from its official release, verifying the published SHA-256 before executing it.
3. Syntax-check and stub-test the imported course deployment reference scripts, then run their three Python offline smoke tests. These are reference tests, not DineQueue cloud smoke/deployment evidence; read [deploy/README.md](../../../deploy/README.md) before any future live use.
4. Run seven offline tests for CI configuration privacy/isolation, validate ports/project identifiers before creating private files, and generate disposable credentials/configuration under runner temporary storage.
5. Check Go formatting without rewriting source, download pinned modules, run vet and all Go tests with integration tags, race detection and cache disabled for execution. The job's PostgreSQL service uses an empty disposable database; no developer/cloud database is involved.
6. Install web packages with `npm ci --ignore-scripts`, run unit tests/lint, build Next.js, then typecheck generated route types on the clean checkout.
7. Validate Compose and build API, web and database-tool production image targets. Nothing is pushed to a registry.
8. Install the Chromium revision matching the pinned Playwright package, start a separate local production-built app database, run migration and explicit seed, and execute the existing browser suite with one worker.
9. Always remove only the CI-created Compose project/volume and its private temporary configuration. This cleanup is not the EP3 persistence test and must never target a developer's project.

The PostgreSQL service password is an intentionally public synthetic CI-only value. Browser database and seed credentials are random per run and masked with GitHub's `add-mask` mechanism before use. Private environment files, cookies, traces and screenshots are not uploaded. The service and browser databases are separate so the integration suite's empty-schema checks remain valid. `scripts/ci/prepare-env.mjs` does not read a local `.env`; its exported GitHub environment values are paths/project names rather than passwords.

From the repository root, run the local CI-helper checks below (no Docker or GCP required):

```bash
python3 -m unittest discover -s scripts/ci/tests -p 'test_*.py' -v
for script in deploy/deploy.sh deploy/teardown.sh deploy/release.sh; do
  bash -n "$script"
done
bash deploy/tests/test_deploy_stub.sh
bash deploy/tests/test_release_stub.sh
python3 -m unittest discover -s deploy/tests -p 'test_smoke.py' -v
```

For a local equivalent of app checks, follow EP3's isolated-project instructions and use a fresh database. CI-helper inputs are `RUNNER_TEMP` and the disposable service's `CI_PG_PORT`; optional local-only `CI_WEB_HOST_PORT`, `CI_API_HOST_PORT`, `CI_DB_HOST_PORT` avoid ports occupied by a developer's app. `GITHUB_ENV` receives only generated file paths and the unique project name. Do not supply GCP or personal credentials. On Windows use WSL for Bash/stub tests; the hosted job itself uses Linux.

### Required GitHub evidence and completion

- Open the EP4 feature PR into `gcp-staging` with `Part of #15`. Confirm that the check starts automatically, inspect each step and record its run URL, conclusion and tested head/merge SHAs.
- Use a separate temporary branch/PR with a clearly identified intentional failing synthetic Go test. Record the real red check, then close that PR without merging it. Do not add that failing file to the implementation branch. A failed check alone is not evidence of enforced merge blocking; EP6 verifies the repository rules.
- Fix/re-run any genuine CI failures until the implementation PR passes. Distinguish a cancelled run from a failure, and local results from hosted results.
- Obtain teammate approval and merge into `gcp-staging`. The Student Guide also requires a reviewed release PR from `gcp-staging` to `main`, with the same passing check, before EP4 is fully complete and EP5 begins. Do not merge/release or claim review on behalf of the teammate.

Evidence to retain: the passing and intentional-failure PR/run links, job/step output, tested SHAs and actual review/merge links for staging and main. Record missing or unexecuted evidence as pending. Reference: [GitHub workflow syntax](https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-syntax).

Copy the PR URL from the pull request page. Open the `test-and-docker` check, then open its workflow run summary and copy the Actions run URL from the browser address bar (`/actions/runs/<run-id>`). Keep the passing and intentional-failure run URLs separately.


## Episode 5 — First GCP deployment

Tracking: [Issue #16](https://github.com/bbxndg/DineQueue/issues/16), branch
`feat/lab5-ep5-first-gcp-deployment`, [contract](ep5-contract.md), DEC-032.
**Status: local preparation verified; see [actual local results and pending cloud gates](ep5-results.md). No GCP resource creation or deployment is claimed.**

### Selected topology and difference from the course example

The user approved one Cloud Run service with two separate production images:
Next.js web is the public ingress on 8080; Go API is a non-public sidecar on
`API_PORT=8081`. Both use localhost for proxy/server-side calls and scale together.
A third image runs explicit migration/provision jobs. Cloud SQL PostgreSQL stores
sessions/tickets outside the service. This adapts the TokTickIT single-image
example to the existing Next.js/Go modular monolith without changing S1 rules.

Only the API container receives `DATABASE_URL`. Both containers still share the
Cloud Run service identity, so this does not isolate secret-manager permissions
between the web and API processes. Jobs use a separate identity; the builder has
no database/password secret access. Migration and provision share the job identity.
See [deploy/README.md](../../../deploy/README.md) for the exact resource plan,
permissions, partial-failure handling and limitations.

Production still requires HTTPS origin and verified TLS for ordinary PostgreSQL
TCP URLs. The managed transport requires `CLOUD_SQL_INSTANCE` and exactly its
canonical `/cloudsql/PROJECT:REGION:INSTANCE` socket with no alternative hosts or
duplicate transport keys. Driver `sslmode=disable` is restricted to that local
socket; Cloud Run's managed Cloud SQL integration authenticates/encrypts the
remote connection. Never use `APP_ENV=development` to bypass this on GCP.

Service deployment uses Cloud Run v2 REST: GET the service, POST to create it
only after an explicit service-level `404 NOT_FOUND`, or PATCH an existing service
with an explicit update mask. It does not rely on PATCH `allowMissing` for first
creation. It uses a native `cloudSqlInstance` volume named `cloudsql`
mounted only into the API at `/cloudsql`. The earlier alpha CLI path generated a
CSI driver rejected by the user's live deployment; it is superseded. Do not use
that driver or replace the database socket with an empty directory. Jobs retain
the working single-container GA `--set-cloudsql-instances` integration.
POST omits the Service body's `name` and selects the name via `serviceId`;
PATCH retains the fully qualified `name` in the body and request path.

The script obtains a short-lived token through the active `gcloud` login and
captures it only in memory. It sends the service body to the fixed Google API
origin with redirects disabled, then polls the returned operation for at most
ten minutes. An operation failure, timeout or non-successful service terminal
condition stops before public access. No token, raw provider error payload or
secret value is printed. The service update specifies the revision template,
labels, ingress and latest-revision traffic; it does not change the IAM policy.

References: [native Cloud SQL volume](https://docs.cloud.google.com/run/docs/reference/rest/v2/Volume),
[service creation](https://docs.cloud.google.com/run/docs/reference/rest/v2/projects.locations.services/create),
[service update](https://docs.cloud.google.com/run/docs/reference/rest/v2/projects.locations.services/patch),
and [operation polling](https://docs.cloud.google.com/run/docs/reference/rest/v2/projects.locations.operations/get).
Live execution checks CLI authentication before resource changes/builds.
API-enable rate limits (HTTP 429) wait 60–61 seconds before retry,
with at most five attempts; permanent permission/authentication failures stop.
Failure messages categorize known quota, IAM, argument-parsing and mount errors
without printing provider payloads. An existing service without a URL can use the
bootstrap origin; a missing or invalid URL after deploy stops before public access.

For an optional offline check against an installed official SDK, run:

```bash
python3 deploy/tests/check_gcloud_sdk.py /path/to/google-cloud-sdk
```

This parses stable CLI commands and round-trips native service specs through the SDK v2 schema, with
network connections blocked. It does not invoke SDK command execution methods or
prove server-side acceptance. See the [scoped audit results](ep5-results.md#deployment-only-audit--20-november-2026).

### 5.0 — Review and validate preparation locally

From the repository root:

```bash
bash deploy/tests/test_deploy_stub.sh
bash deploy/tests/test_release_stub.sh
python3 -m unittest discover -s deploy/tests -p 'test_smoke.py' -v
```

The first command exercises DineQueue deployment planning with fake GCP responses.
The second retains the archived course release checks only; EP7 release automation
is not yet implemented. No test here creates resources. Run the existing Go suite
with integration/race tags against a separate test database; include web and
browser regressions when app/config changes. Record actual results and SHA.

Preview without any cloud calls (use your project ID, never credentials):

```bash
export PROJECT_ID=YOUR_PROJECT_ID
export REGION=asia-southeast1
export SOURCE_SHA="$(git rev-parse HEAD)"
DRY_RUN=1 bash deploy/deploy.sh --demo
```

Dry run prints the resource plan and simulated command sequence, not a deployment.
It uses placeholder image digests and no generated secrets. A feature-branch SHA
is valid for planning only. Review PostgreSQL 18 Enterprise `db-f1-micro`, fixed
10 GB storage with auto-increase off, backups, no authorized networks, dedicated
identities and source bucket, public web ingress, min 0/max 1 and concurrency 10.
Cloud SQL/storage still incur costs while they exist; budgets do not stop billing.

### 5.1–5.3 — Live Cloud Shell steps (not executed by local tests)

After preparation is reviewed/merged through `gcp-staging` into main, in Cloud
Shell sign in with the intended account, select the agreed project and verify
billing/budget. Clone the same repository or update the existing clean checkout:

```bash
gcloud auth list --filter=status:ACTIVE --format='value(account)'
export PROJECT_ID=YOUR_PROJECT_ID
export REGION=asia-southeast1
gcloud config set project "$PROJECT_ID"
git switch main
git pull --ff-only
export SOURCE_SHA="$(git rev-parse HEAD)"
DRY_RUN=1 bash deploy/deploy.sh --demo
```

The checked-out SHA must match reviewed main on GitHub. Dry run creates nothing;
review it before granting the following live confirmations. The live script also
requires main, clean Git status and a matching fetched `origin/main` before any
GCP mutation. Git-archived tracked source alone is uploaded for builds, excluding
ignored local environment files. Images use the full SHA and are resolved to
immutable digests before deployment.

Only after reviewing the concrete plan and accepting resource costs/public access:

```bash
export CONFIRM_PROJECT="$PROJECT_ID"
export ALLOW_PUBLIC_SERVICE=true
bash deploy/deploy.sh --apply --demo
```

`--demo` is opt-in. It creates four random per-account secrets and runs the
separately guarded provisioning CLI against `dinequeue_demo`. The normal production
`seed` command remains disabled. Reruns preserve existing passwords, disabled
accounts and revoked grants. Do not substitute local `.env` passwords or print
secret values. Missing/disabled secret versions or an existing DB user without its
URL secret stop the script for explicit reconciliation; no implicit rotation.

The script waits for migration, then optional provisioning, before updating the
service. Only after configuring the actual HTTPS origin does it grant public web
access. A failed operation is not a completed deployment: inspect the named
resource/job safely in Console before retrying. Do not copy raw debug logs that
might include credentials. Original cloud scripts under `deploy/reference` are
historical TokTickIT examples and must not be used as live recovery tools.

Discovery recognizes the exact requested Storage bucket `not found: 404.` and
Cloud Run `Cannot find service [...]` messages as absent resources. Other
permission/network failures stop execution. Existing source buckets are inspected
with `--raw` and must match the expected name and region, enable uniform
bucket-level access and enforce public access prevention. A mismatch stops before
bucket IAM changes or source upload; it does not modify or delete the bucket.

IAM binding additions retry at most five attempts for recognized transient
`INTERNAL`, `UNAVAILABLE`, `ABORTED` errors or a missing service account matching
the requested member. Retries use exponential backoff with jitter (1–2, 2–3,
4–5 and 8–9 seconds). Permission/authentication/invalid-argument failures stop
immediately. Builds, jobs, account creation and secret writes are not retried by
this wrapper. See [Google's IAM retry guidance](https://docs.cloud.google.com/iam/docs/retry-strategy).
These local regression checks do not establish that a live deployment succeeded.

The wrapper and dry run place global `--project`, `--quiet` and response-format
flags before the command arguments. In multi-container deployments, arguments
after `--container` enter that container's scope; appending global flags there
can produce `unrecognized arguments` before any service is created. See the
[Cloud Run CLI container flag rules](https://docs.cloud.google.com/sdk/gcloud/reference/run/deploy#--container).

### 5.4–5.6 — Verify and collect real evidence

The script configures storage auto-increase off at creation and on safe reruns;
verify that setting on the actual Cloud SQL instance. Record the canonical HTTPS
URL, source SHA, image digests, service revision and migration/provision execution
outcomes. Run the read-only public smoke check:

```bash
python3 deploy/smoke_test.py https://YOUR_SERVICE_ORIGIN
```

Without credentials it checks liveness, database readiness, login HTML and an
anonymous 401; it explicitly skips authenticated verification. An optional
`SMOKE_EMAIL`/`SMOKE_PASSWORD` supplied securely in the shell environment checks
Secure HttpOnly host-only session cookies, identity and logout; never put the
password in command arguments, screenshots, logs or Git. The full journey still
needs separate evidence: Customer login/join → Host login/call → Customer refresh
shows CALLED → Host Complete → Customer active list becomes empty. Use separate
browser profiles/private windows for Host and Customer.

Retrieve/share demo credentials privately through Secret Manager with only the
intended testers. Keep the Console secret-value view out of screenshots. The
script never prints passwords. Preserve deployment, health and journey evidence
with exact versions, then update Issue #16. Native Windows execution, actual GCP
HTTPS/connectivity, and live smoke tests must not be marked passed from local
Linux/offline results. EP8 verifies restart/redeploy persistence separately.

Official references: [Cloud Run containers](https://docs.cloud.google.com/run/docs/configuring/services/containers),
[deployment flags](https://docs.cloud.google.com/sdk/gcloud/reference/run/deploy),
[Cloud SQL from Cloud Run](https://docs.cloud.google.com/sql/docs/postgres/connect-run),
[Cloud SQL supported versions](https://docs.cloud.google.com/sql/docs/postgres/db-versions).
