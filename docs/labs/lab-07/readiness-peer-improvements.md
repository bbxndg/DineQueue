# Readiness peer-improvement preparation — 26 November 2026

> Historical verification snapshot: checks below ran before follow-up commits.
> The user subsequently authorized three atomic commits and a push on 26 November.
> References to uncommitted/unpushed state describe the time of verification.

Base commit: `8deb367`; **all changes here remain uncommitted and unpushed**.
The user requested implementing the three proposed review comments in advance.
This records local work, not a claim that a teammate has submitted or approved review.
No workflow trigger was added.

## Implemented points

1. `TestL7ReadinessValidationPrivacyAndCSRF` now snapshots readiness, status and
   version before each rejected request and compares persisted state afterward.
   All 11 rejection cases retain their HTTP/privacy assertions.
2. `CallNext` uses repository `hasWaitingEntries` with SELECT EXISTS instead of
   loading/sorting every WAITING row. The check remains in the same transaction,
   after the same queue lock, and preserves CALL_IN_PROGRESS, QUEUE_EMPTY and
   NO_READY_PARTIES precedence and response codes.
3. New `readiness-errors.spec.ts` exercises an aborted browser PATCH with recovery,
   and a real 409 when the stale Customer view submits readiness after Host calls.
   The latter updates state through actual owner/Host APIs, rather than returning
   a fake conflict while the underlying ticket is still WAITING. Fixtures have
   generated identities, separate queues and scoped cleanup in an isolated DB.

## Actual checks

- `go test -tags=integration -race -count=1 -v ./internal/queue`: PASS,
  38 top-level tests plus 13 subtests, including all queue regression/concurrency.
- `go vet ./internal/queue`: PASS. Updated API binary compilation: PASS.
- `npm run test:e2e --workspace @dinequeue/web -- readiness-errors.spec.ts readiness.spec.ts`:
  final run PASS, 3/3 (two new cases and the preserved successful journey).
- `npm run lint`, `npm run typecheck`, `git diff --check`: PASS.

[Final Go and browser output](evidence/readiness-peer-improvements.txt).
The first browser attempt had 2 passes and one selector failure: Next's route
announcer also had role=alert. Narrowed the selector by the expected error text,
reran the failed network case (PASS), then all three together (PASS). No application
behavior or assertion was weakened to hide a failure.

The updated API and an existing production-built Next app ran locally with APP_ENV=test,
synthetic credentials and disposable PostgreSQL data. No production image build,
cloud deployment or hosted CI run is claimed. Old full-suite results in
readiness-results.md remain historical; this is a targeted follow-up, not a fresh
23-case full browser regression claim. Previous smoke-script improvements remain
in the working tree and are documented separately in readiness-audit.md.

## Tested working-tree source SHA-256

These identify the uncommitted sources tested, rather than attributing results to
unchanged base commit 8deb367:

- `apps/api/internal/queue/host_repository.go`: `4ae46677f0d0768daa72e22935f2632a31c4208b64fe153340a68d09f5ec4b8c`
- `apps/api/internal/queue/host_service.go`: `d19165ec6536b5a527b96dc7340e9ebd237915008604ffdb4bbf7403fbb34a2a`
- `apps/api/internal/queue/readiness_rules_integration_test.go`: `fef341aaaae2148698e56c89480c39d48df124022f79f41f8dd2c7ace22650eb`
- `apps/web/e2e/readiness-errors.spec.ts`: `a0cf7cd75f7d75d4357de814cd454dee4424b379865366038f057e20bdd508c2`
