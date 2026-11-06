# 02. Feature Inventory and Scope Allocation

Source: CPE 334 §§6 theme 9, 7 and 8.1. A feature's sprint is a **proposed team allocation**, not an instructor-prescribed feature count.

| Feature ID | Feature | Key FR/BR | Sprint | Priority | Dependency | Observable deliverable |
|---|---|---|---|---|---|---|
| F-01 | Login, logout, current user, seeded roles | FR-001, NFR-004 | 1 | P0 | Database + secrets | Customer/Host authenticated session |
| F-02 | Join waitlist and unique ticket issuance | FR-003,004,009, BR-001,002,008,009 | 1 | P0 | F-01, queue seed | Customer receives persisted ticket |
| F-03 | My ticket status and ownership control | FR-005,010, BR-007 | 1 | P0 | F-01, F-02 | Customer sees only own ticket |
| F-04 | Host queue board and Call Next | FR-006,007, BR-003,005,006 | 1 | P0 | F-01, F-02 | Authorized Host calls oldest queue |
| F-05 | Complete called ticket | FR-008, BR-004,006 | 1 | P0 | F-04 | Called ticket becomes COMPLETED |
| F-06 | Interactive table/floor zones + availability | FR-012 | 2 | P1 | Data model | Host views zone/approved connections, table states/capacity |
| F-07 | Capacity-aware matching, atomic call/hold/seat | FR-012,013, BR-010,011,013 | 2 | P1 | F-04, F-06 | Called party seated at one fitting table or connected table set |
| F-08 | Cancel and No-show | FR-014,015, BR-012 | 2 | P1 | F-02, F-04 | Approved terminal transitions |
| F-09 | Group checkout → dirty/cleaning → availability | FR-016 | 2 | P1 | F-07 | All related tables become available after cleaning |
| F-10 | Additional authorized queue/counter | FR-017 | 2/3 | P2 | F-04 | Permission-scoped separate waitlist |
| F-11 | Operations history, analytics and measured metrics | FR-018, BR-014 | 3 | P1 | F-05, F-08 | Counts, time summaries, audit entries |
| F-12 | Non-private public display | FR-019, NFR-010 | 3 | P2 | F-04 | No personal identifiers shown |
| F-13 | Quality/usability hardening, regression | NFR-006,007,009 | 3 | P0 | All flows | Final verified release |
| F-L5 | Container + CI/CD + GCP | FR-020, NFR-008 | Lab 5 | Mandatory | Sprint1 release | Deployed repeatable workflow |
| F-L6 | Independent UAT, fixes and re-test | FR-021 | Lab 6 | Mandatory | Lab5 release | Evidence of 4 cases + 1 accepted fixed defect |
| F-L7 | Change impact + regression + metrics | FR-022 | Lab 7 | Mandatory | Implemented workflow | TA-approved change safely released |


### Expanded feature set (v2 additions; old IDs retained)

| Feature | Name | Requirements / Rules | Sprint | Priority | Demo / evidence |
|---|---|---|---|---|---|
| F-14 | Smart seating recommendations with transparent ranking | FR-030, BR-011,017,023 | 2 | P0 | Oldest eligible party + ranked valid table sets, explicit reason |
| F-15 | Combined adjacent-table allocation | FR-023,025,026,027,028,034; BR-015..020,025 | 2 | P0 | 10 guests at T4(6)+T5(4), cannot allocate nonadjacent set |
| F-16 | Interactive floor zones + state transitions | FR-024,029,035; BR-022,025 | 2 | P0 | Staff picks tables, sees HELD/OCCUPIED/DIRTY/CLEANING, recovers from conflicts |
| F-17 | Customer queue experience + wait estimate | FR-031, BR-024 | 3 | P1 | Live status, position or explicitly unavailable estimate |
| F-18 | Restaurant operating analytics | FR-032, NFR-013 | 3 | P1 | Persisted metrics by day/zone/service and verified report calculations |
| F-19 | Table/connection administration | FR-033, BR-016,025 | 3 | P2 | Validated connection edits and floor layout positions |

**Priority:** P0 core deliverables (not optional for selected v2 product), P1 planned enhancement, P2 deferrable only by recorded scope change. Avoid building floor drag-and-drop before atomic group seating.

## Scope-control rules

1. If Sprint 1 is at risk, remove design extras (public screen, charts, customer cancellation, table floor plan) before touching required tests, API persistence or role enforcement.
2. Sprint 1 needs **one real end-to-end slice**, not a polished multi-branch restaurant platform.
3. Table management is DineQueue's differentiation but must **not prevent** the 8 Nov Sprint 1 release.
4. Maintain `F-xx`, `FR-xxx`, `BR-xxx`, `AC-xxx`, `T-xxx` as stable IDs even when wording changes; record revisions.
5. Add a Feature-Level SDS / AC / STS *before* implementing each Sprint 2/3 feature.
