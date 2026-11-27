# 05. UI/UX Specification

> Source baseline b4d1606 includes the Lab 7 owner readiness controls and automatic Customer/Host polling. The [S2 contract](features/sprint2-restaurant-operations-contracts.md) and [UI/API/data definitions](features/sprint2-ui-api-data.md) specify proposed extensions; S2 is not implemented by this draft.

**Purpose:** Specify observable screens and state for feature engineering. Visuals are a concept, **not an approved mandatory screen layout**. Customer is mobile-first; Host is desktop-first. The previous DineQueue mockup is a visual reference; this file is the normative functional UI draft.

## 1. UI principles

- Restaurant feel: warm burgundy (#6C1728), off-white (#FCF9F6), clear green/amber/red statuses; color **plus label**, never color alone.
- Touch-friendly customer actions; desktop table/list scanning for Host.
- Loading/empty/success/error messages for every API operation; disable Submit while mutation is pending.
- Never display other customers' names or private IDs in a public queue/status screen.
- All forms have associated labels and inline validation; actionable error messages rather than raw server stack traces.

## 2. Sitemap / user journeys

```mermaid
flowchart TD
  LOGIN[Login] --> CHOOSE{Role}
  CHOOSE -->|Customer| HOME[Customer Home]
  HOME --> JOIN[Join Waitlist]
  JOIN --> TICKET[My Ticket]
  CHOOSE -->|Host| BOARD[Host Queue Board]
  BOARD --> CALL[Call Next]
  CALL --> COMPLETE[Complete Called Ticket]
  BOARD --> TABLES[Table Management - S2]
  BOARD --> ANALYTICS[Analytics - S3]
```

### P-01 `/login` — Both roles, Sprint 1

Fields: Email, Password. Actions: Login. States: loading / invalid credentials / account disabled / success. The returned role determines landing route. No public sign-up required. Session persisted in HttpOnly cookie; logout control available on logged-in pages.

### P-02 `/` — Customer home, Sprint 1

Header: DineQueue logo, selected Restaurant/Queue name, My Queue, profile/logout. Primary card: **Join Waitlist**. If customer has active queue, primary action becomes **View My Queue**, and creating a second active entry is blocked both by UI and API. No estimated wait time promise in S1.

### P-03 `/join` — Join waitlist, Sprint 1

Inputs: Queue (seeded/readonly if only one); Party size control (1–16). Summary: queue name, walk-in notice. Confirm button. Client validates integer 1–16; backend independently validates. Success redirects to ticket. Error states: invalid party size (400), already active (409), queue closed (409/404), session expired (401).

### P-04 `/my-queue` — Own queue ticket, Sprint 1

Fields: large human-readable ticket number, party size, current status, created/joined timestamp, selected queue. Optionally show **now called ticket** from privacy-safe data; must not expose names. For status WAITING show a waiting badge; CALLED show prominent 'Please go to the counter'; COMPLETED show 'Your waitlist service has finished'. Sprint 2 adds customer Cancel if WAITING.

The maintained F-03 view includes Lab 7 visible-only polling and manual Refresh (no ETA), queue and
restaurant labels, joined time in the labelled restaurant timezone, and an empty
state with Join Waitlist. `/my-queue/{entryId}` is an owner-only detail bookmark,
including COMPLETED; inaccessible details show generic not-found feedback.
Successful join navigates to `/my-queue`; duplicate feedback offers View My Ticket.
Now-called public data and history listing remain outside F-03.

### P-05 `/host/queue` — Host dashboard, Sprint 1

- Queue selector restricted to authorized queues; Sprint 1 only one.
- Current called card (ticket number/party size/join time) with **Complete** button when CALLED.
- Waiting list sorted by server FIFO: ticket number, party size, joined time; no need to expose customer email.
- On narrow screens the waiting table keeps readable column spacing inside a
  labelled, keyboard-focusable horizontal scroll region; the document itself
  must not overflow horizontally.
- In MANUAL mode **Call Next** is enabled only if no current called ticket and a Ready WAITING entry exists. SEATING uses floor selection and Call & Hold; old bare call/complete cannot bypass seating.
- Important error: two Hosts racing to call next; losing request displays 'The queue has changed. Refreshing.' and refreshes state.
- Complete confirmation belongs to one entry ID. When refresh shows a different
  current ticket, discard the old confirmation and require a new Complete action.
- Retry after a server read failure re-fetches server data without requiring the
  Host to navigate away. A failed read must not be shown as an empty queue.
- Empty state 'No waiting parties'. Pending state blocks repeated button presses. Backend always enforces rules independently.

### P-06 `/host/floor` — Interactive Floor Plan, Sprint 2

Use `/host/floor?queueId=...` with granted queue selection, zones and interactive cards plus an equivalent keyboard/list view. Show capacity, approved edges and AVAILABLE/HELD/OCCUPIED/DIRTY/CLEANING/OUT_OF_SERVICE text. Host selects 1–3 tables first; preview identifies the oldest fitting Ready party for that set and explains skipped parties. A different set may select a different party and requires reconfirmation. Ranking is advisory, not a lease or arbitrary priority override.

Confirm Call & Hold with the exact party/set/versions, then Seat the unexpired HELD group. Floor assignment controls perform whole-group checkout, Start Cleaning and Finish Cleaning. An expired countdown disables Seat and offers server cleanup; browser timers do not mutate state. The detailed response/conflict/access/loading rules are in the S2 interface contract. Retain Lab 7's single refresh coordinator, stale notices, write invalidation and initial owner-detail HTTP404. Customer cancellation and CANCELLED/NO_SHOW detail states are specified in that same contract.

### P-07 `/host/history` and `/host/analytics` — Sprint 3

Filter by date and queue. Display completed/cancelled/no-show counts and measured wait duration. Avoid ungrounded 'live wait estimate'. Clearly separate calculated metrics from optional predictions.

### P-08 `/public/now-serving` — Optional Sprint 3

Shows ticket code and counter only; does not show user names or email. If this feature is deferred, document omission explicitly in Sprint 3 completion status.

## 3. UI controls by sprint

| Control | S1 | S2 | S3 |
|---|---|---|---|
| Customer Login/Logout | ✓ | retain | retain |
| Customer Join with party size | ✓ | retain | retain |
| Customer My Ticket status | ✓ | retain | retain |
| Customer Cancel | — | ✓ | retain |
| Host Call Next/Complete | ✓ | revise for table allocation | retain |
| Host interactive floor/matching/combined assignment | — | ✓ | retain |
| History/analytics | — | — | ✓ |
| Public privacy-limited screen | — | — | optional |

## 4. Critical UI acceptance cases

- Clicking Join with party size 0/17 shows validation and does not create a ticket; choosing 10 is accepted in S1, with a non-guaranteed combined-table seating notice.
- Login as Customer never exposes Host actions; calling host API directly must be 403.
- Loading state prevents UI double-submit, **but database constraints** guarantee correctness even if requests race.
- Host Dashboard refresh after a completed ticket shows correct next waiting entry.
- On expired session, show login state without leaking old user ticket details.
- Screen supports keyboard focus order, visible focus, semantic buttons, and readable text labels.

## 5. Provisional wireframe sketch

```text
CUSTOMER                           HOST
┌────────────────────────┐         ┌─────────────────────────────────┐
│ DineQueue       Profile│         │ DineQueue / Host Queue          │
│ Welcome                │         │ Queue: Walk-in Dining          │
│ [Join Waitlist]        │         │ Current: A-013 [Complete]      │
│ [My Queue]             │         │ Waiting:                        │
└────────────────────────┘         │ A-014 | 4 guests | 12:05        │
┌────────────────────────┐         │ A-015 | 2 guests | 12:07        │
│ Your ticket: A-014     │         │ [Call Next]                     │
│ Party size: 4          │         └─────────────────────────────────┘
│ Status: WAITING        │
└────────────────────────┘
```

These are design expectations only; design handoff should use the previously generated visual mockup if desired, and screen behavior remains driven by ACs rather than pixel matching.


## 6. Expanded UI / screen coverage (v2)

Detailed states, routes, dialogs, accessible floor fallback, semantics of each action, typography/color tokens, privacy review and conflict scenarios are in `docs/ui/operations-interaction-spec.md`. Minimum S2 Host workspace: queue rail → selected party detail → live floor surface, with side drawer `Match Options` and explicit confirmation. `Call & Hold` is **not** equivalent to `Seat`; `Checkout` is **not** equivalent to `Mark Available`. Customer ticket status persists across refresh and uses server state.

**Prior mockups:** illustrative only. Where earlier design depicts only one table per assignment or permits immediate AVAILABLE after checkout, this v2 spec supersedes it. Visual sample values are never proof of live API data.

## Sprint 3 proposed screens (#55)

[The S3 UI/API/data contract](features/sprint3-ui-api-data.md) refines P-07/P-08 and adds Customer `/my-queue/history`, owned position/estimate text, and configuration-granted `/host/floor/setup?restaurantId=...`. P-08 becomes `/public/now-serving/{slug}` with explicit enablement and ticket-code/status-only responses. F-12 remains in the proposed retained scope; there is no approved deferral.

Floor setup uses accessible typed forms/lists and an explicit connection picker; no automatic adjacency or mandatory drag-and-drop. Save Layout and Enable Seating are separate confirmations. Every protected screen handles loading/empty/retry/expiry/denial, preserves safe stale data only with a notice, and ignores superseded requests. The contract specifies conflict reconciliation and exact permissions. New screens remain planned until actual implementation and browser evidence are recorded.
