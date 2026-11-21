# Episode 7 — automated main releases and EP8 image reuse

Tracking: [Issue #18](https://github.com/bbxndg/DineQueue/issues/18).
Preparation branch: `feat/lab5-ep7-auto-deploy`, from `gcp-staging` `8916397`.
See [contract](ep7-contract.md) and [local verification](ep7-results.md).
Remote authentication and deployment are NOT RUN by this local preparation.

## 7.1–7.2 — DineQueue mapping

| Setting | Value |
|---|---|
| Workflow file/name | `.github/workflows/deploy.yml` / `Deploy demo` |
| Integration/release branches | `gcp-staging` / `main` |
| Environment | `demo`, main branch only |
| Service | `dinequeue`, web ingress plus API sidecar |
| Migration job | `dinequeue-migrate`, existing `migrate` CLI entry |
| Registry | `REGION-docker.pkg.dev/PROJECT_ID/dinequeue` |
| Images | `api`, `web`, `db-tools`, all tagged with the same full SHA |
| Public smoke | `/api/health/live`, `/api/health/ready`, `/login`, anonymous `/api/v1/auth/me` denial |

Keep EP6 environment variables `GCP_PROJECT_ID`, `GCP_REGION`, `WIF_PROVIDER`,
`WIF_SERVICE_ACCOUNT`. Provider condition must still require the exact numeric
repo/owner IDs, main, this workflow filename and environment `demo`. No PR or
staging run can authenticate/deploy. Do not change the condition to troubleshoot
an unrelated branch. Check immutable/legacy subject format only against actual
EP7 authentication results; never print an OIDC/access token.

Issue #18 originally described a second staging deployment. This implementation
follows Student Guide EP7 and the user's main-only EP6 scope: staging is reviewed
integration, main is the existing demo release. A second cloud environment would
be a separate scope/access/infrastructure decision, not an implicit requirement.

## 7.3 — Optional authenticated smoke

Default: deliberately skip authenticated smoke. The four public checks always
run and the output explicitly reports SKIP for login. Existing EP5 manual browser
evidence is not a new pipeline login test.

If later enabled, use both `SMOKE_EMAIL` (environment variable) and
`SMOKE_PASSWORD` (environment secret) for a dedicated low-privilege smoke account.
Do not reuse a person's/demo account password or put it in workflow/source/logs.
DineQueue S1 has no signup/admin-user UI; do not add product features just to enable
this optional check. The preflight rejects setting only one input before auth.

## 7.4 — Review and first release

1. Review the feature diff and actual local results. Keep existing production
   Dockerfiles and the existing `ci-docker.yml` unchanged.
2. Commit/push the feature branch when ready. Open a PR into `gcp-staging` with
   `Part of #18`; wait for CI and teammate review, then merge.
3. Open the reviewed release PR from `gcp-staging` into main. It includes the EP6
   docs deferred from PR #36 and this EP7 pipeline. Wait for the main required
   `test-and-docker` check and one approval before merging; no ruleset bypass.
4. Merging pushes to main and starts `Deploy demo`. Do not run the EP5 bootstrap
   script again. This is the first real WIF authentication test from EP6.

The workflow validates environment settings, uses the course-pinned auth/setup
actions and SDK 594.0.0, and invokes the CI workflow before any release auth.
Existing-image input must belong to the checked-out main history; missing or
unreviewed SHAs are rejected before WIF authentication.
OIDC permission exists only in the main-only deploy job/environment. Concurrency
does not cancel a running migration/deployment. Temporary `gha-creds-*.json` files
are ignored and are never part of the Git archive used as Docker build context.

## 7.5–7.6 — First release A and evidence

Normal pipeline:

1. Reused CI must pass. WIF must succeed using `dinequeue-deployer`.
2. Build and push all three images from tracked source at `github.sha`.
3. Resolve all three tags to digests; require the existing service/configuration.
4. Validate the native v2 PATCH before changing the migration job. The validation
   operation is not persisted/polled and does not itself deploy anything.
5. Update only the migration job's image to the resolved db-tools digest, execute
   with `--wait`. A failure stops before the real service update; no provisioning.
6. Copy the existing service revision template, remove the old revision name and
   replace only web/API images and source labels. Preserve origins, identities,
   secrets, Cloud SQL volume/mount configuration, ports/probes and scaling. Use
   an etag conflict guard and traffic 100% to latest. Never POST/create a service.
7. Await the scoped operation, ready revision/image pair and observed traffic.
8. Smoke the canonical HTTPS URL. Only after smoke passes, write run summary with
   workflow SHA, target image SHA, API/web/db-tools digests, ready revision and URL.

Record actual run/PR URLs and full release A SHA. In Console verify latest
revision receives 100%, both API/web digests equal the summary, migration execution
succeeded with A's db-tools image and Artifact Registry tags equal A's SHA.
Keep screenshots where a link cannot show the setting; never expose credential values.

## 7.7 — Second release B proves repeatability

Only after A is green and checked in Console:

1. Sync staging with latest main. Use a reviewed sync PR if the repository rules
   require it; never bypass main protection.
2. Create a separate proof branch from updated staging, e.g.
   `feat/lab5-ep7-deploy-proof`.
3. Make one small user-visible wording change without changing business rules;
   run checks appropriate to that change.
4. Feature PR into staging → green CI/review/merge → reviewed release PR into main.
5. Confirm automatic release B is green without typing a deploy command, the
   browser shows the new wording, and Console/digests/SHA match B.
6. Save A and B's full 40-character image tags for EP8. Leave Issue #18 open until
   both actual releases and evidence criteria are met. Do not count local tests
   or EP5's manual release as these two pipeline releases.

## EP8 support — reuse / rollback / roll-forward

Actions → Deploy demo → Run workflow → branch **main**:

- `image_tag` empty: normal current-main CI/build/migrate/deploy/smoke.
- `image_tag` = full SHA of A: skip CI/build/migration/provision; resolve existing
  A API/web digests and update both containers, wait ready and smoke.
- `image_tag` = full SHA of B: same reuse path restores B exactly, without migration.

Rollback changes application images, not the database. Choose an image pair
compatible with already-applied migrations. No automatic rollback is performed
after smoke failure. Preserve failure logs, inspect named service/job and choose
the reviewed known-good SHA manually. Do not reset/reseed/delete the database.
EP8 separately verifies ticket/session persistence before/after A and B.

## Local commands and live failure handling

From repo root, these commands are offline:

```bash
bash -n deploy/build-images.sh
bash -n deploy/release.sh
bash deploy/tests/test_release_stub.sh
python3 -m unittest discover -s scripts/ci/tests -p 'test_*.py' -v
PROJECT_ID=cpe334-dinequeue-514914 REGION=asia-southeast1 \
IMAGE_TAG="$(git rev-parse HEAD)" ROLLBACK=false bash deploy/release.sh
```

Optional official SDK grammar/schema audit (no network):

```bash
python3 deploy/tests/check_release_sdk.py /path/to/google-cloud-sdk
```

`release.sh --validate-only` is a live **read/validation** command: it requires
authenticated gcloud and pre-existing tagged images; it runs no migration or
real service update. Normal live apply belongs in the reviewed main workflow.

If auth fails, check exact EP6 provider condition, subject format, four variables
and main/environment/workflow identity. If a digest is missing, confirm all three
pushes completed for the normal release SHA. If migration fails, inspect that
execution without re-running bootstrap. If the service is reconciling or etag
conflicts, inspect the existing rollout before retrying. If readiness/smoke fails,
the run must remain failed; inspect revision conditions/logs without copying secrets.

References: Student Guide EP7 §§7.1–7.7 and EP8;
[course action authentication implementation](https://github.com/google-github-actions/auth),
[Cloud Run v2 PATCH](https://docs.cloud.google.com/run/docs/reference/rest/v2/projects.locations.services/patch),
[revision template/traffic schema](https://docs.cloud.google.com/run/docs/reference/rest/v2/projects.locations.services).
