# Lab 7 shared contract — readiness-aware queue calling

Version: DQ-L7-CR44-v0.1, 25 November 2026 (Asia/Bangkok).
Issue: https://github.com/bbxndg/DineQueue/issues/44
Status: **CONTRACT REVIEWED on PR #45; readiness implemented in the local worktree;
polling implemented in the local worktree on feat/lab7-ticket-polling (uncommitted,
not yet reviewed)**. Local evidence: [readiness results](readiness-results.md),
[polling results](polling-results.md).
Original contract commit: `323d8e7c1796466fd6d3cea46044c72cd4ac20d4`;
review by softkoi: 26 Nov 2026 00:00:50 ICT (25 Nov 17:00:50 UTC).
These implementation notes do not replace the original reviewed contract.

This is the shared source of truth for both feature branches and their AI assistants.
Read this file and [STS](readiness-sts.md) before changing code. The contract was committed and reviewed before readiness implementation; do not backfill pre-implementation
review or RED evidence after coding.

## Assessment and versions

The instructor announcement supplied by the user on 25 November removes the
instructor/TA approval requirement for Lab 7 change requests. Team discussion and
teammate contract review remain required. One bounded, sufficiently substantial
change is assessed: readiness-aware calling; polling supports that workflow.

| Version | Value / evidence |
|---|---|
| Accepted Sprint 1 baseline from Lab 6 | PENDING team confirmation of accepted SHA and UAT/re-test evidence |
| Actual pre-change local main | `1524502386065be2eb6025c04bae72a5398c7228` (includes #41 ticket-page HTTP 404 fix via #43) |
| Contract commit and teammate review | `323d8e7`; softkoi APPROVED [PR #45](https://github.com/bbxndg/DineQueue/pull/45), merged into staging at `8539b24` |
| RED / GREEN / integrated release / deployed revision | Local worktree results linked above; feature integration/release/deployment PENDING |

A clean local checkout of main is a source baseline, not proof of accepted UAT or
current cloud deployment. Later main fixes must be integrated deliberately and
recorded; preserve the tested pre-change SHA.

## Rule change and decisions to review

Old S1: F-04/FR-007/BR-003 calls the oldest WAITING ticket, ordered by joined_at,id.
F-03/FR-005 and F-04 views update through manual Refresh.

Proposed revision `BR-003-L7-v1`: eligible means WAITING **and** isReady=true;
choose the oldest eligible ticket by original joined_at,id. Readiness is a separate
boolean, not a new ticket status. Lifecycle remains WAITING -> CALLED -> COMPLETED.

1. Owner may set Ready or Not ready only while WAITING, including inactive queues
   whose existing tickets may still be drained. Host/other Customers cannot do it.
2. No change to ticket code, joined_at, party size, daily counter or ownership.
3. Returning to Ready retains original order for the next Call Next; it never
   displaces an already CALLED party. A not-ready party can wait indefinitely;
   explain that it may be skipped and provide no wait-time guarantee.
4. Keep BR-005: at most one CALLED ticket per queue. Existing CALLED takes priority
   over empty/no-ready error checks. Complete semantics remain manual S1 completion.
5. **Reviewed team decision (PR #45):** new tickets start Not ready. Join UI explicitly tells
   Customers to confirm Ready. Existing rows are backfilled Ready so migration
   does not silently remove existing waiting parties from eligibility. Lifecycle
   guards still make readiness uneditable for existing CALLED/COMPLETED rows.
6. A repeated set to the current value while WAITING returns 200 without a second
   version increment. This is a set operation, never a server-side toggle.

Items 1–6 and the API below were reviewed in PR #45. Implementation PR review
and release acceptance are still pending.
Future S2 matching must filter readiness alongside capacity/availability and recheck
it inside call+hold transactions. Implementing S2 matching/holds is outside #44.

## Shared HTTP/JSON interface (reviewed in PR #45)

Preserve existing envelopes and fields. Add required JSON boolean `isReady` to
Customer Entry/Ticket and HostEntry DTOs, including join and transition responses.
Example addition: `{ "status": "WAITING", "isReady": false }`.
Never rename it independently to `ready`, introduce a readiness lifecycle status,
or return owner identity/session material.

| Request | Result |
|---|---|
| PATCH `/api/v1/me/queue-entries/{entryId}/readiness` with JSON `{ "isReady": true }` or false | 200 full Customer Ticket DTO with committed isReady |
| GET `/api/v1/me/queue-entries/active` | existing `{entries:[Ticket]}`, with isReady |
| GET `/api/v1/me/queue-entries/{entryId}` | existing Ticket, with isReady |
| GET `/api/v1/host/queues/{queueId}/entries?status=WAITING` | existing `{entries:[HostEntry]}`; include **all** waiting parties and readiness, original order |
| GET `/api/v1/host/queues/{queueId}/current` | existing `{entry:HostEntry|null}`, with isReady |
| POST `/api/v1/host/queues/{queueId}/call-next` | existing success; select only ready WAITING parties |

Readiness PATCH requires existing session, CUSTOMER role, exact Origin and CSRF
header. Strict JSON accepts exactly one non-null boolean isReady; reject missing,
null, wrong type, unknown fields and malformed bodies as 400 VALIDATION_ERROR.
Guest 401; Host 403 FORBIDDEN; malformed/missing/foreign entry ID 404 NOT_FOUND
with the same privacy-preserving shape; owned non-WAITING entry 409
INVALID_TRANSITION. Keep existing no-store and error.requestId conventions.

Call Next: 409 CALL_IN_PROGRESS if a called party exists; otherwise 409 QUEUE_EMPTY
if no WAITING tickets; otherwise 409 NO_READY_PARTIES if all WAITING are not ready.
No conflict changes any ticket. Keep existing role, grant and CSRF enforcement.

Polling GETs are read-only; mutations are never triggered by a timer. Browser uses
same-origin `/api/v1/...`, existing HttpOnly cookie and credentials:'same-origin',
cache:'no-store'. Never embed API_INTERNAL_URL or secrets in client code. Readiness
branch owns the new exact Next rewrite for PATCH; polling uses existing GET routes.

## Persistence, transactions and concurrency

Readiness branch adds a new numbered SQL migration; never edit applied 0001/0002.
Use `queue_entries.is_ready BOOLEAN NOT NULL`: backfill existing rows true and set
the default for subsequent inserts false. Check actual migration numbering on the
implementation branch. Keep existing data and constraints; no seed rerun or reset.
Follow existing migration runner and explicit projections (GORM does not AutoMigrate).

Both readiness updates and Call Next acquire the service queue row FOR UPDATE
before locking/rechecking the entry. An owner-scoped lookup may locate queue ID,
but recheck ownership/status inside the transaction. Follow queue -> entry lock
order also used by Complete. Conditional calling must require WAITING AND is_ready;
check affected rows. Successful readiness changes increment the existing version.

If Not ready commits first, subsequent Call Next skips that ticket. If calling
commits first, readiness update returns 409 and the UI fetches current state.
Two Hosts must not create two CALLED tickets. Do not split a transaction across
service calls or treat a client-side Ready badge as authoritative.

## UI and polling lifecycle

Readiness: show explicit readiness text and labelled owner controls on WAITING
cards in `/my-queue` and owned detail. Disable while a mutation is pending; use
confirmed response, not an optimistic authoritative status. Readiness PATCH has
a 15-second client timeout; timeout does not prove the write failed, so show the
existing uncertainty message and let the Customer refresh before retrying. On 409 refetch state
and explain the ticket was changed. Non-WAITING has no editable readiness control.
Host waiting list shows readiness, keeps skipped parties visible, and explains
NO_READY_PARTIES. Disable Call Next if none eligible, but backend still decides.

Polling: Customer active list, owned detail, and selected Host queue/current card
refresh every 5 seconds **after the previous request settles** while visible.
Fetch on returning to visibility; no catch-up burst. One in-flight refresh per
view; abort/invalidate on unmount, queue change or mutation. Ignore outdated
responses. Coordinate mutation/refresh so pre-mutation snapshots cannot overwrite
a committed result; suspend while writes are pending and refetch afterward.

Host waiting/current reads are separate existing endpoints: they are not an atomic
snapshot. Reconcile duplicate IDs (CALLED wins), retain backend authority, and
never enable an unsafe mutation based solely on a potentially stale view.

Preserve focus, form input and Complete confirmation only for the same current
entry ID. Clear confirmation if current entry changes. Retain manual Refresh
through the same refresh coordinator, avoiding a second timer/request stream.
Show last successful refresh and a stale/error notice on failure, not a fabricated
empty list; retry on the next interval. On 401 or forbidden scope, clear protected
state and redirect/show the existing access flow. A detail becoming 404 clears
its data and shows not-found. COMPLETED leaves active list but remains owned-readable.

Keep initial server-side auth and detail 404 handling. Issue #41 fix deliberately
keeps ticket detail outside loading/Suspense boundaries: polling must not reintroduce
HTTP 200 for initial foreign/missing detail routes. Existing `server-only` read
helpers cannot be imported into client components; split types/read-only client
fetch helpers as needed, without weakening server entry checks.

The five-second interval is not a guaranteed five-second wall-clock SLA: latency,
hidden tabs and failures add delay. Tests assert scheduling and successful updates.
Polling may keep Cloud Run warm and increase requests while pages are visible;
stop unused tabs during scale-to-zero evidence collection. No cloud resource,
workflow, Docker or infrastructure changes are needed for this contract.

## Ownership and integration

| Branch / owner | Primary responsibility |
|---|---|
| `docs/lab7-engineering-contract` / both | shared engineering contract and STS, integrated into staging before feature implementation |
| `feat/lab7-customer-readiness` / user | SQL migration, Go DTO/repository/service/HTTP, readiness PATCH rewrite, readiness controls/badges, call-next filtering/error, tests |
| `feat/lab7-ticket-polling` / teammate | polling coordinator/client reads, automatic Customer/Host updates, lifecycle/error/stale-response tests |
| `lab7-staging` / both | shared contract, integration, regression, release evidence |

Shared conflict hotspots: `ticket-card.tsx`, `host-board.tsx`, Customer pages,
`lib/tickets.ts`, `lib/host.ts`, e2e fixtures. Agree shared DTO first. Prefer additive
components/helper files; integrate readiness before final polling wiring. Polling
may build isolated tests against documented synthetic DTOs, but cannot claim real
integration until readiness API exists. Missing isReady must not be interpreted
as true in the integrated app.

Current source map:
- `apps/api/internal/queue/{http,service,repository}.go` — Customer routes/reads/join.
- `apps/api/internal/queue/host_{http,service,repository}.go` — scoped Host operations.
- `apps/api/db/migrations/` — immutable numbered SQL migrations.
- `apps/web/src/{components,lib}/` — ticket-card, host-board, refresh-ticket,
  server-only tickets/host helpers and auth controls.
- `apps/web/src/app/my-queue/(list)/page.tsx`, `[entryId]/page.tsx` and
  `apps/web/src/app/(main)/host/queue/page.tsx` — authenticated initial views.

## Instructions for teammate's AI

Read README, AGENTS, SRS, domain rules, this contract and STS. Work on
`feat/lab7-ticket-polling` branched from `lab7-staging` **after the readiness feature PR is reviewed and
merged**, per the user's integration sequence agreed on 26 November. Pull staging
first so the readiness API and UI are available; do not branch from old main. Do not create a second readiness API/migration or independently
change FIFO/defaults. Both PR descriptions use `Part of #44`; neither closes the
issue before integrated acceptance. Both teammates review each other's work. The documentation branch is integrated
first; merging it is not evidence of teammate approval. Record the actual contract
review before RED/implementation, and branch/sync both feature branches from that
staging version.

Before code: teammate review and commit the contract/STS, then keep meaningful
failing tests against actual pre-change behavior. Compile errors, missing imports
or deliberately broken fixtures are not valid RED evidence. Existing strict-FIFO
tests legitimately need versioned Ready fixtures; retain ordering/permission and
concurrency assertions rather than deleting coverage. Record actual commands,
SHAs, dates, pass/fail/skip counts and CI URLs. This contract alone does not establish PASS; actual dated results are linked above.

Final release: reviewed feature PRs -> lab7-staging -> regression -> reviewed main
PR -> pipeline deploy -> changed/preserved behavior checks. Capture all versions
and evidence for Parts A–D of the self-contained Lab 7 PDF; include five verifiable
metrics, <=100-word action, readiness/defect/debt status, contributions and AI use.
