# Sprint 1 review fixes — local execution evidence, 13 November 2026

Branch: `fix/sprint1-review`. Base HEAD:
`d7f27a325b95fdb437a851e5e76947f74ea813e7` (merged F-04 integration baseline).
The user authorized correcting the Sprint 1 review findings. These results cover
that base **plus the uncommitted fix worktree**, not a later commit, main merge,
CI run, teammate approval or release. Existing F/FR/BR/AC/T identifiers are retained.
The final completion checkpoint below supersedes the initial counts and source fingerprint.

The initial review code/test snapshot used for GREEN has SHA-256:
`cf68f2d4ed91d291d067aba31b6a627702adc25fcfd7c6f2b6e2a31fc0e9f271`.
It hashes sorted relative paths and bytes of files under `apps/api/internal`,
`apps/api/db`, `apps/api/cmd`, `apps/web/src`, `apps/web/e2e`, `apps/web/config`
and `apps/web/tests`. Documentation and the pre-existing generated
`apps/web/next-env.d.ts` change are excluded from this source fingerprint.

## Changes and demonstrated failures

| Finding / regression | Before production fixes | Final behavior |
|---|---|---|
| Queue ID validation | URN queue IDs returned 500 on Join and four protected Host routes; compact UUID allowed Join | Noncanonical queue IDs return 404 before SQL; no ticket/counter writes |
| Complete confirmation | With two real Host contexts, confirmation remained visible after another Host completed A and called B | Confirmation component is keyed by entry ID and submits that selected ID; B needs a fresh confirmation |
| Customer home | After a persisted join, View My Queue was absent | WAITING/CALLED shows View My Queue; COMPLETED returns the Join Waitlist action |
| Host read-error Retry | After restoring the temporarily renamed ticket table, Retry did not restore the board | Retry refreshes server data and resets the boundary; same-route board recovers |
| FIFO/inactive queue | New coverage, already correct before fixes | Equal timestamps use ID order, previous-day entries stay eligible, inactive queue can be drained |
| Mixed concurrent transitions | New coverage, already correct before fixes | Complete and Call Next serialize; an early call conflicts or the next ticket is called once |
| SQL failure rollback | New coverage, already correct before fixes | Trigger-injected update errors preserve status, version and audit metadata; retry after removing trigger succeeds |

RED Go command used the new four `TestS1*` tests: one validation test failed and
three other regression tests passed. RED browser run selected the three
`Sprint 1 review` cases; all three failed on the original UI for the expected
assertions. This is recorded worktree fail-first evidence, not a RED commit.

## Environment and isolation

- Linux amd64; Go 1.26.10, Node 22.17.0, PostgreSQL 18.1.
- Existing pinned dependencies; no package/version changes or installs.
- Fresh temporary PostgreSQL cluster with generated credentials and loopback
  ports. Go fixtures each migrate/seed/drop a separate schema.
- Web production build runs from a scratch copy of the web workspace with
  existing node_modules. It does not overwrite the developer's `.next` or
  generated `next-env.d.ts`. API and web use unused ephemeral ports.
- Playwright 1.65.0, one worker, locally installed Chrome 157.0.8103.50 using a
  temporary config. This is not execution on Playwright's pinned browser binary.
- Browser fixtures use the same disposable DB as their API. Real read-error
  tests rename/restore `queue_entries`; never run them on a shared/demo/live DB.
- Temporary API/web processes and DB cluster were stopped; scratch DB/build
  directories removed after both RED and GREEN runs. The running Docker demo
  database was not used for destructive regression fixtures.

## Initial review checks (completed about 22:57 Asia/Bangkok)

| Command / check | Observed result |
|---|---|
| `go test -tags=integration -race -count=1 -v ./...` from `apps/api` | PASS: 52 top-level tests, no failures or race reports; includes F-01..F-04 regression and four new API/DB tests |
| `go vet -tags=integration ./...` | PASS |
| `go mod verify` | PASS: all modules verified |
| `go build -buildvcs=false -o <temporary binary> ./cmd/api` and `./cmd/db` | PASS |
| Fresh db CLI `migrate`, `seed` | Both exit 0 |
| `npm run lint` | PASS in scratch checkout |
| `npm test` | PASS: 2 web configuration tests |
| `npm run build`, then `npm run typecheck` | PASS in scratch checkout |
| `playwright test --config <temporary local Chrome config>` | PASS: all 18 browser tests, including three new review cases |
| T-035 actual API process stop/restart without DB reset | PASS: same cookie/session retrieves identical persisted active ticket |
| Generated DB/seed credential scan of isolated service logs | PASS: generated passwords absent |

Local orchestration: `python3 /tmp/dineq-s1-review-check.py`. It supplies generated
environment values, runs the commands above, then cleans up. Local captured logs:
`/tmp/dineq-s1-review-red-go.log`, `/tmp/dineq-s1-review-red-browser.log`,
`/tmp/dineq-s1-review-green.log`. These are local session artifacts, not published
CI evidence. Reproduce the repository tests using dedicated `TEST_DATABASE_URL`,
the documented seed/setup, and browser environment/config on a disposable DB.

## Local Docker follow-up

After GREEN, ran:

```bash
docker compose -f compose.yaml -f /tmp/dinequeue-local-ports.yaml up --build -d
```

