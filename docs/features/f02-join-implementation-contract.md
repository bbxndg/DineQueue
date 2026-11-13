# F-02 implementation addendum — 6 November 2026

User authorization: implement F-02 on `feat/F-02-join-waitlist-ticket`. Base: `4ff1b32`.
Written before code in the worktree. The user commits it; no pre-implementation
addendum commit, Issue, teammate review or TA approval is claimed by this file.
Parent contract: [`sprint1-contracts.md`](sprint1-contracts.md) F-02 (AC-006..010).

## Behavior

- `GET /api/v1/queues` (any authenticated role): active service queues only, as
  `{queues:[{id,name,restaurantName}]}`. No private or customer data.
- `POST /api/v1/queues/{queueId}/entries` (Customer only): Authenticate → exact
  Origin + X-CSRF-Token → role CUSTOMER. Body is strictly `{"partySize": <int>}`:
  JSON media type, at most 1 KiB, no unknown/duplicate trailing values, and the raw
  number must be a plain decimal integer (rejects `1.5`, `4.0`, `1e1`, `"4"`, `null`,
  missing) in [1,16] (BR-008 v2). Client-supplied number/status/date are not accepted.
- One service transaction (service-owned `db.Transaction`, same `tx` for every
  repository call):
  1. Load queue + restaurant timezone `FOR SHARE` of the queue row. Unknown/malformed
     ID → 404 `NOT_FOUND`; inactive → 409 `QUEUE_INACTIVE`.
  2. `service_date` = server clock in the restaurant IANA timezone (BR-009).
  3. Allocate: `INSERT … ON CONFLICT (queue_id, service_date) DO UPDATE SET
     last_number = last_number + 1 RETURNING last_number`; the counter row lock
     serializes concurrent joins for that queue/day (BR-002).
  4. Insert the `WAITING` entry. Partial unique index
     `uq_one_active_ticket_per_customer_queue` violation → 409 `ACTIVE_QUEUE_EXISTS`
     (BR-001); the whole transaction, including the counter increment, rolls back.
- 201 response: `{id,queueId,ticketCode,serviceDate,partySize,status,joinedAt}`;
  `ticketCode` = queue prefix + `-` + three-digit minimum sequence (e.g. `A-001`).
- Service re-checks role CUSTOMER (defense-in-depth beyond the HTTP gate).
- Errors: 400 `VALIDATION_ERROR`, 401 `UNAUTHENTICATED`, 403 `FORBIDDEN` /
  `CSRF_REJECTED`, 404 `NOT_FOUND`, 409 `QUEUE_INACTIVE` / `ACTIVE_QUEUE_EXISTS`.

## Data — migration `0002_queue_entries.sql`

`queue_day_counters` (PK queue/date, `last_number >= 0`) and `queue_entries` with
FKs, `chk_party_size` 1..16, status WAITING/CALLED/COMPLETED, timestamp/actor
consistency CHECK per status, `uq_queue_day_number`,
`uq_one_active_ticket_per_customer_queue`, `uq_one_called_per_queue` (used by F-04)
and `idx_queue_fifo`. Applied by the existing opt-in migration CLI only.

## UI

Customer home links to `/join`. `/join` (Customer only; Host redirected): queue
select (read-only label when only one), party size 1–16 integer control, combined-
table notice for 9+ ("not guaranteed"), loading state that blocks double submit,
banners for 400/401/409. On success the page shows the created ticket summary.
`/my-queue` belongs to F-03; F-03 will switch the success state to a redirect.

## Tests (STS mapping)

| Test | Name | Covers |
|---|---|---|
| T-011 unit | `TestParsePartySize` | raw-number grammar and 1..16 range |
| T-009, T-036 | `TestF02JoinCreatesWaitingTicket` | 201 `A-001`, persisted row, 10 accepted |
| T-011, T-012 | `TestF02InvalidPartySizeCreatesNothing` | 0,17,1.5,string,missing,extra field → 400, no rows/counter |
| T-013 | `TestF02DuplicateSequentialJoinConflicts` | second 409, one active row, no counter gap |
| T-014 | `TestF02ConcurrentDuplicateJoinCreatesOne` | 10 parallel → one 201, nine 409 |
| T-015 | `TestF02ConcurrentCustomersGetUniqueNumbers` | 20 parallel → 1..20 unique |
| T-016 | `TestF02UnknownAndInactiveQueue` | 404 / 409, no rows |
| T-005, T-017 | `TestF02RoleSessionAndCSRFGates` | guest 401, Host 403, missing CSRF 403 |
| T-034 | `TestF02ServiceDateUsesRestaurantTimezone` | Bangkok 23:59 vs 00:00 → new day restarts at 1 |
| T-010 | `e2e/join.spec.ts` | browser join, validation and duplicate banner |
