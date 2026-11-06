# F-01 local execution evidence — 6 November 2026

Branch: `feat/F-01-auth-session-rbac`. Base HEAD: `42ebd66`.
All results below describe the **modified worktree before commits**, not execution
on a subsequently created commit. The user later authorized focused commits to
capture this work; those commits do not retroactively change the tested version.
No push, Issue, PR, teammate review, CI, release or deployment was performed.
User authorized F-01 implementation and subsequent commits in chat.

## Implemented scope

- POST `/api/v1/auth/login`, GET `/api/v1/auth/me`, POST `/api/v1/auth/logout`.
- Argon2id salted hashes; 12h opaque sessions, only SHA-256 digest stored in DB;
  rotation and old-token revocation in one service transaction, logout revocation,
  active-user lookup on every authenticated request.
- HttpOnly, SameSite=Lax, Path=/ cookie; HTTPS Secure + `__Host-` prefix;
  exact Origin on login and Origin + per-session CSRF on authenticated mutations.
- Strict login JSON, 4 KiB body cap, request deadlines, generic errors, bounded
  password-verification concurrency and per-process socket-IP attempt limiter.
- Reusable backend role/queue-grant/ownership gates. Queue/ticket routes are not
  shipped here; owner IDs must be loaded by the future resource service.
- Explicit transactional/advisory-locked/checksum-checked SQL migration and
  idempotent environment-password seed CLI; no migration on API startup.
- Login, session-ended feedback, Customer/Host authenticated shells and logout UI.
  There are no join/call/complete/seating implementations or placeholder API actions.

## Actual checks

Toolchain: Go 1.26.10, Node 22.17.0, npm 10.9.2, installed PostgreSQL 18.1,
Playwright 1.65.0 + its Chromium 1254. Checks ran around 21:10–21:28 Asia/Bangkok.
Go used `/tmp/dineq-toolchain/go/bin/go`, `GOCACHE=/tmp/dineq-go-cache`,
`GOMODCACHE=/tmp/dineq-go-mod`. This does not validate target container versions.

| Command/check | Actual result |
|---|---|
| Initial `go test ./internal/auth` with password test before implementation | RED: undefined HashPassword / VerifyPassword (compile-time missing implementation) |
| `go test -race ./...` | PASS; F-01 unit tests and foundation regression |
| `go test -tags=integration -race -count=1 -v ./...` | PASS: 22 top-level Go tests, including 6 F-01 real-DB tests and 3 F-01 unit tests; nested config subtests also ran |
| `go vet ./...` | PASS |
| `go mod verify` | PASS |
| API + db CLI `go build -buildvcs=false -o <temporary binary> ./cmd/api` / `./cmd/db` | PASS |
| Compiled db CLI migrate twice + seed twice | PASS on a separate empty DB, then live API readiness 200 |
| `npm test` | PASS: existing api-origin test file |
| `npm run lint` / `npm run typecheck` | PASS, including E2E/config TypeScript |
| `npm run build` | Sandbox failed parsing TypeScript --showConfig; unchanged elevated build PASS; final build after session-ended UI change also PASS |
| `npm run test:e2e --workspace @dinequeue/web` | First run 2 passed/1 failed because alert selector also matched Next route announcer; selector narrowed to the exact error message; final fresh isolated run **3 passed** |
| `docker compose config --quiet` with validation-only dummy environment | Sandbox Docker CLI plugin panic; elevated retry PASS |
| Docker image build/run | NOT RUN; host processes were used |
| `git diff --check` | PASS |
| Gitleaks 8.30.1, redacted scan of tracked + nonignored untracked source snapshot | 0 findings; dependencies/build/ignored credentials excluded |
| `npm audit --json` | 10 Moderate tooling packages (existing minimatch advisory chain), 0 High/Critical; no automatic dependency fixes |
| `npm audit --omit=dev --json` | 0 known production advisories |
| `govulncheck -json ./...` | Exit 0; GO-2026-6860/6866/5932 are module-only findings, with no affected package/function in current imports/calls |

The ephemeral harness `/tmp/dineq-f01-check.py` created a random-password cluster
and schemas, ran the commands above, built/launched Fiber and production Next.js,
passed ephemeral seed passwords to browser tests through environment, and checked
runtime logs for those passwords. It stopped processes and removed the cluster in
finally cleanup. Final output: `/tmp/dineq-f01-final.log`. Scanner reports use
`/tmp/dineq-f01-*.json`. No test used an existing application database.

## Test names and acceptance mapping

| Test | Evidence / IDs |
|---|---|
| TestPasswordHashSaltAndVerification | distinct random salts, correct/wrong password, malformed/unsupported hash, bounded password input |
| TestTokenAndCSRFAreBoundToSession | CSRF bound to opaque token; invalid/cross-session rejection |
| TestLoginLimiterIsBoundedAndExpires | 10/minute quota, expiry, bounded key storage |
| TestF01LoginMeLogoutAndRotation | T-001/004, T-038/039; persisted hashed session, 12h lifetime, me, rotation, logout and old-token rejection |
| TestF01InvalidDisabledAndExpiredIdentity | T-003/038; generic invalid/unknown credentials, disabled account, expired/forged session |
| TestF01RoleQueueAndOwnershipGuards | T-002 and guard equivalents of T-005..008; Customer/unassigned Host denied, assigned Host accepted; revoked grant denied on existing session |
| TestF01OriginCSRFAndValidation | T-037/039; forged logout has no side effect, strict login body, live HTTP 429 |
| TestF01MigrateSeedIdempotencyAndConstraints | T-040; stable accounts/passwords/grants, unique/role/FK constraints, seed rollback, modified-applied-migration rejection |
| TestF01LoginRotationRollback | T-039; injected session insert failure rolls back old-token revocation |
| e2e/auth.spec.ts (3 journeys) | T-041; Customer/Host login+refresh+logout; Customer host-page redirect, real CSRF rejection, no localStorage, wrong credentials and invalid-session prompt |

## Remaining acceptance/process limits

- T-005..008 refer to actual join/call/ticket endpoints. Those checks remain pending
  F-02..04 integration. Test-only protected routes are not deployed product routes.
  F-01 auth is locally verified; complete whole-Sprint AC-004/005 is not asserted.
- T-009..036, queue concurrency, ticket privacy on real resources and all S2/S3
  product behavior remain unimplemented/not run. At execution time the four-feature
  regrouping was still under discussion. Subsequent user-authorized DEC-028 groups
  S1 into F-01..F-04, with F-05 retained inside F-04; this does not alter these results.
- The original F-01 baseline contract was committed before implementation. The new
  addendum was written before code but committed afterward, with no separate pre-implementation commit;
  the RED/GREEN worktree sequence is not a historical commit sequence.
- The branch existed when work began. Issue linkage, teammate review and release
  provenance must be added honestly; no approval is inferred from these local tests.
- Socket-IP limits are per process; behind Next proxy clients may share the quota.
  Trusted proxy and distributed limiting, expired-session retention, production TLS
  and multi-instance behavior must be tested before deployment.
- A dev-tool advisory and three module-only Go advisories remain for a separate
  reviewed dependency update. A clean scan is not an absolute security guarantee.

## References used

- https://cheatsheetseries.owasp.org/cheatsheets/Password_Storage_Cheat_Sheet.html
- https://docs.gofiber.io/api/ctx/
- Existing repository session/CSRF contracts and official pinned module source.
