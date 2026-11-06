# 12. Submission, Rubric Mapping and Evidence Index

**Master source:** CPE 334 PDF §§3–4, 9–11; submission through LEB2; one submission per team; all deadlines 23:59 on date given. Detailed companion handouts may add instructions.

## Deliverable 1 — 8 Nov 2026, 80 raw points / 8% course

| Rubric part | Points | Planned artifact | Required REAL evidence at submission |
|---|---:|---|---|
| SRS and product scope | 10 | `docs/01-srs.md` | Actors, unique FR/BR/NFR, assumptions/scope/exclusions and resolved choices |
| Feature inventory + traceability | 10 | `docs/02-features.md`, `docs/09-traceability.md` | Requirement -> feature -> sprint coverage |
| System-Level SDS | 15 | `docs/03-system-sds.md`, `docs/04-data-and-api.md`, `docs/05-ui-spec.md` | Architecture, data/roles/permissions, UI/API, config/deploy plan |
| Three-sprint plan | 5 | `docs/sprints/plan.md` | Sprint goals/features/dependencies/remaining work |
| Sprint 1 feature contracts | 20 | `docs/features/sprint1-contracts.md`, `docs/testing/sprint1-sts.md` | Feature SDS, UI/API/data, AC, STS, DoD, history before implementation |
| Working Sprint 1 + verification | 20 | actual code, repo, tests, evidence index | UI/API/DB real journey, rule rejection, auth/access tests, Issues/PRs/review/integration |
| **Total** | **80** | | |

**Deliverable 1 must include:** runnable main release, exact tag/SHA, setup instructions, test commands/results and brief reproducible demo sequence. **Cloud deployment NOT required for D1.**

## Deliverable 2 — 29 Nov 2026, 70 raw points / 7% course

| Rubric part | Points | Evidence |
|---|---:|---|
| Sprint 1 continuity/change status | 5 | Released baseline, defect and change status |
| Sprint 2 contracts + working increment | 25 | Pre-implementation contracts, AC/tests, reviewed PRs, release |
| Sprint 3 contracts + working progress | 15 | Contracts/tests, actual work in progress, explicit remaining list |
| Updated requirements/test traceability | 10 | FR/BR/NFR → feature → sprint → AC/verification → actual test/results |
| Integrated release candidate | 15 | S1+S2+done S3, CI/deploy SHA, regression, exam completion plan |
| **Total** | **70** | |

**All Sprint 2 functionality finished; Sprint 3 may contain bounded incomplete tasks. Deployed integrated release candidate required.**

## Final Project Delivery — 13 Dec 2026, 100 raw points / 10% course

| Rubric part | Points | Evidence |
|---|---:|---|
| Final working product | 35 | All major workflows/rules, failures, persistent DB, permissions |
| Engineering process/team | 15 | Real Issues/PRs/reviews/integration/releases and both students |
| Requirements/verification traceability | 20 | Requirement → feature → AC → test → final actual result |
| Final QA & DoD | 20 | Regression, UAT/retests, defect status, deploy/config/known debt |
| Demo video | 8 | <=10 minutes, both students, normal journey/rejected case, architecture/test/CI/traceability example |
| Final release summary | 2 | One page maximum, scope, version/SHA, links, completion, limitations, team |
| **Total** | **100** | |

**Submission:** one concise final PDF *evidence index*, repository/release reference, deployed application URL and video. Deployed app must remain available through grading period until instructor announces close.

## Group labs: additional separate grading pools

- **Lab 5, 15 Nov: 100 points / 5% course** — see `docs/labs/labs5-7.md`.
- **Lab 6, 22 Nov: 100 points / 5% course** — see same.
- **Lab 7, 29 Nov: 100 points / 5% course** — see same.

Project deliverables total **25%**, group Labs 5–7 total **15%**; combined these project-associated submissions are **40%** of course grade. The other individual Labs 1–4 are a separate 20% contribution.

## Evidence index template (one per assessment)

| Rubric part | Artifact/direct URL | IDs and exact submitted SHA | Verification result/status |
|---|---|---|---|
| SRS | GitHub URL `[fill]` | FR-001..022; SHA `[fill]` | Draft / reviewer confirmation `[fill]` |
| Sprint 1 contract | GitHub URL `[fill]` | F-02, AC-006..010, T-009..017; SHA `[fill]` | Planned → implemented `[fill]` |
| DB concurrency test | file/CI run URL `[fill]` | BR-001, T-014, SHA `[fill]` | NOT RUN |
| Queue E2E | file/CI run URL `[fill]` | F-04 (includes legacy F-05 completion), T-029, SHA `[fill]` | NOT RUN |
| PR review | PR URL `[fill]` | linked Issue/F/AC `[fill]` | NOT DONE |
| Release | tag URL `[fill]` | exact commit `[fill]` | NOT RELEASED |

The final submission should **not** merely attach this draft package. It must include actually executed evidence and correct links to the team repository and deployed release.
