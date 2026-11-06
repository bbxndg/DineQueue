# 11. Labs 5–7 — Implementation and Evidence Plan

The labs all use the SAME repository/application as the group project. The source PDF establishes the assessment requirements; all proposed commands and hosting choices are team decisions to validate with Lab handouts.

## Lab 5 — Docker, CI/CD, GCP deployment (due 15 Nov; 100 points)

### Required grading categories

| Category | Points | Concrete evidence to capture |
|---|---:|---|
| CI and quality gate | 30 | PR build/tests, failing required check, enforced branch protection |
| Docker and reproducible config | 20 | Docker build/run from clean checkout and `.env.example`, no secrets |
| GCP deploy and persistence | 35 | URL, working UI/API/DB, data surviving restart/redeploy |
| Repeatable release pipeline | 15 | Reviewed PR → passing CI → main merge → deployment with SHA |

### Design

- Dockerize API and web as separate app containers (or equivalent supported topology); Postgres local via Compose for reproducible development.
- For GCP choose course-approved cloud deployment (e.g. Cloud Run and an approved managed/other persistent Postgres service; **confirm allowed GCP resources in Lab 5**).
- Required PR workflow checks: install → lint/typecheck/build → unit → isolated DB integration; E2E selected path if infrastructure supports it reliably.
- Protect `main`: required check cannot be bypassed in normal release path. Show failing check on a temporary branch; **do not intentionally break the accepted release**.
- Deploy only accepted release from main; attach commit/tag to deployed version; smoke test a domain journey and re-read persisted record after restart/redeploy.

### Suggested acceptance/evidence

1. A PR with a failing test is blocked from normal merge; screenshot/log of actual branch-protection behavior.
2. A valid PR with passing checks merges after teammate review.
3. Build/run from clean state using documented commands; no untracked secret needed.
4. Public deployment URL shows UI and working API; Postgres persists a ticket after service restart.
5. Deployment logs/CI run identify same git SHA as release/tag.

**Status:** NOT IMPLEMENTED / NOT TESTED / NOT DEPLOYED.

## Lab 6 — Cross-team UAT and defect management (due 22 Nov; 100 points)

### Required grading categories

| Category | Points |
|---|---:|
| UAT preparation and handoff | 20 |
| Independent execution of partner's app | 25 |
| Defect logging/triage | 20 |
| Accepted defect → fix → independent re-test | 35 |

Each team tests a partner app AND prepares tests/triages fixes in its own app. Record release SHA, UAT tester, steps, expected/actual, PASS/FAIL and defect link. At least **four user-facing UAT cases**:

| UAT ID | Category | Goal and preconditions | Steps | Expected |
|---|---|---|---|---|
| UAT-01 | Normal journey | C1, H1 accounts; Q1 open | C1 login → join 4 → H1 call-next → complete → C1 reload | Ticket created, called, completed; consistent and persisted |
| UAT-02 | Business rule/boundary | C1 already has WAITING in Q1 | C1 submits another join; try size 0 | Second join rejected; invalid party size rejected; existing ticket unchanged |
| UAT-03 | Role/ownership | C1/C2/H2 accounts | C2 open C1 ticket URL; C1 invoke Host call; H2 access Q1 Host panel | Protected data/actions denied with no private leak |
| UAT-04 | State/history/persistence | An existing ticket and known release | Refresh page, restart service if tester has controlled access, inspect status/history; Host tries invalid transition | Saved state survives; invalid transition rejected; S1 no history page required |

### UAT record template

`UAT ID | goal | release SHA | tester | account | preconditions | steps | expected | actual | PASS/FAIL | evidence link | defect ID`

### Defect workflow

`UAT failure -> GitHub defect Issue (repro, expected/actual, severity) -> triage (accepted/duplicate/not-defect/enhancement + rationale) -> regression test FAIL -> fix branch -> peer PR review -> CI + deployment -> partner independent re-test on exact version`.

Resolve and independently re-test **at least one accepted defect**. If none is legitimate, TA provides a controlled defect exercise on an isolated branch/version. **Never fabricate a finding.**

**Status:** NOT PERFORMED.

## Lab 7 — Maintenance, metrics, release readiness (due 29 Nov; 100 points)

### Required grading categories

| Category | Points |
|---|---:|
| Change impact + contract revision | 25 |
| Changed behavior: failing test → fix → regression/redeploy | 50 |
| Five dated objective engineering metrics + <=100 word interpretation | 15 |
| Release-readiness checklist + three short retrospective answers | 10 |

**Important:** A feature already in baseline Sprint backlog does not itself fulfill Lab 7. Obtain a **separate instructor/TA-approved change** to an already implemented rule.

Possible **candidate to request approval**, not pre-approved: Change no-show handling from immediately terminal after one call to requiring a configurable minimum time delay; or adjust the max party-size validation. Do not implement as a Lab 7 item without approval.

### Change-impact template

- Change ID: `CR-[fill]`; approved by `[fill]`, date `[fill]`.
- Old rule: `[fill]` → new rule `[fill]`.
- Affected IDs: FR-__, BR-__, F-__, SDS sections, AC-__, tests T-__, database/API/UI paths.
- Updated reviewed contract PR: `[fill]`.
- RED changed-behavior test command and failing log/SHA: `[fill]`.
- GREEN implementation command and passing regression/CI/SHA: `[fill]`.
- Deployed version URL/SHA: `[fill]`.

### Objective engineering metrics (five, date window/source required)

1. GitHub Issues completed in selected period; source board filter.
2. PRs merged and reviewed; source GitHub PR query.
3. Automated tests passing out of tests run; source CI job/run.
4. UAT defects opened and closed; source labeled Issues.
5. CI pipeline success and failed checks; source GitHub Actions run list.

Write one **action-oriented interpretation <=100 words** based on actual figures. High counts by themselves do not earn points.

### Release-readiness checklist

- [ ] No open critical defects (or explicit documented residual risk/decision)
- [ ] Relevant unit/integration/E2E/regression checks passed on release SHA
- [ ] Latest UAT findings have re-test status
- [ ] Environment/secrets/config and migrations checked
- [ ] Docker/GCP deployed app and persistent DB smoke-tested
- [ ] SRS/AC/STS/traceability updated to match release
- [ ] Remaining debt/limitations listed honestly
- [ ] Release tag/commit and evidence index complete

### Retrospective (one or two sentences each)

- What helped? `[fill]`
- What hurt? `[fill]`
- What will change? `[fill]`

**Status:** NOT PERFORMED.
