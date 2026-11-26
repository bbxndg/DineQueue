# Lab 7 STS — readiness and automatic status updates

Version DQ-L7-CR44-v0.1; 25 November 2026. Issue #44.
Status: **Readiness coverage locally executed; polling coverage NOT RUN**.
See [actual test names/results and limits](readiness-results.md); the matrix below
is planned coverage and does not mark every row complete. Contract: [shared source](readiness-contract.md).

Use isolated real PostgreSQL for transaction/migration tests and synthetic accounts
for API/browser tests. Never run destructive fixtures on Cloud SQL/demo data.
Record exact test names/files when implemented; IDs below are planned coverage.

| AC / Test ID | Preconditions/action | Expected result |
|---|---|---|
| L7-AC-01 / L7-T-01 | Join with valid party and authenticated Customer | WAITING, isReady=false, original fields/number preserved |
| L7-AC-01 / L7-T-02 | Owner sets true, false, same false again | persisted values; same-value write is 200/no extra version change |
| L7-AC-02 / L7-T-03 | Guest, Host, foreign Customer, missing/malformed ID attempt PATCH | 401/403/indistinguishable 404; no writes or private leak |
| L7-AC-02 / L7-T-04 | Missing/wrong/unknown/null JSON, CSRF/Origin rejection | 400 validation or existing CSRF rejection; no writes |
| L7-AC-03 / L7-T-05 | Owned CALLED/COMPLETED PATCH | 409 INVALID_TRANSITION, unchanged |
| L7-AC-04 / L7-T-06 | A/B/C joined in order; A false, B/C true; call | B called, A/C waiting; all not-ready parties remain visible |
| L7-AC-04 / L7-T-07 | Complete B; set A true; Call Next | A before C; unchanged code/joined_at; tie-break id deterministic |
| L7-AC-05 / L7-T-08 | Called exists; empty queue; waiting-only all false | CALL_IN_PROGRESS / QUEUE_EMPTY / NO_READY_PARTIES respectively; no writes |
| L7-AC-06 / L7-T-09 | Two DB connections; hold/commit readiness false before call acquires queue lock | call skips target; deterministic synchronization, not sleep-only race |
| L7-AC-06 / L7-T-10 | Call commits before pending readiness update obtains lock | CALLED persists, readiness update 409 |
| L7-AC-06 / L7-T-11 | Two authorized Hosts call simultaneously with ready fixtures | exactly one success, one CALLED, preserved index/grants |
| L7-AC-07 / L7-T-12 | Upgrade populated old schema, rerun migrate, restart API | existing rows true; new joins false; data/counters unchanged; persistence verified |
| L7-AC-08 / L7-T-13 | Controlled clock/fetch; visible Customer and Host | refresh on next scheduled cycle, committed status/readiness displayed |
| L7-AC-08 / L7-T-14 | Slow fetch > interval; manual Refresh; mutation concurrent with refresh | no overlap; stale responses ignored; post-write refetch wins |
| L7-AC-09 / L7-T-15 | Hide/show tab, change selected queue, unmount | pause/one immediate fetch/abort or invalidate/no leaked timer |
| L7-AC-10 / L7-T-16 | Network error then recovery; 401/403/detail 404 | stale notice then recovery; protected state cleared on loss of access; no fake empty state |
| L7-AC-11 / L7-T-17 | Complete confirmation open while polling | same-ID confirmation retained, changed-ID confirmation reset, focus/forms preserved |
| L7-AC-12 / L7-T-18 | Initial foreign/missing/malformed ticket page request | actual HTTP 404 preserved (#41), not streamed 200 |
| L7-AC-13 / L7-T-19 | Two real browser contexts: Customer and Host; no manual refresh | toggle readiness visible to Host; call visible to Customer; completed leaves active list and owned detail remains readable |
| L7-AC-13 / L7-T-20 | Inactive queue with old WAITING owned ticket | readiness may change, ready party may be called, no new joins allowed |

## RED / GREEN sequence

1. Review and commit contract + STS first; record reviewer/date/review link.
2. Readiness RED: existing A is not ready, B ready; old Call Next incorrectly
   calls A. Against old schema, use an API-level scenario that demonstrates old
   calling behavior, not a missing-column/compile error. Preserve exact setup,
   assertion/output and tested SHA. Independently demonstrate unsupported readiness
   behavior if useful; do not claim missing-route evidence proves concurrency.
3. Polling RED: actual page remains WAITING after another context successfully
   calls a ready fixture without manual refresh. Before readiness integration,
   a legacy called-ticket update can establish old manual-refresh behavior.
4. Implement, rerun the same meaningful tests for GREEN, then targeted additional
   cases and regression. These are instructions, not executed results.

## Regression and evidence

Select existing queue/ownership/auth/Host concurrency suites plus #41 HTTP status
coverage. Keep old preserved-rule assertions. Only update strict-FIFO expectations
and new-ticket fixture setup where the reviewed requirement changed; explain why.

Verification commands (historical outcomes in readiness-results.md; rerun for each changed release): root `npm test`, `npm run lint`,
`npm run typecheck`, `npm run build`; from apps/api `go test -race ./...`,
`go vet ./...`, and `go test -tags=integration -race -count=1 -v ./...` with isolated
TEST_DATABASE_URL. Run existing Playwright harness with isolated API/DB (see its
config and current scripts); record actual command and selected suite. Use browser
polling tests for lifecycle/render behavior, not only a test mirroring a timer.

Evidence record per run: ID, command, tested SHA, date, setup, expected/actual,
pass/fail/skip count, log path, CI URL. Keep source changes and evidence separate.
Post-deployment manual checks must verify readiness-aware calling and automatic
updates plus preserved auth/ownership; local checks cannot establish cloud PASS.
