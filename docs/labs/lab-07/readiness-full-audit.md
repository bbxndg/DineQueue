# Readiness full change-set audit — 26 November 2026

> Historical verification snapshot: checks below ran before follow-up commits.
> The user subsequently authorized three atomic commits and a push on 26 November.
> References to uncommitted/unpushed state describe the time of verification.

Audit base: six commits `8539b24..8deb367` (51 files), plus current uncommitted
peer-review/smoke fixes. **No new commit or push is authorized or performed.**

## Confirmed findings and corrections

1. **OpenAPI rejected valid runtime DTOs.** The implemented API export still had
   additionalProperties=false without isReady on Entry/Ticket/HostEntry, omitted
   readiness PATCH, and described strict FIFO without NO_READY_PARTIES. Updated
   schemas, required booleans, request/response/error/header definitions and calling
   description. This versions implemented S1 behavior; no S2 routes were added.
2. **Stalled readiness PATCH could leave the button disabled indefinitely.**
   Reproduced with an intercepted request that never completed. Added a 15-second
   AbortSignal timeout using the existing uncertain-outcome message. On timeout the
   client does not assume the write failed; the Customer can refresh and retry.
   New browser coverage tests actual fetch cancellation, unchanged visible state,
   restored controls and successful recovery. Only the first signal is accelerated
   in that test, preserving the normal recovery timeout to avoid timing flakiness.
3. **STS status text was stale.** Replaced the remaining blanket NOT RUN wording
   for commands that already have recorded runs; retained per-run/historical
   evidence and pending polling scope. Contract now labels the PR #45 defaults and
   interface as reviewed, while implementation/release approval remains pending.

Previous smoke regression and the three proposed peer-review improvements remain
in this worktree; see readiness-audit.md and readiness-peer-improvements.md.
Workflow triggers, Dockerfiles and cloud resources were not changed.

## Review coverage

- Migration: additive, existing rows Ready, new joins Not ready, earlier migration
  checksums unchanged; no startup AutoMigrate or destructive data reset.
- Backend: owner/role/Origin/CSRF guards, strict boolean parsing, queue-before-entry
  locks, conditional writes, original FIFO/tie-break, one-CALLED invariant,
  same-value idempotency, DTO privacy and authoritative server state.
- UI: readiness controls, disabled mutations, conflict recovery, session redirects,
  exact rewrite, own-detail HTTP 404 boundaries, mobile/keyboard existing coverage.
- Tests: original preserved-rule assertions, explicit ready fixtures, deterministic
  concurrency, new failure/timeout coverage and isolated fixture cleanup.
- Docs/evidence: relative links resolve; original source manifest matches the
  recorded committed version; original 66-test and 21-browser summaries match
  their logs, and failed attempts are retained. Existing raw outcomes were not
  edited to claim success. Original evidence is historical, not this new run.
- OpenAPI local references resolve; validated against actual API DTOs separately.

Separate Host waiting/current reads are still not an atomic snapshot; the existing
polling contract explicitly assigns snapshot reconciliation to the later polling
branch. Backend locks remain authoritative. This audit does not implement polling
or claim that the whole Lab 7 acceptance is complete.

## Actual verification of this working tree

Verification ran on base `8deb367` plus the uncommitted working tree below.
All databases were disposable PostgreSQL; the developer database was untouched.

| Actual command / check | Result |
|---|---|
| `go test -race ./...` | PASS |
| `go vet ./...` | PASS |
| `go test -tags=integration -race -count=1 -v ./...` | PASS: 66 top-level tests; no failure/race report |
| `npm run test:e2e --workspace @dinequeue/web` | PASS: 24/24, including timeout, network recovery, 409 and S1 regression |
| `npm test` | PASS: 2/2 |
| `npm run lint`, `npm run typecheck` | PASS |
| `API_INTERNAL_URL=http://127.0.0.1:18081 npm run build` | PASS |
| `python3 /tmp/dq-l7-openapi-check.py` | PASS: actual Entry, Ticket, readiness PATCH and HostEntry DTOs; old export mismatch reproduced |
| OpenAPI local references / changed-doc relative links | All 78 refs and file links resolve |
| Original committed source manifest | All 26 hashes match the historical recorded commit |
| `git diff --check` | PASS |

[Captured audit outputs](evidence/readiness-full-audit.txt). The stalled-request
case failed on the old built UI before the fix, then passed its targeted rerun,
then passed in the complete 24-case suite. Temporary native signal acceleration
was limited to the first request so successful retry uses the normal deadline.

Go used the cached test image with read-only source, a separate internal network
and disposable DB. Browser used a newly built production Next app plus APP_ENV=test
API and synthetic accounts on loopback ports. Actual API responses were validated
with the locally available jsonschema package. This does not claim production
Docker-image, hosted CI, HTTPS or Cloud Run acceptance. Accepted Lab 6 baseline,
real peer approval, polling integration and release/deployment remain open items.

## Original 51-file inventory

The 51-file diff contains 26 source/test files, 9 Markdown documents and 16 evidence
files. Additional peer/smoke/new audit files are outside that original count.

