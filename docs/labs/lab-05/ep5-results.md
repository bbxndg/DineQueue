# EP5 preparation — actual local verification

Date: 18 November 2026 (Asia/Bangkok). Tracking: Issue #16.
Branch: `feat/lab5-ep5-first-gcp-deployment`.

**Local preparation passed. GCP resources, Cloud Build, Cloud Run deployment,
Cloud SQL connectivity/IAM and live smoke tests have NOT been executed.**
Do not close Issue #16 based on this report.

## Tested snapshot

The worktree is based on reviewed main `68bbe2f4948481800c92412cbc2ec5f365affae3`.
The pre-implementation plan/DEC-032 was committed as `5823451`. Implementation
and tests were executed in the worktree after that commit, not on an accepted
main release. The [source/config manifest](evidence/ep5/source-manifest.txt)
identifies the checked files; its SHA-256 is
`9450311fba9f9829d36f1af0b0ad27d7b3a5be71b88bf6427c8f1a736bd94ae6`.
The implementation was subsequently grouped into focused commits; committed
source/config at `7fbf0f4a7405bdf1f34b335888cf564af8cc1332` matches every entry in that tested manifest.
This is a source-equivalence check, not a fresh test run on the commit.
Documentation/evidence edits after those checks do not establish a hosted CI run.

## Results

| Check | Actual result | Evidence |
|---|---|---|
| Go formatting, `go vet -tags=integration ./...` | Exit 0 | [Go log](evidence/ep5/go-tests.txt) |
| `go test -tags=integration -race -count=1 -v ./...` | 58 top-level tests passed; no failure/race report | [Go log](evidence/ep5/go-tests.txt) |
| Deployment planning/command tests | 17/17 passed, GCP responses mocked | [Deploy log](evidence/ep5/deploy-tests.txt) |
| DineQueue smoke unit tests | 6/6 passed, network mocked | [Smoke unit log](evidence/ep5/smoke-tests.txt) |
| Archived course release stub | Five scenarios passed; not DineQueue EP7 automation | [Reference output](evidence/ep5/reference-release-tests.txt) |
| Existing CI configuration suite | 7/7 passed; helper/workflow source unchanged | Recorded local unittest command below |
| Web tests | 2/2 passed outside restricted subprocess sandbox | [Captured output and initial TAP summary](evidence/ep5/web-checks.txt) |
| Web lint/build/typecheck | All exited 0 outside restricted subprocess sandbox | [Captured build/typecheck output](evidence/ep5/web-checks.txt) |
| Production API/db-tools/web images | All built successfully; no registry push | [API](evidence/ep5/api-build.txt), [db-tools](evidence/ep5/db-tools-build.txt), [web](evidence/ep5/web-build.txt) |
| Existing browser regression | 20/20 passed on the local shared-localhost topology | [Browser log](evidence/ep5/browser-tests.txt) |
| Production-mode migration/provisioning | Compiled CLI connected through the exact Unix socket; migration passed, provisioning passed and reran successfully | Generic CLI completion output observed; no credentials printed |
| Ordinary production `seed` | Rejected with `demo seed is disabled in production` | Actual container invocation; expected nonzero exit |
| HTTPS production-image smoke | Real local HTTPS requests passed with an explicitly trusted temporary certificate; Secure/HttpOnly host-only session and logout passed | [Production smoke](evidence/ep5/production-smoke.txt) |
| HTTPS Customer/Host journey | Customer joined 10; Host called; Customer read CALLED; Host completed; Customer active list empty; both sessions revoked | [Production journey](evidence/ep5/production-journey.txt) |
| Zero-network dry run | Printed summary/simulated commands, no gcloud calls or secret generation | [Dry-run output](evidence/ep5/dry-run.txt); mocked test enforces no subprocess/random generation |
| Cloud Build config | YAML parsed locally; three targets and digest-pinned builder checked | Actual Python/YAML inspection; remote build NOT RUN |
| Credential scan | No configured developer/test values found in checked files/history/logs/46 saved-image members; selected high-confidence token/key patterns absent | [Scoped scan](evidence/ep5/credential-check.txt) |
| `git diff --check` and Bash syntax | Exit 0 | Local Git/Bash inspection |

Go's total combines unit and integration functions and excludes nested subtests
from the count. The 23 Python tests combine 17 deployment tests and six smoke
unit tests. Do not add the five archived reference scenarios to an application
unit-test count. The original browser assertions and workflow were not changed.

## Environment and commands

Go 1.26.10 ran in the existing toolchain image with current Go source mounted.
A fresh PostgreSQL 18.6 container on a dedicated network and tmpfs data served
`TEST_DATABASE_URL`. Provisioning integration fixtures created/drop their own
randomly named `_demo` databases and schemas; they did not use a developer DB.
The suite checked both new guards and all existing Sprint 1 rules/concurrency.

