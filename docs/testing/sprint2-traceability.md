# Sprint 2 requirements and verification traceability

Version DQ-S2-TRACE-2.1, 27 November 2026. [Contract](../features/sprint2-restaurant-operations-contracts.md), [interfaces](../features/sprint2-ui-api-data.md), [STS](sprint2-operations-sts.md). This is a pre-implementation mapping under [#49](https://github.com/bbxndg/DineQueue/issues/49), not evidence of executed tests.

## 1 Acceptance implementation and execution status

Every row is Sprint 2. Test IDs refer to planned cases in the STS, where levels, inputs and expected results are specified. Actual inventory implementation and local evidence are recorded in [floor results](sprint2-floor-results.md); later-feature verification remains pending. A target module is not an actual test-file citation.

| AC | Requirement and rule | Feature / Issue | Planned STS | Implementation / verification |
|---|---|---|---|---|
| S2-AC-001 | FR-002/012/024/035; BR-006/025; NFR-003 | F-06/F-16, #50 | S2-T-001,002,033,034 | Inventory implemented; local GET/scope tests recorded. Assignment/lifecycle portions pending #52 |
| S2-AC-002 | FR-025/026; BR-010/015/016/025 | F-14/F-15, #51/#52 | S2-T-003,004,005,030 | NOT IMPLEMENTED / NOT RUN |
| S2-AC-003 | FR-023/026/030; BR-008/015/017 | F-14/F-15, #51 | S2-T-006,007,008,035 | NOT IMPLEMENTED / NOT RUN |
| S2-AC-004 | FR-012/013/027; BR-010/018/019 | F-07/F-15, #52 | S2-T-009,010,015 | NOT IMPLEMENTED / NOT RUN |
| S2-AC-005 | FR-008/028; BR-013/019/020/021 | F-07/F-15, #52 | S2-T-011,012,031,045 | NOT IMPLEMENTED / NOT RUN |
| S2-AC-006 | FR-013/034; BR-005/018/019; NFR-002/011/012 | F-07/F-15, #52 | S2-T-013,014,015,046 | NOT IMPLEMENTED / NOT RUN |
| S2-AC-007 | FR-016/029; BR-022 | F-09/F-16, #54 | S2-T-016,017,047 | NOT IMPLEMENTED / NOT RUN |
| S2-AC-008 | FR-007/030; BR-003-L7-v1/011/017/023 | F-14, #51/#52 | S2-T-018,019,036,037,038 | NOT IMPLEMENTED / NOT RUN |
| S2-AC-009 | FR-010/014; BR-001/007/012 | F-08, #53 | S2-T-020,021,039,040 | NOT IMPLEMENTED / NOT RUN |
| S2-AC-010 | FR-015; BR-012/018/021 | F-08, #53 | S2-T-022,023,031,041 | NOT IMPLEMENTED / NOT RUN |
| S2-AC-011 | FR-016/026/029; BR-022; NFR-005 | F-09/F-16, #54 | S2-T-024,025,032 | NOT IMPLEMENTED / NOT RUN |
| S2-AC-012 | FR-029/034; BR-018/019/022 | F-09/F-16, #54 | S2-T-026,027,048 | NOT IMPLEMENTED / NOT RUN |
| S2-AC-013 | FR-024/035; NFR-006/014 | F-06/F-16, #50 | S2-T-049,050 | Floor Map/List, keyboard, responsive and read-recovery tests recorded; preview/write-conflict portions pending |
| S2-AC-014 | FR-007/030; BR-011/017/023 | F-14, #51 | S2-T-036,038,051 | NOT IMPLEMENTED / NOT RUN |
| S2-AC-015 | FR-008/009/011; BR-001/002/004/008/009/013; NFR-005 | F-07/F-15, #50/#52/#54 | S2-T-042,043,044,052 | 0004 migration preservation and guarded provisioning tested; activation/SEATING guards pending #52/#54 |
| S2-AC-016 | FR-013/034; BR-019/020; NFR-012 | F-07/F-15/F-09, #52/#54 | S2-T-053,054 | NOT IMPLEMENTED / NOT RUN |
| S2-AC-017 | FR-015/027/028; BR-012/021 | F-07/F-08/F-15, #52/#53 | S2-T-012,031,045,055 | NOT IMPLEMENTED / NOT RUN |
| S2-AC-018 | FR-005/010/014/015; BR-001/003-L7-v1/007/012; L7-AC-01–13 regression | F-03/F-08/F-04, #53 | S2-T-039,040,056,057 | NOT IMPLEMENTED / NOT RUN |
| S2-AC-019 | FR-001/002/009/011/020/034; BR-001–009; NFR-001–006/008/009/012/015 | Cross-feature delivery/security, #50–#54 | S2-T-028,029,033,046,047,058,059,060 | NOT IMPLEMENTED / NOT RUN |

Requirement references identify regression responsibilities, not a claim that all S1 functionality is newly unimplemented. The NOT IMPLEMENTED status above describes the specified S2 increments/guards. Historical S1/Lab 7 evidence remains versioned separately; integrated regression must run again on the actual candidate.

## 2 Data API and module responsibilities

| Responsibility | Planned modules / interface | Layer requiring no new behavior |
|---|---|---|
| F-06/F-16 inventory and scoped reads | Actual #50 adapter: `internal/queue/floor*.go`, floor/zone/table/edge migrations, `/host/queues/{id}/floor`, Host floor components | Login/session store, party issuance, deployment topology |
| F-14 deterministic preview | `internal/matching`, read snapshot adapter and matching drawer | No new migration or lease write; no client-supplied capacity/role authority |
| F-07/F-15 transactional seating | `internal/seating` service/repositories; guarded queue adapter; assignments/members/events/request results; call/seat/expire routes | Existing authentication mechanism and MANUAL queue behavior |
| F-08 owner cancellation and Host no-show | Queue owner-scoped service and UI; common seating release coordinator | No duplicate expiry/assignment implementation; no credential change |
| F-09/F-16 turnover | Shared seating transaction service, assignment turnover phase, floor action panel | Customer ticket remains waitlist COMPLETED; no meal/payment feature |

The module paths above are planned locations. Use the actual selected files in the completion record after code exists. Every API mutation needs direct role/scope/CSRF checks even if a browser journey is green.

## 3 Retained and deferred responsibilities

| Requirement / feature | Sprint 2 treatment | Later work or evidence gap |
|---|---|---|
| FR-001–011; BR-001–009; F-01–04, legacy F-05 | Preserve MANUAL queue behavior, auth, ownership, party1–16, daily numbering and Lab 7 readiness/polling; version SEATING behavior explicitly | Actual integrated regression results, accepted Sprint 1 reference and Lab 6 UAT reference must be identified in the release evidence |
| FR-017 / F-10 additional counters | No new counter-management UI in the five S2 implementation groups; Q2 race fixture tests an existing shared-resource boundary | Allocation remains S2/S3 per baseline; record a reviewed allocation decision in the Sprint 3 contract rather than silently declaring it optional or complete |
| FR-018/032; BR-014; F-11/F-18 | Persist accurate lifecycle events/timestamps needed by reporting; retain manual-completion provenance | Reporting/history contract #55 and Customer history increment #56; event storage alone does not satisfy the analytics requirement |
| FR-019; NFR-010/013; F-12 | Do not introduce public reads or leak identities through new protected views | Public-display allocation and its own privacy tests belong to the retained S3 scope; protected API tests alone do not prove public-display coverage |
| FR-031; BR-024; F-17 | Preserve implemented status updates without inventing position or ETA | S3 must specify position/estimate behavior and insufficient-data cases; Lab 7 polling alone is not a new tested S3 increment |
| FR-033; F-19 | Seed/provision approved graph safely; no Host drag-based configuration or new Manager role | Retained configuration/admin scope and any deferral need recorded review in #55 |
| FR-021/022; F-L6/F-L7 | Preserve prior UAT/change records and account for affected findings | Do not treat this contract or earlier CI counts as a new UAT re-test or Lab 7 acceptance |
| NFR-007; F-13 | Keep bounded requests and a future load-test responsibility | p95/20-user measurement remains planned S3 verification; no performance result is claimed |

## 4 Completion and assessment records

For each feature issue record ACs achieved, exact source/test files, executed command/name, tested SHA/date, actual pass/fail/skip result, reviewer/PR and accepted main tag. A feature is not complete because its contract row exists or an unrelated test passes.

For the integrated candidate, identify the Sprint 2 acceptance tag independently of later S3 progress, its CI run and deployed version. Keep failures/skips and uncovered requirements visible. Deliverable 2 Part D requires the complete current baseline traceability, combining this S2 matrix with S1, lab changes and S3 records; this file alone is not the final full-project submission.

## Matching increment (#51)

Read-only graph/ranking, scoped preview and floor preview UI are implemented locally. Actual commands and partial STS coverage are recorded in [matching results](sprint2-matching-results.md). Mutation races, holds, auditing and deployed acceptance remain pending later increments.

## Call/Hold/Seat increment (#52)

Atomic group lifecycle, idempotent successful actions, whole-group expiry and SEATING guards are implemented in `apps/api/internal/queue/seating*.go` with migration 0005 and Host floor controls. [Seating verification](sprint2-seating-results.md) maps actual tests to STS cases and records their limits. Host no-show, cancellation, turnover and production activation remain later work.
