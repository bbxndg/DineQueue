# 09. Requirements → Feature → Sprint → Acceptance → Test Traceability

**Document purpose:** Required evidence chain. `Planned` means designed, not delivered; results become `Passed/Failed` only after actual execution tied to git SHA.

Foundation checks `FD-AC-01..07` are tracked separately in
`features/sprint1-foundation-contract.md` and `testing/sprint1-foundation-results.md`.
They do not change any product feature/test status below. The original foundation runs
predated Git initialization; initial import commits do not retroactively attach a tested
SHA to those runs. Subsequent checks must identify the source version actually tested.

## Sprint 1 traceability (complete design mapping)

DEC-028 groups Sprint 1 into four parent features F-01..F-04. F-05 remains the
legacy completion sub-scope of F-04; all FR/BR, AC and test IDs are retained.

| Requirement / rule | Feature | Sprint | AC IDs | Test IDs | Current status |
|---|---|---|---|---|---|
| FR-001 | F-01 | 1 | AC-001..003 | T-001..004 | Local API/DB execution; see testing/f01-auth-results.md |
| FR-002 / NFR-003 | F-01 | 1 | AC-004..005 | T-005..008 | Join (T-005), foreign detail (T-008) and Host endpoints T-005..007 checked; testing/f04-host-queue-results.md |
| NFR-015 | F-01 | 1 + regression S2/S3 | AC-022 | T-037 | Origin/CSRF starts S1; execution in F-01 results |
| NFR-004 | F-01 | 1 | AC-023..024 | T-038,039 | Session/hash/rotation/rollback execution in F-01 results |
| FR-011 / NFR-005 (auth setup) | F-01 prerequisites | 1 | AC-025 | T-040 | Migration/seed executed; ticket persistence still pending |
| FR-001 (browser) | F-01 | 1 | AC-026 | T-041 | Auth browser journey; not queue E2E |
| FR-003 / FR-004 | F-02 | 1 | AC-006..007 | T-009..012 | F-02 local evidence; full T-010 My Ticket journey checked in F-03 worktree |
| FR-009 / BR-001 | F-02 | 1 | AC-008 | T-013,T-014 | Local RED→GREEN worktree evidence (f02-join-results.md) |
| BR-002 / BR-009 / NFR-001 | F-02 | 1 | AC-009 | T-015,T-034 | Local PASS (f02-join-results.md) |
| BR-008 / FR-023 | F-02 | 1 | AC-007 (+ valid 10) | T-011,T-012,T-036 | Local PASS (f02-join-results.md) |
| FR-003 inactive/role | F-02 | 1 | AC-010 | T-016,T-017 | Local PASS (f02-join-results.md) |
| FR-005 | F-03 | 1 | AC-011..012 | T-018..020 | Local API/DB/browser PASS; testing/f03-my-ticket-results.md (uncommitted worktree) |
| FR-010 / BR-007 | F-03 | 1 | AC-013 | T-008,T-021 | Local owner-scoped API and browser denial PASS; F-03 results |
| FR-006 / BR-003 | F-04 | 1 | AC-014..015 | T-022,T-023 | Local API/DB PASS (f04-host-queue-results.md, uncommitted worktree) |
| FR-007 / BR-005 | F-04 | 1 | AC-016..017 | T-024..026 | Local PASS incl. 10-way parallel Call Next (f04-host-queue-results.md) |
| BR-006 | F-04 (board + completion, legacy F-05) | 1 | AC-018,021 | T-027,T-032 | Local API and browser denial PASS (f04-host-queue-results.md) |
| FR-008 / BR-004 | F-04 (completion, legacy F-05) | 1 | AC-019..021 | T-028..033 | Local API/DB and browser T-029 PASS (f04-host-queue-results.md) |
| FR-011 / NFR-005 | F-02,F-03 | 1 | AC-006,011 | T-010,T-019,T-035 | Local reload and actual API restart persistence PASS; F-03 results |
| NFR-004 | F-01 | 1 | AC-001..004,023..024 | T-001..004,038,039 plus config review | Local auth execution; queue endpoint checks still pending |
| NFR-006 | All S1 UI | 1 | AC-011..012,014 | T-010,T-019,T-020 plus manual check | F-03 and F-04 board browser checks PASS; manual accessibility check pending |
| NFR-009 | All S1 | 1 | AC-001..021 | T-001..035 | Planned, not run |

## Sprint 2–3 and labs planned traceability

These feature contracts and corresponding AC/Test IDs must be created **before feature implementation**; placeholders below are not complete tests.

