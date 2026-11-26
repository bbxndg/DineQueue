# Sprint 2 restaurant operations engineering contract

Version: DQ-S2-CONTRACT-2.1, 27 November 2026. Issue: [#49](https://github.com/bbxndg/DineQueue/issues/49). Status: pre-implementation draft for teammate review. No Sprint 2 implementation, test execution, release acceptance or instructor approval is established by this document.

## 1 Source baseline and review boundary

The source baseline is main commit [`b4d160675695328c2a73d098a185c4be05022e02`](https://github.com/bbxndg/DineQueue/commit/b4d160675695328c2a73d098a185c4be05022e02), including the Lab 7 readiness and polling changes. Both `sprint2-staging` and this documentation branch started at that commit. It is a source baseline, not a newly executed regression or confirmation of the accepted Lab 6 UAT version.

The assessment authority is `project_deliverable_2_v1.pdf`, sections 3.1–3.5 and 4.1–4.2: complete Sprint 2, preserve Sprint 1, maintain contracts before code, show actual tests and reviewed integration, and deploy the identified candidate. Sprint 3 contracts and a tested Sprint 3 workflow are separate work under #55 and #56; these five implementation groups do not replace that requirement.

Read this contract together with [UI/API/data definitions](sprint2-ui-api-data.md), [STS](../testing/sprint2-operations-sts.md), [traceability](../testing/sprint2-traceability.md) and decisions DEC-034–038 in [the decision register](../13-decisions.md). Those definitions supersede conflicting older S2 sketches; existing implemented S1/Lab 7 interfaces remain valid except for explicitly specified SEATING-mode guards.

The user selected table-set-first matching and five-minute expiry to NO_SHOW on 27 November. Remaining technical choices below are proposals for the contract review. Record the actual reviewer, PR, reviewed commit and date before implementation; a documentation merge alone does not prove approval. Do not label unrelated pending domain decisions as TA-approved.

## 2 Work packages and ownership

| Issue | Features | Branch | Dependencies | Responsibility |
|---|---|---|---|---|
| [#49](https://github.com/bbxndg/DineQueue/issues/49) | Engineering documents, not a product feature | `docs/sprint2-engineering-contract` | Baseline above | Both students review contracts; feature ownership is not assigned yet |
| [#50](https://github.com/bbxndg/DineQueue/issues/50) | F-06/F-16 floor inventory | `feat/sprint2-floor-inventory` | Reviewed #49 | Owner and reviewer to be agreed before starting |
| [#51](https://github.com/bbxndg/DineQueue/issues/51) | F-14 matching | `feat/sprint2-table-matching` | #50 merged | Owner and reviewer to be agreed before starting |
| [#52](https://github.com/bbxndg/DineQueue/issues/52) | F-07/F-15 call, hold and seat | `feat/sprint2-call-hold-seating` | #50 and #51 merged | Owns shared assignment, event, idempotency and expiry services |
| [#53](https://github.com/bbxndg/DineQueue/issues/53) | F-08 cancellation and no-show | `feat/sprint2-cancel-no-show` | #52 merged | Reuses the shared release service; no duplicate expiry implementation |
| [#54](https://github.com/bbxndg/DineQueue/issues/54) | F-09/F-16 checkout and cleaning | `feat/sprint2-checkout-cleaning` | #52 merged; integrate #53 before acceptance | Owns turnover transitions and guarded activation tooling |

Branch each feature from updated `sprint2-staging` after its dependencies merge. Each issue covers backend, UI, tests and evidence, rather than separate unintegrated frontend/backend deliveries. Use `Part of #<issue>` in the relevant PR, obtain substantive teammate review, then integrate and run regression. This branch changes documents only; it does not add migrations or runtime configuration.

## 3 Rules and versioned changes

| Rule/version | Required behavior |
|---|---|
| BR-008-S2-v1; BR-015 | Retain integer party size 1–16 and physical table capacity 1–8. Joining never depends on a currently fitting table. No new change to the existing party-size boundary is introduced. |
| BR-003-L7-v1 retained | New tickets start Not ready. Only the owner of WAITING may set readiness; returning Ready retains original `(joined_at,id)`. Never displace CALLED. |
| BR-010-S2-v1; BR-016–019 | Host selects 1–3 distinct AVAILABLE tables, one restaurant/zone, with approved active connections forming a connected graph. Call+hold and seat are separate atomic transactions. |
| BR-011-S2-v1 | For that selected set, select the oldest `WAITING AND is_ready=true` party whose size fits. An earlier Ready party may be skipped because this set is too small even if a different set elsewhere could fit it. Show `NOT_READY` or `CAPACITY_NOT_ELIGIBLE` to Host, preserve order, and persist the call selection rationale. There is no arbitrary party override. |
| BR-017; BR-023-S2-v1 | Recommend fitting sets for a requested Ready party by fewest tables, least spare capacity, then lexicographically sorted canonical table IDs. Host may choose another valid set; for that final set the server recomputes the eligible party. Record `RECOMMENDED` or `ALTERNATIVE_SET` and the selected set; neither permits bypassing eligibility. |
| BR-013-S2-v1; BR-020 | In SEATING mode, CALLED→COMPLETED occurs only through Seat with a valid unexpired HELD assignment. It ends waiting, not dining. Tables stay OCCUPIED until checkout. |
| BR-012; BR-021-S2-v1 | Owner cancellation is WAITING→CANCELLED. Host no-show and server expiry are CALLED→NO_SHOW, HELD→RELEASED and every member HELD→AVAILABLE together. Expiry uses server time, with `now >= hold_expires_at` expired. |
| BR-022-S2-v1 | Checkout closes the entire ACTIVE assignment and sets all members DIRTY. Whole-group DIRTY→CLEANING→AVAILABLE requires separate confirmations; never release only part of a group. |
| BR-001/002/005/007/009 retained | One active ticket per owner/queue, unique daily number, one CALLED per queue, own-ticket privacy, and server-issued times/numbers remain authoritative. |

The seeded example has Main T1(2)–T2(4)–T3(8), Terrace T4(6)–T5(4)–T6(2), and isolated Private T7(8). Edges are approved configuration, not proximity on screen. Ten guests fit T4+T5; T7 alone and T4+T7 do not. Party size 16 is valid for joining even if this sample floor has no adequate same-zone set; do not promise eventual seating or invent a match. Boundary tests use a separate valid 8+8 connected fixture for a seatable party of 16.

## 4 Compatibility, expiry and transaction boundaries

### Manual and seating queues

Add a persisted per-queue `operations_mode` with MANUAL default for existing records. MANUAL retains the implemented Lab 7 Call Next and manual Complete behavior. SEATING uses the new table lifecycle. In SEATING, the old bodyless `/call-next` and `/complete` endpoints return documented 409 guards; they cannot bypass a hold or seat tables. The Host board displays the mode and the corresponding actions.

Activation is explicit after the integrated lifecycle is ready. Require configured floor data and no legacy CALLED entry. An operator completes any existing manual call first; activation must not silently seat, cancel or fabricate an assignment for it. Preserve historical COMPLETED rows without assignments and every WAITING row's code, time and readiness. Provisioning never reruns production synthetic seed or changes existing passwords.

### Expiry processing

The hold lifetime is 300 seconds by default, captured on the assignment from the queue's validated policy. Database time sampled after acquiring locks decides whether seating is allowed. The browser countdown is informational.

Expiry cleanup is a bounded, idempotent server operation. Execute it before processing authorized seating mutations and provide an explicit Host `expire-holds` action for an expired current call. Cleanup commits as its own system transaction, with reason `HOLD_EXPIRED`; subsequent validation failure must not roll that cleanup back. Read-only GETs and polling do not perform expiry writes. Do not promise a timer will run while Cloud Run has no CPU/instances: safety takes effect at the deadline, and persisted NO_SHOW/release is completed at the next cleanup invocation. The UI explains an expired hold and offers cleanup rather than presenting it as seatable.

This distinction is observable: a rejected seat cannot create OCCUPIED tables, while an independently committed expiry may already have changed that ticket to NO_SHOW. Other validation rejections make no state changes. Tests must separate those two transactions and verify the expiry event.

### Shared lock order and atomicity

Seating mutations authorize the queue, then lock the restaurant floor-state row, queue, entry, selected member tables in canonical ID order, and assignment/action records. Recheck authorization and all relevant state inside the transaction. A restaurant-level lock coordinates shared tables even across two different queues; a queue lock alone is insufficient. Readiness/cancel retain queue→entry ordering and never acquire a floor lock after a queue lock. Any code path needing both must start at the floor lock.

Call+hold commits queue status, assignment, every table/member and audit event together. Seat, no-show/expiry, checkout and each cleaning step also commit their complete group changes together. Failure injection must prove rollback of state, memberships, action records and events. The same `tx *gorm.DB` is passed through every participating repository; no nested independently committed service transactions and no startup AutoMigrate.

## 5 Feature contracts

### F-06 and F-16 floor inventory, issue #50

SDS: resolve a granted queue to its restaurant and read zones, approved edges, table capacities, statuses and that queue's assignments as a consistent snapshot. Shared-floor occupancy from another queue is shown as state only, without its ticket, assignment or customer identifiers. Render `/host/floor` with queue/zone selection, interactive table selection and equivalent keyboard/list controls. Arbitrary state editing and drag-based adjacency are excluded.

UI/API/data: new floor view and scoped GET; explicit floor/configuration migrations and guarded fixtures. Existing login, session storage and queue issuance need no change. No call/seat/checkout behavior is claimed by the floor issue.

DoD: S2-AC-001/013, persisted fixture/restart checks, role/scope tests, 360px and 1280px keyboard/text-label checks, actual results and reviewed feature PR.

### F-14 matching, issue #51

SDS: a pure graph/ranking function enumerates valid connected sets; an authenticated read service supplies authoritative state. Preview for selected tables returns the server-selected oldest fitting Ready party and skip reasons. Party-based recommendations are advisory: selecting a set may identify an older fitting party, which must be shown and reconfirmed. No preview acquires a lease or updates data.

UI/API/data: matching drawer, one read-only preview endpoint and typed candidate/selection snapshots. Reuse #50 data; no new migration, credential or deployment mechanism is needed. Query parameters cannot supply capacity, readiness, rank or role as authority.

DoD: S2-AC-002/003/008/014, graph/ranking/FIFO RED→GREEN, no-side-effect and permission tests, empty/stale preview UI, and reviewed PR.

### F-07 and F-15 call hold and seat, issue #52

SDS: confirm exact selected party, set and versions; re-evaluate the selected-set queue policy under locks. Persist HELD/CALLED for a successful call. Seat before expiry changes all tables to OCCUPIED, assignment to ACTIVE and ticket to COMPLETED. Shared release/expiry logic belongs here and is reused by #53.

UI/API/data: Call & Hold and Seat confirmations, current assignment view, explicit assignments/members/events/action-record migration and SEATING guards. Existing auth/cookie/CSRF transports do not change. Production activation remains gated until #53/#54 integrate; isolated test fixtures may exercise SEATING earlier.

DoD: S2-AC-004/005/006/015/016/017, real PostgreSQL same-queue and cross-queue races, stale/version/idempotency and expiry boundary tests, injected rollback, restart persistence, preserved MANUAL regression and reviewed PR.

### F-08 cancellation and no show, issue #53

SDS: owner cancels WAITING regardless of readiness, including an inactive queue whose existing tickets can drain. CALLED cannot be customer-cancelled. Authorized Host marks only a held CALLED ticket NO_SHOW in SEATING mode and releases all members through #52's coordinator. Rejoining creates a new ticket after a terminal status, with a new daily number and Not ready default; never reactivate the old row.

UI/API/data: Customer Cancel and Host No-show confirmations and scoped POSTs. Reuse #52's terminal-status columns/events; no second expiry worker or assignment table. Existing owned detail remains readable after CANCELLED/NO_SHOW, active lists exclude them, and initial foreign detail still returns actual HTTP 404.

DoD: S2-AC-009/010/018, ownership/CSRF/state tests, cancel-versus-call and no-show-versus-seat races, persisted release and polling transitions, and reviewed PR.

### F-09 and F-16 group checkout and cleaning, issue #54

SDS: checkout an ACTIVE assignment atomically to CLOSED/DIRTY; Start Cleaning and Finish Cleaning operate on its complete original membership. Keep an explicit turnover phase so a historical CLOSED assignment cannot clean tables that have since been allocated again. Return group members to AVAILABLE only after cleaning; retain the historical association.

UI/API/data: floor assignment panel and three confirmed transitions; reuse #52's turnover fields/events and scoped Host authorization. No payment/POS or meal status on the Customer ticket. Deliver guarded activation tooling only after all required lifecycle handlers integrate.

DoD: S2-AC-007/011/012/019, rollback and repeated/old-assignment tests, whole-group browser journey and relevant S1/Lab 7 regression, reviewed PR and integrated deployment verification.

## 6 Acceptance criteria

IDs 001–012 are retained from version 2.0; their detailed expectations are versioned here. IDs 013–019 add explicit compatibility, lifecycle and evidence boundaries. See the STS for inputs, expected outcomes and levels, and the traceability matrix for requirements and every AC's current implementation/execution status.

| AC | Observable acceptance outcome | STS IDs |
|---|---|---|
| S2-AC-001 | Scoped floor shows correct inventory/states; denied actor cannot read private operating data | S2-T-001,002,033,034 |
| S2-AC-002 | Only active approved connected same-zone sets of 1–3 distinct tables qualify | S2-T-003,004,005,030 |
| S2-AC-003 | Ten guests can use 6+4; eight alone fails; ranking is deterministic | S2-T-006,007,008,035 |
| S2-AC-004 | Call+hold claims every member and CALLED ticket atomically | S2-T-009,010,015 |
| S2-AC-005 | Only valid unexpired HELD group can be seated atomically | S2-T-011,012,031,045 |
| S2-AC-006 | Same-queue/cross-queue races cannot overlap claims or leave partial writes | S2-T-013,014,015,046 |
| S2-AC-007 | Checkout/cleaning guards apply to the entire assignment | S2-T-016,017,047 |
| S2-AC-008 | Selected-set oldest fitting Ready policy and alternate-set rationale are enforced | S2-T-018,019,036,037,038 |
| S2-AC-009 | Only owner cancels WAITING; terminal row remains private/readable | S2-T-020,021,039,040 |
| S2-AC-010 | No-show releases all held members and cannot affect occupied groups | S2-T-022,023,031,041 |
| S2-AC-011 | Ten-person group completes checkout/cleaning and visible states persist | S2-T-024,025,032 |
| S2-AC-012 | Duplicate, direct-to-available and old-group mutations cannot free tables | S2-T-026,027,048 |
| S2-AC-013 | Keyboard/list floor, responsive states and explicit recovery work | S2-T-049,050 |
| S2-AC-014 | Preview is read-only, handles no match and cannot skip a fitting earlier Ready party | S2-T-036,038,051 |
| S2-AC-015 | Legacy queues/records survive additive migration; activation and old endpoints cannot bypass seating | S2-T-042,043,044,052 |
| S2-AC-016 | Successful keyed retries do not duplicate transitions; mismatched key payload conflicts | S2-T-053,054 |
| S2-AC-017 | Expiry at/after deadline ends in NO_SHOW/release on cleanup, exactly once | S2-T-012,031,045,055 |
| S2-AC-018 | New terminal states and writes integrate with Lab 7 polling, privacy and inactive-queue draining | S2-T-039,040,056,057 |
| S2-AC-019 | Every mutation retains access/Origin/CSRF checks, evidence and cross-feature regression | S2-T-028,029,033,046,047,058,059,060 |

## 7 Review and release evidence

Contract review records are NOT RECORDED on this draft. Add the actual PR, reviewer, reviewed SHA/date, and unresolved decision outcomes before the corresponding implementation. The user deferred feature ownership; assign an owner and independent reviewer before starting each branch. This is an open #49 acceptance item, not an invented assignment.

For important rules keep meaningful RED on the actual pre-change code, then GREEN with the same intended assertions. Compilation/import/setup failures do not establish RED. Record the contract version, command, actual file/test, time, source SHA, pass/fail/skip counts, CI URL and review chain. Planned filenames are not execution evidence.

Sprint acceptance requires all nineteen ACs implemented and verified, explicit unchanged or versioned S1/Lab 7 expectations, migrated historical records, clean-clone setup, selected browser journeys, and reviewed integration into main. Tag the accepted Sprint 2 release so later Sprint 3 work cannot replace its identity. Verify the deployed SHA/digests, real critical journey and persistence; disclose failures/skips. No new GCP resource or workflow change is authorized by this contract-only branch.

For Deliverable 2 embed complete contracts/AC/STS/DoD and current system documents in the team's self-contained PDF, plus two selected Sprint 2 demonstrations, the Sprint 1 continuity journey, two real automated test excerpts (domain and access), two substantive Issue/PR chains and actual release evidence. The separate Sprint 3 work still requires its contracts, a tested workflow and dated remaining-work plan.
