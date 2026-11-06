# Sprint 1 Tactical Execution Checklist (proposed, 5–8 Nov 2026)

This is a timebox suggestion for a two-person team; the PDF requires a working Sprint 1 release by **8 Nov 2026 23:59**, but does not prescribe daily assignments. Adjust to team availability.

## Day 1: Review, contract, repo skeleton, auth

- [ ] Verify both student IDs permit Theme 9 and review unresolved decisions (DEC-001..008).
- [ ] Both review SRS, role matrix, FR/BR/NFR, Feature Inventory, 3-sprint plan, and commit baseline before code.
- [ ] Agree API and DB migration structure; create repo/board, sprint-1-staging and first feature issues.
- [ ] Implement local PostgreSQL migrations and reproducible demo seeds.
- [ ] Define STS tests before implementation, record test-first evidence where useful.
- [ ] Build F-01 login/logout/me with real DB-backed session and invalid/role tests; peer review.

## Day 2: Core customer ticket and status

- [ ] F-02 atomic join + constraints + negative/boundary tests.
- [ ] Test simultaneous identical-customer join and multiple distinct-customer joins on PostgreSQL.
- [ ] F-03 My Ticket screen + direct ownership test.
- [ ] Integrate frontend/API and persist data between restart; review PRs.

## Day 3: Operator queue, transitions and end-to-end

- [ ] F-04 authorized queue list, FIFO call-next inside transaction and concurrency integration test.
- [ ] F-05 complete CALLED and invalid transitions tests.
- [ ] Run browser E2E Customer -> Host -> Customer and 2 negative journeys.
- [ ] Fix integration bugs via fix PRs, update docs, traceability and test record.

## Day 4 (8 Nov): QA freeze and submission before 23:59

- [ ] From fresh checkout: run migrations/seed, start app, run documented build/tests and full demo.
- [ ] Verify main branch release PR is reviewed, test results are correct; tag exact SHA.
- [ ] Fill evidence-index URLs and test commands/results, contribution table, AI declaration.
- [ ] Submit **one team entry to LEB2** with repo/tag/setup and rubric evidence before deadline.
- [ ] Preserve all failing/passing test logs actually run, PR history and release commit.

## Triage if behind schedule

Keep first: F-01..05 actual UI/API/DB, 2 roles, key negative/concurrency tests, contracts and PR review. Defer: fancy animation, public digital display, cancellation, tables, reports, estimated wait times, multi-branch architecture.

## Submission blockers

- No deployed cloud yet is **not** a Sprint 1 blocker (Lab5); no local running app **is**.
- Pretty screenshots alone are **not** tests.
- No real PR review / no evidence links are grading risks even if code works.
- The present docs are a draft. The team must review, adapt and commit contracts **before** code to prove intended process.


### v2 boundary change

- [ ] Update BR-008 / AC-007 / T-011 to reject `partySize=17` rather than `9`; add T-036 acceptance for `partySize=10`.
- [ ] Do not implement floor map, connected table group allocation or recommendation engine as a prerequisite for Sprint 1; those are Sprint 2 features.
- [ ] Record actual decision approval and pre-implementation revision date in Git.
