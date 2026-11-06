# 16. Security, Privacy and Quality Controls — v2

**Status:** design checklist, not an assurance of security. No production pentest has been run. Source references: OWASP Session Management / CSRF / Authorization Cheat Sheets; PostgreSQL row-level locks; Fiber v3 middleware documentation. The course requires authenticated API permission enforcement, tests, no real secrets, Docker/GCP deployment.

## 1. Trust boundary

Browser client (untrusted) → Next.js UI/proxy → Go Fiber API (enforces identity, role, scope, validation, rate limits) → PostgreSQL (constraints, locks, durable state). The UI may hide controls but never grants access. Every Host action checks restaurant/queue grant AND resource association. Do not put sensitive DB state or raw stack trace in public JSON.

## 2. Controls to implement and verify

| Control | Implementation decision | Test / review |
|---|---|---|
| Authentication | Seeded Customer/Host, Argon2id password hashes | good/bad login; disabled accounts |
| Sessions | cryptographically random opaque token, store **hash** in DB, HttpOnly Secure SameSite cookie, rotate on login, expire, revoke on logout | stolen/expired/revoked token negative tests |
| CSRF | same-site deployment + origin allowlist and synchronizer/anti-CSRF token on unsafe cookie-auth requests | forged POST rejected |
| Authorization | backend role+queue+restaurant checks, foreign ticket not-found response | 401/403/404 tests across routes |
| SQL injection | SQL parameters (`pgx/sqlc`), never string-concatenate user IDs/SQL | invalid input tests + code review |
| XSS / content | React escapes by default, prohibit raw HTML rendering and unsafe `dangerouslySetInnerHTML`; sanitize optional notes | escaping test, CSP review |
| Brute force / abuse | rate limit login/join/call; bounded payload sizes and numeric/UUID validation | 429 and boundary tests |
| CORS / cookies | specific trusted origin(s), no wildcard credentialed CORS; secure cookie domain/path | deployment smoke test |
| Concurrency | sorted row locks, DB unique indexes on active tickets and open table mapping, bounded retries only if idempotent | 20-way integration race |
| Sensitive data | public display ticket code only, anonymize analytics, minimize customer data | public shape and access tests |
| Dependency safety | pinned Go/npm versions, lockfiles, audited vulnerabilities, do not run unreviewed install scripts | CI audit / reviewed changes |
| Observability | request ID, structured audit of transitions; avoid logging cookies/passwords/CSRF tokens | log review |
| Reliability | idempotent actions, keep immutable action history, migration rollback plan | retries and failure injection |

## 3. Go Fiber-specific considerations

- Fiber v3 uses middleware for sessions; follow official v3 APIs and register session middleware before CSRF when using session-backed CSRF. The v3 docs say Go 1.26+ is required; lock the version the team actually uses and document it.
- Request/context values from Fiber may be reused after a request; **copy** any data intentionally retained in goroutines or async processing, rather than keeping temporary request-scoped buffers/references.
- Persist sessions in a durable supported store or use a validated custom opaque session implementation; do not assume default in-memory middleware storage survives multi-instance GCP redeployment.
- Keep DB transaction handling in `internal/queue` / `internal/seating` services, independent of HTTP framework, to make integration tests deterministic.

## 4. Infrastructure guidance

- GCP configuration via secret manager or runtime secret variables; no `.env` secrets committed. Never place DB password in `NEXT_PUBLIC_*` variables.
- If using Cloud Run, ensure database access/pooling fits connection limits. Avoid creating a pool per request.
- Build frontend and backend with pinned, reviewed dependencies; use non-root container user, health/readiness endpoints, least privilege service account and private DB connectivity.
- CI PR gates: `go test`, `go vet`, Next.js lint/build, SQL migration check, integration tests; deploy only accepted main SHA after review.
- Backup/restore is desirable, but persistence/reproducible fixtures/deployment are the course minimum.

## 5. Risk register

| Risk | Impact | Mitigation |
|---|---|---|
| Double table booking | High | DB row locks + unique open table mapping + concurrency tests |
| Unauthorized Host table mutation | High | Scoped backend permissions + negative tests |
| Session misuse/CSRF | High | secure cookie + origin/CSRF + logout revocation |
| Incorrect table connectivity | Medium/High | approved graph + no location-based inference + staff confirmation |
| Unfair matching and silent skip | Medium | documented algorithm + explicit rationale + audit |
| Lost table status on deployment | High | DB persistence + repeatable migration/seed + restart test |
| UI stale data vs DB state | Medium | transactional commit/revalidation + conflict refresh |

## 6. References (read-only, no external code execution)

- https://www.postgresql.org/docs/current/explicit-locking.html
- https://www.postgresql.org/docs/current/indexes-partial.html
- https://docs.gofiber.io/
- https://docs.gofiber.io/blog/fiber-v3-sessions-cookies/
- https://cheatsheetseries.owasp.org/cheatsheets/Session_Management_Cheat_Sheet.html
- https://cheatsheetseries.owasp.org/cheatsheets/Cross-Site_Request_Forgery_Prevention_Cheat_Sheet.html
- https://cheatsheetseries.owasp.org/cheatsheets/Authorization_Cheat_Sheet.html