| Requirement / rule | Feature | Sprint / assessment | Planned verification | Current status |
|---|---|---|---|---|
| FR-012 / BR-010 | F-06,F-07 | S2 | capacity, connected group, hold/seat lifecycle, UI assignment | Draft S2 contract linked; tests NOT RUN |
| FR-013 / BR-010 | F-07 | S2 | two Hosts attempt overlapping combined tables concurrently | Draft S2 contract linked; tests NOT RUN |
| BR-011 | F-07 | S2 | oldest eligible party vs too-large earlier party; explain skipped reason | Draft S2 contract; TA policy PENDING |
| BR-013 | F-07 | S2 | complete without table assignment rejected | Draft S2 contract linked; tests NOT RUN |
| FR-014 / BR-012 | F-08 | S2 | owner cancels WAITING; unauthorized/state negatives | Draft S2 contract linked; tests NOT RUN |
| FR-015 / BR-012 | F-08 | S2 | Host NO_SHOW only from CALLED | Draft S2 contract linked; tests NOT RUN |
| FR-016 | F-09 | S2 | complete group checkout, DIRTY→CLEANING→AVAILABLE; prevent double-release | Draft S2 contract linked; tests NOT RUN |
| FR-017 | F-10 | S2/3 | operator per queue permissions | Optional allocation |
| FR-018 / BR-014 | F-11 | S3 | deterministic wait time and count calculations | Draft S2 contract linked; tests NOT RUN |
| FR-019 / NFR-010 | F-12 | S3 | public response excludes PII | Optional allocation |
| NFR-007 | F-13 | S3 | p95 documented load test | Draft S2 contract linked; tests NOT RUN |
| FR-020 / NFR-008 | F-L5 | Lab 5 | CI required gate and GCP persistence | Lab evidence pending |
| FR-021 | F-L6 | Lab 6 | 4 UAT cases + accepted defect + re-test | Lab evidence pending |
| FR-022 | F-L7 | Lab 7 | changed failing test → passing → regression | Lab evidence pending |

## Traceability maintenance instructions

- Keep ID identity stable across document changes and PRs.
- When rule BR-011 changes FIFO policy in Sprint 2, update SRS, Feature SDS, API, AC, tests, and this table together in a reviewed PR.
- Required evidence chain for each sampled feature: `FR -> BR (when relevant) -> F -> AC -> T -> real test file/run -> commit/tag -> PR/review`.
- E2E passing alone does not replace unit, API and database persistence tests.

### v2 extended traceability — to elaborate into S2/S3 contracts **before implementation**

| Requirement / BR | Feature | Sprint | Proposed AC | Planned test IDs | Status |
|---|---|---|---|---|---|
| FR-024, FR-035, BR-025 | F-06,F-16 | S2 | S2-AC-001 floor displays true status; denied actor blocked | S2-T-001,002 | Planned |
| FR-025, BR-016 | F-15,F-19 | S2/S3 | S2-AC-002 connected set only within zone | S2-T-003,004,005 | Planned |
| FR-026, BR-015,017 | F-14,F-15 | S2 | S2-AC-003 10 guests assigned valid 6+4 set | S2-T-006,007,008 | Planned |
| FR-027, BR-018,019 | F-07,F-15 | S2 | S2-AC-004 call+hold atomic across both tables | S2-T-009,010 | Planned |
| FR-028, BR-020 | F-07,F-15 | S2 | S2-AC-005 seat transaction marks all OCCUPIED | S2-T-011,012 | Planned |
| FR-034, NFR-011,012 | F-07,F-15 | S2 | S2-AC-006 concurrent overlapping holds result one winner, no partial writes | S2-T-013,014,015 | Planned |
| FR-029, BR-022 | F-09,F-16 | S2 | S2-AC-007 checkout/cleaning applies to all group members | S2-T-016,017 | Planned |
| FR-030, BR-011,023 | F-14 | S2 | S2-AC-008 recommendation and audited override are explainable | S2-T-018,019 | Planned |
| FR-031, BR-024 | F-17 | S3 | S3-AC-001 visible own ticket; no guaranteed ETA | S3-T-001,002 | Planned |
| FR-032, BR-014 | F-11,F-18 | S3 | S3-AC-002 metrics reconcile with seeded events | S3-T-003,004 | Planned |
| FR-033 | F-19 | S3 | S3-AC-003 manager edit rejects invalid connectivity | S3-T-005,006 | Planned |
| NFR-013,015 | F-01,F-12,F-16 | S2/S3 | S3-AC-004 privacy and CSRF/security controls verified | S3-T-007,008 | Planned |

| FR-014, BR-012 | F-08 | S2 | S2-AC-009 only owner cancels WAITING | S2-T-020,021 | Planned |
| FR-015, BR-012,021 | F-08 | S2 | S2-AC-010 NO_SHOW releases all held members | S2-T-022,023 | Planned |
| FR-029, BR-022 | F-09 | S2 | S2-AC-011 group checkout leaves all DIRTY | S2-T-024,025 | Planned |
| FR-029, BR-022 | F-09,F-16 | S2 | S2-AC-012 no repeated checkout or early reallocation | S2-T-026,027 | Planned |
| NFR-011,015 | F-01,F-07,F-15 | S2 | S2-AC-006 auth/CSRF/parallel locks | S2-T-028,029,030,031,033 | Planned |
| NFR-005,NFR-012 | F-07,F-15 | S2 | S2-AC-005 persisted group after restart | S2-T-032 | Planned |

S2 and S3 IDs are the proposed contracts in `features/sprint2-restaurant-operations-contracts.md` and `testing/sprint2-operations-sts.md`. Final `PASS` requires actual test execution and evidence.
