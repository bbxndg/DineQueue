# Sprint 1 repository foundation

Date: 2026-11-06 (Asia/Bangkok). Scope authorized in chat: repository foundation only.
This does not approve the remaining proposed product rules or establish course/TA sign-off.

## Scope and design

- Next.js App Router, TypeScript and Tailwind web shell; no fake login or queue actions.
- One Go Fiber API process, pgx PostgreSQL pool, sqlc configuration for future repositories.
- HTTP handlers call services; repositories own SQL execution, not business transactions.
- Only liveness/readiness endpoints are public. No customer or operator endpoints exist yet.
- Readiness performs a bounded PostgreSQL ping; failures return generic 503 without credentials.
- Explicit environment configuration, no committed passwords; local Compose binds web/API to loopback.
- PostgreSQL uses a named volume; local Compose requires an operator-supplied password.
- Development containers and reproducible dependency manifests; no cloud deployment or Git operations.
- No domain migrations/seeds, F-01..05 implementation, authentication claims, or S2/S3 code.

## Acceptance and planned verification

| AC | Observable result | Verification |
|---|---|---|
| FD-AC-01 | Web shell compiles with TypeScript and Tailwind | Typecheck, ESLint, production build |
| FD-AC-02 | Missing/invalid DB URL, port, environment or origin prevents API startup without leaking values | Go config tests |
| FD-AC-03 | Liveness works without DB; readiness is 200 only on successful probe and otherwise generic 503 | Go HTTP tests |
| FD-AC-04 | Requests have a request ID; unexpected routes/errors expose no internal details | Go HTTP tests |
| FD-AC-05 | Local Compose has persistent DB, internal service DNS and required secrets | Compose validation; container smoke test if daemon available |
| FD-AC-06 | Real PostgreSQL readiness uses pgx | Opt-in integration test against TEST_DATABASE_URL |
| FD-AC-07 | Environment files are ignored and Docker build contexts exclude secrets | Configuration review |

## Definition of done

Record exact commands and actual outcomes, including unavailable tools, in the foundation evidence document.
Keep T-001..036 as NOT RUN: foundation checks do not prove any product feature acceptance criteria.
No claim of teammate review, released SHA, CI success or deployment without evidence.

## Process limitation

At foundation implementation time this checkout had no Git repository, so no Issue/branch/
contract-commit/PR history could be recorded. Git was subsequently initialized on `main`
and connected to `bbxndg/DineQueue`. Initial commits import existing artifacts; they do
not establish historical pre-implementation Git evidence or teammate review.
