# F-01 implementation addendum — 6 November 2026

User authorization: implement F-01 on `feat/F-01-auth-session-rbac`. Base: `42ebd66`.
This addendum was written before code and committed afterward with user authorization;
no pre-implementation addendum commit, Issue, teammate review or TA approval is claimed.
Existing F-01 contract was committed in
the baseline. This F-01 addendum did not change F-04/F-05 grouping; subsequent
user-authorized DEC-028 includes legacy F-05 completion under parent F-04.

- Implement login/logout/me, login UI, authenticated Customer/Host shells and reusable
  backend role, queue-grant and ownership guards. Queue endpoints and ticket records
  remain F-02..F-04 (completion keeps legacy F-05); their end-to-end T-005..008
  checks must run again when those exist.
- Passwords: Argon2id v19, 64 MiB, 3 iterations, parallelism 2, random 16-byte salt;
  seed password 12–128 UTF-8 bytes, login max 128 bytes, generic invalid credentials.
- Session: 32 random bytes, base64url cookie, only SHA-256 digest stored in PostgreSQL;
  fixed 12 hours, active-user check per request, replacement on login, revocation on
  logout. HttpOnly, Path=/, SameSite=Lax; Secure and __Host- prefix for HTTPS.
- Same-origin browser routing. Every unsafe auth request requires exact APP_ORIGIN;
  logout/future authenticated mutations additionally require X-CSRF-Token.
  A per-session HMAC-SHA256 token is returned by login/me, held in browser memory;
  no cookie/token in localStorage. Login itself requires Origin and JSON.
- Login limit: 10 attempts/minute/IP per API process; IP comes from the socket,
  not arbitrary forwarded headers. Bound hash concurrency and limiter storage.
  Multi-instance shared limiting remains deployment work.
- Explicit SQL migration creates users, sessions, restaurant, service queue and queue
  grants. Migration CLI is opt-in, transactional, checksum-checked, advisory-locked;
  app startup never migrates. Seed is transactional/idempotent and does not reset
  existing passwords or grants; four demo passwords come from environment only.

## Added acceptance and STS (NFR-015 starts in S1)

| AC / test | Input | Expected |
|---|---|---|
| AC-022 / T-037 | Missing/foreign Origin; missing/wrong CSRF on authenticated mutation | 403, no session revocation/write |
| AC-023 / T-038 | Expired/revoked session or disabled account | 401; cookie flags and token hashing verified |
| AC-024 / T-039 | Login again with old session, invalid JSON/extra role, repeated failures | old token revoked; strict 400; bounded 429 |
| AC-025 / T-040 | Fresh DB migrate twice, seed twice, constraint failures | stable schema/accounts/grants; no partial changes |
| AC-026 / T-041 | Browser Customer/Host login, refresh, logout, expired session | correct role shell, persistent identity, login redirect after logout/expiry |

T-001..004 run through real PostgreSQL + Fiber HTTP tests. T-006..008 guard
equivalents use test-only routes and pure ownership checks, not invented product
queue endpoints. Full product endpoint acceptance stays pending those features.
Record actual RED/GREEN output and commands in the F-01 execution report.
