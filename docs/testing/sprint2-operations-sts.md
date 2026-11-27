# Sprint 2 software test specification

Version DQ-S2-STS-2.1, 27 November 2026. Related [contract](../features/sprint2-restaurant-operations-contracts.md), [interfaces](../features/sprint2-ui-api-data.md) and [traceability](sprint2-traceability.md). All cases below are PLANNED / NOT RUN. No actual test file, result or acceptance is implied by a reserved ID.

## 1 Test data and isolation

Use a dedicated disposable PostgreSQL database, never Cloud SQL production or a shared developer database. Recreate fixtures before cases; do not run destructive fixture cleanup against a URL merely because it is reachable. Run the existing migration CLI with the next new migrations; check the source baseline and applied migration list in the evidence.

R1 has queue Q1, zones Main/Terrace/Private and T1(2),T2(4),T3(8),T4(6),T5(4),T6(2),T7(8). Approved edges are T1–T2, T2–T3, T4–T5, T5–T6. Q2 is a fixture queue in R1 for shared-table race tests, not a claim that additional-counter UI has been delivered. R2/Q3 provides cross-restaurant negatives. Hosts H1/H3 have explicit grants as each test states; H2 lacks the target grant. Customers C1/C2/C3 have separate sessions and original joined_at/id order.

Normal matching fixtures are Ready unless the case explicitly tests Not ready; actual new joins must remain Not ready. Reset resource states, active assignments and counters deliberately. A separate connected 8+8 fixture exercises party 16; do not pretend the seven-table sample has a same-zone capacity of 16. Use fixed UUIDs and a controlled clock at service/test boundaries; the production clock remains server-controlled.

Concurrency uses separate real DB connections and a barrier or observed lock wait, not sleeps alone. Record the winning/losing results and inspect persisted entries, assignments, every membership/table, event and operation-request row. Failure-injection hooks exist only in tests. Verify zero partial writes, not only an HTTP status.

## 2 Retained cases 001 to 019

These IDs were reserved in v2.0. Their preconditions are clarified for readiness, selected-set matching and cross-queue serialization. T-019 now specifies a valid alternate table set without an arbitrary party-order override; preserve that design revision in Git history.

| ID | Level | Preconditions and action | Expected outcome | AC |
|---|---|---|---|---|
| S2-T-001 | API/DB | Granted H1 reads Q1 floor with seeded zones/tables | Correct capacities, edges, states, versions and scoped assignments | S2-AC-001 |
| S2-T-002 | API/DB | Anonymous, Customer and ungranted H2 request floor and a Host mutation | 401/403 as applicable; no private operating data or writes | S2-AC-001 |
| S2-T-003 | Unit | Party 10, active same-zone T4(6)–T5(4), both AVAILABLE | Connected candidate accepted with capacity 10 | S2-AC-002 |
| S2-T-004 | Unit/API | Select disconnected same-zone pair, T4+T7 across zones, or R1+R2 tables | Reject connection/scope according to contract; unchanged DB | S2-AC-002 |
| S2-T-005 | Unit | Select T4–T5–T6 chain; repeat with missing middle edge/inactive zone | Chain accepted only when connected and active | S2-AC-002 |
| S2-T-006 | Unit | Recommend for Ready party 10 with T7(8) and connected 6+4 | Eight alone excluded; valid 6+4 remains | S2-AC-003 |
| S2-T-007 | Unit | Vary eligible set sizes, spare seats and UUID input order | Exact deterministic ranking by table count, spare capacity, sorted IDs | S2-AC-003 |
| S2-T-008 | API/DB | Change T5 to HELD/OCCUPIED/DIRTY/CLEANING/OUT_OF_SERVICE | Exclude T4+T5; preview creates no claim | S2-AC-003 |
| S2-T-009 | API/DB | Q1 SEATING; oldest fitting Ready C1; current preview T4+T5; confirm call | One CALLED, one HELD group, both tables HELD, complete claims/event/key result | S2-AC-004 |
| S2-T-010 | API/DB | Include valid-format unknown table ID with T4 in a call request | Generic missing-resource rejection; no call, hold or partial member | S2-AC-004 |
| S2-T-011 | API/DB | Valid HELD T4+T5 before expiry; Seat with current version/key | COMPLETED ticket, ACTIVE group, both OCCUPIED and one seat event | S2-AC-005 |
| S2-T-012 | API/DB | Seat at deadline and after cleanup released that expired hold | HOLD_EXPIRED; no OCCUPIED/ACTIVE result; cleanup may persist NO_SHOW/release as its own event | S2-AC-005,017 |
| S2-T-013 | DB concurrency | Two granted Hosts in Q1 call concurrently with valid overlapping alternatives | Exactly one CALLED/HELD; other documented conflict; no duplicate claims | S2-AC-006 |
| S2-T-014 | DB concurrency | 20 synchronized attempts across Q1/Q2 in R1 for sets sharing T5 | At most one outstanding T5 claim across queues; losing business writes rolled back | S2-AC-006 |
| S2-T-015 | DB rollback | Inject failure after first table update or event insert in call+hold | Ticket/tables/assignment/claims/event/key result all roll back | S2-AC-004,006 |
| S2-T-016 | API/DB | ACTIVE T4+T5; checkout with current version | CLOSED/DIRTY; both DIRTY; claims retained until cleaning ends | S2-AC-007 |
| S2-T-017 | API/DB | Try dirty→available or held/occupied→cleaning shortcuts | INVALID_TRANSITION; no partial release; only valid group sequence succeeds | S2-AC-007 |
| S2-T-018 | Unit/API | Earlier Ready party 6, later Ready party 4; Host chooses T2(4), even with a different free 6-seat set elsewhere | Later party 4 selected for T2; earlier party retained with CAPACITY_NOT_ELIGIBLE; this is the user-selected policy | S2-AC-008 |
| S2-T-019 | API/DB | Host chooses valid non-top-ranked set, then tries invalid set or later fitting party | ALTERNATIVE_SET audited; invalid set/order rejected; reason never bypasses guards | S2-AC-008 |

