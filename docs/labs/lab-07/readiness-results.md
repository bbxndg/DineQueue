# Lab 7 #44 — Customer readiness local implementation results

Date: 26 November 2026 (Asia/Bangkok). Branch: `feat/lab7-customer-readiness`.
Status: verification was performed on the local worktree before commits. The user
subsequently authorized six atomic commits. No push, feature PR approval, CI result,
release or cloud deployment is claimed.

## Version and review sequence

- Contract: `323d8e7c1796466fd6d3cea46044c72cd4ac20d4`.
- Teammate softkoi APPROVED [PR #45](https://github.com/bbxndg/DineQueue/pull/45)
  at 26 Nov 00:00:50 ICT; review explicitly accepts new Not ready/existing Ready.
- Staging/feature source base: `8539b24cb7374064668f2053544b1e4c706eb0bc`.
  This merges the contract; the application code remains the pre-change main
  `1524502386065be2eb6025c04bae72a5398c7228` (includes Lab 6 #41 HTTP 404 fix).
- Accepted Lab 6 baseline/version acceptance remains a team-supplied evidence field.
- RED was run before changing application source, using two newly added test cases.
  [Original test SHA-256](evidence/readiness-red-test.sha256) identifies that initial
  uncommitted test file. The later source uses a separate Not-ready join fixture
  because old calling regressions deliberately set ready fixtures.
- GREEN is **base SHA + uncommitted worktree**, not a passing test claim on the
  unchanged base commit. [Tested-source manifest](evidence/readiness-source-manifest.json)
  records the final code/test hashes. The user subsequently authorized commits; every
  app/test file in the manifest matches committed source `c7b6b27120edf1eefb057a84c114e5c4f0f75a84`.
  No integration/browser rerun after committing is claimed; source bytes are unchanged.
  After those code commits, `npm test` (2/2), lint and typecheck were rerun and passed.

## Change implemented

- New PostgreSQL migration 0003 adds is_ready; existing rows are true, subsequent
  joins default false. Applied earlier migrations are unchanged.
- Customer-only PATCH sets an explicit boolean on an owned WAITING ticket; strict
  JSON, Origin/CSRF, role, ownership and state guards remain enforced.
- Readiness updates, Call Next and Complete share queue -> entry lock order.
  Call Next selects the oldest ready WAITING ticket; no-ready differs from empty.
- Customer active/detail pages expose readiness controls; Host shows readiness and
  cannot initiate a call when all visible waiting parties are not ready.
- Returning Ready keeps ticket code/join time/order; CALLED cannot be displaced.
- Existing Ticket/HostEntry response fields and Next rewrite match the shared API.
- No polling, table matching, Dockerfile, CI workflow or cloud resources changed.

## Actual local checks

All database fixtures used isolated PostgreSQL, not the developer database or Cloud
SQL. Existing `dinequeue-db-1` was left running. The cached local Go test image
provides Go 1.26.10; test source is mounted read-only. Next web runs a production
build locally, while the test API uses APP_ENV=test with generated synthetic seed
credentials. This does not emulate deployed HTTPS/Cloud Run.

| Command / check | Actual result | Evidence |
|---|---|---|
| `go test -tags=integration -count=1 -v -run '^TestL7' ./internal/queue` before implementation | expected RED: 2 failed, 0 passed; old code omits isReady and calls an unconfirmed party with HTTP 200 | [RED](evidence/readiness-red.txt) |
| `go test -race ./...` from apps/api | PASS | [final Go log](evidence/readiness-final-go.txt) |
| `go vet ./...` from apps/api | PASS (silent success between unit and integration commands) | same final Go log |
| `go test -tags=integration -race -count=1 -v ./...` from apps/api | PASS: 66 top-level tests, 26 subtests, no failures; includes unit and integration tests, not 66 unit tests | same final Go log |
| `npm test` from root | PASS: 2 tests | [web tests](evidence/readiness-web-tests.txt) |
| `npm run lint` from root | PASS | [lint](evidence/readiness-web-lint.txt) |
| `npm run typecheck` from root | PASS | [typecheck](evidence/readiness-web-typecheck.txt) |
| `API_INTERNAL_URL=http://127.0.0.1:18081 npm run build` | PASS | [build](evidence/readiness-web-build.txt) |
| `npm run test:e2e --workspace @dinequeue/web -- readiness.spec.ts` | PASS: 1 new Customer/Host journey | [readiness browser](evidence/readiness-browser.txt) |
| `npm run test:e2e --workspace @dinequeue/web -- sprint1-review.spec.ts ticket-states.spec.ts` after fixture revisions | PASS: 6 cases, including keyboard/layout at 360px and 1280px | [targeted regression](evidence/readiness-browser-targeted.txt) |
| `npm run test:e2e --workspace @dinequeue/web` final full run | PASS: 21/21, no failures/skips | [full regression](evidence/readiness-browser-regression.txt) |
| Actual temporary API stop/start, same DB and existing session cookie | PASS: identity, joinedAt, party size, status and isReady unchanged; session still valid | [restart](evidence/readiness-restart.txt) |

Go execution wrapper (all Go checks above used this cached image and an isolated
internal Docker network; no host database ports or credentials are needed):

```sh
docker run --rm --network dq-l7-test \
  -v "$PWD/apps/api:/app:ro" \
  -v dq-l7-go-cache:/root/.cache/go-build -w /app \
  -e 'TEST_DATABASE_URL=postgres://postgres@dq-l7-test-db:5432/dq_lab7_test?sslmode=disable' \
  dinequeue-ep4-go-check sh -c \
  'go test -race ./... && go vet ./... && go test -tags=integration -race -count=1 -v ./...'
```

The test database container used trust authentication only on a temporary internal
network with no published port. This is test infrastructure, not deployment config.
Browser runs used F01_BASE_URL, F01_TEST_PASSWORD, F01_BROWSER_CHANNEL=chrome and
TEST_DATABASE_URL for the separate disposable local cluster; no real passwords or
.env contents are recorded. Restart command was `python3 /tmp/dq-l7-restart-check.py`:
it started the compiled API on port 18082, logged a synthetic Customer in, joined
and set Ready, terminated that API, restarted it, and read the exact same ticket
with the same in-memory session cookie. It ran separately from final regression.

## New test mapping

- `TestParseReadinessStrictBoolean`: exact boolean, null/type/unknown/duplicate keys.
- `TestL7NewJoinStartsNotReady`, `TestL7NoReadyPartiesCannotBeCalled`: RED/GREEN core.
- `TestL7ReadinessOwnerLifecycleAndFIFO`: skip A, call B, A returns ahead of C;
  unchanged code/time, idempotent versioning, called/completed guards.
- `TestL7ReadinessValidationPrivacyAndCSRF`: guest/Host/foreign/missing/malformed,
  CSRF, malformed input, Customer/Host DTOs without owner fields.
- `TestL7ReadinessInactiveQueueAndPersistence`: existing inactive queue can drain;
  stored readiness visible through a fresh service instance.
- `TestL7ReadinessMigrationPreservesExistingTickets`: old-row backfill, unchanged
  ticket identity and repeat migration, new default false.
- `TestL7ReadinessCallRaceBothCommitOrders`: PostgreSQL-confirmed blocked queue lock
  establishes both winning commit orders; no sleep-only concurrency assumption.
- `e2e/readiness.spec.ts`: owner toggles on list/detail, Host skips unavailable A,
  B finishes before A returns, called ticket has no readiness control.

Evidence text files have trailing whitespace, terminal carriage returns and trailing
blank lines normalized for Git whitespace checks. Test outcomes and content were
not changed; raw logs were preserved in a temporary local backup.

## Failed attempts and regression revisions

- Initial typecheck saw stale generated Next route types from before the Lab 6
  route-group fix. Preserved the old .next directory in /tmp and rebuilt types.
- Restricted subprocess execution caused Next's TypeScript config parse failure;
  the approved unrestricted local build passed. Node's restricted test runner
  initially reported only a file-level result; the rerun reports both named tests.
- Fixed an initial ParseMediaType return-arity compile error; this is not RED evidence.
- First Go regression had one exact DTO-length assertion that did not include
  isReady. Updated the exact count and readiness assertion; retained privacy checks.
- Existing calling fixtures now explicitly create ready parties. Their lifecycle,
  FIFO, concurrency and version assertions remain; owner UI journeys confirm Ready.
- First full browser run: 16 passed, 5 failed. Some old fixtures called not-ready
  parties; the new demo journey also left a CALLED ticket, conflicting with later
  stored-state fixtures. Corrected setup/cleanup and reran failing files first.
  A separate restart probe also wrote into that disposable dataset; final full
  regression runs alone from clean tickets. No production defect is inferred from
  that fixture interference. [First browser run](evidence/readiness-browser-first-regression.txt),
  [first Go regression](evidence/readiness-go-first-regression.txt).
- An intermediate unit/vet run overlapped a repository-file refactor and saw an
  incomplete source snapshot; [diagnostic](evidence/readiness-go-checks.txt).
  The final sequential command above is the authoritative Go result.

## Handoff and remaining acceptance

1. User authorized six atomic commits after local review. Commits are local; push
   only after explicit authorization.
2. Feature PR into lab7-staging with `Part of #44`, teammate review and real CI.
3. After merge, teammate pulls lab7-staging and creates feat/lab7-ticket-polling.
   Use `isReady` and the existing GETs in the shared contract; do not create a new
   readiness endpoint/migration or auto-toggle readiness from a timer.
4. Integrate polling, regression, reviewed main release, migrate and deploy through
   the existing pipeline, then real changed/preserved behavior smoke checks.
5. Complete #44 and PDF Parts A–D with actual release/CI/deploy URLs, metrics,
   contributions and AI-use declaration. Local evidence does not close the issue.
