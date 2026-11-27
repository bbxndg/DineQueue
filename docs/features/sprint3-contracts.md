# Sprint 3 engineering contracts

Version DQ-S3-1.0, proposed for teammate review on 27 November 2026 (ICT). Issue [#55](https://github.com/bbxndg/DineQueue/issues/55). The pre-implementation baseline is Sprint 2 release `602cd50566dd3480f600af9d879cd4436c9d8c55`. [#56](https://github.com/bbxndg/DineQueue/issues/56) is the first implementation increment: Customer Ticket History.

Read this document with the [UI/API/data contract](sprint3-ui-api-data.md), [STS](../testing/sprint3-sts.md), [traceability and dated work plan](../testing/sprint3-traceability.md), [SRS](../01-srs.md) and [restaurant invariants](../06-restaurant-operations.md). These specify proposed behavior; they do not establish implementation, approval, test results or acceptance. The course authority is `project_deliverable_2_v1.pdf`, sections 3.3–3.4. Section 3.3 requires contracts for all remaining features and at least one tested domain increment by 29 November, with substantial product completion before the examination period of 1–11 December.

## 1. Scope, ownership and review gate

Retain F-10 through F-13 and F-17 through F-19. Customer history is an explicitly identified customer-facing extension of F-11 together with F-03 ownership; it does not replace staff history. F-12 public display and F-19 floor setup remain in this proposed completion plan. Removing either requires a recorded scope revision and revised traceability, not an undocumented omission. Numerical ETA is withheld under BR-024; the retained F-17 deliverable provides current status and a meaningful manual-queue position.

Issue #55 assigns both Benjamin Garforth ([bbxndg](https://github.com/bbxndg)) and Kawinpop Churari ([softkoi](https://github.com/softkoi)). They jointly own this plan until the team records the individual implementation split. Before each implementation PR, record one author and the other teammate as reviewer. This document does not invent an agreed individual allocation.

Review must resolve the proposed rules in DEC-041–044, especially restaurant configuration grants and occupancy coverage, before affected code starts. Capture the reviewed contract commit, PR URL, reviewer, review date and decision. Approval is pending; do not fill these fields from an assumed review. Preserve the reviewed version in Git when subsequent contract changes are needed.

## 2. Preserved behavior and boundaries

S1 authentication, RBAC, own-ticket privacy, 1–16 joining, daily numbering and MANUAL call/complete remain. Lab 7 Ready/Not ready retains original joined order and authorized polling. S2 keeps selected-set oldest fitting Ready selection, connected 1–3 table groups, five-minute hold, atomic seat/no-show/checkout/cleaning, server expiry and whole-group claims. No appointment booking, POS, payment, customer search, history editing/deletion, CSV export or ML predictions are added.

A COMPLETED ticket is a completed waitlist interaction. It may be manually completed or associated with an ongoing seated meal. History and analytics must distinguish that from a CLOSED dining assignment. Status reads never infer a checkout from ticket COMPLETED.

## 3. F-11 Customer Ticket History — increment #56

Requirements: FR-010 ownership and the customer history extension of F-11; FR-018 remains the staff history requirement. NFR-003/005/006/009 apply.

| Acceptance criterion | Observable behavior |
|---|---|
| S3-AC-01 | Only authenticated CUSTOMER can list their own history. Anonymous/expired sessions receive 401; HOST receives 403. Owner comes from the session on every page. A foreign detail returns 404. No owner selector is accepted. |
| S3-AC-02 | History contains only COMPLETED, CANCELLED and NO_SHOW, including legacy manual rows. Show original code, party size, service date, joined instant, queue/restaurant and final status. Preserve timestamps and rows; no event backfill is necessary for this read. |
| S3-AC-03 | Descending joined_at then UUID pagination is stable at equal timestamps, bounded to 1–50 rows and owner-scoped on each request. Invalid/duplicate/unknown query inputs return 400; final nextCursor is null. A new row above the boundary is visible after starting a fresh list. |
| S3-AC-04 | Customer navigation opens history, Load more appends without duplicates, and each item opens owned read-only detail. Loading, empty, retry, expiry and timezone presentation work. Leaving history does not break My Ticket, readiness or active polling. |

UI responsibility: `/my-queue/history` and links to existing `/my-queue/{entryId}`. API responsibility: new owned list, existing owned detail reused. Service owns the terminal filter and validation; repository always combines owner + terminal status + keyset boundary. Existing schema supports the read; any performance index must be justified with a query plan and explicit migration. No lifecycle mutation or new session mechanism.

Definition of Done: H01–H12 in the STS, actual RED/GREEN for owner/terminal filtering and stable pagination, isolated real-DB checks, browser/error states, teammate review, integrated regression and deployed smoke of the released SHA. This contract alone satisfies none of those execution gates.

## 4. F-10 Additional authorized queue/counter

Requirements: FR-017, FR-009 and NFR-003/001. Reuse queue selection, queue grants, counters and the restaurant floor.

| Acceptance criterion | Observable behavior |
|---|---|
| S3-AC-05 | Two named queues have independent daily sequences and explicit Host grants. Selection never grants access. Deny ungranted queues; shared-floor claims across queues remain exclusive. Setup is repeatable without resetting counters, existing tickets or grants. |

Operator provisions the second queue and grants through a reviewed, explicit DB tool, never unrestricted SQL in the browser. No queue-creation web UI or new privilege escalation is included. Existing list/join/Host endpoints are reused. Queue-local ticket codes may repeat across queues; every authenticated view shows the queue name. Configuration remains compatible with MANUAL and SEATING. Definition of Done: Q01–Q03, repeatable setup instructions, two-queue UI demonstration and concurrency evidence.

## 5. F-17 Customer status and guarded estimate

Requirements: FR-031, BR-024, NFR-003/006. Reuse owned reads and Lab 7 refresh coordination.

| Acceptance criterion | Observable behavior |
|---|---|
| S3-AC-06 | Show server status/readiness and its refresh state. For MANUAL + Ready WAITING, position is one plus the number of earlier Ready WAITING entries in the same queue/service date using joined_at/id. Not ready, SEATING, CALLED and terminal entries have position null with a clear reason. ETA is explicitly unavailable; no invented minutes or guaranteed seat time. |

Position is a current eligible-order observation, not a promise: readiness changes can change it and an in-progress call can delay it. For SEATING, selected table capacity determines eligibility, so a single numeric rank would mislead. No other customer IDs or details are exposed. Add fields only to the owned DTO; persistence stays unchanged. Hidden-page pause, fresh-on-visible, stale recovery and 401 handling retain the Lab 7 behavior. Definition of Done: U01–U03, accessible states, ownership and polling regression.

## 6. F-11 staff history and F-18 operations analytics

Requirements: FR-018/032, BR-014, NFR-003/006/007/009. NFR-013 governs public privacy, not the accuracy of analytics.

| Acceptance criterion | Observable behavior |
|---|---|
| S3-AC-07 | An assigned Host can read queue-scoped operational history and a date/zone-filtered report. Counts and durations reconcile with persisted records, sample counts and exclusions are shown, and unsupported occupancy coverage returns null with a reason. Foreign scope is denied. No customer names/emails/notes appear. |

History lists ticket outcomes and assignment/turnover events with ticket code, action, server time and table codes. Do not require an event that legacy S1 rows never recorded. Reports distinguish ticket outcomes, seated groups and completed dining groups. Use the precise cohorts, formulas and examples in the interface contract. Existing tickets, assignments, members and events are read-only sources. Occupancy configuration coverage needs a forward-only snapshot/audit migration; do not fabricate historical capacity or opening hours.

Definition of Done: R01–R07, manually reconciled fixtures, timezone/range/empty checks, private pagination, report UI and independent review of definitions. Numerical ETA is not inferred from average wait.

## 7. F-12 Public minimal display

Requirements: FR-019, NFR-010/013. All roles may read an explicitly enabled display; enablement requires the configuration grant defined below.

| Acceptance criterion | Observable behavior |
|---|---|
| S3-AC-08 | Enabled displays show only current CALLED ticket codes/status. Disabled or unknown slugs return 404. Neither response, errors nor rendered source exposes customer/queue/ticket IDs, names, email, notes, timestamps or session data. A previously called code disappears after its terminal transition. |

Use a dedicated public DTO and page, not a filtered serialization of private Host data. A queue-specific reviewed slug identifies the display URL. Store explicit enablement separately from queue activity. Poll every five seconds while visible with one in-flight read; on an error show unavailable rather than retaining a misleading current call. A response is a snapshot, not push delivery. Definition of Done: P01–P03, response-key allowlist, disabled-state and change propagation checks plus public deployment smoke.

## 8. F-19 Floor Setup & Seating Activation

Requirements: FR-033, BR-015/016/025, NFR-003/005/006/015. Proposed revision: core roles remain CUSTOMER/HOST. A HOST with an explicit restaurant configuration grant may edit that restaurant's floor; an ordinary queue grant alone never permits configuration. This replaces the earlier optional Manager assumption for this feature, subject to teammate review of DEC-042.

| Acceptance criterion | Observable behavior |
|---|---|
| S3-AC-09 | Configuration-granted Host can create/edit zones, table capacity/positions and explicit approved edges through the web, persist an atomic versioned layout, and separately enable a granted queue for SEATING. Deny other actors and foreign resources. Reject stale, invalid, busy or historically destructive edits without partial writes. Activation reuses existing no-CALLED and valid-floor checks. |

The initial configuration grant is provisioned by an operator; no Host may self-grant or grant another Host. Queue grants still control queue operations. The setup UI supplies typed forms and an accessible list; dragging is not required. Saving positions never creates adjacency. Existing physical identities and historical membership remain immutable. Disable unused resources instead of deleting referenced rows. The interface contract defines safe edit boundaries, lock order and migrations.

Definition of Done: F01–F08, permission/CSRF/stale/concurrent tests, populated-data migration preservation, web creation of approved T4(6)+T5(4), rejection of T7(8) for ten people and a separate explicit activation demonstration. A cloud SQL command is not evidence that this web feature exists.

## 9. F-13 Quality, integration and release

Requirements: NFR-001–009 and NFR-010/013/015 at the relevant boundaries.

| Acceptance criterion | Observable behavior |
|---|---|
| S3-AC-10 | All retained ACs have actual command/test/SHA evidence, regression covers S1/Lab 7/S2 and new S3 work, read p95 is below one second under 20 documented local concurrent synthetic users, and keyboard/360px/1280px checks and public smoke pass before acceptance. |

Definition of Done: G01–G04, defect reproduction/fix/retest links, reviewed staging integration and main release, exact deployment revision/digests, persistence check and evidence report/video. No dependency or CI redesign is implied by this documentation task. Failures remain open with an owner and retest target.

## 10. Review and handoff record

| Record | Current value / required evidence |
|---|---|
| Contract baseline | 602cd50566dd3480f600af9d879cd4436c9d8c55 |
| Contract version | DQ-S3-1.0, proposed 27 November 2026 ICT |
| Contract PR / reviewed SHA | Pending creation/commit; record exact URL and SHA after authorization |
| Teammate review | Pending; record actual reviewer, decision and date |
| #56 start condition | Reviewed history AC/API/STS available on sprint3-staging |
| Further feature start condition | Relevant proposed policy approved, named author/reviewer recorded and feature branch created from updated staging |
| Actual execution/acceptance | Pending implementation and recorded evidence; no PASS claimed here |

Legacy planning IDs S3-AC-001/T-001–002 map to AC-06/U01–U03; S3-AC-002/T-003–004 to AC-07/R01–R07; S3-AC-003/T-005–006 to AC-09/F01–F08; S3-AC-004/T-007–008 to AC-08 and AC-10/P01–P03/G01–G04. Preserve historical references; the two-digit AC IDs in this version denote the expanded contract, not previous execution.
