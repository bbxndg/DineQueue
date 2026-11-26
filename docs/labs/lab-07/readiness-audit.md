# Readiness follow-up audit — 26 November 2026

> Historical verification snapshot: checks below ran before follow-up commits.
> The user subsequently authorized three atomic commits and a push on 26 November.
> References to uncommitted/unpushed state describe the time of verification.

Base: `8deb367` on feat/lab7-customer-readiness; changes below are uncommitted.
No push, CI execution or cloud deployment is claimed.

## Scope

Readiness ownership/lifecycle guards, queue-before-entry locks, migration defaults,
DTOs, Customer/Host call sites and the existing local smoke journey were reviewed.
At this initial smoke-audit checkpoint the three suggested peer-review items were left untouched: rejected-write DB assertions,
CallNext existence-query optimization, and browser error-path coverage.
The user subsequently requested those improvements in advance of review; see
[peer-improvement results](readiness-peer-improvements.md).
The user explicitly cancelled the workflow-trigger change; ci-docker.yml is unchanged.

## Confirmed regression and fix

The older `scripts/lab5/docker-smoke.mjs` joined a new ticket and called it without
Customer readiness confirmation. Reproduced against the real isolated readiness
app: the old journey failed at Call Next with 409 instead of expected 200.
Its request helper also attached Origin/CSRF headers only for POST, so adding a
readiness PATCH without extending the helper would fail CSRF validation.

The script now supplies mutation headers for PATCH, asserts the new ticket is
Not ready, verifies NO_READY_PARTIES, then sets Ready as the authenticated owner
before calling. It checks code/join time remain unchanged. Persistence create/verify
also saves and compares isReady. Existing privacy/CSRF/Origin/logout checks remain.

## Actual verification

Source used for the application: the previously verified readiness API binary and
production Next build (same app source as base); no app code or images rebuilt.
APP_ENV=test, synthetic accounts, a separate dq_l7_audit PostgreSQL database and
loopback ports were used. This was not a Docker-image or Cloud Run acceptance run.
Credentials were generated only in a private /tmp configuration, never read from
.env or copied into evidence. The developer database was untouched.

| Command / action | Outcome |
|---|---|
| `node scripts/lab5/docker-smoke.mjs journey` before the script fix | Expected failure: Call Next 409 vs 200 |
| `node --check scripts/lab5/docker-smoke.mjs` | PASS |
| `node scripts/lab5/docker-smoke.mjs journey` after the fix | PASS: readiness, privacy, RBAC, CSRF/Origin, call/complete, SSR Host view and logout |
| `node scripts/lab5/docker-smoke.mjs create` | PASS: saved WAITING ticket with isReady=false |
| Stop/start temporary API, same audit database, then `node scripts/lab5/docker-smoke.mjs verify` | PASS: saved code/status/readiness match after fresh login |

[Captured RED/GREEN/create/verify output](evidence/readiness-smoke-audit.txt).
Environment names for reproduction are WEB_HOST_PORT, SEED_CUSTOMER_PASSWORD,
SEED_CUSTOMER2_PASSWORD, SEED_HOST_PASSWORD and EP3_TICKET_FILE; use only a disposable
seeded database. Full Go/browser suites were not rerun because application and
existing test source did not change; these are targeted checks of the changed script.
Feature PR review and actual hosted checks remain pending.
