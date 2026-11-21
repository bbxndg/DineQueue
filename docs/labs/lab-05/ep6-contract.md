# Lab 5 Episode 6 — deployment identity and merge gates

Date: 20 November 2026 (Asia/Bangkok).
Tracking: [Issue #17](https://github.com/bbxndg/DineQueue/issues/17).
Branch: `feat/lab5-ep6-identity-merge-gates`, based on `gcp-staging`
`7269e9ed28b10293a80f6f2c58c9c8a64937da99` (same as reviewed main).
Status updated 21 November 2026: main-only ruleset/environment and failing-PR
evidence reviewed; IAM/WIF setup is user-reported. Real WIF authentication remains
NOT RUN until EP7. See [EP6 results](ep6-results.md).

## Scope and course mapping

Student Guide EP6 §§6.1–6.4 configure repository IDs, the deployer, WIF and
the GitHub `demo` environment. §6.5 enables the main ruleset; §6.6 proves a
failing required check blocks merging. §6.7 records evidence and closes the issue.
The guide requires no implementation branch for the Console work. This branch
holds the runbook and the user's deferred EP5 evidence update in the same PR.
The intentional failing test belongs on a separate throwaway branch/PR into main.

The original Issue #17 scope additionally mentioned `gcp-staging`. On 21 November
2026 the user chose the course-required **main-only** gate; staging protection is
not required by this contract. Issue #17 is already closed by its owner. Its
checked authentication criteria are not evidence of an actual WIF exchange;
keep successful/failed authentication NOT RUN until exercised in EP7.

No app, production Docker or existing CI change is needed. EP7 owns
`.github/workflows/deploy.yml` and automated releases. EP8 owns restart/redeploy
persistence verification. EP6 does not rerun the first-deployment bootstrap from CI.

## Identity and access contract

- Repository `bbxndg/DineQueue`; numeric repository ID `1448000822`, owner ID
  `111443837`, verified with the GitHub connector on 20 November 2026.
- Project `cpe334-dinequeue-514914`, region `asia-southeast1`.
- New identity `dinequeue-deployer`; no downloaded/user-managed service-account key.
- Cloud Run Developer on the project, Artifact Registry Writer only on repository
  `dinequeue`, Service Account User only on `dinequeue-runtime` and `dinequeue-jobs`.
  The jobs identity is necessary when the later release updates migration jobs.
  No Owner/Editor, Secret Accessor, Cloud SQL administration or project IAM writer
  grant is part of this plan. Existing builder/runtime/jobs grants stay separate.
- WIF pool `github-actions`, OIDC provider `github-main`, Google subject mapped
  to `assertion.sub`, repository attribute mapped to `assertion.repository_id`.
- Provider condition requires exact numeric repository/owner IDs, main ref,
  `bbxndg/DineQueue/.github/workflows/deploy.yml@refs/heads/main` and the `demo`
  environment subject. Determine subject format from repository settings or
  follow the guide's immutable-first approach; never remove the other conditions.
- Workload Identity User binding is on the deployer account only and restricts
  principalSet membership to the numeric repository ID.
- Environment `demo`: selected deployment branches/tags policy, one branch rule
  `main`, no tag rule. Four environment variables use the course names exactly.
- Untrusted PR CI receives no deployment identity, OIDC permission or cloud key.

## Merge and evidence acceptance

For main only: active branch ruleset, empty bypass list,
restricted deletions/force pushes, PR plus one teammate approval, required actual
GitHub Actions check `test-and-docker`, branch up to date before merge.
Select the existing completed check from GitHub, not an unverified display name.
Staging integration still uses teammate review. This EP6 branch targets
`gcp-staging`; the user deferred the release PR into main until EP7 is ready.
Do not open a separate EP6 release PR or claim main has these docs before merge.

| Criterion | Required proof | Current result |
|---|---|---|
| Keyless identity configured | Provider condition, deployer scoped IAM and no user-managed keys | User reports setup complete; provider/IAM/key export not independently inspected |
| Environment restricted | `demo`, main branch rule, four variables | Screenshot attached to Issue #17; selected branch main and four variable names visible, some values redacted |
| Main gate active | Ruleset targets, enforcement, approval and check settings | protect-main Active, main only, empty bypass list, one approval and strict test-and-docker requirement visible |
| Bad PR blocked | Red required check AND blocked merge box on throwaway main PR | PR #35: failed required check and disabled merge captured; closed without merging |
| Valid PR accepted | Teammate approval, green required check, reviewed merge | Pending current documentation integration; main release deferred to EP7 |
| WIF authentication accepted/rejected | Actual Actions result and restricted negative case; no token dumps | NOT RUN; coordinate with EP7 |

Do not fabricate failures or approvals. Do not merge the gate-proof PR or change
the accepted release to make it fail. The optional direct-main-push proof is not
needed for the primary gate demonstration. Mask personal details/project numbers
in the course screenshots; never capture tokens or secret values.

See [the EP6 runbook](ep6-runbook.md) for exact settings and evidence tracking.
