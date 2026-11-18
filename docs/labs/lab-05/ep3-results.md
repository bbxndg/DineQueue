# Lab 5 Episode 3 — Docker execution results

Date: 17 November 2026 (Asia/Bangkok). Issue: [#21](https://github.com/bbxndg/DineQueue/issues/21).

Tested implementation commit: `ea4d7ee771849fd8743ae8b8929bc5cbad997c58`.
Later evidence/documentation commits do not change the tested Dockerfiles, Compose, environment example, smoke script, application or tests.

Status: local technical acceptance verified; teammate review and merge into `gcp-staging` are still required before closing the Issue. No GCP deployment is claimed.

## Environment and isolation

- Linux, host Node 22.17.0; Docker Compose v5.0.2. Linux/WSL instructions are provided; native Windows execution was not performed.
- Existing dependency versions and lockfiles were retained. Installation used `npm ci --ignore-scripts --no-audit --no-fund`; the installer reported a transitive ESLint deprecation warning, with no install failure. No dependency vulnerability audit is claimed here.
- API and database tools use pinned Go `1.26.10-bookworm` builds; web uses pinned Node `22.23.4-bookworm-slim`; database uses PostgreSQL `18.6-bookworm`.
- Initial project `dqep3audit` used host web/API ports 3012/8092 and non-default internal web/API ports 3100/18080. Requests through the web rewrites and server-rendered Host page succeeded, exercising both configured listeners.
- Clean-clone project `dqep3clean` used host ports 3032/8093 and default container ports 3000/8080. Each project had its own newly created database volume and fresh randomly supplied local credentials.
- Browser fixtures temporarily published only the isolated database to loopback (55412 for the initial project, 55413 for the clean clone). These overrides are outside the repository; the shipped Compose file does not publish PostgreSQL.
- Go regression used a separate empty `go_regression` database, never the app's seeded database or the developer database. Tests additionally isolate their schemas.
- All Compose app runs used local HTTP/non-TLS `APP_ENV=development` with production-built API/web artifacts. This verifies packaging, not successful production HTTPS/TLS connectivity. Production config and seed guards remain intact.

## Clean clone and production startup

An independent clone was created with:

```bash
git clone --no-local --single-branch --branch feat/lab5-ep3-production-docker /home/user/Downloads/DineQueue /tmp/dinequeue-ep3-audit/clean
git -C /tmp/dinequeue-ep3-audit/clean rev-parse HEAD
```

The reported SHA was `ea4d7ee771849fd8743ae8b8929bc5cbad997c58`. Before building, the clone had no host `node_modules` or `.next`. Its own `.env.example` was used to create a fresh private `.env.docker`, with new database and seed credentials; no developer `.env` or old data volume was copied. Docker build cache was available; a cold-cache download benchmark was not performed.

From that clone:

```bash
docker compose --env-file .env.docker -p dqep3clean -f compose.production.yaml config --quiet
docker compose --env-file .env.docker -p dqep3clean -f compose.production.yaml build api web migrate seed
docker compose --env-file .env.docker -p dqep3clean -f compose.production.yaml up -d --wait
docker compose --env-file .env.docker -p dqep3clean -f compose.production.yaml run --rm seed
docker compose --env-file .env.docker -p dqep3clean -f compose.production.yaml ps -a
node --env-file=.env.docker scripts/lab5/docker-smoke.mjs journey
```

All commands exited 0. PostgreSQL and web were healthy; API was running `/app/api`; migration exited 0. The seed command completed explicitly and migration was safely rerun. No source bind mounts were used. API/database-tool runtime UID/GID is `10001:10001`; web runs as `node` (UID 1000). The API image retains CA roots and embedded timezone data; the database binary includes its SQL migrations.

The HTTP smoke journey created party-size-10 ticket A-001, verified cross-customer 404, Customer-to-Host 403, missing-CSRF and foreign-Origin 403, then read CALLED and COMPLETED through Customer endpoints after Host transitions. The authenticated Host SSR page loaded and logout revoked the session (204, then 401 from `/me`). This is an HTTP smoke journey; the separate browser regression below exercises UI interactions.

Captured text logs have terminal carriage returns and trailing whitespace normalized for repository formatting; result lines are retained.

Evidence: [build output](evidence/ep3/clean-build.txt), [startup and smoke output](evidence/ep3/clean-start-smoke.txt).

## Regression on the committed clone

| Command / check | Actual result |
|---|---|
| `npm ci --ignore-scripts --no-audit --no-fund` | Exit 0; lockfile unchanged |
| `npm test` | 2/2 cases passed in the clean-clone run |
| `npm run lint` | Exit 0 |
| `npm run build` | Exit 0; all Next.js routes built |
| `npm run typecheck` (after build generated route types) | Exit 0 |
| `go test -tags=integration -race -count=1 -v ./...` | 53 top-level functions passed, zero failures, no race reports; subtests are not added to 53 |
| `go vet -tags=integration ./...` | Exit 0; includes normal and integration-tagged sources |
| Existing Playwright suite | 20/20 passed, one worker, installed Google Chrome, approximately 1.5 minutes |
| `git diff --check` | Exit 0 |

Go commands ran in the development/toolchain target built from the clone:

```bash
docker build --target development -f apps/api/Dockerfile -t dinequeue-ep3-clean-go-check .
docker run --rm --network dqep3clean_default --env-file /tmp/dinequeue-ep3-audit/clean-go.env dinequeue-ep3-clean-go-check go test -tags=integration -race -count=1 -v ./...
docker run --rm dinequeue-ep3-clean-go-check go vet -tags=integration ./...
```

`clean-go.env` was a private generated file containing only `TEST_DATABASE_URL` pointing to `go_regression` on the disposable server. That database's public schema was empty before the tests. Do not point this suite at a developer or shared database.

The browser command was:

```bash
node --env-file=/tmp/dinequeue-ep3-audit/clean-e2e.env node_modules/@playwright/test/cli.js test --config /tmp/dinequeue-ep3-audit/clean-playwright.config.mjs
```

The private E2E environment supplied `F01_BASE_URL=http://localhost:3032`, the generated local test password, and the isolated loopback `TEST_DATABASE_URL`. The temporary config selected the clone's existing `apps/web/e2e` directory, `workers: 1`, `retries: 0`, list reporter, `use.baseURL: process.env.F01_BASE_URL`, `use.channel: 'chrome'`, and disabled trace/video/screenshots. No assertions or application rules were changed. These existing tests include real database fixtures and intentional read-failure injection, so this runner must remain isolated.

Evidence: [web checks](evidence/ep3/clean-web-checks.txt), [all Go test names/results](evidence/ep3/clean-go-tests.txt), [browser test names/results](evidence/ep3/clean-browser.txt). Go vet produced no diagnostics and exited 0.

## Database outage and recovery

Using the same clean-clone Compose project/file/environment options (plus the temporary browser DB-port override), stopped only `db`, then checked direct/proxied readiness with `curl -sS --max-time 15`. An EXIT trap restored the database even if a check failed.

| Endpoint | DB stopped | After `up -d --wait db` |
|---|---|---|
| API `/health/live` on 8093 | 200, `{"status":"ok"}` | Liveness available |
| API `/health/ready` on 8093 | 503, `{"status":"unavailable"}` | 200, `{"status":"ready"}` |
| Web `/api/health/ready` on 3032 | 503, `{"status":"unavailable"}` | 200, `{"status":"ready"}` |

The same results were also observed on initial audit ports 8092/3012. No volume was deleted for these checks.

## Persistence and session survival

After regression, the committed clone's `docker-smoke.mjs create` produced a separate WAITING ticket A-006 for party size 10. Its private local state file contained only the entry identifier/code/status, not credentials or a session token.

An actual Chrome session opened My Ticket. A screenshot of the ticket card was captured, then the clean-clone Compose project was stopped with `down` and restarted with `up -d --wait`, preserving the volume. No seed or reset occurred between the two reads. The same browser session reloaded successfully, with A-006 still WAITING; a fresh-login `docker-smoke.mjs verify` also returned the same saved code/status. The initial audit separately retained A-008 WAITING across the same procedure.

Commands used the same project and environment throughout:

```bash
EP3_TICKET_FILE=/tmp/dinequeue-ep3-audit/clean-ticket.json node --env-file=.env.docker scripts/lab5/docker-smoke.mjs create
docker compose --env-file .env.docker -p dqep3clean -f compose.production.yaml -f /tmp/dinequeue-ep3-audit/clean-ports.yaml down
docker compose --env-file .env.docker -p dqep3clean -f compose.production.yaml -f /tmp/dinequeue-ep3-audit/clean-ports.yaml up -d --wait
EP3_TICKET_FILE=/tmp/dinequeue-ep3-audit/clean-ticket.json node --env-file=.env.docker scripts/lab5/docker-smoke.mjs verify
```

The temporary override only published the isolated fixture DB on loopback. Browser screenshots were captured before and after those Docker commands in the same automation process. They intentionally show only the non-sensitive ticket card, without account details, cookies or private URL identifiers. The two independently captured images are pixel-identical because the stored ticket did not change.

Evidence: [persistence command output](evidence/ep3/clean-persistence.txt), [before restart](evidence/ep3/persistence-before.png), [after restart](evidence/ep3/persistence-after.png).

## Credential and production-guard checks

- Inspected runtime image user/entrypoint/environment: non-root, compiled API/CLI and `next start`, no baked credential variables.
- Scanned exported layer/config bytes for all configured developer/audit/clean-clone credential values (without displaying them), plus tracked files, working diff, Git patch history and audit container logs. No matching configured credentials were found. This checks the known configured values and is not proof that every possible secret or dependency vulnerability is absent.
- Build context excludes `.env*` except the empty `.env.example`, `.aws`, `.git`, host `node_modules` and `.next`. Existing explicit COPY instructions and non-secret build arguments were reviewed.
- Production API image rejected HTTP `APP_ORIGIN`; rejected PostgreSQL without `sslmode=verify-full`; database-tool image rejected production demo seed before DB access. These were actual container invocations using synthetic invalid connection values, not successful production TLS connectivity tests.

## Failed setup attempts and limits

The first new HTTP harness used `{}` on Host mutations and expected 200 for logout. Existing contracts require an empty POST body and logout 204; those harness errors were corrected, then the full journey passed. No application rule was changed to make the harness pass.

An initial temporary browser config incorrectly spread an imported TypeScript config and lost its base URL/worker setting. Tests failed before navigation; that attempt was interrupted, the temporary runner was corrected explicitly, and all 20 tests subsequently passed, including the separate committed clean-clone run. These setup failures are not counted as passing tests.

Not performed: GCP deployment, CI pipeline/branch-protection changes, successful production HTTPS/verified-DB-TLS journey, native Windows execution or dependency vulnerability audit. Teammate review and merge remain pending. The local readiness and clean-clone results satisfy EP3 packaging verification; cloud evidence belongs to later episodes.
