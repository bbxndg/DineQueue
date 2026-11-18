# Lab 5 Episode 5 — First GCP deployment contract

Date: 18 November 2026 (Asia/Bangkok).
Tracking: [Issue #16](https://github.com/bbxndg/DineQueue/issues/16).
Branch: `feat/lab5-ep5-first-gcp-deployment`.
Baseline: reviewed main release `68bbe2f` (EP2–EP4, release PR #26).
Status: topology approved by the user on 18 November 2026; pre-implementation
contract. No cloud commands, resource creation, deployment or test PASS are
claimed by this plan.

## Course boundary

Student Guide EP4 §4.5 puts the reviewed CI/Docker/deploy folder on main.
EP5 §5.0 requires adapting the TokTickIT example to the actual stack before
execution; §5.1 deploys a checkout matching main; §5.2 previews the plan before
§5.3 creates resources. Keep budget alerts and billing prerequisites. The user's
latest schedule and Issue #16 set 22 November 2026, 23:59 ICT as the deadline.

EP5 includes the first deployment and a real Sprint 1 smoke journey. GitHub
identity/branch rules, automated main releases and restart/redeploy evidence
remain separate EP6, EP7 and EP8 work. Do not drop later requirements or claim
those episodes are completed by the first deployment.

## Findings in the current source

| Source | Gap for the first deployment |
|---|---|
| `deploy/deploy.sh` | TokTickIT names, one root image, Prisma migration and authentication assumptions do not match DineQueue. |
| `deploy/release.sh`, `deploy/teardown.sh`, `deploy/smoke_test.py` | Still course references; must not run live unchanged. Keep later episode scope explicit when adapting shared names/interfaces. |
| `apps/api/internal/platform/config/config.go` | Production requires HTTPS origin and `sslmode=verify-full`; the guide's Cloud SQL Unix-socket URL is currently rejected. Do not work around this with `APP_ENV=development` on GCP. |
| `apps/api/cmd/db/main.go` | `seed` is intentionally disabled in production. A separately guarded provisioning command is needed for demo/UAT accounts. |
| `apps/web/next.config.mjs`, `apps/web/src/lib/auth.ts` | Proxy and server-side reads use the API origin; rewrites are frozen at build time. Private cross-service IAM would require token-aware forwarding; ordinary rewrites alone do not provide it. |
| `apps/api/internal/platform/postgres/pool.go` | Ten connections per API instance; choose bounded instance/concurrency limits compatible with the Cloud SQL tier and migration/provisioning jobs. |

## Selected topology — user approved (DEC-032)

**Selected for this initial lab deployment:** one public Cloud Run service
with separate Next.js ingress and Go API sidecar images. Next.js is the only
public ingress; existing server-side reads and browser rewrites reach Go over
localhost on a distinct port. This preserves separate application images and the
modular monolith; both containers scale together. Use a separate database-tools
image for explicit Cloud Run migration and opt-in provisioning jobs.

Alternative: separate web/API Cloud Run services with a private API, web runtime
identity granted API invoker only, and authenticated server-side forwarding.
This offers independent scaling but requires more routing/authentication work.
Do not silently make the API public as a substitute for private service IAM.

Both options use Cloud SQL PostgreSQL, Artifact Registry and Secret Manager.
Review the region, PostgreSQL version/tier and budget before resource creation.
Cloud SQL socket support must be a narrow, explicitly configured transport:
retain `verify-full` for ordinary TCP, permit only the configured canonical
`/cloudsql/PROJECT:REGION:INSTANCE` socket for the managed authenticated/encrypted
Cloud SQL connection, and reject arbitrary socket/TCP downgrade parameters.
This contract authorizes the narrow transport revision; tests and implementation
evidence are recorded separately.

## Implementation order and acceptance tests

1. Record the selected topology/transport and account-provisioning contract in
   the decision register. Keep pending/approved statuses truthful.
2. Add fail-first configuration tests for the exact Cloud SQL transport and
   downgrade rejection; preserve HTTPS origin, Secure/HttpOnly sessions, Origin,
   CSRF and existing TCP TLS protections. No authentication bypass.
3. Add a separately invoked demo/UAT provisioning CLI: explicit opt-in and exact
   target database confirmation, no default passwords, Secret Manager supplied
   credentials, existing identity/password preservation and transaction rollback
   on conflicts. Keep normal production `seed` forbidden. Test with an isolated
   real database; never the developer or cloud database for destructive tests.
4. Adapt deployment planning/scripts for the chosen topology and three image
   roles (web/API/db-tools), immutable source SHA tags, distinct runtime/job
   identities and per-secret access. Never bake credentials into build arguments,
   source archives or image layers. Exclude ignored secrets from Cloud Build
   source uploads as well as Docker contexts. Preserve names for later episodes.
5. Make dry run independent of live `gcloud`, credential generation or cloud
   mutation. Require explicit project/region and resource selections for live
   execution. Distinguish missing resources from IAM/network failures; abort on
   ambiguous discovery. Fail before service update if migration fails. Never
   rotate an existing DB user's password implicitly after a partial failure.
6. Add offline command-stub tests for planned names, IAM/secrets, existing-resource
   handling, partial failures, migration gate and no secret output. Run relevant
   Go/web regression, production-image checks and the existing browser journey
   against isolated local resources. Record actual commands, SHA and results.
7. Review and integrate preparation through `gcp-staging` and main. In Cloud Shell
   verify the deployed checkout SHA matches reviewed main, inspect the dry run,
   then separately authorize resource creation/public exposure and costs using
   the concrete plan. Local preparation does not grant blanket cloud mutation.
8. Verify real HTTPS login/session, Customer join, Host call/complete and Customer
   refresh on GCP. A read-only smoke script may check health/login page/anonymous
   authorization; it must not claim the authenticated journey passed without
   execution. Record source SHA, service revisions, image digests, project/region,
   public URL and actual evidence. Redact passwords/cookies/tokens and DB URLs.

## Evidence and completion

No real account credentials may appear in screenshots, PR descriptions, command
logs or chat. Record readiness status, migration/provisioning execution outcomes,
Cloud Run revisions and actual smoke observations. Keep Issue #16 open until its
public deployment and Host/Customer journey criteria have evidence.

## Official technical references

- [Cloud Run multi-container runtime contract](https://docs.cloud.google.com/run/docs/container-contract)
- [Deploy images and localhost sidecar communication](https://docs.cloud.google.com/run/docs/deploying)
- [Cloud Run to Cloud SQL PostgreSQL](https://docs.cloud.google.com/sql/docs/postgres/connect-run)
- [Cloud SQL connection management](https://docs.cloud.google.com/sql/docs/postgres/manage-connections)

These sources support platform capabilities, not course approval, cost estimates
or proof that this application is deployed.