```bash
# In the Go toolchain container, with a private isolated TEST_DATABASE_URL:
gofmt -l .
go vet -tags=integration ./...
go test -tags=integration -race -count=1 -v ./...

# From repository root:
bash deploy/tests/test_deploy_stub.sh
bash deploy/tests/test_release_stub.sh
python3 -m unittest discover -s deploy/tests -p 'test_smoke.py' -v
python3 -m unittest discover -s scripts/ci/tests -p 'test_*.py' -v
npm test
npm run lint
npm run build
npm run typecheck
```

Host Node 22.17.0 was within the repository engine range. The first sandboxed web
attempt could not execute the TypeScript config subprocess; it reported a build
error and an incomplete one-file TAP report. The same checks were rerun outside
that restriction: the two real named tests, lint, build and typecheck passed.
No web code/dependency was changed to make that environment problem disappear.
Production web images used pinned Node 22.23.4 and the existing lockfile.

API, web and database-tool images were built from the existing production
Dockerfile targets, without source mounts at runtime. The web image's build-time
and runtime `API_INTERNAL_URL` both used `http://127.0.0.1:8081`. Web and API
containers shared a network namespace, reproducing the sidecar localhost hop.
The 20 browser tests used local HTTP/development API configuration for their
existing cookie fixtures; this alone is not production-cookie evidence.

A separate production-mode phase then used the canonical socket
`/cloudsql/dinequeue-test:asia-southeast1:dinequeue-pg` backed by a disposable local
Postgres socket volume. `API_PORT=8081` overrode the image's default `PORT`.
Compiled db-tools ran migration and explicit `provision-demo`; provisioning was
rerun after the final metadata-conflict guard was compiled. No ordinary production
seed was enabled. The full Go suite covered identity conflict rollback, mismatched
database, missing flags/passwords, existing password/active/grant preservation and
the new metadata-conflict guard.

A loopback TLS proxy on 3445 forwarded to the production-built app on 3045. A
one-day localhost certificate was explicitly trusted with `SSL_CERT_FILE`;
certificate verification was not disabled. The production API origin was
`https://localhost:3445`. Public/authenticated smoke and a separate real API
Customer/Host journey used independent cookie jars, printed only generic outcomes,
and revoked their sessions. This proves local production-mode socket/session
behavior, not Google-managed proxy encryption, cloud IAM or public Cloud Run TLS.
The production journey is an API journey through Next.js, while the preceding
20-test suite separately covers the UI in its existing local fixture mode.

Cloud Build's official Docker builder manifest was read from Google Container
Registry and its content SHA-256 matched the declared digest before pinning:
`gcr.io/cloud-builders/docker@sha256:dccbdf9b7877dc1b66427d769a55fe9d82c21655a88301e5eba3d0754c76d146`.
The builder was not run on GCP. No packages/versions or CI workflow were changed.

## RED observations

- Before configuration changes, the new config tests failed because the managed
  socket was rejected and production query transport overrides were accepted.
- Before smoke adaptation, all six new DineQueue smoke cases failed against the
  old TokTickIT interface. After adaptation, all six passed.
- Initial local Go formatting could not write as the image's UID; rerunning with
  the source owner's UID succeeded. No host file ownership was changed.
- The first database readiness check ran before initialization finished; it
  returned not ready. It was confirmed ready before any test suite was run.

These are observed local failures/corrections, not an intentional failing hosted
PR demonstration. No hosted EP5 CI result is claimed.

## Remaining gates and limitations

1. Review the implementation and integrate through `gcp-staging`/main with actual
   CI results before deployment. Live source guards require clean main at the
   same SHA as fetched origin/main.
2. In Cloud Shell confirm account/project, region, billing/budget and actual quotas;
   inspect the concrete dry run before explicit live execution. No billable
   operation, public IAM binding or secret mutation was performed locally.
3. Actual Cloud SQL/Cloud Run flags, permissions, mounted socket, service startup,
   Secret Manager version access and three-image Cloud Build execution still need
   real platform validation. Fix any genuine platform error before claiming PASS.
4. Verify the public HTTPS health and Customer/Host UI journey, retain source SHA,
   digests/revisions/run evidence and update Issue #16 only after acceptance.
5. Containers share the service identity; job identity is shared by migration and
   provisioning, and the first-deployment DB user owns its migration schema.
   These are documented privilege/isolation limitations. EP6/EP7 and destructive
   cleanup are not implemented by this change.
6. Known-value/key-pattern scanning is limited; no comprehensive dependency audit,
   security guarantee, production pentest or native Windows execution is claimed.

