# F-03 local execution evidence — 10 November 2026

Branch: `feat/F-03-my-ticket-privacy`. Base HEAD: `21d13da`.
Results below describe the uncommitted worktree, not a release commit or CI run.
The committed Sprint 1 F-03 contract precedes this work; the implementation
addendum was written in the worktree before changes, without a new commit.
No Issue, teammate approval, integration, deployment, commit or push had been
performed for F-03 when these checks ran. The user subsequently requested five
focused commits; the checks below precede those commits.

## Scope and environment

- Own active/detail reads hardened in `internal/queue`: CUSTOMER role gates in
  HTTP and services, owner predicate in SQL, explicit projection and no-store.
- My Ticket, empty/detail/loading/error pages, explicit refresh and join redirect.
  Queue/restaurant/timezone labels, UTC timestamps and YYYY-MM-DD service dates.
- No new dependency, migration or Host/Sprint 2/3 workflow.
- Linux amd64; Go 1.26.10 (official go.dev archive, SHA-256 verified against
  official release metadata); Node 22.17.0/npm 10.9.2; PostgreSQL 18.1 binaries.
  Runtime targets remain Node 22.23.4/PostgreSQL 18.6.
- Go modules downloaded through the official proxy with go.sum verification.
  Existing npm dependencies were used; no install scripts or lockfile changes.
- PostgreSQL runs in disposable `/tmp` clusters with random credentials. Go
  integration fixtures migrate/seed independent schemas and clean them up.
- Browser runner: installed Playwright 1.65.0 with local Google Chrome
  150.0.7871.46 via temporary config, rather than pinned Chromium revision 1254.
  Production `next start` and compiled Fiber API; isolated seeded demo accounts.
  Traces/video/screenshots disabled; no generated password printed by harness.

## Actual checks

| Command / check | Actual result |
|---|---|
| `go test -tags=integration -race -count=1 -v -run TestF03 ./internal/queue` before backend changes | RED: 4 failed, 2 passed. Host active read returned 200; GORM projection returned zero-valued entry fields and existing own-detail test also failed |
| `go test -tags=integration -race -count=1 -v ./...` after backend changes | PASS: 38 top-level tests, including all six F-03 tests, F-02 10/20-way join races and F-01 regression; no race report |
| `go vet -tags=integration ./...` | PASS |
| `go build -buildvcs=false` for `./cmd/api` and `./cmd/db` | PASS |
| `go mod verify` | PASS, all modules verified; initial sandbox DNS block retried with authorized network access |
| `npm test` | PASS, existing API-origin test file |
| `npm run lint` | Initial failure: plain anchor for refresh; changed to router-refresh button, subsequent run PASS |
| `npm run typecheck` | PASS |
| `npm run build` | PASS for dynamic `/my-queue` and `/my-queue/[entryId]`; rebuilt after join-response fix |
| Browser first execution | 9 passed, 2 failed while reading join response after navigation. Added response consumption before redirect; follow-up execution recorded below |
| UUID review regression | New URN UUID case initially returned 500; canonical-form guard added. Final API suite below passes |
| Browser second execution | 11 browser tests PASS; actual API restart PASS. Extra real-DB error test showed retry remained on the error boundary; fixed by refreshing server data when resetting |
| Final `go test -tags=integration -race -count=1 -v ./...` | PASS: all 38 top-level tests on final backend, including URN regression |
| Final `go vet -tags=integration ./...`, API/db builds | PASS |
| Final `npm run lint`, `npm run typecheck`, `npm run build` | PASS on final frontend |
| Final Playwright run (`playwright test --config <temporary local Chrome config>`) | **12 PASS**: seven F-01, two F-02/join and three F-03 browser tests; production web/API and fresh real PostgreSQL |
| T-035 actual API restart without DB reset | PASS: same session retrieves identical active ticket before/after stopping and restarting the compiled API |
| API/web service log scan | Generated seed and DB passwords absent |
| Harness cleanup | Temporary processes stopped and cluster removed |

Final execution was around 19:28–19:29 Asia/Bangkok on 10 November; source is
base `21d13da` plus this worktree. Final log: `/tmp/dineq-f03-final.log`.
The final browser suite includes the stored CALLED/COMPLETED and read-error/retry
check in `e2e/ticket-states.spec.ts`; it requires psql and TEST_DATABASE_URL for the
same disposable seeded API database. The test restores the table and deletes its
generated customer/session/ticket fixtures in `finally`.

Initial full harness stopped before browser startup because its CLI output path
collided with the temporary database directory. The harness path was corrected;
Go results above had already completed. No product failure is inferred from that
setup error. Local logs are `/tmp/dineq-f03-red.log`,
`/tmp/dineq-f03-green.log`, `/tmp/dineq-f03-browser.log` and
`/tmp/dineq-f03-web-build.log`; temporary logs are not durable CI artifacts.

## Test mapping

| Test / check | Acceptance coverage |
|---|---|
| `TestF03CustomerRoleRequired` | T-021 anonymous 401; Host 403 on active/detail; independent service role checks |
| `TestF03PersistedValuesLifecycleAndPrivacy` | T-008/T-018/T-020/T-021; exact persisted number/size/status/time/date, DTO allowlist, ownership, generic 404, revoked session; CALLED active and COMPLETED owner-readable only |
| `TestF03ActiveAcrossQueuesHasStableOrderAndNoCache` | Per-owner multi-queue reads, `(joined_at,id)` order, no-store on success and not-found |
| Existing `TestF03ActiveAndDetailEntry`, `TestF03EmptyActiveList`, `TestF03CrossUserPrivacy` | Original own-read, empty and cross-user API regression |
| `e2e/join.spec.ts` | T-010 full join-to-My-Ticket journey, duplicate banner and recovery link; F-02 regression |
| `e2e/my-ticket.spec.ts` | T-010/T-018/T-019/T-020/T-021; empty/join/reload/detail, persisted API comparison, foreign browser/API denial, logout and expired/Host routes |
| `e2e/ticket-states.spec.ts` | Real stored CALLED refresh, COMPLETED removal from active and retained own detail; actual DB read failure produces error rather than empty, Try again recovers after DB restoration |

## Remaining limits

No Docker, CI, GCP deployment, pinned Chromium 1254 or production TLS/proxy test
was run here. The local Go/browser checks do not constitute teammate review or
TA approval. User-authorized commits follow execution; review and integration
remain separate pending steps.
Temporary local logs are evidence for this session, not retained CI artifacts.

CALLED/COMPLETED setup uses direct test-only SQL, not F-04 endpoints. These checks
do not establish that Host Call Next/Complete exists. No history listing, rank,
ETA, public display or cancellation has been added.
