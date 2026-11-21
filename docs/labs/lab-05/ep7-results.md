# Episode 7 — local implementation results

Date: 21 November 2026 (Asia/Bangkok). Tracking: Issue #18.
Branch: `feat/lab5-ep7-auto-deploy`, based on `gcp-staging` `8916397`.
Pre-implementation contract commit: `add7384`.
The deployment suite (77 tests), CI helpers (7 tests) and actionlint were rerun
on implementation/workflow commit `8e2e43002e4467f5e62c83a1d00a6d8af82d1bd1`.
The other offline checks below were performed on the same script content before
that commit. This record is a later documentation-only change; it does not claim
a hosted Actions run or a deployed release. See [runbook](ep7-runbook.md).

## Implemented scope

- Main-only `Deploy demo` workflow reuses existing CI, authenticates through WIF,
  builds/pushes API, web and db-tools from a tracked-source archive, resolves
  digests, validates the existing service update, awaits migration, updates both
  app containers, verifies readiness/traffic and runs public smoke checks.
- Existing-image dispatch requires a full SHA in main history and skips
  build, migration and provisioning. It changes application images only.
- Existing origins, secret references, identities, Cloud SQL configuration and
  probes are preserved. No infrastructure bootstrap, password reset or seed.
- Application source, production Dockerfiles and `ci-docker.yml` are unchanged.
  The existing CI shell entry now executes the DineQueue deployment suite rather
  than counting archived course-reference release scenarios as project tests.

## Actual local verification

| Command/check | Observed result | Limits |
|---|---|---|
| `bash deploy/tests/test_release_stub.sh` | PASS: 77 tests | 56 existing deployment/smoke tests + 16 release tests + 5 build-script tests; cloud/Docker calls are mocked or executable stubs |
| `python3 -m unittest discover -s scripts/ci/tests -p 'test_*.py' -v` | PASS: 7 tests | Existing CI helper suite; not a hosted workflow run |
| `bash -n` on deploy, release, build-images, teardown and release-test shell entries | PASS | Shell syntax only |
| actionlint 1.7.12 on `ci-docker.yml` and `deploy.yml`, with external ShellCheck disabled | PASS | Workflow syntax/static validation; no GitHub execution |
| `python3 deploy/tests/check_release_sdk.py /tmp/dinequeue-deploy-audit/sdk-source` | PASS: 11 CLI command parses and 5 v2 PATCH schema round-trips | Official local Google SDK grammar/types; command execution and network disabled |
| `python3 deploy/tests/check_gcloud_sdk.py /tmp/dinequeue-deploy-audit/sdk-source` | PASS: 54 CLI command parses and 2 service schema round-trips | Regression of unchanged first-deployment request generation; offline only |
| Default `bash deploy/release.sh`, with valid settings, normal and existing-image modes | PASS: prints plan without cloud calls | Does not resolve live images or validate with GCP |
| `git diff --check` | PASS | Whitespace only |

The first release-test run failed at import because `release.py` did not yet
exist. After implementation the release assertions passed; this was an import
failure first, not evidence that every business assertion failed first.

Release tests cover validation-before-migration ordering, migration/image/request
failures stopping the service update, rollback skipping jobs, configuration
preservation, etag conflicts, scoped operation polling, readiness/traffic checks
and sanitized failures. Build tests execute the shell script with stubbed
Docker/gcloud, inspect the temporary context, and exercise build/push failure,
credential-file rejection and cleanup. They perform no real image builds.

No new Go, web or browser regression run is claimed here: their source is
unchanged and the actual main pipeline must run the reused CI before deployment.

## Live evidence still required

- Hosted PR CI and teammate review of the final committed snapshot.
- First main pipeline release A: real WIF authentication, registry pushes,
  migration execution, ready Cloud Run revision and passing public smoke.
- Run/PR URLs, full A SHA, digest/revision/traffic evidence from Console.
- Separate small visible-change release B after A succeeds, demonstrating a
  second automatic deployment without a manual deploy command.
- Keep both full image SHAs for EP8 rollback/roll-forward and persistence proof.

Optional authenticated scripted smoke remains SKIP unless both inputs are
configured; previous EP5 manual browser evidence is not a new pipeline test.
Issue #18 stays open pending actual release/evidence acceptance. Its older
staging-deployment wording differs from the course/EP6 main-only scope used here;
no second cloud environment or additional WIF permissions were created.

EP5's deployed SHA remains `7269e9ed28b10293a80f6f2c58c9c8a64937da99`.
Local preparation does not replace that deployment evidence with a feature SHA.