All test resources/configuration are disposable and separate from developer dev
and EP3 projects. Cleanup status is recorded after completion below.


Cleanup completed: removed only the EP5-created app/Go test containers, temporary
socket volume/network and loopback TLS proxy. Private generated env files and the
localhost TLS key/certificate were deleted. Developer dev and EP3 containers
remained running. Local build images and sanitized evidence logs were retained.


## Deployment recovery regression — 20 November 2026 (local only)

Branch: `fix/lab5-ep5-bucket-discovery`. Tested uncommitted working-tree changes
on base HEAD `7452aaebf8f893bd27ccca42eef470b1c3e7945d`; this base SHA does not
identify the fixes until they are committed.

- Added regression cases before implementation: the targeted suite failed
  (`22` test methods, `2` failures and `11` errors, including subtest errors).
- After the fixes: `python3 -m unittest discover -s deploy/tests -p 'test_*.py' -v`
  — **34/34 PASS**, comprising 26 first-deployment tests and 8 smoke-tool tests.
  Cloud/HTTP responses are mocked; these are not live smoke results.
- `PROJECT_ID=dinequeue-test SOURCE_SHA=7452aaebf8f893bd27ccca42eef470b1c3e7945d DRY_RUN=1 bash deploy/deploy.sh --demo`
  — PASS, simulated commands only.
- `bash -n deploy/deploy.sh` and `git diff --check` — PASS.

Coverage includes exact Storage/Cloud Run absence messages and rejection of
wrong-resource/mixed-denial errors; bounded IAM retries, immediate permanent-error
abort and credential-safe output; matching existing bucket metadata and refusal
to upload when name/region/access policy differs; failed build/provisioning,
wrong image digest and failed public IAM binding stop their dependent steps.
Bucket inspection uses the raw Storage API metadata shape, not display aliases.

No resources, IAM permissions, credentials, Docker images or CI configuration were
changed by this local recovery work. Cloud Run/Cloud SQL execution still requires
real validation after the reviewed fixes reach main.

### Cloud Run global flag ordering — 20 November 2026

The user reported that the Cloud Shell deployment rejected trailing `--project`
and `--quiet` after the API container arguments, and that service discovery found
no service. The wrapper now places global flags before the command arguments;
the preview uses the same order.

Branch: `fix/lab5-ep5-gcloud-global-flags`, uncommitted working-tree changes on
base `97a8f8281ced3c402809316215597fce48a801a1`.

- A new regression test exercises `deploy_service` through the subprocess wrapper,
  rejects global flags inside container scope, and checks preview/live argv parity.
  Before the fix: 27 deployment tests ran with one failure in the new test.
- After the fix: `python3 -m unittest discover -s deploy/tests -p 'test_*.py' -v`
  — **35/35 PASS** (27 deployment tests and 8 mocked smoke-tool tests).
- `PROJECT_ID=dinequeue-test SOURCE_SHA=97a8f8281ced3c402809316215597fce48a801a1 DRY_RUN=1 bash deploy/deploy.sh --demo`
  — PASS, simulation only. `git diff --check` — PASS.

These checks validate command construction with mocked subprocesses. No live SDK
parser, Cloud Run deployment, new image build or cloud mutation was run locally.

### Reserved Cloud SQL mount recovery — 20 November 2026

The user reported the live API rejection:
`VolumeMount name 'cloudsql' is reserved`. The service command now declares an
explicit Cloud SQL volume `db-sockets` and mounts it into the API at `/cloudsql`
using the documented `gcloud alpha run deploy` volume flags. It does not combine
the custom volume with the legacy service annotation. Jobs retain their existing
single-container built-in connection configuration.

Branch: `fix/lab5-ep5-cloudsql-volume`, tested working-tree changes on base
`c21982612f21113f2c2960a7e03ed4387198dde4` (not a commit identifying this fix).

- New reserved-name/explicit-instance/container-scope regression: failed against
  the old command (28 deployment tests, one failure).
- `python3 -m unittest discover -s deploy/tests -p 'test_*.py' -v` — **36/36 PASS**
  after the fix (28 deployment tests, 8 mocked smoke-tool tests).
- `PROJECT_ID=dinequeue-test SOURCE_SHA=c21982612f21113f2c2960a7e03ed4387198dde4 DRY_RUN=1 bash deploy/deploy.sh --demo`
  — PASS, simulation only. `git diff --check` — PASS.

The local checks validate command construction and failure ordering with mocked
responses. The alpha SDK parser, real volume mount, startup/readiness and public
HTTPS journey remain unverified on GCP. No Docker image or cloud resource was
changed locally during this recovery.

### Deployment-only audit — 20 November 2026

