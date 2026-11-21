# Episode 7 — local implementation results

Date: 21 November 2026 (Asia/Bangkok). Tracking: Issue #18.
Branch: `feat/lab5-ep7-auto-deploy`, based on `gcp-staging` `8916397`.
Pre-implementation contract commit: `add7384`.
The deployment suite (77 tests), CI helpers (7 tests) and actionlint were rerun
on implementation/workflow commit `8e2e43002e4467f5e62c83a1d00a6d8af82d1bd1`.
The other offline checks below were performed on the same script content before
that commit. The initial local record was a documentation-only change; subsequent
live release A evidence is recorded separately below. See [runbook](ep7-runbook.md).

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

## Actual pipeline release A

Verified through read-only GitHub Actions metadata and deployment-job logs on
21 November 2026. The user separately reports completing the GCP Console
comparison in 7.6 and opening the public URL; those Console/browser checks were
not executed by the agent.

- Feature PR: [#37](https://github.com/bbxndg/DineQueue/pull/37).
- Release PR: [#38](https://github.com/bbxndg/DineQueue/pull/38).
- Release A SHA: `65c7a496127e8d04615162c85c7dffb0ea9babdd`.
- [Deploy demo run 44902787482, attempt 2](https://github.com/bbxndg/DineQueue/actions/runs/44902787482/attempts/2)
  completed successfully. [CI job](https://github.com/bbxndg/DineQueue/actions/runs/44902787482/job/136848001553)
  and [deploy job](https://github.com/bbxndg/DineQueue/actions/runs/44902787482/job/136849243077)
  both passed.
- Deploy job confirms WIF authentication, all three image builds/pushes,
  migration/deployment and public smoke success.
- URL: `https://dinequeue-zdfotp22gq-as.a.run.app`.
- Ready revision: `dinequeue-00003-qwp`.
- Registry prefix: `asia-southeast1-docker.pkg.dev/cpe334-dinequeue-514914/dinequeue`.

| Image | Resolved digest from the release output |
|---|---|
| api | `sha256:c1ddb68a2ccb998f8e49ffb3e65e9eaf1abfd1694b40d978ce25adc3fb3f79c6` |
| web | `sha256:e8655a7e54dd576818780f352148e8d40085fc9ce3b3aeb0ce03f41a676ce8af` |
| db-tools | `sha256:b643383214de9477cf04c4b9b311b0c802465a723f8d96c8ee6bd57b0cc7960d` |

The deploy log reports PASS for `/api/health/live`, `/api/health/ready`, `/login`
and anonymous `/api/v1/auth/me`. Authenticated smoke is SKIP, as deliberately
chosen for optional 7.3; no automated Host/Customer journey is claimed.

Attempt 1 is retained as failure evidence: WIF and image builds passed, but the
first registry push was denied `artifactregistry.repositories.uploadArtifacts`.
Migration/deploy/smoke were skipped. After the repository-permission walkthrough,
the owner reran the same workflow; attempt 2 succeeded without a source change.

## 7.7 release B preparation and remaining evidence

Staging is fast-forwarded to A before creating `feat/lab5-ep7-deploy-proof`.
The pre-implementation proof contract is in [the contract](ep7-contract.md).
The login introduction changes from `Sign in to your restaurant waitlist account.`
to `Sign in to join the waitlist or manage your restaurant queue.` Signed-out
visitors can use this sentence to distinguish B from A at `/login`.

Proof source commit: `2a2c451d7d2ad890255916f337e5234fa9149fd4` (the same
one-line source change was checked in the working tree before committing).

| Local proof command | Actual result |
|---|---|
| `npm test` | PASS; local runner reports 1 passing test-file entry, no failures |
| `npm run lint` | PASS, exit 0 |
| `npm run typecheck` | PASS, exit 0 |
| `npm run build` | Local verification blocked: two attempts failed with `Could not parse output from TypeScript's --showConfig.` A retry outside the sandbox made no further progress and was interrupted (exit 130); no build PASS is claimed |
| `git diff --check` | PASS |

The local diagnostic ran TypeScript `--showConfig` directly and parsed valid JSON.
The same CLI launched with Node `child_process.spawn` returned exit 0 with zero
stdout/stderr bytes, reproducing the missing output that prevents Next's JSON
parse. No source/dependency/configuration workaround was made. Hosted PR CI must
verify the proof build before merge; release A's earlier passing build does not
establish that this proof commit passed.

Hosted proof/release PR CI, review, release B pipeline, full B SHA, Console
comparison and public visible-change verification are still pending. Preserve
A and B for the separate EP8 persistence/rollback walkthrough; Issue #18 stays
open until B and evidence acceptance are complete.

Issue #18's older staging-deployment wording differs from the course/EP6 main-only
scope used here; no second cloud environment is created. EP5's historical SHA
remains `7269e9ed28b10293a80f6f2c58c9c8a64937da99`; A above is the subsequent
pipeline release, not a replacement of that historical evidence.
