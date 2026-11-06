# 10. Team Engineering Workflow, Git and AI Use

Required by CPE 334 §§2, 5.3, 8.2–8.4. Two-person team, **both** participate in requirements/design, tests, coding, review, AI supervision, integration and deployment.

## 1. Branching and PR workflow

```text
main (accepted releases only)
  ^  reviewed release PR, passing integration
  |
sprint-1-staging (shared sprint integration)
  ^  feature PR reviewed by teammate
  |
feat/F-02-join-waitlist
```

1. Create a GitHub Issue with F/FR/BR IDs, short acceptance criteria, SDS/STS link, owner and collaborator.
2. Create `feat/F-xx-short-name` or `fix/ISSUE-id-short-name` from current sprint staging.
3. Add/adjust tests first for meaningful rules, capture RED evidence, then implement, capture GREEN evidence; inspect diff.
4. Open PR into sprint staging; **other student** reviews behavior, tests, migration, security, contract alignment and gives a specific comment/finding/rationale.
5. Merge after review; run integrated sprint tests and critical journey checks.
6. Open a reviewed release PR from sprint staging → `main`; do not bypass failed required checks after CI is active.
7. Tag `v0.1.0-sprint1` (proposal), record full git SHA; from Lab5 deploy main accepted release by repeatable pipeline.

Authors **must not approve their own PRs**. A screenshot is supportive evidence, not a substitute for real tests or review history.

Sprint 1 has four parent feature branches (DEC-028):
`feat/F-01-auth-session-rbac`, `feat/F-02-join-waitlist-ticket`,
`feat/F-03-my-ticket-privacy`, `feat/F-04-host-queue-operations`.
F-04 covers board, call and completion. Retain F-05 as the completion traceability
ID within the F-04 Issue/PR; it does not need a separate parent feature branch.

## 2. Proposed pair collaboration

- Student A: draft queue SRS and F-02 AC; implement and test part of F-02, review F-04 PR.
- Student B: challenge/revise requirement wording and F-04 AC; implement and test F-04, review F-02 PR.
- Both: design Auth contract together, pair on concurrency tests, share E2E, rotate Release/CI responsibility, explain sampled tests and decisions.
- Capture contribution using Issues, PR authors/review, commits and a short summary table; equal commit counts not required.

| Member | Requirements/Design evidence | Implementation evidence | Test evidence | Review/release evidence |
|---|---|---|---|---|
| Student A | `[fill]` | `[fill]` | `[fill]` | `[fill]` |
| Student B | `[fill]` | `[fill]` | `[fill]` | `[fill]` |

## 3. PR template

```markdown
## Feature and traceability
Issue: #___; Feature: F-__
FR/BR: __; AC: AC-__ ; STS: T-__
## What changed
...
## Tests (command, exact result, SHA)
...
## Negative/boundary/race case
...
## Screens/API/data changes
...
## AI support and human verification
...
## Reviewer findings / reasoning
...
```

## 4. AI-agent use declaration template

**Tools used:** `[fill actual tools, e.g. ChatGPT/Codex]`

**Assisted tasks:** `[fill specific tasks, e.g. drafted test fixture, scaffolded controller, suggested SQL index]`

**Human validation:** `[fill reviewed diff, verified DB constraints, ran test command, manual demo, peer-reviewed PR]`

**Known limitations:** `[fill]`

This must reflect what actually happened. Full prompt transcripts and long reflection essays are **not required**.

## 5. Definition of Done for issues

- Requirement and feature contract were authored/approved internally **before implementation**.
- AC IDs linked to automated test IDs (test level, input/precondition, expected result) and test results.
- Negative/boundary and authorization checks cover relevant change.
- Relevant migration/seed and documentation updated.
- Feature implemented via branch → PR → teammate review → sprint staging integration.
- No unapproved secret committed. Changes reviewed for privacy and scope correctness.
- Real result/evidence link and git SHA recorded. No placeholder PASS status.


## v2 update workflow

Record `BR-008` change, `FR-023..035`, new joined-table entities and the shift to Go Fiber as dated proposals in the Decision Register. Require separate reviewed pre-implementation S2 contracts for F-14..F-16, and run old S1 regression tests alongside the new S2 test catalog. Never claim historical sign-off or tested code based on these specification-only edits.
