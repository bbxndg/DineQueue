# Episode 7 — repeatable DineQueue releases

Date: 21 November 2026 (Asia/Bangkok). Tracking: Issue #18.
Branch: `feat/lab5-ep7-auto-deploy`, from reviewed `gcp-staging` `8916397`.
Status: pre-implementation contract; cloud authentication/deployment NOT RUN.

## Scope and decisions

Follow Student Guide §§7.1–7.7 and prepare §8 rollback support. Only reviewed
main deploys to the existing `demo` environment. `gcp-staging` is the integration
branch, not a second GCP deployment. Issue #18's older staging-deployment wording
does not match the course/EP6 main-only provider/environment setup; track that
scope mismatch explicitly rather than broadening IAM or creating infrastructure.
Optional authenticated smoke is deliberately skipped unless both optional
environment inputs are configured. Public smoke still runs on every deployment.

Add `Deploy demo` in `.github/workflows/deploy.yml`: push main and manual dispatch;
manual `image_tag` must be a full 40-character SHA and selects reuse/rollback mode.
Normal releases reuse existing CI, authenticate with pinned WIF actions, build
three production targets from a tracked-source-only archive, push SHA tags,
resolve all digests, validate the update request, update/await migration, patch
both app images together on the existing service, await ready, smoke and summarize.
Deployer uses existing EP6 permissions; no SQL/IAM/secrets provisioning, public
binding changes or account password reset. No product features or dependency bumps.

## Important rules and failures to test first

- Default release CLI is a no-network plan. Only `--apply` mutates resources;
  `--validate-only` resolves existing images and checks the PATCH without persisting.
- Reject malformed/missing project, region, tag and rollback settings before GCP.
- No PR trigger or PR OIDC credentials. Environment `demo`, main-only job guards,
  `contents: read` by default and `id-token: write` in deploy job only. Serialize
  releases without cancelling a running migration.
- Reject missing/invalid image digests before any job/service mutation. Require
  an existing expected service/template/URL. Never POST/create the service.
- Build context excludes ignored `.env`/WIF credential files; auth JSON cannot be
  committed or included in images. Add credential-file ignore patterns as defense.
- Copy existing v2 revision configuration, remove its old revision name, change
  only named web/API images and source labels, and request latest traffic 100%.
  Preserve runtime identity, secrets, Cloud SQL volumes, origins, ports and probes.
- Validate PATCH before migration. Normal release updates only the migration
  image (db-tools digest) and waits; any failure stops before the real service PATCH.
- Rollback/roll-forward via existing SHA skips tests/build/migration/provision;
  update both API/web digests from the same SHA. Database is not rolled back.
- Await the returned operation with bounded polling and exact operation scope;
  reject errors/unready/mismatched images/revision/traffic. Keep tokens/payloads
  out of diagnostics. Smoke failure fails the workflow, never a green release.
- No automatic rollback after smoke failure or destructive cleanup. Human chooses
  a known compatible earlier image SHA; migrations must remain backward-compatible.

## Verification and evidence

Fail-first offline release tests cover ordering, no writes after failure,
rollback skipping jobs, preservation of configuration, etag conflicts, invalid
targets, redaction, operation timeouts and output injection. Build-script stubs
prove only tracked source reaches context, all three targets/tags agree and any
failed build/push stops. Workflow checks/actionlint cover triggers, gates, WIF
scope, rollback skipping and smoke failure propagation. Audit v2 payload and CLI
grammar against the official installed SDK; these checks are not live admission.

Before review: run offline deployment/CI configuration suites and Bash syntax;
app/Go/browser checks remain the reused CI workflow and need not be rerun locally
for unchanged app code. Record actual results and SHA/worktree scope honestly.
Then feature PR → gcp-staging → reviewed release PR → main. Only the main merge
can supply real WIF/deploy evidence. Keep Issue #18 open until the first release
and a second visible-change release both pass and match Cloud Run/registry/jobs.
§7.7's visible change is a separate proof branch after first live success.
EP8 persistence/rollback execution remains separate, though the pipeline must
support existing-image dispatch from the start.

## 7.7 visible-change proof contract

Recorded before the copy change on 21 November 2026. Release A is main
`65c7a496127e8d04615162c85c7dffb0ea9babdd`; Actions run `44902787482`, attempt 2,
completed successfully. The user reports completing the Console comparison and
opening the public URL. Staging is fast-forwarded to A before creating
`feat/lab5-ep7-deploy-proof`.

Change only the login-page introductory sentence from
`Sign in to your restaurant waitlist account.` to
`Sign in to join the waitlist or manage your restaurant queue.`
The new text must be visible to a signed-out visitor at `/login` after release B.
Authentication, queue rules, styles, Dockerfiles and CI/deploy configuration are
outside this change. Run existing web unit tests, lint, build and typecheck;
record their actual results and tested source commit. No extra assertion that
merely compares this sentence is needed.

Use reviewed proof PR into staging, then reviewed release PR into main. B must
start through the existing push workflow, complete successfully, match the
revision/digests in Console and display the new sentence after a hard refresh.
Until those live checks happen, B is pending and Issue #18 stays open. Preserve
the full A and B SHAs for the separate EP8 persistence/rollback walkthrough.
