# Lab 5 Episode 4 — PR CI contract (Issue #15)

Status: pre-implementation plan; checks and hosted CI NOT RUN at this point.
Date: 18 November 2026 (Asia/Bangkok).
Base: `gcp-staging` after EP3 PR #23, commit `3c80163`.
Branch: `ci/lab5-15-ep4-pr-checks`.

## Scope and design

- Add `.github/workflows/ci-docker.yml`, named `CI & Docker Build`, with stable job ID/name `test-and-docker`.
- Run for pull requests targeting `gcp-staging` and `main`; retain `workflow_call` and `workflow_dispatch`. Do not use `pull_request_target` or grant write/OIDC permissions to PR code.
- Use `contents: read`, checkout without persisted credentials, Actions pinned to verified official commit SHAs, a bounded timeout and cancellation of superseded runs.
- Install the repository's pinned Node 22 and Go 1.26.10 toolchains; use the existing lockfiles and disable npm install scripts.
- Check Go formatting, vet, unit and real PostgreSQL integration tests with race detection. The service database is disposable and distinct from the browser app database; do not use developer or GCP credentials.
- Run web tests, lint, production build and typecheck (build first to generate Next.js route types on a clean checkout).
- Build all three EP3 production image targets through the production Compose file. Run the existing 20 browser checks against those images with one worker and the Playwright Chromium matching the pinned package version.
- Generate browser database/seed credentials per run, mask them and keep private environment files under runner temporary storage. Run explicit migration/seed only against the isolated local app. Clean up only that CI-created project.
- Import the course starter's entire `deploy/` folder and run its Bash syntax, stubbed gcloud tests and Python offline tests. Label these as **TokTickIT starter reference checks**, not DineQueue deployment verification. EP5 section 5.0 requires adapting its single-image/Prisma/DB connection assumptions to our separate Next.js/Go images before any live use. No imported cloud script is to be run against real GCP in EP4.
- Add provenance/limitations beside the imported reference scripts and document the CI check name, run commands and evidence procedure in the Lab guide. No Sprint 2/3 business behavior, GCP resource, release pipeline or branch-protection setting is changed here.

## Acceptance and test strategy

1. A PR into `gcp-staging` starts `test-and-docker` automatically and records the tested head/merge SHA plus actual results for each stage.
2. A separate temporary branch/PR introduces one deliberately failing synthetic Go test. Capture the actual failed run and close the PR without merging it; retain the accepted branch without the failing file.
3. The passing implementation PR reports all Go/web/offline-script/Docker/browser outcomes with run links. Local execution is evidence only of the local command, not a hosted Actions PASS.
4. Inspect logs and changes for real credentials; do not upload environment files, cookies, traces or private customer data. Synthetic service-only test credentials are not deployment credentials.
5. The stable required-check name and its triggers are documented for EP6. A red CI run in EP4 does not prove enforced merge blocking; that requires EP6 branch rules and a separate actual check.
6. Obtain teammate review before merge into `gcp-staging`. Per the Student Guide EP4 completion, the reviewed release PR from `gcp-staging` to `main` must run the same check and be merged before declaring EP4 fully complete. Author approval, review, release merge and deployment must not be fabricated.

Expected regression: existing DineQueue Go and browser suites retain their own counts; starter offline tests are reported separately. Any setup failure, skip or unavailable GitHub capability is recorded explicitly. The helper scripts and workflow must stop on a failed check, and deployment credentials are never supplied to this workflow.