## 3 Retained cases 020 to 033

| ID | Level | Preconditions and action | Expected outcome | AC |
|---|---|---|---|---|
| S2-T-020 | API/DB | C1 cancels own WAITING with either readiness value | CANCELLED, inactive, one owner event; same code/join time/history | S2-AC-009 |
| S2-T-021 | API/DB | C2 cancels C1; C1 cancels CALLED/COMPLETED/CANCELLED/NO_SHOW | Foreign 404; owned wrong-state 409; no mutation | S2-AC-009 |
| S2-T-022 | API/DB | Granted Host marks held CALLED C1 NO_SHOW before expiry | Whole group RELEASED/AVAILABLE, claims released, NO_SHOW and actor/reason recorded | S2-AC-010 |
| S2-T-023 | API/DB | Repeat no-show with new key; ungranted Host attempts it | Conflict/403; no extra event, release or orphan | S2-AC-010 |
| S2-T-024 | API/DB | Ten-person ACTIVE assignment checks out then cleans | Both DIRTY, both CLEANING, both AVAILABLE in separate group transactions | S2-AC-011 |
| S2-T-025 | Browser/API | Separate Customer/Host contexts: join 10, Ready, preview T4+T5, call/hold, seat, checkout, clean; reload at each boundary | UI and persisted group states agree; no manual table-status shortcut | S2-AC-011 |
| S2-T-026 | API/DB | New-key duplicate checkout on CLOSED assignment | INVALID_TRANSITION, no duplicate turnover event | S2-AC-012 |
| S2-T-027 | API/DB | Another party attempts DIRTY/CLEANING T4 before finish-cleaning | Reject; claim/state prevent early allocation | S2-AC-012 |
| S2-T-028 | API/DB | Customer calls every new Host POST directly | 403 independently of UI; target rows and action records unchanged | S2-AC-019 |
| S2-T-029 | API/DB | Missing/bad Origin or CSRF on each new cookie-authenticated POST | 403 before business/expiry work; no writes | S2-AC-019 |
| S2-T-030 | Unit/API | Empty/duplicate/four table IDs; forged capacity; duplicate JSON/version entries; malformed IDs/null/fractions | Documented 400/404; no SQL parsing error or partial writes | S2-AC-002 |
| S2-T-031 | DB concurrency | Seat races explicit server cleanup; also no-show races Seat | Exactly one valid outcome: ACTIVE/OCCUPIED/COMPLETED or RELEASED/AVAILABLE/NO_SHOW, never mixed | S2-AC-005,010,017 |
| S2-T-032 | API/DB restart | Commit HELD, ACTIVE and CLOSED/CLEANING groups in separate cases; restart real API on same DB | Exact ticket/member/table/turnover states persist; no reset/provisioning at startup | S2-AC-011 |
| S2-T-033 | API/DB | H2 uses another queue/restaurant's assignment ID, including same-restaurant Q2 | 403 for ungranted path scope, generic 404 for foreign assignment in granted path; no writes | S2-AC-001,019 |