The existing local override avoids unrelated services on ports 3000/8080:
web `127.0.0.1:3001`, API `127.0.0.1:8081`, APP_ORIGIN `http://localhost:3001`.
It is outside the repository and is not required on another developer's machine
when the default ports are available.

- API and web images built; Go module download and npm install layers were CACHED.
- API/web containers restarted, PostgreSQL 18.6 container remained healthy.
- API `/health/live`: 200 `{"status":"ok"}`.
- Web `/api/health/ready`: 200 `{"status":"ready"}`.
- Web `/login`: HTTP 200.
- No new SQL migration or seed rerun was needed for these code fixes. Existing
  demo data and local credentials were retained.

This Docker follow-up verifies build/start/readiness, not a second full browser
suite on Docker's Node 22.23.4/PostgreSQL 18.6 environment. No cloud deployment,
CI, manual accessibility acceptance, teammate approval or release tag is claimed.
Record checks on the eventual release/merge commit separately.

## Final Sprint 1 completion checkpoint — 13 Nov, about 23:29 Asia/Bangkok

The user requested one complete audit/fix of the required S1 scope. The resulting
[completion checklist](sprint1-completion-checklist.md) maps all S1 FRs, BRs,
applicable NFRs, UI P-01..P-05 and existing AC/test IDs. Additional corrections:

- Customer header now shows available/active restaurant and queue context, current
  account identity, My Queue navigation and logout. Active tickets retain their
  context even when the queue no longer accepts joins.
- Host current card includes both joined/called timestamps, visibly labelled in
  the restaurant timezone. Table columns are spaced and use a focusable horizontal
  scroll region on small screens instead of squeezing labels/values together.
- Queue selector/input sizing prevents document overflow at 360px.
- Auth failure events use fixed redacted messages, without submitted identity,
  password or session token. The new real-DB log test failed before logging was
  added and passed afterwards.
- Duplicate-active race test now sends 20 requests from the same Customer,
  requiring one success and 19 conflicts; the separate 20-distinct-Customer
  unique-number race remains.
- The implemented API is exported in `../api/sprint1.openapi.json` (14 paths and
  operations). JSON parsing and local structure/reference checks passed. This is
  a documentation check, not a claim that an external OpenAPI validator ran.
- System SDS implementation/module/cache descriptions and status were synchronized.

Final code/test snapshot SHA-256 (same path/byte hashing procedure as above):
`2072d652f4981f16d65cb4bdbba7bb75f9b23f18633a821a4a8aa7cb0921d150`.

| Final check | Actual result |
|---|---|
| `go test -tags=integration -race -count=1 -v ./...` | PASS: 53 top-level tests; no race report |
| `go vet -tags=integration ./...`, `go mod verify`, API/db builds | PASS |
| Fresh temporary DB migrate/seed; actual API restart | PASS; session and ticket preserved |
| `npm run lint`, `npm test`, `npm run build`, `npm run typecheck` in scratch build | PASS; 2 web unit tests |
| Complete browser suite | PASS: 20 tests, including both viewport keyboard/layout cases |
| Generated test credential scan | PASS; no generated DB/seed passwords in service logs |
| Final Docker rebuild/start | PASS; dependency install layers CACHED; existing DB retained |
| Final API live / web ready / login | 200 `ok`, 200 `ready`, HTTP 200 |

The first added UI-field run failed on missing header context and Host joined
time. The first expanded layout run had 18 pass/2 fail: mobile Join overflow and
a test locator matching both the app alert and Next route announcer. After fixing
the form and narrowing that locator, visual inspection found cramped Host columns;
the initial scroll-wrapper attempt then exposed a grid minimum-width overflow
(19 pass/1 fail). `min-w-0` fixed the grid item. Both targeted keyboard/layout cases
passed before the final full combined run above. Assertions were retained, including
12px header-text separation and actual ArrowRight scrolling of the focusable region.

Agent visual inspection covered captured login, home, join, ticket WAITING/COMPLETED,
Host waiting and current-called screens at 360px and 1280px. Labels and lifecycle
status text are visible; controls are keyboard-reachable and activatable; no document
horizontal overflow. CSS foreground/background contrast calculations: wine/paper
11.18:1, white/wine 11.73:1, ink/paper 14.35:1. Disabled controls and full WCAG
certification are not inferred from these calculations. This is local agent
verification, not another student's UAT or approval.

Final local artifacts: `/tmp/dineq-s1-final-green.log`,
`/tmp/dineq-s1-completion-red.log`, `/tmp/dineq-s1-completion-auth-red.log`,
`/tmp/dineq-s1-completion-green.log`, `/tmp/dineq-s1-completion-table-red.log`,
`/tmp/dineq-s1-completion-layout-check.log`, `/tmp/dineq-s1-final-docker.log`,
`/tmp/dineq-s1-final-smoke.log`, screenshots under `/tmp/dineq-s1-completion-ui/`.
Final harness: `/tmp/dineq-s1-final-check.py`, using the same isolated setup and
commands as the earlier harness. The first local Docker health probe arrived
while `go run` was still compiling startup code and got an empty reply; after the
server reported listening, the final health/login checks above all passed.

The source fingerprint excludes documentation and generated local type declarations.
No application/test source changed after this final verified snapshot. Tests did
not use the developer's Docker database; no migration/seed or password change was
needed for the running demo. Peer review, committing/pushing and release/tagging
remain real team workflow steps. CI/GCP are Lab 5 and independent UAT is Lab 6.
