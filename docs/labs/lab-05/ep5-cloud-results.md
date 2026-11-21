# EP5 — first real GCP deployment and verification

Date: 20 November 2026 (Asia/Bangkok).
Tracking: [Issue #16](https://github.com/bbxndg/DineQueue/issues/16), updated by the user.
Recorded later on the EP6 documentation branch at the user's request. This is
evidence for the following deployed EP5 release, not the later documentation SHA.

## Exact release and results

- Deployed main SHA: `7269e9ed28b10293a80f6f2c58c9c8a64937da99`.
- Public URL: <https://dinequeue-zdfotp22gq-as.a.run.app>.
- Project: `cpe334-dinequeue-514914`; region: `asia-southeast1`.
- Service: `dinequeue`; Cloud SQL instance: `dinequeue-pg`; DB: `dinequeue_demo`.
- Screenshot shows ready revision `dinequeue-00002-zkw` receiving 100% latest
  traffic; previous revision `dinequeue-00001-q48` receives 0%.

The user supplied the successful `bash deploy/deploy.sh --apply --demo` output:
build and digest resolution, migration/provision jobs, native v2 service POST,
PATCH to the actual origin, and the public invocation binding all completed.
No new cloud command was executed by the agent when recording these results.

| Image | Resolved immutable digest from successful apply output |
|---|---|
| api | `sha256:c1a214acaaed9f74ba4b67fdb90cb9712cdf5931eb6682b612ec46569b8bf386` |
| web | `sha256:e0bb5385443eb2aee0b6ddef1aca8e91c65df4237fa6c654e92032d948f54856` |
| db-tools | `sha256:53554501f856cb110da1214570bebff54b4ac9bc37e358692b10cbf4ec3805fa` |

All images use the prefix
`asia-southeast1-docker.pkg.dev/cpe334-dinequeue-514914/dinequeue/` and their
respective image name before `@sha256:...`.

## Public smoke and manual browser verification

User-supplied Cloud Shell commands/output, also visible in the deployment screenshot:

```text
curl --fail --show-error https://dinequeue-zdfotp22gq-as.a.run.app/api/health/ready
{"status":"ready"}

python3 deploy/smoke_test.py https://dinequeue-zdfotp22gq-as.a.run.app
PASS /api/health/live
PASS /api/health/ready
PASS /login
PASS /api/v1/auth/me
SKIP authenticated smoke: credentials not configured; Host/Customer journey not verified
```

The anonymous `/api/v1/auth/me` PASS means the expected 401 privacy response,
not a successful authenticated identity request. Scripted authenticated smoke
was **SKIPPED** and must not be reported as executed.

After these commands, the user reported that manual browser testing on the cloud
app completed without problems, following the requested flow: separate Customer
and Host sessions, join, call next, refresh to CALLED, complete, refresh to no
active ticket, and logout. This is user-reported manual verification, not a new
agent-run browser suite. No cloud browser trace or journey screenshots have
been attached in this repository yet.

## Screenshot inventory inspected locally

Original screenshots are kept on the user's machine under:

```text
/home/user/Downloads/Work - DineQueue/GCP/หลักฐานภาพ/EP 5/
```

| Original filename | What it establishes |
|---|---|
| `Deployed + Smoke test.png` | Successful apply summary with SHA/digests, readiness and four public smoke PASS results |
| `Cloud Run revision.png` | Ready `dinequeue-00002-zkw`, 100% latest traffic, previous revision at 0% |
| `migration และ provision jobs.png` | Screenshot inspected shows migration execution `dinequeue-migrate-wtpf8`, 1 succeeded, 0 failed, exit 0 |
| `Automatic storage increase.png` | SQL instance `dinequeue-pg`, 10 GB SSD and Auto storage increase Disabled |

The migration screenshot's filename mentions provisioning, but its visible page
only proves migration. Provision completion is supported by the successful apply
output; a separate provision execution screenshot/ID is not yet indexed here.
Other originals `dry run.png` and `Cloud SQL storage.png` are present locally but
were not used as verification for this record.

These paths are a local inventory, not hosted evidence links. Before final
submission, attach sanitized copies to Issue #16/the evidence report and replace
the inventory with actual attachment links as needed. The originals include
browser tabs/account identity; inspect/redact personal information and retain
the original screenshots privately. Never capture Secret Manager secret values.

## Remaining boundaries

The live deployment/public smoke are no longer NOT RUN. Full automatic-release
acceptance, real WIF authentication/denial and restart/redeploy persistence have
not been established by EP5. They remain EP6/EP7/EP8 evidence respectively.
No rebuild or redeployment is required solely to merge this documentation.
