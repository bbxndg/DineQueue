# F-03 implementation contract — My Ticket and Privacy

Date: 10 November 2026. User authorized implementation on
`feat/F-03-my-ticket-privacy`, base `21d13da`. The committed F-03 baseline is in
`sprint1-contracts.md`; this addendum was written in the worktree before changes.
The user subsequently authorized committing it after implementation. This does
not establish an Issue, a pre-implementation addendum commit or TA approval.

## Behavior, API and data

- Customer-only `GET /api/v1/me/queue-entries/active` returns `{entries:[]}` or
  the owner's WAITING/CALLED tickets across queues, ordered by `(joined_at,id)`.
  Do not filter by today's date or whether the queue still accepts new joins.
- Customer-only `GET /api/v1/me/queue-entries/{entryId}` returns one owned ticket,
  including COMPLETED. Unknown, malformed and foreign IDs return the same
  404 `NOT_FOUND` shape. Guest: 401; Host: 403 on both endpoints.
- The service checks CUSTOMER independently of HTTP middleware. Repository
  queries bind both the authenticated owner ID and ticket ID for detail reads.
- Read DTO extends the existing entry summary with `queueName`, `restaurantName`
  and `timezone` for unambiguous queue/time labels, including inactive queues.
  `serviceDate` is YYYY-MM-DD; `joinedAt` is a UTC instant. Do not return customer
  identity, email, raw sequence, session material or staff audit fields.
- Reads are no-store, use bounded existing request contexts and parameterized
  queries. Existing migrations suffice; no new state transitions or schema.

## UI

- Customer home links to `/my-queue`. Successful join redirects there; duplicate
  join feedback links there. Preserve the existing join validation/CSRF controls.
- `/my-queue` lists owned active tickets; empty state offers Join Waitlist.
  `/my-queue/{entryId}` supports a bookmarked owned detail, including COMPLETED;
  inaccessible details use a generic not-found page. No history listing in S1.
- Cards show ticket code, queue/restaurant, party size, service date, joined time
  with explicit restaurant timezone and text status. WAITING: stay nearby;
  CALLED: go to the counter; COMPLETED: waitlist service finished.
- Explicit Refresh status fetches current persisted state. No polling/ETA/rank,
  public display, cancellation or Host mutations. Sign out, loading, retryable
  error and role/session redirects remain available. Never store tickets in
  browser persistent storage or treat an API failure as an empty ticket list.

## Acceptance and STS

| AC / tests | Observable acceptance |
|---|---|
| AC-011; T-010,T-018,T-019 | Join redirects to My Ticket; active and detail values equal DB values; reload preserves the ticket |
| AC-012; T-020 | Empty customer list displays Join action, no other customer's ticket |
| AC-005/013; T-008,T-021 | Foreign/missing/malformed detail 404 with no ticket fields; guest 401; Host 403; own list excludes other customers |
| AC-011; T-035 | Ticket remains readable after actual API process restart without resetting DB |
| F-03 regression | CALLED remains active; COMPLETED is excluded from active but readable by owner; queue inactivity/day boundary does not hide active tickets; service role checks |
| UI resilience | Unavailable API shows error/retry; no fabricated empty state; protected pages redirect expired sessions and Hosts |

Important read/privacy tests run against isolated real PostgreSQL; add fail-first
coverage for missing role guards and read formatting. Browser tests use isolated
seed accounts. Stored-state/error tests also require psql and TEST_DATABASE_URL
for the same disposable seeded API database; they clean up a generated customer
and restore the ticket table after a controlled read failure. Run one worker.
CALLED/COMPLETED fixtures may be written directly in test setup;
these are not evidence that F-04 transitions exist. Record actual commands and
results in `../testing/f03-my-ticket-results.md`, including failures and limits.

DoD: AC-011..013 plus real T-008 coverage, positive/negative API/DB and browser
checks, regression checks, synchronized API/traceability and evidence. Teammate
review, integration, CI and release remain separate workflow steps.