Scope narrowed to making deployment succeed: CLI argument handling, Cloud SQL
mount, container startup/readiness, bootstrap URL and deployment recovery. No
application, business-rule, Dockerfile or CI changes. The exploratory changes to
`deploy/smoke_test.py` and its tests were removed from the working tree.

Tested working tree: `fix/lab5-ep5-cloudsql-volume`, based on
`c21982612f21113f2c2960a7e03ed4387198dde4`. The base SHA does not identify these
uncommitted fixes. In addition to the volume correction above:

- Check alpha CLI availability before resource mutation/build, with a 60-second
  preflight timeout and a fixed diagnostic if the SDK is absent or unavailable.
- Retry API-enable HTTP 429 failures after 60–61 seconds, at most five attempts.
  Do not retry permanent denials or unrelated secret writes/jobs/builds.
- Report fixed error categories without provider payloads; reject ambiguous
  discovery responses mixing not-found with denial/network failures, or a
  non-object JSON response.
- Permit bootstrap when an existing service has no URL; stop before granting
  public access if a deploy still returns no valid HTTPS URL.

Validation of the narrowed working tree:

| Check | Result and limit |
|---|---|
| `python3 -m unittest discover -s deploy/tests -p 'test_*.py' -v` | 44/44 PASS: 36 deployment tests plus 8 unchanged smoke-tool tests, with mocked cloud responses |
| `python3 deploy/tests/check_gcloud_sdk.py /tmp/dinequeue-deploy-audit/sdk-source` | SDK 594.0.0 parsed 56 planned fresh-deployment commands; SDK configuration transforms built both bootstrap/exact-origin service specs; no command Run method or cloud connection |
| Real installed SDK `GCloud.preflight()` | PASS, alpha CLI help only, isolated SDK config |
| Dry run with project `dinequeue-test`, region `asia-southeast1`, source SHA equal to the base above | PASS, simulation only |
| Existing production API/web images using shared localhost and a disposable local PostgreSQL Unix socket | Startup/readiness passed; readiness returned 503 with DB stopped and recovered to 200 after DB restart. Local socket verification does not emulate Cloud Run's managed CSI driver |
| `bash -n` on deploy/release/teardown entrypoints and `git diff --check` | PASS |

SDK core, dependencies, alpha and Unix wrapper archives were downloaded from
Google's official component manifest and SHA-256 verified before use. The SDK
audit uses isolated configuration and blocked socket connections; it does not
exercise Cloud Run admission, regional feature availability, IAM or live quotas.
The prior local runtime audit used existing image IDs, not newly built images.
Disposable local audit containers/socket volume/network/private credentials were
removed; developer dev/EP3 resources were excluded.

Cloud Run server-side acceptance and actual cloud startup/readiness remain pending
after the fixes reach reviewed main. This audit does not claim a successful GCP
deployment, real managed socket connection, or approval to close Issue #16.


### Cloud Run v2 native-volume correction — 20 November 2026

**Supersedes the alpha CSI deployment above.** The user reported a real failure
on main `160f5dd8772778e42dc2c590b938eb5fd964c4c9`:
`Unsupported driver: cloudsql.run.googleapis.com`. Parsing/transforming a CLI
command locally did not establish server support for that generated driver.

Branch `fix/lab5-ep5-cloud-run-v2` changes only deployment transport, its tests and
documentation. Main at the SHA above is the base, not the tested fix commit.
Service creation/update now uses the documented v2 native `cloudSqlInstance`
field through `services.patch?allowMissing=true`, with an explicit update mask,
short-lived in-memory credentials, no redirects, bounded HTTP requests and an
operation readiness gate. Jobs/images/database/application rules are unchanged.
The alpha CLI prerequisite and CSI driver were removed from the active path.

Local verification:

- New v2 regressions failed before implementation (5 test methods, 8 errors
  including setup errors for absent methods), then passed after implementation.
- `python3 -m unittest discover -s deploy/tests -p 'test_*.py' -v`: 52/52 PASS
  (36 first-deployment, 8 v2 transport/spec and 8 existing smoke-tool tests).
- `python3 deploy/tests/check_gcloud_sdk.py /tmp/dinequeue-deploy-audit/sdk-source`:
  SDK 594.0.0 parsed 54 stable CLI commands and round-tripped 2 native service
  requests through its v2 Service schema without dropped/changed fields.
- Dry run and `git diff --check`: PASS. All responses in transport tests are
  mocked. No API token was obtained and no Cloud Run API call was made locally.

Scope is deployment only; no browser/business-flow tests, Docker builds or live
cloud mutations were performed for this correction. API v2 server acceptance and
actual Cloud SQL startup/readiness still require live verification. Do not claim
deployment success or close Issue #16 based on these local results.
