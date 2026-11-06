# DineQueue v2 — Agent / Developer Instructions

Follow `README.md` → `docs/01-srs.md` → `docs/06-restaurant-operations.md` → the applicable pre-implementation feature contract and STS. **These are proposed specs, not authorization to claim implementation or passing tests.** The course PDF is the authority for assessment process; our algorithms/stack need team/TA approval.

## Scope and priorities

- This is a walk-in **queue** product, not appointment booking, POS, payments or automatic restaurant reservations.
- Preserve S1 F-01..F-05: login/logout/me; backend RBAC + own-ticket privacy; join party 1–16; daily unique queue number; FIFO call-next/complete and concurrency tests. S1 completion means *manual waitlist handling*.
- S2 requires F-06..09 and F-14..16: **interactive floor, approved adjacency graph, deterministic matching, atomic hold of 1–3 connected tables, seat, no-show, group checkout and cleaning**. Do not implement seating as a single `table_id` on ticket.
- Sample 10-person group: T4(6)+T5(4) approved connected pair qualifies; T7(8) alone and unconnected sets do not. All chosen tables must share restaurant/zone and be AVAILABLE.
- Keep old FR/F/BR IDs and explicitly version changed BR-008/010/011/013; new IDs FR-023..035, BR-015..025, F-14..19.

## Architecture and safety

- Leading **proposed** backend: Go Fiber v3 + pgx/sqlc + PostgreSQL, Next.js frontend; do not scaffold until team confirms version and stack.
- Module design: HTTP handler → business service (rules/transaction/permissions) → SQL repository; no transaction split across service calls.
- PostgreSQL is authoritative: partial unique indexes, row locks in deterministic order, transactional table-group membership, concurrency/rollback tests. Never trust UI status, capacity, adjacency, ranks or user role.
- Sessions: HttpOnly Secure cookie, opaque token hash in DB, logout revocation, CSRF defense and Origin restrictions, no secrets in client bundles. Strict input validation and parameterized SQL.
- Public display is ticket-code-only; never leak customer email/name/free-text notes or private DB identifiers.
- Do not execute third-party install scripts from untrusted sources. Pin and review dependencies, use official documentation as references. No security claim is absolute.

## Workflow / evidence

1. Log pending decisions in `docs/13-decisions.md`, get team/TA sign-off before freezing rules.
2. Create Issue → feature branch from sprint staging → commit pre-implementation Feature SDS / UI/API/Data / AC / STS / DoD.
3. Implement tests fail-first for important rules, add code, run test suite against isolated real DB, inspect diff, request teammate review.
4. Integrate sprint, regression-test S1 whenever S2 lifecycle changes, release through reviewed main PR and tag.
5. Record actual test names/commands/dates/commit/CI URLs. **Do not report PASS, deployment or approval without evidence.**
6. Keep docs, traceability and decision history synchronized with code.

## Read specific files

- Domain invariants: `docs/06-restaurant-operations.md`
- Data / API: `docs/04-data-and-api.md`
- S2 contracts: `docs/features/sprint2-restaurant-operations-contracts.md`
- S2 STS: `docs/testing/sprint2-operations-sts.md`
- Security: `docs/16-security-quality-review.md`
- Course labs and evidence: `docs/labs/labs5-7.md`, `docs/release/submissions.md`
