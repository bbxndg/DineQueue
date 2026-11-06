# UI/UX Interaction Specification v2 — Operational Product, Not a Generic Dashboard

**Status:** design proposed. **Primary platforms:** Customer 360–430px mobile, Host 1024–1440px desktop/tablet. **Visual direction:** warm restaurant editorial character, ink typography, wine-red accent, sage/amber status colors and deliberate spacing; no excessive gradients, glassmorphism, animated decorations or fake metrics.

## 1. Design tokens and accessibility

| Token | Example | Usage |
|---|---|---|
| `--brand-700` | `#542B31` | Primary CTA, selected navigation |
| `--ink` | `#252725` | Text |
| `--background` | `#F6F5F2` | Page background |
| `--surface` | `#FFFFFF` | Cards/panels |
| `--border` | `#E2E1DC` | Dividers |
| `--success` | `#668F71` | AVAILABLE; always with text |
| `--warning` | `#C28A3B` | WAITING / HELD; always with text |
| `--danger` | `#A8433C` | Destructive/error; always with text |

Use locally installed/system fallback fonts when licensed assets are unavailable; never bundle fonts taken from unknown websites. Use 8px layout rhythm, minimum 44px touch target, semantic buttons, focus ring, readable labels, aria-live feedback for queue updates, WCAG contrast checks. Avoid exposing guest names in operations/public display unless a documented privacy need exists.

## 2. Route and screen inventory

| ID | Route / Surface | Actor | Sprint | Primary task |
|---|---|---|---|---|
| UI-01 | `/login` | both | 1 | Sign in securely, handle invalid/expired session |
| UI-02 | `/` | customer | 1 | Restaurant status / Join / My Ticket |
| UI-03 | `/join` | customer | 1 | Choose party size **1–16**, review constraints, join |
| UI-04 | `/my-queue` | customer | 1–3 | Ticket/position/status; cancel later |
| UI-05 | `/host/queue` | host | 1–2 | FIFO queue; CALLED party/action panel |
| UI-06 | `/host/floor` | host | 2 | Interactive zone/table map with accessible list fallback |
| UI-07 | `Match Drawer` | host | 2 | Ranked single/connected table sets and explanations |
| UI-08 | `Call & Hold Dialog` | host | 2 | Confirm exact ticket/table IDs and hold time |
| UI-09 | `Seating Detail` | host | 2 | Seat → checkout → cleaning lifecycle |
| UI-10 | `/host/activity` | host | 3 | Audited operational timeline |
| UI-11 | `/host/analytics` | host | 3 | Date/zone metrics with definitions and uncertainty |
| UI-12 | `/host/settings/floor` | manager | 3 optional | Configure approved adjacency and positions |
| UI-13 | `/public/now-serving` | anonymous | 3 optional | Non-private ticket codes only |

## 3. Customer design details

**Home:** calm restaurant photo/banner only if owned/licensed; open/closed status from backend; one dominant Join CTA and one secondary View Ticket. No unsupported 'open now' or made-up time claims.

**Join form:** large numeric stepper + direct numeric input 1–16, optional party notes (length limited and private), privacy line, callout for 9–16 guests: “Large groups may be seated across adjacent tables; confirmation depends on availability.” Before POST show queue/service and size; block duplicate submits visually, still enforce in DB. Invalid 0/17 gets explicit inline message.

**My Ticket:** large `A-014`, `WAITING/CALLED/COMPLETED/CANCELLED/NO_SHOW` text + icon, party size, timestamp, now-serving ticket if publicly safe, refresh time. Do not promise wait time. In S3 an estimate may be shown as a range with sample-size caveat or `Unavailable`. Cancel only while WAITING. If another login occurs, wipe cached protected ticket data.

## 4. Host operations workspace — detailed anatomy

**Layout:** persistent narrow navigation → queue list (left ~32%) → selected party/action panel (center ~33%) → live floor plan/right details (~35%) on desktop. On 1024px, floor collapses behind a tab. On mobile Host UI, show read-only operational overview and one task at a time, never microscopic 3-column controls.

