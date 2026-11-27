# Sprint 3 traceability and product completion plan

DQ-S3-1.0 proposed 27 November 2026 ICT for [#55](https://github.com/bbxndg/DineQueue/issues/55). Baseline `602cd50566dd3480f600af9d879cd4436c9d8c55`. Read [contracts](../features/sprint3-contracts.md), [interfaces/data](../features/sprint3-ui-api-data.md) and [STS](sprint3-sts.md). This matrix covers all remaining Sprint 3 scope; [the main matrix](../09-traceability.md) retains S1/S2 baseline mappings and their historical evidence.

## Current status and missing acceptance

Status vocabulary: PLANNED = contract/design only; IN PROGRESS = code work exists; IMPLEMENTED = code exists without all required verification; TESTED = linked actual tests on identified SHA; ACCEPTED = required review and acceptance evidence recorded. Existing foundations do not make new Sprint 3 ACs achieved. At this checkpoint all new ACs below are PLANNED and all associated cases NOT RUN. No Sprint 3 tested commit or approval is recorded.

The owner in every row is the issue's two named assignees, Benjamin Garforth (bbxndg) and Kawinpop Churari (softkoi), jointly accountable. Individual implementation author/reviewer allocation remains a team decision before each feature starts. Targets are proposed dates in 2026 ICT, not completed work or an agreed allocation.

| Feature / requirement | New AC / STS | Existing foundation at baseline | Exact remaining action / dependency | Accountable owner / target |
|---|---|---|---|---|
| F-11 Customer history + F-03; FR-010, NFR-003/005/006/009 | 01–04 / H01–H12 | IMPLEMENTED locally (uncommitted, unreviewed): API, migration 0007, `/my-queue/history`; H01–H12 executed locally on baseline `a70055c` + working tree, see [history results](sprint3-history-results.md). Not TESTED on a committed SHA or ACCEPTED | #56: review contract, owned terminal query/cursor, history page/states, RED/GREEN, restart/regression and reviewed deployed smoke | Benjamin + Kawinpop / 29 Nov |
| F-10; FR-017/009, NFR-001/003 | 05 / Q01–Q03 | Queue-scoped grants, counters and shared floor | Repeatable second-queue setup; selector isolation, independent numbering and cross-queue concurrency evidence | Benjamin + Kawinpop / 30 Nov |
| F-17; FR-031, BR-024, NFR-003/006 | 06 / U01–U03 | Lab 7 readiness/polling and owned reads | Implement scoped MANUAL Ready rank, explicit null reasons and unavailable ETA; polling/accessibility regression | Benjamin + Kawinpop / 30 Nov |
| F-11 staff + F-18; FR-018/032, BR-014, NFR-003/006/007/009 | 07 / R01–R07 | Ticket/assignment/member/event persistence | Private history/report read paths, date/zone cohorts, duration/count reconciliation; configuration coverage dependency for utilization | Benjamin + Kawinpop / 28 Nov formulas; 30 Nov first slice; 3 Dec completion |
| F-12; FR-019, NFR-010/013/015 | 08 / P01–P03 | CALLED state and scoped operations | Explicit disabled-by-default display config, protected settings, minimal public DTO/page, privacy/change propagation tests | Benjamin + Kawinpop / 4 Dec |
| F-19; FR-033, BR-015/016/025, NFR-003/005/015 | 09 / F01–F08 | Existing inventory and CLI activation; no web-setup acceptance established | Review Host configuration-grant revision; migrations/audit, atomic safe editor and web activation; history/concurrency regression | Benjamin + Kawinpop / 28 Nov rules; 30 Nov first slice; 3 Dec completion |
| F-13; NFR-001–009 + relevant privacy/CSRF | 10 / G01–G04 | Existing automated suites and release pipeline | Run exact candidate regression, assess defects, local p95/accessibility, reviewed release + public verification | Benjamin + Kawinpop / 29 Nov progress gate; 6 Dec candidate; 12 Dec freeze; 13 Dec delivery |

Module responsibility: history/status and counters extend `apps/api/internal/queue`; floor setup extends the existing floor/seating repository/service and explicit DB tools; a reporting service can read ticket/assignment/event data without lifecycle writes. New schema stays under `apps/api/db/migrations`. UI belongs to the corresponding `apps/web/src/app` pages, shared owned/Host DTOs and existing refresh coordinator. API errors/authorization remain in existing middleware. These are planned module boundaries, not assertions that new files exist.

## Dated work and dependencies

| Target | Work / reviewable output | Entry or exit condition |
|---|---|---|
| 27 Nov | Finish #55 contract, UI/API/data, STS, traceability; record review PR | Both issue assignees inspect proposed scope/policies; no code starts on an unreviewed contract |
| 28 Nov | Contract review; start #56 after merge to staging; freeze floor grant and metric definitions | Actual reviewer/SHA/date recorded; assign author and other teammate reviewer; retain change history |
| 29 Nov | #56 implementation/test/deploy evidence; reconcile Sprint 2 acceptance and Deliverable 2 remaining-work table | At least one meaningful tested Sprint 3 increment, actual achieved AC and tested SHA; no empty placeholder accepted as progress |
| 30 Nov | Additional counter/status completion; working floor editor and report first slices | Main product workflows already exist; verify scoped write and report calculation before exam-period bounded completion |
| 1–3 Dec | Complete bounded floor/report cases and review migrations/privacy/concurrency | No new major workflow discovery; gaps identified from prior slices, not first design here |
| 4 Dec | Finish minimal public display and privacy review | Disabled default and response allowlist verified; no discretionary feature expansion |
| 5–6 Dec | Integrated candidate, UAT/defect retests, latency and accessibility | Blockers have reproduction, owner and retest; accepted scopes exercised together |
| 7–11 Dec | Examination buffer: small fixes/retests and evidence/video preparation | Avoid scheduling new feature design; escalate a missed earlier target immediately rather than silently moving work to 12–13 Dec |
| 12 Dec | Final regression, reviewed main release and deployment/evidence freeze | Exact SHA/revision/CI and retained AC evidence agree; remaining failures disclosed |
| 13 Dec | Final self-contained report, repository/release links and <=10-minute video with both students | Final acceptance recorded separately from test success; submit by course deadline |

Dependency path: reviewed #55 → #56 history → integrated progress evidence. Floor permission/snapshot design → safe setup → complete utilization coverage. Existing S2 persisted events → report counts/durations. Configuration permission → public settings; public read is independent of Customer sessions. Existing S1/Lab 7/S2 behavior is regression-tested whenever shared lifecycle/schema code changes.

The period 1–11 December is the examination period. This is a proposed completion schedule with early working slices and a buffer, not permission to delay the main product until the last two days. If the team cannot staff the early targets, revise the plan and explicitly review scope before promising completion. No feature is silently removed by a missed date.

## Change, defect and review records

No new runtime defect is asserted by this documentation review. When one is found, add its Issue, affected AC/test, reproduction, impact, named implementation owner, target date, fixed SHA, original failure and actual retest result. Open failures cannot be described as accepted.

The known product gap is web floor setup (F-19): current operator provisioning/activation does not satisfy AC-09. Other rows identify missing Sprint 3 acceptance, not newly discovered bugs. Contract changes must state old/new rule, reason, impacted tests and reviewer; do not rewrite a prior reviewed commit or backfill supposed test-first evidence.

Before implementation, record in the feature PR: Part of #55 (and #56 for Customer history), reviewed contract SHA/URL, author/reviewer and relevant AC/STS. On completion update the row's achieved AC, tested commit, actual commands/CI URL and remaining work. Keep implementation, test, review and product acceptance as separate records.

Legacy S3 planning references are mapped in [contract section 10](../features/sprint3-contracts.md#10-review-and-handoff-record). No old feature/FR/BR IDs are removed. The customer-history extension is explicitly documented without relabelling FR-018 as an existing customer-list requirement. Review this addition with the SRS before implementation.