## 4 Added cases 034 to 060

| ID | Level | Preconditions and action | Expected outcome | AC |
|---|---|---|---|---|
| S2-T-034 | API/DB | H1 grants Q1 only; floor has Q2 occupied tables | See shared physical state without Q2 ticket/customer/assignment identifiers | S2-AC-001 |
| S2-T-035 | Unit/API | Parties 1 and 16; connected 8+8 fixture; invalid 0/17 at join | Valid boundaries work; sample-floor no-fit stays empty; invalid join unchanged | S2-AC-003 |
| S2-T-036 | Unit/API | A older Not ready, B Ready; select set fitting both; A returns Ready before next call | B first while A unready; A retains original priority next time; no CALLED preemption | S2-AC-008,014 |
| S2-T-037 | API/DB | Readiness changes after preview but before call lock; test both commit orders | If Not ready wins, never call it; if call wins, readiness returns 409; no contradictory states | S2-AC-008 |
| S2-T-038 | Unit/API | A earlier Ready fits selected set; client requests later B using a valid preview for another set | QUEUE_ORDER_CHANGED, no skip/hold; new confirmation required | S2-AC-008,014 |
| S2-T-039 | API/DB/browser | Cancel/no-show then read owned detail/active list and create new ticket | Terminal row owner-readable but absent active list; new unique number/Not ready; old row unchanged | S2-AC-009,018 |
| S2-T-040 | DB concurrency | Owner cancellation races call+hold; snapshot rows/events before requests | Exactly one wins; no CALLED+CANCELLED contradiction, partial table claim or skipped counter | S2-AC-009,018 |
| S2-T-041 | API/DB | No-show against ACTIVE seating or another assignment's members | INVALID_TRANSITION/NOT_FOUND; occupied tables remain occupied | S2-AC-010 |
| S2-T-042 | Migration/DB | Populate 0001–0003 with WAITING/CALLED/COMPLETED and readiness; apply new migrations twice | Same IDs, times, numbers, passwords and readiness; MANUAL default; new lifecycle/indexes valid | S2-AC-015 |
| S2-T-043 | API/DB | Same queue first MANUAL then explicitly SEATING; invoke legacy bodyless call/complete | MANUAL retains Lab 7; SEATING guards TABLE_SELECTION_REQUIRED/SEATING_ASSIGNMENT_REQUIRED | S2-AC-015 |
| S2-T-044 | CLI/DB | Activation with legacy CALLED, invalid floor or wrong database confirmation; then valid configuration | Refuse without changing rows; valid activation preserves WAITING/history, including inactive-queue draining | S2-AC-015 |
| S2-T-045 | Unit/DB | Hold at deadline−1µs, exactly deadline, deadline+1µs; include transaction waiting on a lock | Only strictly before deadline can seat; decision clock sampled after acquiring locks | S2-AC-005,017 |
| S2-T-046 | DB concurrency/rollback | Q1/Q2 shared floor: hold versus finish-cleaning, checkout versus new hold; inject partial failures | Deterministic lock order, one valid serial outcome, no stale/partial claims | S2-AC-006,019 |
| S2-T-047 | DB rollback | Fail second member/event update during Seat, No-show, Checkout, Start/Finish Cleaning | Full rollback for each operation, including event/key record; never mixed member states | S2-AC-007,019 |
| S2-T-048 | API/DB | Clean group A, allocate same tables to B, then call A's old cleaning endpoint with a fresh key | Old turnover is CLEANED; reject without changing B or its claim | S2-AC-012 |
| S2-T-049 | Browser/manual | Keyboard/list selection, labels/focus/dialog cancellation; 360px and 1280px | Same accessible actions, readable statuses, no page-level overflow or color-only meaning | S2-AC-013 |
| S2-T-050 | Browser | Floor/preview slow/failing reads, queue changes and 409 after other Host wins | Keep stale notice; no false empty/success; clear stale confirmation; recover via one coordinator | S2-AC-013 |
| S2-T-051 | API/DB | Preview valid set with empty/all-unready/all-too-large parties; inspect all tables/events/claims before and after GET | Correct null/empty reason, no lease/expiry/event/version writes | S2-AC-014 |
| S2-T-052 | CLI/DB | Provision floor twice with existing busy tables and existing credentials; conflicting graph on rerun | Stable IDs/states/passwords; no counter reset; conflicting config rejected; old migrations unchanged | S2-AC-015 |
| S2-T-053 | API/DB concurrency | Same actor/operation/key and canonical payload sent twice concurrently and after later transitions | Original 200 replay, one transition/event, authorization still required | S2-AC-016 |
| S2-T-054 | API/DB | Reuse successful key with changed tables, entry, path assignment ID or expected version; use fresh key for duplicate terminal action | IDEMPOTENCY_CONFLICT for changed path/payload; new-key repeat conflicts; no duplicate event | S2-AC-016 |
| S2-T-055 | API/DB | Expire after deadline twice; then issue failing seating mutation after independent cleanup | One NO_SHOW/HOLD_EXPIRED release event; second cleanup empty; later failure cannot resurrect hold | S2-AC-017 |
| S2-T-056 | Browser | Visible Customer/Host views observe Ready, call, seat, cancel/no-show without manual refresh | Committed states propagate through existing polling; terminal states leave active list and remain owned-readable | S2-AC-018 |
| S2-T-057 | Browser/API | Hide/show/unmount views; overlapping write/read; 401/403/foreign detail404; inactive queue | No stale overwrite/timer burst; protected data cleared; true initial404; existing WAITING can drain/cancel | S2-AC-018 |
| S2-T-058 | Integration/browser | Run existing S1/Lab 7 suites on MANUAL fixtures, and versioned seating variants on SEATING fixtures | Preserve auth, ownership, join1–16, numbering, readiness, queue grant/concurrency; no deleted assertions | S2-AC-019 |
| S2-T-059 | Build/API contract | Clean-clone migration/provision/run, frontend/backend production build; inspect exact Next route allowlist | New scoped endpoints reachable same-origin; no secret in client; existing health/readiness work | S2-AC-019 |
| S2-T-060 | Deployed acceptance | Deploy identified integrated candidate, retrieve saved ticket/group after restart or redeploy; test Host flow and Customer denial | Version matches CI/tag; real persistence and backend403; disclose any skipped/failing check | S2-AC-019 |

