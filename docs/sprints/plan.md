# 06. Three-Sprint Development Plan

**Source:** CPE 334 §§4 and 7; deadlines all 23:59 per specification. Three sprints are one continuous codebase and repository.

## Sprint 1 — Working deployable-ready vertical slice (due 8 Nov 2026)

**Goal:** A small **real** restaurant waitlist application with authenticated Customer and Host and tested domain flow.

### Committed functionality

- F-01 Login/logout/current identity with seeded 2 roles and server-backed authorization.
- F-02 Join active seeded service queue, validated party size, concurrency-safe unique ticket.
- F-03 View own ticket/status; deny cross-customer access.
- F-04 Host view waiting/called and `Call Next` in FIFO order; scoped permission.
- F-05 Host complete called entry; reject invalid transitions.
- Postgres schema/migrations/idempotent seed, local setup, automated unit/API-DB/E2E tests.
- GitHub Issues + sprint staging + reviewed feature PRs + main release tag + reproducible demo.

### Sprint 1 demo script

1. Seed demo restaurant, one queue, two customers, one authorized Host, one unauthorized Host.
2. Customer A logs in; joins with party size 4; sees `A-001` WAITING in own ticket page.
3. Customer B logs in; joins with party size 2; sees `A-002` WAITING.
4. Customer A tries to join same queue again; receives business-rule rejection (409).
5. Assigned Host logs in and calls next; `A-001` becomes CALLED; `A-002` remains WAITING.
6. Host attempts Call Next again before completing first; gets `CALL_IN_PROGRESS`.
7. Host completes A-001; then can call A-002.
8. Customer B tries Host call endpoint (403); Customer B tries Customer A ticket detail (404).
9. Restart app/API, show tickets still persisted. Show actual test command and result tied to release commit.

### Definition of success

All F-01..F-05 AC pass, state and unique-index invariants hold even under concurrent tests; local API/UI/database flows work; release in `main` with tagged commit, no cloud required **yet**.

## Sprint 2 — Core restaurant table workflows (complete by 29 Nov 2026)

**Goal:** Connect waitlist queue to actual restaurant table resources.

- F-06 Table catalog and availability grid; seed 2/4/6/8-seat tables with an approved adjacent 6+4 pair.
- F-07/F-14/F-15 Atomic candidate match, call + short table hold, single/combined table seating; connectivity and capacity checks, exclusivity and tested fairness policy.
- F-08 Customer cancellation (WAITING), Host No-show (CALLED), terminal-state restrictions.
- F-09 Full-group checkout to DIRTY, cleaning confirmation and reavailability, with resource history.
- F-10 Additional counters only if it does not jeopardize required core workflows.
- Contract-first Feature SDS, UI/API/data specs, STS/AC, reviewed Issues/PRs and integrated deployment after Lab5.

**Acceptance demonstration:** Calling and seating a 4-person party at capacity-4 table succeeds. A 10-person group seats at two explicitly connected 6+4 tables; insufficient, nonadjacent, HELD or occupied sets fail. Two Hosts cannot claim overlapping sets. Demonstrate cancel/no-show and allowed transitions.

## Sprint 3 — Completion, integration, QA and release (13 Dec 2026)

**Goal:** Finish all baseline scope, usability, reporting and release readiness.

- F-11/F-18 Service history, operational counts, capacity utilization, wait and turnover durations, reconciliation tests.
- F-12 Public minimal display and F-17 private waitlist status/optional guarded ETA (if retained; record deferrals).
- F-13/F-19 Floor configuration where feasible, error handling, permission boundary tests, full regression suite, UAT fixes, accessibility, restart and deployment checks.
- By 29 Nov: **contracts + actual implemented progress + explicit remaining-work list**, not empty placeholder screens.
- 13 Dec: verified final product, one concise final PDF evidence report, repository/release link, deployed URL and video <=10 minutes with both students.

## Labs overlay (same evolving product)

| Assessment | Date | Engineering requirement |
|---|---|---|
| Lab 5 | 15 Nov | Docker + guarded CI + GCP deployment; exact release commit, persistence through restart/redeploy |
| Lab 6 | 22 Nov | Swap UAT with partner; four cases; at least one accepted defect fixed and independently re-tested (TA exercise if none found) |
| Lab 7 | 29 Nov | TA-approved maintenance/change, changed test failing before change, regression, 5 dated engineering metrics, release readiness |

## Dependencies and critical path

`SRS/Decisions -> System SDS -> Feature contracts + AC/STS -> DB/Auth -> Join -> Own Ticket -> Call Next -> Complete -> Integration Tests -> Reviewed PR -> Main Tag`.

After S1: `Lab 5 deployment -> Lab 6 partner UAT` and `S1 queue -> S2 table management -> S3 analytics/QA` proceed together. Budget exam time in Dec; product should be substantially complete on 29 Nov per PDF.

## Risks and mitigations

| Risk | Probability/impact | Mitigation |
|---|---|---|
| Too many UI screens before S1 | high/high | Sprint 1 has 4 primary pages; remove extras |
| Concurrency bug | medium/high | Postgres constraints + row locking; deterministic race tests |
| Weak evidence despite working code | high/high | Link every AC/test/PR/commit in evidence index |
| Last-minute deployment | medium/high | Design Docker-ready architecture now; implement Lab5 by due date |
| Pair work uneven | medium/high | Both contribute to specs, tests, code, reviews and releases; alternate task ownership |
| S2 table-matching interpretation | medium/medium | Record FIFO exception and confirm with instructor/TA early |

## Explicitly not scheduled

Online reservations, actual POS/payment, SMS, restaurant ordering, predictive ML wait times, map of restaurants, loyalty system. These create scope without improving required queue engineering outcomes.


## v2 critical-path and capacity safeguards

- Preserve the Sprint 1 core queue deliverable; **do not retroactively require table features for 8 Nov**. Change party size max to 16 with the new T-036 boundary case when contracts are updated.
- Sprint 2 P0 path: table/zone seed → explicit connection graph → atomic group hold → call+hold → seat all → checkout all → cleaning → concurrency tests and UI. Multi-queue, animation, and drag-and-drop layout editing are secondary.
- Worst-case example: A 10-person party can be seated by T4(6)+T5(4) only if physically connected, same zone, both AVAILABLE, no overlapping hold, no conflicting assignment. The **single 8-seat table** is correctly rejected.
- Sprint 3 path: measured dashboard, private queue status, secure operations audit, UX/accessibility, comprehensive regression.
- Critical combined-table failure: any locked table status/edge/permission change aborts the full transaction; no partial table group reservation.