**Queue list:** table columns Number, Party, Waited, Status, Joined; own-queue only, stable order. Filters are server-backed; do not hide higher priority or change allocation order silently. Selected ticket visually anchored and keyboard navigable.

**Floor:** zone selector with table cards/visual positions (T4 `6 seats · AVAILABLE`), explicit connection lines/markers from `table_connections`, status legend with words. A connected set selection draws border around all chosen tables; detached/cross-zone set shows reason, not just disabled button. Never mutate layout from dragging a card in normal Host operation.

**Match drawer:** ticket size, list of candidate sets, total seats, unused seats, number of tables, reason `2 adjacent tables required`, timestamp/version of snapshot, note that availability can change. Use `Recommend` as non-binding read; show `Call & Hold` as explicit confirmation, not single click on recommendation.

**Call & Hold dialog:** `Queue A-014 — 10 guests`, tables T4(6) + T5(4), combined seats 10, zone Terrace, expiry policy, action `Confirm call & hold`. On 409: `One or more tables changed. Refresh options`, keep ticket visible, refresh snapshot and clear selection.

**Active Call panel:** queue status CALLED, selected held tables, hold-expiry relative text from server timestamp, buttons `Seat party` and `No-show`. When hold is stale/expired, show recovery and reload. When seated, ticket moves out of waitlist, tables OCCUPIED and Host sees checkout action in the floor.

**Checkout/Clean:** confirmation `Finish dining — release T4 + T5` changes them to DIRTY; next explicit whole-group cleaning step, then mark all members available together. No dangerous shortcut directly from OCCUPIED to AVAILABLE. All transitions show live result and history actor/time.

## 5. Required states for each action

| Surface | Loading | Empty | Success | 409/conflict | Permission/session | Offline |
|---|---|---|---|---|---|---|
| Join | blocking CTA + progress | no active queue | ticket persisted | already active, reload own ticket | login prompt | retry without duplicate ticket assumption |
| My ticket | skeleton preserving structure | Join CTA | status refreshed | stale ticket refresh | wipe protected view | last-updated label, do not show fake live |
| Host queue | list skeleton | no WAITING | called/complete confirmation | called by someone else, reload | 403/401 state | read-only retry |
| Floor | layout skeleton | no configured tables | status change acknowledged | table state changed, clear selection | 403 | show last refresh, disable mutations |
| Matching | spinner | no connected set | preview rationale | snapshot expired | forbidden | recompute when online |
| Seat | pending/action disabled | N/A | every table OCCUPIED | hold expired/stale | forbidden | do not assume success |
| Analytics | trend skeleton | no measured data | metrics + source window | stale filter refresh | forbidden | last cached period marked clearly |

## 6. UI acceptance scenarios

- Customer selects 10 people → warning about adjacent tables → ticket successfully issued in S1; 17 is invalid.
- Staff on Terrace sees T4+T5 as a *connected pair*, not T4+T7. Status labels distinguish HELD/OCCUPIED/DIRTY/CLEANING.
- Staff previews then confirms Call & Hold; another staff claims T5 first → 409, current selection clears, no fabricated success toast.
- Keyboard-only Host can select zone/table group, open dialog, cancel, and perform appropriate operation; focus returns to the triggering control after dialog close.
- Analytics shows real data definitions: waiting = `called_at - joined_at`, occupancy = assignment window, not guessed times.
- Public queue view never includes user email, ID, phone, free-text notes or internal DB identifiers.

## 7. UI component candidates (Next.js)

`AppShell`, `RestaurantHeader`, `QueueTicket`, `PartySizeControl`, `QueueStatusBadge`, `WaitlistDataGrid`, `CurrentCallCard`, `FloorZoneTabs`, `FloorMap`, `TableTile`, `TableStatusLegend`, `ConnectedSetOverlay`, `MatchExplanation`, `CallAndHoldDialog`, `SeatingLifecyclePanel`, `ConfirmTransitionDialog`, `MetricDefinitionPopover`, `EmptyState`, `ErrorBoundary`, `AuthGuard` (UX only). Prefer server query validation and suspense/loading boundaries; keep API response types documented. No remote script is needed for the UI itself.