## 5 Planned test locations and execution record

These are target locations, not existing test files or results. Feature authors must replace them with actual names/paths and record the run before claiming coverage.

| Area | Planned location | Important IDs |
|---|---|---|
| Floor and provision | `apps/api/internal/floor/*_test.go`, integration-tagged DB cases | 001,002,033,034,042,052 |
| Pure matching and preview | `apps/api/internal/matching/*_test.go` and integration cases | 003–008,018,019,035–038,051 |
| Allocation/turnover | `apps/api/internal/seating/*_test.go`, integration-tagged races/rollback | 009–017,022–027,031,032,041,043–048,053–055 |
| Customer cancellation | `apps/api/internal/queue/*_test.go` | 020,021,039,040,057,058 |
| Browser | `apps/web/e2e/floor.spec.ts`, `seating.spec.ts`, `cancellation.spec.ts`, `turnover.spec.ts` or reviewed equivalents | 025,049,050,056–060 |

From repository root after implementation: `npm test`, `npm run lint`, `npm run typecheck`, `npm run build`, `npm run test:e2e --workspace @dinequeue/web`. From `apps/api`: `go test -race ./...`, `go vet ./...`, `go test -tags=integration -race -count=1 -v ./...` with a dedicated `TEST_DATABASE_URL`. Do not print credential values or treat ordinary `go test` as integration coverage.

Preserve actual RED/GREEN for connectedness/selection and cross-queue claim exclusion, plus changed lifecycle and ownership rules. A missing package or migration setup failure is not a meaningful RED assertion. Keep the same intended assertions; isolate fixture changes when versioning manual-versus-seating expectations. Historical S1/Lab 7 counts are not new S2 results.

Every actual execution record must contain test IDs, actual file/name, contract version, preconditions, expected/actual result, command, source SHA, date/time/timezone, pass/fail/skip counts, log/CI URL and related Issue/PR/reviewer. At this draft all 60 test IDs remain NOT RUN and all 19 ACs remain unverified. Unit/API/DB/browser coverage may share an ID through named subcases; do not count planned rows or repeated runs as passed tests.
