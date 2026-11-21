# DineQueue deployment — first deploy and repeatable releases

The user selected one Cloud Run service with separate Next.js ingress and Go API
sidecar images (DEC-032). A third image contains the Go database CLI. PostgreSQL
persists in Cloud SQL. The first deployment is manual from reviewed main. EP7
adds the main-only automated release pipeline; see [the EP7 runbook](../docs/labs/lab-05/ep7-runbook.md).

**Evidence:** the user supplied [successful EP5 cloud results](../docs/labs/lab-05/ep5-cloud-results.md).
EP7 is locally verified preparation until actual Actions/WIF/release evidence exists.
Offline/local tests do not establish a successful GCP rollout. Follow [the EP5 guide](../docs/labs/lab-05/deployment.md#episode-5--first-gcp-deployment)
and inspect the dry run before any live command. Live execution creates billable
resources and public ingress; it is not part of local verification.

## Files

- `deploy.sh` invokes `first_deploy.py` from the repository root. Python 3.12+ and
  Git are required; live execution additionally needs authenticated `gcloud`.
- `cloudbuild.yaml` builds web/API/db-tools with full source SHA tags. Web rewrites
  and runtime upstream both use `http://127.0.0.1:8081`. Live uploads use a Git
  archive of the reviewed main SHA, not the working directory or ignored `.env`.
- `smoke_test.py` checks DineQueue HTTPS liveness, database readiness, login HTML
  and anonymous denial. Optional environment credentials check secure session,
  identity and logout. It does not create tickets or prove the Host journey.
- `release.sh` invokes `release.py`: default is a no-network plan; `--validate-only`
  performs read-only discovery plus server validation; `--apply` validates, migrates
  and updates the existing service. Existing-SHA rollback skips migrations/jobs.
- `build-images.sh` builds API/web/db-tools from a tracked source archive and pushes
  one SHA-tagged set. It excludes ignored local files and temporary WIF credentials.
- `.github/workflows/deploy.yml` reuses CI and performs keyless releases from main
  in environment `demo`. It never calls the first-deployment/bootstrap script.
- `teardown.sh` remains fail-closed pending separate cleanup work. Do not substitute
  the course reference scripts for DineQueue live commands.
- `reference/` preserves the original seven EP4 course files as historical
  TokTickIT references. They are not DineQueue cloud commands. In particular,
  their deployment, migration, credentials and destructive teardown assumptions
  do not match this project. Their original test counts remain historical.

## Offline checks (no cloud calls)

From the repository root:

```bash
for script in deploy/deploy.sh deploy/release.sh deploy/teardown.sh; do
  bash -n "$script"
done
bash deploy/tests/test_deploy_stub.sh
bash deploy/tests/test_release_stub.sh
python3 -m unittest discover -s deploy/tests -p 'test_smoke.py' -v
```

The deploy entry runs DineQueue's mocked first-deployment tests. The release entry
now runs DineQueue's deployment/release/build safety suite, including stubbed
Docker/gcloud and disposable Git fixtures. It no longer reports the archived
TokTickIT release scenarios as DineQueue results. No real GCP operation or Docker
build is invoked by these tests. Windows users can run Bash
checks in WSL; Cloud Shell supplies the live deployment environment.

## Dry run

```bash
export PROJECT_ID=YOUR_PROJECT_ID
export REGION=asia-southeast1
export SOURCE_SHA="$(git rev-parse HEAD)"
DRY_RUN=1 bash deploy/deploy.sh --demo
```

This prints a resource summary and simulated commands with placeholder digests
and redacted secret input. It calls no subprocess/cloud CLI and generates no
credentials. It simulates new resources; apply uses guarded discovery to reuse
existing ones. The source SHA in a feature-branch dry run is a planning input,
not an accepted deployment. Live execution requires clean reviewed main and
checks it against `origin/main` before any GCP call.

Service creation/update uses the documented Cloud Run v2 REST API with a native
`cloudSqlInstance` volume mounted only into the API. It does not use alpha CLI or
the unsupported `cloudsql.run.googleapis.com` CSI driver. `gcloud` supplies a
short-lived access token captured in memory; tokens and provider payloads are not
printed or written. The script waits up to ten minutes for the long-running
operation and requires a successful service terminal condition before proceeding.
Other resource, build, job and public-IAM commands remain on the stable CLI.
The service is discovered with v2 GET first. Only an explicit service-level
`404 NOT_FOUND` selects `services.create` (POST with `serviceId=dinequeue`).
Existing services use PATCH with an explicit update mask; discovery errors stop.
The first deployment no longer depends on PATCH `allowMissing` creating a service.

## Planned runtime and safety boundaries

- Only Next.js has public ingress (8080); API listens on `API_PORT=8081` inside the
  shared instance. The web container receives no database environment variable.
  Both containers share the service identity/network: this is deployment
  separation, not an IAM isolation boundary between web and API.
- API production still requires HTTPS Origin and server-side RBAC/CSRF. Ordinary
  TCP PostgreSQL still requires `verify-full`. The explicit `CLOUD_SQL_INSTANCE`
  path permits only its exact managed Unix socket, through Cloud Run's
  authenticated/encrypted Cloud SQL integration. Never set production to
  `development` to bypass validation.
- Default plan: PostgreSQL 18 Enterprise, `db-f1-micro`, 10 GB SSD, backups,
  storage auto-increase off, no authorized IP networks; Cloud Run min 0/max 1,
  concurrency 10. Cloud SQL and stored artifacts remain billable while present.
  Verify availability, quotas and budget before apply; no price estimate is made.
- Runtime/jobs identities have Cloud SQL Client and named-secret access. The
  builder has repository writer, source-bucket viewer and log writer permissions;
  it receives no runtime secrets. Migration and provision jobs share the jobs
  identity, but only the explicit provision job receives account passwords in
  its environment. The first-deployment DB user owns its schema for migrations;
  separate least-privilege DB roles are not implemented here.
- Secret version `1` is pinned; existing enabled credentials are reused. Missing
  credentials after partial setup, disabled versions, wrong resource settings and
  discovery errors abort. No implicit password rotation or deleting resources.
  Credential recovery/rotation needs an explicit reviewed operation.
- Migration and provisioning are separate jobs with no automatic retries. A
  failed job stops the script before updating the service. The normal production
  seed command remains forbidden. Provisioning requires `--demo`, an explicit
  CLI opt-in and exact connected database confirmation ending in `_demo`/`_uat`.
- The script initially deploys a new service without granting public access,
  obtains its canonical URL, then applies the exact HTTPS origin before adding
  the public web invoker binding. Reruns reuse the existing canonical origin.
- EP5 itself does not establish pipeline/rollback evidence. EP7 implements the
  existing-image reuse path; real rollback/persistence proof belongs to EP8.
  Destructive teardown remains separate; preserve the resource inventory.

Official references: [Cloud Run containers](https://docs.cloud.google.com/run/docs/configuring/services/containers),
[deploy flags](https://docs.cloud.google.com/sdk/gcloud/reference/run/deploy),
[Cloud SQL connection](https://docs.cloud.google.com/sql/docs/postgres/connect-run),
[Cloud SQL versions](https://docs.cloud.google.com/sql/docs/postgres/db-versions),
[Cloud Build configuration](https://docs.cloud.google.com/build/docs/building/build-containers).

## EP7 release plan (no cloud calls)

```bash
PROJECT_ID=cpe334-dinequeue-514914 REGION=asia-southeast1 \
IMAGE_TAG="$(git rev-parse HEAD)" ROLLBACK=false bash deploy/release.sh
```

Only the reviewed main workflow runs `bash deploy/release.sh --apply`. It resolves
all required image digests, preserves the existing service template/configuration
with new web/API images, validates the PATCH, awaits migration on normal releases,
patches with an etag and waits for ready images/revision/traffic. It exports only
validated public URL, revision and digest metadata. Smoke failure fails the workflow.
The code does not recreate infrastructure, grant IAM, rotate passwords or reseed.

The existing-SHA dispatch path skips CI/build/migration and restores the API/web
image pair only. Choose a schema-compatible release; rolling back images does not
undo database migrations. See EP7/EP8 instructions before real rollback.
