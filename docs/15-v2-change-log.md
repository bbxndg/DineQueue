# 15. Change Log — Engineering Package v1 → v2

**Date:** 2026-11-05. This is a specification revision and **does not** indicate that the group implemented, ran tests or obtained instructor sign-off.

## Requested changes and exact impact

| User change | Old specification | v2 behavior | Artifacts to update |
|---|---|---|---|
| Customer Waitlist Management | party 1–8, basic status | party 1–16; 10+ accepted; private status, future ETA/cancel/history | SRS BR-008 FR-023/031; Feature F-02/F-17; AC-007 T-011/T-036; UI |
| Interactive Floor | basic table grid | zones, state, accessible floor/list, graph and cleaning | SRS FR-024/029/035; F-06/F-16; SDS/Data/UI; STS |
| Smart Matching | oldest fitting single table | deterministic joined set recommendations; explain skipped cases | FR-025/026/030, BR-011/016/017, F-14 |
| Joined Tables | one table per assignment | 2–3 connected tables, one group assignment, atomically held/seated | FR-026..028/034, BR-015..021, F-07/F-15, Data/API and S2 tests |
| Restaurant Operations Analytics | counts/average wait | queue wait, occupancy, turnover, no-show, data-quality notes | FR-032, F-11/F-18, S3 plan |
| Backend direction | NestJS/Prisma proposal | Go Fiber v3 + pgx/sqlc proposal, Next.js, PostgreSQL retained | SDS, API, README, AGENTS, STS |

## Traceability / release constraints

- Existing `FR-001..022`, `F-01..13`, `BR-001..014` identities are kept for historical reference; conflicts (BR-008/010/011/013) are **explicitly revised**, not silently ignored.
- New FR-023..035, BR-015..025, NFR-011..015, F-14..19 are tracked in the SRS/features/traceability tables.
- Sprint 1 remains the minimal authenticated queue slice; the party-size boundary changes and requires a visible contract/test revision before implementation.
- Sprint 2 contracts and detailed tests are new; they cannot retroactively become proof of pre-implementation history.
- `README.md` is the entrypoint; detailed nontechnical explanation in `docs/00-thai-overview.md`; joined-table scenarios in `docs/06-restaurant-operations.md`.

## Remaining team/TA decisions

**6 November baseline clarification:** the user confirmed v2 as the latest project baseline.
Stale SRS party-size text now reads 1–16 (physical tables remain 1–8), and AC-007 now
requires a persisted WAITING ticket for a valid 10-person party. Existing IDs and the
documented BR-008/010/011/013 revision history are preserved. This is not TA approval
of unresolved policies, implemented functionality or executed product-test evidence.

- Confirm per-party max 16, maximum joined-table count 3 and floor safety restrictions.
- Confirm how an operator may skip an earlier group and how the action is audited.
- Confirm hold expiry/no-show handling and in-service semantics.
- Confirm Go Fiber stack with both developers; the course encourages reuse of Lab 1–4 stack but does not mandate it.
- Confirm whether a manager editing UI is in final scope or seeded admin config is enough.
- Record acceptance dates/decision owners before treating proposed rules as frozen requirements.
