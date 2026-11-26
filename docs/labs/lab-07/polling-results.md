# Lab 7 #44 — Automatic ticket/queue updates (polling) local results

Date: 26 November 2026 (Asia/Bangkok). Branch: `feat/lab7-ticket-polling`.
Status: **implemented and verified on the local worktree only — uncommitted, unpushed.**
No commit, PR, CI run, teammate review, release or cloud deployment is claimed.
Readiness results remain in [readiness-results.md](readiness-results.md) and are
historical; nothing here replaces them.

## Versions

| Record | Value |
|---|---|
| Base (branch created from `lab7-staging`) | `33ebad5010da16ef9ed9069fe2c4f461243cc7f4` (merge of readiness PR #46) |
| RED | base SHA + only the new test file `apps/web/e2e/polling.spec.ts`; [file SHA-256](evidence/polling-red-test.sha256) |
| GREEN and regression | base SHA + **uncommitted worktree**; [tested-source manifest](evidence/polling-source-manifest.sha256) |
| Committed-version verification | NOT RUN — repeat after the user authorizes commits (CI must run on the PR) |

The RED test file is byte-identical in the GREEN runs (same SHA-256).
Manifest hashes are of the Windows working tree (`core.autocrlf=true`): eight
pre-existing files were checked out with CRLF, so their committed LF blobs hash
differently; after removing `\r`, every committed file in the user-authorized
commits from `4758103` to the docs commit matches the tested bytes (checked 26 Nov 2026).
Lifecycle tests live in a separate file so the RED file did not change.

## Change implemented

- `lib/poll-coordinator.ts` (framework-free): one in-flight read per view; next
  read 5 s **after the previous one settles**; pause while hidden, one immediate
  read when visible again (no catch-up burst); 10 s read timeout; generation
  counter so superseded/aborted responses are never applied; `runMutation`
  suspends and invalidates reads during a write, then refetches once.
  401/403/404 stop polling and are reported as access loss.
- `lib/use-live-resource.ts`: React hook; one poller per mounted view, stopped
  (timers cleared, read aborted) on unmount or key change.
- `lib/live-reads.ts`: client same-origin GETs (`credentials: 'same-origin'`,
  `cache: 'no-store'`) on the four existing endpoints; malformed bodies are
  failures, never empty lists; Host waiting/current are separate reads
  reconciled so a CALLED ID is not also shown as WAITING (also used server-side).
- `lib/ticket-types.ts`: shared DTO types, so client code does not import the
  `server-only` helpers. `lib/tickets.ts` / `lib/host.ts` re-export them.
- `components/live-tickets.tsx`: `/my-queue` list and owned `/my-queue/{id}`
  detail. COMPLETED leaves the list; detail stays readable. 401 → existing
  `/login`; 403 → `/`; detail 404 clears data and shows "Ticket not found".
- `components/host-board.tsx`: polled waiting/current; Call Next and Complete go
  through `runMutation`; background polls do not disable controls; the
  Complete confirmation is keyed by ticket ID (kept for the same ID, reset when
  the current ticket changes).
- `components/refresh-ticket.tsx`, `readiness-control.tsx`: inside a polled view,
  manual Refresh and readiness PATCH use the same coordinator; outside it the
  old `router.refresh()` path is unchanged.
- `components/live-refresh.tsx`: "Last updated" time and a stale notice that
  keeps the last successful data visible.
- Pages still authenticate and read on the server first. The ticket-detail route
  still calls `notFound()` before rendering and remains outside every
  loading/Suspense boundary (#41); no new route boundary was added.
- Not changed: API, migrations, readiness rules, FIFO, ownership, transactions,
  Docker, CI, deployment, cloud resources, dependencies. No SSE/WebSocket/ETA.

## Actual checks

Environment: Windows 11, Node v22.23.4, npm 10.9.9. Disposable PostgreSQL
`postgres:18.6-bookworm` container `dq-l7poll-db` (127.0.0.1 only), migrated and
seeded with generated synthetic passwords (not recorded). API built from the
unchanged `apps/api` with Go 1.26.10 (`GOTOOLCHAIN=go1.26.10`), `APP_ENV=test`,
port 18087. Web: production `next build` + `next start` on port 3107 with
`API_INTERNAL_URL=http://127.0.0.1:18087`. Playwright used installed Chrome
(`F01_BROWSER_CHANNEL=chrome`). Fixture SQL used a local test-only `psql` shim
that forwards to `psql` inside the disposable container. This does not emulate
HTTPS/Cloud Run.

| ID / command | Result | Evidence |
|---|---|---|
| RED: `npm run test:e2e --workspace @dinequeue/web -- polling.spec.ts` on base | **3 failed** as expected: Customer stays `WAITING` 15 s after a committed call; Host never shows `Ready`; completed ticket never leaves the list | [RED](evidence/polling-red.txt) |
| GREEN: same command, same unchanged test file | **3 passed** (26.0 s) | [GREEN](evidence/polling-green.txt) |
| `npm test` (root) | **10 passed**, 0 failed (8 new coordinator + 2 existing) | [web checks](evidence/polling-web-checks.txt) |
| `npm run lint`, `npm run typecheck` | PASS | same |
| `API_INTERNAL_URL=http://127.0.0.1:18087 npm run build` | PASS | same |
| `git diff --check` | clean | same |
| Full browser: `npm run test:e2e --workspace @dinequeue/web` (clean DB, fresh API process) | **34 passed**, 0 failed, 0 skipped (4.8 min) | [full regression](evidence/polling-browser-regression.txt) |
| Go, golang:1.26.10-bookworm, separate DB `dq_go_test`: `go test -race ./... && go vet ./... && go test -tags=integration -race -count=1 -v ./...` | PASS: 66 top-level, 26 subtests, 0 failed, 0 skipped (`apps/api` unchanged) | [Go](evidence/polling-go-regression.txt) |

## Test mapping (STS)

| STS | Test |
|---|---|
| L7-T-19 | `polling.spec.ts` › L7-T-19a (Call Next → CALLED on list and detail), L7-T-19b (readiness and call reach Host), L7-T-19c (completion leaves list; detail COMPLETED) |
| L7-T-13, L7-T-15 | `polling-lifecycle.spec.ts` › schedule after settle; hidden = 0 reads for 12 s; visible = exactly one immediate read; client navigation stops the list poller |
| L7-T-15 | › Host queue change stops polling the previous queue |
| L7-T-14 | › slow read (8 s) + Call Next: max one browser request in flight, late pre-mutation snapshot ignored, manual Refresh shares coordinator; › readiness PATCH during a slow read keeps the committed value |
| L7-T-16 | › network abort / 503: stale notice, last ticket kept, no fake empty list, recovery (Customer and Host); › real 404 (ticket made foreign) clears detail; 403 → `/`; real session deletion (401) → `/login`; Host grant removed → leaves queue |
| L7-T-17 | › same-ID confirmation and focus kept across polls; new current ID resets confirmation; Customer readiness-button focus kept |
| L7-T-18 (#41) | existing `my-ticket.spec.ts` foreign/missing/malformed `goto` status 404 — passed in the full run |
| Coordinator units | `tests/poll-coordinator.test.mjs` (fake timers): no overlap, shared manual refresh, mutation invalidation, late stale response, hidden/visible, stop without leaked timer, timeout recovery, failure vs access loss |

## Existing tests changed and why

- `ticket-states.spec.ts`, `sprint1-review.spec.ts` (Host read failure): a failed
  **manual refresh** used to replace the page with the error boundary. The
  reviewed contract now requires keeping the last successful state with a stale
  notice. Only those assertions changed; "never show a fake empty list" and the
  recovery checks remain. Initial-load failures still use the error boundary.
- `readiness.spec.ts`: wait for the detail heading before clicking Mark ready.
  The click previously could land on the list view that was being replaced;
  `router.refresh()` used to mask that race.

## Failed attempts (kept for honesty)

- First RED attempt was invalid: `next start` lacked `API_INTERNAL_URL`, so pages
  showed the error boundary. Restarted correctly; not used as RED evidence.
- Coordinator unit test found a real leak: the 10 s timeout timer stayed pending
  after `stop()`. Fixed by clearing it on invalidation.
- First lifecycle run: 5 passed, 2 failed — default 30 s test timeout for a test
  that waits ~40 s by design, and an in-flight counter that counted route-handler
  activity instead of browser requests. [Log](evidence/polling-lifecycle-first-attempt.txt)
- First full run: 30 passed, 4 failed — the expected-behavior changes above and
  a second `<time>` element from the new "Last updated" label (now plain text).
  [Log](evidence/polling-browser-first-regression.txt)
- Two back-to-back full runs then failed `join.spec.ts` on login rate limiting
  (10 attempts/IP/minute in memory; all local traffic is 127.0.0.1 via the Next
  proxy). `auth.spec.ts` + `join.spec.ts` pass once the window clears
  ([check](evidence/polling-auth-join-limiter-check.txt)); the final run used a
  clean DB and a fresh API process. Login code was not touched.

## Limitations

- Local verification only; committed-version, CI and deployed checks are pending.
- Five seconds is a scheduling interval, not a wall-clock SLA; latency, hidden
  tabs and failures add delay. Polling adds requests while pages are visible
  and can keep Cloud Run warm.
- Host waiting/current are two reads, not an atomic snapshot; duplicates are
  reconciled, and the backend still decides every mutation.
- Hidden tabs are simulated by overriding `document.visibilityState` in tests.
- A write started in a view that is unmounted mid-request is reflected by the
  next view on its next scheduled read.

## Manual check (Customer + Host)

1. Customer: open `/my-queue` (and a ticket's details in another tab).
2. Host (other browser/profile): open `/host/queue`.
3. Customer clicks Mark ready → Host row shows Ready within ~5–10 s, no refresh.
4. Host clicks Call next → Customer list and details show CALLED within ~5–10 s.
5. Host Complete → Confirm → Customer list shows "You do not have an active
   ticket."; details show COMPLETED.
6. Switch the Customer tab away for a while and back: the view updates at once.