| File | Category |
|---|---|
| `apps/api/db/migrations/0003_customer_readiness.sql` | source/test |
| `apps/api/internal/queue/host_http.go` | source/test |
| `apps/api/internal/queue/host_integration_test.go` | source/test |
| `apps/api/internal/queue/host_repository.go` | source/test |
| `apps/api/internal/queue/host_service.go` | source/test |
| `apps/api/internal/queue/http.go` | source/test |
| `apps/api/internal/queue/my_ticket_integration_test.go` | source/test |
| `apps/api/internal/queue/readiness.go` | source/test |
| `apps/api/internal/queue/readiness_integration_test.go` | source/test |
| `apps/api/internal/queue/readiness_repository.go` | source/test |
| `apps/api/internal/queue/readiness_rules_integration_test.go` | source/test |
| `apps/api/internal/queue/readiness_test.go` | source/test |
| `apps/api/internal/queue/repository.go` | source/test |
| `apps/api/internal/queue/service.go` | source/test |
| `apps/web/e2e/queue-operations.spec.ts` | source/test |
| `apps/web/e2e/readiness.spec.ts` | source/test |
| `apps/web/e2e/sprint1-review.spec.ts` | source/test |
| `apps/web/next.config.mjs` | source/test |
| `apps/web/src/app/my-queue/(list)/page.tsx` | source/test |
| `apps/web/src/app/my-queue/[entryId]/page.tsx` | source/test |
| `apps/web/src/components/host-board.tsx` | source/test |
| `apps/web/src/components/join-form.tsx` | source/test |
| `apps/web/src/components/readiness-control.tsx` | source/test |
| `apps/web/src/components/ticket-card.tsx` | source/test |
| `apps/web/src/lib/host.ts` | source/test |
| `apps/web/src/lib/tickets.ts` | source/test |
| `docs/01-srs.md` | documentation |
| `docs/04-data-and-api.md` | documentation |
| `docs/05-ui-spec.md` | documentation |
| `docs/06-restaurant-operations.md` | documentation |
| `docs/09-traceability.md` | documentation |
| `docs/13-decisions.md` | documentation |
| `docs/labs/lab-07/evidence/readiness-browser-first-regression.txt` | evidence |
| `docs/labs/lab-07/evidence/readiness-browser-regression.txt` | evidence |
| `docs/labs/lab-07/evidence/readiness-browser-targeted.txt` | evidence |
| `docs/labs/lab-07/evidence/readiness-browser.txt` | evidence |
| `docs/labs/lab-07/evidence/readiness-final-go.txt` | evidence |
| `docs/labs/lab-07/evidence/readiness-go-checks.txt` | evidence |
| `docs/labs/lab-07/evidence/readiness-go-first-regression.txt` | evidence |
| `docs/labs/lab-07/evidence/readiness-green-go.txt` | evidence |
| `docs/labs/lab-07/evidence/readiness-red-test.sha256` | evidence |
| `docs/labs/lab-07/evidence/readiness-red.txt` | evidence |
| `docs/labs/lab-07/evidence/readiness-restart.txt` | evidence |
| `docs/labs/lab-07/evidence/readiness-source-manifest.json` | evidence |
| `docs/labs/lab-07/evidence/readiness-web-build.txt` | evidence |
| `docs/labs/lab-07/evidence/readiness-web-lint.txt` | evidence |
| `docs/labs/lab-07/evidence/readiness-web-tests.txt` | evidence |
| `docs/labs/lab-07/evidence/readiness-web-typecheck.txt` | evidence |
| `docs/labs/lab-07/readiness-contract.md` | documentation |
| `docs/labs/lab-07/readiness-results.md` | documentation |
| `docs/labs/lab-07/readiness-sts.md` | documentation |

## Audited working-tree source hashes

- `apps/api/internal/queue/host_repository.go`: `4ae46677f0d0768daa72e22935f2632a31c4208b64fe153340a68d09f5ec4b8c`
- `apps/api/internal/queue/host_service.go`: `d19165ec6536b5a527b96dc7340e9ebd237915008604ffdb4bbf7403fbb34a2a`
- `apps/api/internal/queue/readiness_rules_integration_test.go`: `fef341aaaae2148698e56c89480c39d48df124022f79f41f8dd2c7ace22650eb`
- `apps/web/e2e/readiness-errors.spec.ts`: `4011ec2286f486e20508ec8fa3469a9567799d3dcf09e9125bcb383be44b735f`
- `apps/web/src/components/readiness-control.tsx`: `c7129bea2737fba7f8d5202669746c26f859ae1e1f6900031e40753d9e130612`
- `docs/api/sprint1.openapi.json`: `cabf5e65842b82807214ea61a23e6167e949f878680a60c51fb1aeb08663e663`
- `scripts/lab5/docker-smoke.mjs`: `ccfa48ba9c70794325e912617a45ab7b4f6cfaed40c2d51ee06735997390a1bd`

## Second pass — fixture scope

Found an additional test-isolation issue: queue-operations.spec.ts marked every
WAITING ticket Ready across all queues before draining the selected queue. Scoped
the UPDATE to its queueId and validated the UUID before inserting it into fixture
SQL. No production behavior, authorization, migration or CI trigger changed.

`python3 /tmp/dq-l7-scope-check.py` executed the original and updated actual fixture
queries against two disposable PostgreSQL queues: old SQL modified both queues;
new SQL readied only the selected queue and left the other Not ready. Test records
were rolled back. Lint, typecheck and diff whitespace checks passed afterward.
No full browser rerun is claimed for this fixture-only follow-up; the prior 24-case
result remains the previous run. No additional confirmed runtime bug was found in
this pass. All work remains uncommitted and unpushed.

Updated fixture SHA-256: `10221b77771c4a80cd3c8b3f2ff55c7e2d39f9d5efd918e21e08535bd96d1557`.
