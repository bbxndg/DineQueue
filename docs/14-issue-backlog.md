# 14. GitHub Issue-Ready Work Breakdown

**Status:** Proposed breakdown. Create real GitHub Issues and a Project Board; no issue numbers/PRs have been created by this package.

## Sprint 1 Issue candidates (P0)

| Working title | F | Depends on | Required evidence / acceptance |
|---|---|---|---|
| `docs: SRS, decisions, feature inventory, traceability` | Baseline | none | FR/BR/NFR, roles, exclusions, sprint allocation |
| `docs: system architecture, DB design, API + UI contract` | Baseline | SRS | role matrix, constraints, endpoint shapes |
| `test: contract-first STS for four S1 features F-01..F-04` | Baseline | feature SDS | test IDs/preconditions/expected/AC coverage, including legacy F-05 completion |
| `chore: repo/monorepo setup and PostgreSQL migrations` | Infra | SDS | clean setup, constraints and idempotent seeds |
| `feat: login/logout/me with session guards` | F-01 | DB | AC-001..005, API tests, password hashing |
| `feat: atomic daily ticket allocation and join` | F-02 | F-01 | AC-006..010, duplicate + race tests |
| `feat: customer own-ticket page and privacy` | F-03 | F-02 | AC-011..013, browser/API tests |
| `feat: Host queue board, FIFO call-next and complete` | F-04 (includes legacy F-05) | F-01, F-02 | AC-014..021, race/permission/negative transition tests |
| `test: integrated demo, release setup, peer reviews` | Release | all S1 | real run logs, 3 key journeys, PR review, tag |
| `release: sprint1 staging -> reviewed main/tag` | Release | integration | exact SHA and reproducible demo |

## Suggested two-person sequencing

**Both:** Agree core SRS/BR/AC first; review each other's wording.
**Student A initially leads** DB/Join/T-014; **Student B initially leads** Auth/Host Call Next/T-026; cross-review each. Then pair on UI integration, E2E, main release. No fixed permanent spec-only/coding-only separation.

## Sprint 2 Issue candidates

1. Pre-implementation contract for F-06 table model and API.
2. Table catalog + Host table grid with seeded capacities.
3. Table assignment transaction + capacity/occupancy negative cases and concurrency.
4. Document/implement FIFO-to-first-eligible contract revision after TA decision.
5. Cancel WAITING + Host no-show from CALLED; transition/ownership tests.
6. Table release and assignment history.
7. Integration regression, deployment and traceability update.

## Sprint 3 Issue candidates

1. History and service metrics from actual timestamps.
2. Public status display with privacy test, only if retained.
3. Cross-team UAT defect fixes + re-test evidence reconciliation.
4. Accessibility, error/loading states, regression performance checks.
5. Final evidence index/PDF, release summary, demo video script and tagged deploy.

## Issue template

```markdown
# [F-XX] Short name
**Source Requirements:** FR-..., BR-..., NFR-...
**Sprint:** 1/2/3
**Feature Contract:** docs/features/...
**AC IDs:** AC-...
**Planned Tests:** T-...
**Dependencies:** ...
**Steps:** test (RED) -> code -> test (GREEN) -> PR -> review -> integration
**DoD:** ...
**Evidence URL, test command, commit:** [fill after execution]
```


## v2 issue candidates (create with real assignees/AC before implementation)

| Issue | Priority | Sprint | Evidence/Definition of Done |
|---|---|---|---|
| DQ-20 Floor zones, table capacities (<=8), layout seed, connection graph | P0 | 2 | DB migrations + negative edge/zone tests |
| DQ-21 Smart candidate enumeration and deterministic ranking | P0 | 2 | S2-T-003..008 pure function tests |
| DQ-22 Atomic multi-table hold and call | P0 | 2 | S2-T-009,010,013,014 |
| DQ-23 Seat group / complete queue | P0 | 2 | S2-T-011,012,015 |
| DQ-24 Checkout → dirty → clean → available | P0 | 2 | S2-T-016,017 |
| DQ-25 Staff interactive floor and conflict recovery | P0 | 2 | S2-T-001,002 + browser UAT |
| DQ-26 Audited operator override and policy | P1 | 2/3 | S2-T-018,019 |
| DQ-27 Customer waitlist enhancements / estimation guard | P1 | 3 | S3-T-001,002 |
| DQ-28 Restaurant metrics and reconciliation | P1 | 3 | S3-T-003,004 |
| DQ-29 Manager floor/config editor | P2 | 3 | S3-T-005,006 |
