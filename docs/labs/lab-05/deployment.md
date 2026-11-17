# Lab 5 — Episode 2: Cloud readiness and deployment notes

**Scope:** Sprint 1 cloud-readiness review and initial deployment notes only. This episode does not create GCP resources, deploy the application, add production Docker packaging, CI/CD, or Sprint 2 features.

## Environment configuration inventory

The names below were compared with `rg -n 'os\.(Getenv|LookupEnv)|process\.env|NEXT_PUBLIC_' apps/api apps/web` and `.env.example`. Keep actual credentials in the local, ignored `.env` or an approved secret manager; never add their values here.

| Variable | Consumer | Purpose | Secret / scope |
|---|---|---|---|
| `APP_ENV` | Go API and database CLI | Selects `development`, `test`, or `production` configuration. | Non-secret |
| `APP_ORIGIN` | Go API | Exact allowed browser origin for authentication/CSRF protections; production requires HTTPS. Local Compose uses `http://localhost:3000`. | Non-secret; deployment-specific |
| `DATABASE_URL` | Go API and database CLI | PostgreSQL connection URL. Local Compose uses service host `db`; production requires verified TLS. | Secret: contains DB credentials |
| `PORT` | Go API | HTTP listener port; defaults to `8080`. The API listens on `:<port>` inside its container. | Non-secret |
| `API_INTERNAL_URL` | Next.js configuration and server-side session helper | API origin used for Next.js rewrites/server requests. Compose sets `http://api:8080`; a host-run web process uses `http://127.0.0.1:8080`. | Non-secret; server-side only, also affects the Next.js build |
| `NEXT_TELEMETRY_DISABLED` | Next.js/Docker configuration | Disables Next.js telemetry when set to `1`. | Non-secret |
| `POSTGRES_USER` | Docker Compose/PostgreSQL | Local database role name. | Non-secret |
| `POSTGRES_DB` | Docker Compose/PostgreSQL | Local database name. | Non-secret |
| `POSTGRES_PASSWORD` | Docker Compose/PostgreSQL | Local database role password. | Secret; leave blank in `.env.example` |
| `SEED_CUSTOMER_PASSWORD`, `SEED_CUSTOMER2_PASSWORD`, `SEED_HOST_PASSWORD`, `SEED_OTHER_HOST_PASSWORD` | Explicit demo seed CLI only | Operator-supplied passwords for the four local demo users; 12–128 UTF-8 bytes each. Seeding is disabled in production. | Secrets; blank in `.env.example`, never display or commit values |

The source scan also finds test-only variables: `TEST_DATABASE_URL` for isolated Go integration tests; `F01_BASE_URL` and `F01_TEST_PASSWORD` for Playwright E2E tests; and optional `S1_REVIEW_SCREENSHOT_DIR` for review-test output. These are not application runtime configuration and are intentionally not defaulted in `.env.example`. No `NEXT_PUBLIC_` variables are currently used; do not expose server credentials through client-prefixed variables.

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
