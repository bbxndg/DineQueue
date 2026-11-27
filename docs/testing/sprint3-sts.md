# Sprint 3 software test specification

DQ-S3-1.0 proposed for [#55](https://github.com/bbxndg/DineQueue/issues/55), 27 November 2026 ICT. All cases below are **NOT RUN**. Read the [feature contracts](../features/sprint3-contracts.md) and [UI/API/data definitions](../features/sprint3-ui-api-data.md) before implementation. Existing run records remain historical evidence for their original SHAs, not results for these cases. Exception: H01–H12 (#56) were executed locally on uncommitted changes over `a70055c`; see [history results](sprint3-history-results.md). They are not results for a committed or deployed SHA.

## Setup, levels and evidence

Use CUSTOMER A/B, granted Host H, queue-ungranted Host U, configuration-granted Host C and configuration-ungranted queue Host H. Two restaurants, two same-restaurant queues Q1/Q2 with independent grants, and one foreign queue/zone provide scope boundaries. Passwords come from private test configuration. API/DB tests run against an isolated disposable PostgreSQL `_test` database with synthetic rows and explicit migrations; restore/drop only their own fixtures. Browser tests use a separately built app and that test database, one worker where fixtures are shared. No public Cloud SQL reset or destructive production test.

The floor fixture includes active-zone T4(6)+T5(4) approved adjacent, T7(8) isolated, a foreign-zone table and an out-of-service table. Include populated legacy MANUAL tickets and S2 assignments, simultaneous server timestamps, terminal/nonterminal tickets and time boundaries in Asia/Bangkok. Add a DST-observing restaurant fixture for local-day interval conversion. Preserve rows before/after each rejected mutation to detect partial writes.

Unit checks cover pure validation/formulas; integration checks prove permissions, queries, row locks and transaction rollback on real PostgreSQL. Browser checks prove visible behavior rather than repeating the implementation. Record fail-first RED and then GREEN for important new rules before marking acceptance; an intentionally failing unrelated assertion is not RED evidence. No new test runner or dependency is required by this contract.

## Customer Ticket History (#56)

| ID | AC | Level / precondition and input | Expected result |
|---|---|---|---|
| H01 | 01 | API/DB: A/B each own terminal records; request as A, no session, expired session and Host | A sees only A; anonymous/expired401 and Host403; no DB writes |
| H02 | 01 | API/DB: A follows B detail; sends ownerId/userId query; uses B-derived cursor | Foreign detail404; owner query400; cursor never changes authenticated owner predicate |
| H03 | 02 | API/DB: legacy MANUAL COMPLETED, seated COMPLETED with ACTIVE meal, CANCELLED, NO_SHOW, WAITING and CALLED | Exactly the three terminal ticket states; seated meal remains ACTIVE and is not labelled checked out |
| H04 | 02 | API/DB: compare rows/timestamps before history read, including old rows with no event | Exact code/party/date/joinedAt retained; no synthetic event or write |
| H05 | 03 | API/DB: equal joined_at rows plus microsecond differences; limit1 then traverse all pages, also limits20/50 | Descending time/UUID without duplicates or gaps; cursor retains microseconds; bounded responses and null end |
| H06 | 03 | Unit/API: limit0/51/fraction/nonnumber, duplicated limit, unknown key; malformed/oversize/unsupported cursor, missing fields/invalidUUID | All400; no SQL/parser details or private row existence leaked |
| H07 | 03 | API/DB: insert newly terminal row above page boundary between requests | Existing traversal remains exclusive; fresh first page shows new row; no snapshot guarantee implied |
| H08 | 04 | Browser: navigation, Load more, own legacy detail and back | Correct append/order/detail; original active My Ticket remains reachable |
| H09 | 04 | Browser: no terminal rows; initial failure; later-page failure followed by Retry | Empty distinct from failure; loaded rows retained after later error; same page retried once |
| H10 | 04 | Browser: browser timezone differs from restaurant; stored service date near midnight | Stored date unchanged; joined instant rendered with restaurant timezone |
| H11 | 01,04 | Browser/API: expire session or change logged-in user while history request is delayed | Clear protected data, ignore late previous-user response, route to login/reinitialize; private no-store headers |
| H12 | 02,04 | Integration/browser: restart app with same DB then reopen history/detail | Same historical rows; existing ownership, join, readiness and active polling regressions pass |

## Additional queue and Customer status

| ID | AC | Level / precondition and input | Expected result |
|---|---|---|---|
| Q01 | 05 | API/DB/browser: provision Q2 twice, join Q1/Q2 on same service date and select queues | Independent counters/queue names; repeat setup preserves tickets, sequences and existing grants |
| Q02 | 05 | API/DB: Q1-only Host requests Q2 list/detail/action; customer supplies queue outside accessible setup | Scope policy enforced server-side; no access gained by selector or guessed ID |
| Q03 | 05 | DB concurrency: Q1/Q2 Hosts simultaneously claim overlapping shared-floor sets | One valid winner, other conflict; no duplicate live table claim or partial group |
| U01 | 06 | API/DB: MANUAL Ready WAITING A, earlier Ready and Not ready rows, tied timestamps, then toggle readiness | Position counts only earlier Ready same-queue/service-day entries by original order; no foreign details |
| U02 | 06 | API/DB/browser: Not ready, SEATING, CALLED and terminal states | Null rank with NOT_READY/TABLE_DEPENDENT/NOT_WAITING; estimate unavailable with no numeric minutes |
| U03 | 06 | Browser: Host call while Customer visible, hidden then visible, network error and session expiry | Existing refresh coordination retained; fresh state on return, stale/error labelled, expiry clears protected data |

## Staff history and analytics

| ID | AC | Level / precondition and input | Expected result |
|---|---|---|---|
| R01 | 07 | API/DB: granted/ungranted queues, missing/foreign zone, malformed dates and 31/32-day ranges | Granted scope only; invalid400, queue denial403, foreign zone404; inclusive 31 accepted, 32 rejected |
| R02 | 07 | Unit/DB: waits 120/240 seconds plus uncalled cancellation, ten-person T4/T5 group | Wait mean 180/count 2/excluded 1; seated group count1; no duplication through member joins |
| R03 | 07 | DB: manual completion, ACTIVE/closed meals, NO_SHOW/cancelled, null/negative timestamp pairs, zero denominator | Ticket completion distinct from dining; exact terminal counts; invalid pairs excluded; empty means/rates null |
| R04 | 07 | Unit/DB: Bangkok midnight and DST-local date bounds, interval-spanning assignments | Correct half-open instants, clipped member occupancy; cohort labels distinguish service date from event time |
| R05 | 07 | DB: two tables occupied 600 s within fully covered 1200 s; pre-snapshot interval; two queues sharing floor | Occupied 1200 / table-denominator 2400 / utilization 50%; historical coverage or shared-queue denominator yields specified null reason |
| R06 | 07 | API/DB: legacy outcomes without events, multi-member events, tied event times and changed filters during paging | Labelled legacy record time, unique stable history items; keyset order; mismatched filter cursor400; no customer PII |
| R07 | 07 | Browser: filters, no rows, failure after successful load, grant revocation | Counts/units/sample exclusions and filters visible; stale-labelled last snapshot on error; protected state cleared on denial |

## Public display

| ID | AC | Level / precondition and input | Expected result |
|---|---|---|---|
| P01 | 08 | API/DB: unknown/disabled/enabled slug; malformed slug; duplicate settings slug | Unknown/disabled404, malformed400; enabled exact public DTO; duplicate slug409 |
| P02 | 08 | API/security: all public success/error response keys and page/source; Customer/ordinary Host attempts enablement | No IDs/names/emails/notes/timestamps/session data; writes require scoped config+queue grants and Origin/CSRF |
| P03 | 08 | Browser: call then complete/no-show, hidden/visible, read failure | Code/status updates then clears; visible polling single-flight; unavailable shown on failure rather than stale current call |

## Floor setup and activation

| ID | AC | Level / precondition and input | Expected result |
|---|---|---|---|
| F01 | 09 | API/DB: C versus Customer, anonymous, queue-only Host, foreign restaurant, revoked grant and invalid CSRF/Origin | Only configuration-granted Host writes; existing queue grant alone insufficient; no self-grant route |
| F02 | 09 | DB/browser: create zone/T4/T5 with request-local keys and approved edge; save then reload/restart | IDs assigned once; capacity/positions/edge persist; layout version increments once; ten-person set qualifies and lone T7 does not |
| F03 | 09 | Unit/API/DB: capacity 0/9/fraction, invalid positions, duplicate codes/edges, self/cross-zone edges, bad refs/limits/unknown JSON | Validation400 or documented domain conflict; complete rollback, no orphan row; isolated valid tables remain allowed |
| F04 | 09 | DB: HELD/OCCUPIED/DIRTY/CLEANING members; try capacity/code/zone/state/edge edit or zone disable; position-only edit | Busy destructive edit409; permitted position-only edit preserves membership and increments layout version; history immutable |
| F05 | 09 | DB concurrency: two layout writers same version, and overlapping call versus layout edit | One config writer wins, stale writer409; seating invariants preserved regardless of winner, no partial group/deadlock |
| F06 | 09 | API/DB: valid MANUAL activation, active CALLED, invalid floor, stale version, repeated key and different intent | Valid mode changes atomically; refusal leaves queue unchanged; authorized repeat no-op/replay, changed key intent409; no disable route |
| F07 | 09 | Migration/DB: populated S1/S2 plus configuration audit insert failure during edit; repeat migration | No altered ticket/member history or automatic grants; full rollback on audit failure; explicit migrations repeat safely |
| F08 | 09 | Browser: keyboard form/list, busy/stale errors, unsaved changes and activation confirmation | Explain scope/busy state; preserve edits for reconciliation, no silent overwrite or auto-enable; ordinary Host cannot access setup |

## Quality and release

| ID | AC | Level / precondition and input | Expected result |
|---|---|---|---|
| G01 | 10 | Automated regression: S1 login/ownership/numbering/MANUAL; Lab 7 readiness; S2 matching/expiry/seating/no-show/turnover; S3 cases | Actual suite outputs and counts tied to tested SHA; required CI green, no skipped rule described as verified |
| G02 | 10 | Local load: 20 concurrent authenticated synthetic readers over five minutes after 30 s warm-up, fixed recorded mix of own active/history and granted floor/report reads | Per-route and aggregate p95< 1 s; report request counts/error rate/hardware/DB size/limits; do not hide failing route in aggregate |
| G03 | 10 | Manual/browser:360px and1280px, keyboard-only labels/focus, empty/error/conflict and non-color status | Main journeys usable with readable labels, no clipped required action or color-only information; record observations |
| G04 | 10 | Reviewed release/deployment public smoke and same-DB app restart | Exact SHA/revision/digests/CI/run URLs; authorized synthetic workflows persist and denial works; no destructive load/reset on public DB |

## Execution record to complete during implementation

For each executed case record test ID/name, command or manual steps, ICT timestamp, tested full SHA, environment/DB fixture identifier, expected/actual result, exit code and log/CI link. RED/GREEN records must identify the pre-change and changed commits. Screenshots must retain their real URL/environment and no credential values. A deployment alone is not authenticated acceptance. Mark a case PASS only after its expected result is observed; record FAIL/BLOCKED/NOT RUN and defect/retest issue otherwise. Teammate approval and final acceptance are separate from a passing test run.
