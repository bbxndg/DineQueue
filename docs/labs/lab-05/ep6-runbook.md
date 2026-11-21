# Episode 6 — DineQueue Console and GitHub runbook

Tracking: [Issue #17](https://github.com/bbxndg/DineQueue/issues/17).
Status updated 21 November 2026: user completed Console/GitHub steps; environment,
main ruleset and failing PR evidence were inspected. [Actual results](ep6-results.md)
distinguish screenshots/API observations from user-reported IAM/WIF setup.
The following is the reproducible runbook, not a claim that every step was rerun.
Follow the [contract](ep6-contract.md). Keep the existing EP5 service up.

## 6.1 — Verified identifiers

| Item | DineQueue value |
|---|---|
| GitHub repository | `bbxndg/DineQueue` |
| Repository ID | `1448000822` |
| Owner ID | `111443837` |
| Project ID | `cpe334-dinequeue-514914` |
| Region | `asia-southeast1` |
| Runtime service account | `dinequeue-runtime@cpe334-dinequeue-514914.iam.gserviceaccount.com` |
| Jobs service account | `dinequeue-jobs@cpe334-dinequeue-514914.iam.gserviceaccount.com` |
| New deployer service account | `dinequeue-deployer@cpe334-dinequeue-514914.iam.gserviceaccount.com` |

GitHub numeric IDs were read from the connected repository metadata, not copied
from another team. If GitHub CLI is already authenticated with repository admin
rights, inspect subject configuration with this read-only command:

```bash
gh api repos/bbxndg/DineQueue/actions/oidc/customization/sub
```

Do not assume that a missing `use_immutable_subject` field is `false`. If it cannot
be determined, follow the guide's immutable-first format below and keep actual
authentication pending until the main deploy workflow runs.

## 6.2 — Deployer account (Google Cloud Console)

1. Select project `cpe334-dinequeue-514914`.
2. APIs & Services → Library: enable **IAM Service Account Credentials API**
   (`iamcredentials.googleapis.com`) and **Security Token Service API**
   (`sts.googleapis.com`). If already enabled, continue without changing them.
3. IAM & Admin → Service Accounts → Create service account: ID `dinequeue-deployer`.
   Grant **Cloud Run Developer** (`roles/run.developer`) at the project step.
   Leave optional principals empty; finish without creating a key.
4. Artifact Registry → Repositories → `dinequeue` in `asia-southeast1` → Permissions:
   grant the deployer email **Artifact Registry Writer**
   (`roles/artifactregistry.writer`) on this repository only.
5. IAM & Admin → Service Accounts → `dinequeue-runtime` → Principals with access:
   grant the deployer email **Service Account User** (`roles/iam.serviceAccountUser`).
6. Repeat step 5 on `dinequeue-jobs`, because the future release updates migration
   jobs using that identity. Do not grant this role across every service account.
7. Inspect deployer Keys: no user-managed keys. Never choose Add key.

Capture scoped role settings and the empty user-managed-key list. Existing
`dinequeue-builder` belongs to EP5 Cloud Build; it is not the GitHub deployer.
This account will not run the project/bootstrap/IAM/secret setup in `deploy.sh`.

## 6.3 — WIF provider and impersonation

IAM & Admin → Workload Identity Federation:

1. Create enabled pool `github-actions` (confirm the pool ID).
2. Add OpenID Connect provider `github-main` (confirm the provider ID).
3. Issuer `https://token.actions.githubusercontent.com/`; default audience;
   leave JWK file empty.
4. Attribute mapping: `google.subject` → `assertion.sub`;
   `attribute.repository_id` → `assertion.repository_id`.
5. Paste the following attribute condition if immutable subjects are enabled
   or not yet known (the guide's initial choice):

```text
assertion.repository_id == '1448000822' && assertion.repository_owner_id == '111443837' && assertion.ref == 'refs/heads/main' && assertion.workflow_ref == 'bbxndg/DineQueue/.github/workflows/deploy.yml@refs/heads/main' && assertion.sub == 'repo:bbxndg@111443837/DineQueue@1448000822:environment:demo'
```

If repository metadata explicitly says `use_immutable_subject: false`, replace
only the final subject comparison with:

```text
assertion.sub == 'repo:bbxndg/DineQueue:environment:demo'
```

Do not paste both variants. Save, then read back every condition.

6. Pool → Grant access → Service Account impersonation → select
   `dinequeue-deployer`; choose only identities matching attribute
   `repository_id` = `1448000822`. This grants Workload Identity User
   (`roles/iam.workloadIdentityUser`) on the deployer to that repository's
   principalSet, not to the entire pool.
7. Copy the provider's full resource name for the GitHub variable below. The
   project component uses the **project number**, not the project ID. It can also
   be read in Cloud Shell without changing anything:

```bash
gcloud iam workload-identity-pools providers describe github-main \
  --workload-identity-pool=github-actions \
  --location=global \
  --project=cpe334-dinequeue-514914 \
  --format="value(name)"
```

## 6.4 — GitHub environment

Repository → Settings → Environments → New environment → `demo`:

- Deployment branches and tags → Selected branches and tags → add a **branch**
  rule `main`. Do not add tags or wildcard branch patterns.
- Add these as **Environment variables**, not repository secrets:

| Variable | Value |
|---|---|
| `GCP_PROJECT_ID` | `cpe334-dinequeue-514914` |
| `GCP_REGION` | `asia-southeast1` |
| `WIF_PROVIDER` | Copy the full provider resource name from 6.3: `projects/PROJECT_NUMBER/locations/global/workloadIdentityPools/github-actions/providers/github-main` |
| `WIF_SERVICE_ACCOUNT` | `dinequeue-deployer@cpe334-dinequeue-514914.iam.gserviceaccount.com` |

`PROJECT_NUMBER` is a placeholder to replace from the real provider; do not save
the literal placeholder. The provider subject, GitHub environment and future
workflow's `environment: demo` must match exactly. Do not add any JSON key.
Capture environment restrictions and variable names; redact project numbers and
personal details for course submission screenshots.

## 6.5 — Active merge gates

Settings → Rules → Rulesets → New branch ruleset:

1. Name `protect-main`; Enforcement **Active**; bypass list **empty**.
2. Target `main` (include default branch or exact branch pattern).
3. Restrict deletions; block force pushes.
4. Require pull request before merging; required approvals **1** (this is a team).
5. Require status checks; require branch up to date; Add checks → select the
   actual completed **`test-and-docker`** check from GitHub Actions. Its workflow
   is named `CI & Docker Build`; do not rename the existing workflow/job.
6. Save and read back the active rule. This is **main only**, as required by the
   course and confirmed by the user on 21 November 2026. No staging ruleset is
   required; the earlier extra staging requirement is superseded.

The rules make review/checks mandatory even for the ordinary administrator merge
path. Do not add a bypass to complete the demonstration. Staging remains the
integration branch; this runbook does not require protecting it.

## 6.6 — Real failing-check proof (separate PR)

Only after the rules are active, create a throwaway branch from current main
such as `test/lab5-ep6-prove-gate`. Introduce one intentionally failing synthetic
test that the existing CI executes, and confirm that targeted test fails locally.
Commit/push this branch separately; open a normal PR **into main**. Wait for
`test-and-docker` to fail and inspect the merge box.

Capture both the red **Required** check and the blocked merge message, plus PR
and Actions run URLs. This differs from EP4's red CI example, which did not prove
enforcement. Close without merging; delete the throwaway branch after evidence
is saved. Do not put that test into this EP6 documentation branch or main.
Use the ordinary reviewed documentation PR into staging for integration evidence.
The user deferred the release PR from staging to main until EP7 is ready; that
later PR provides the positive main-gate proof. Do not release these docs alone.

## 6.7 — Evidence and Issue #17 completion

| Item | Status / evidence |
|---|---|
| Numeric repository and owner IDs | Verified through GitHub metadata, 20 Nov 2026 |
| APIs enabled, deployer and resource-scoped bindings | User reports completed; see evidence limitations in EP6 results |
| WIF mapping/condition and restricted impersonation | User reports configured; actual token exchange NOT RUN |
| No deployer user-managed keys | No key created in this work; no Console key-list evidence attached |
| `demo` main restriction and four variables | Screenshot reviewed; linked in EP6 results |
| Active main-only ruleset | Active/target/bypass/rules screenshots reviewed |
| Real red required check blocks merge | PR #35 and failed Actions run; closed without merge |
| Valid PR reviewed and merged through the gate | Pending; main release combined with EP7 |
| Real WIF authentication and rejection evidence | NOT RUN; first deploy-workflow use is EP7 |

Record URLs, exact tested SHA and screenshots for completed rows. The issue adds
successful/failed authentication evidence beyond merely creating the provider:
keep those evidence rows NOT RUN until real Actions results exist. The owner
already closed Issue #17; do not reinterpret its checked boxes as authentication
logs or reopen it automatically. Follow-up belongs to EP7.
Do not create an unrestricted authentication workflow to work around a main-only
`deploy.yml` condition. EP7 will add that reviewed workflow and run it on main.

References: course Student Guide EP6 §§6.1–6.7;
[Google WIF deployment pipelines](https://docs.cloud.google.com/iam/docs/workload-identity-federation-with-deployment-pipelines),
[GitHub rulesets](https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/managing-rulesets/creating-rulesets-for-a-repository),
[GitHub environments](https://docs.github.com/en/actions/how-tos/deploy/configure-and-manage-deployments/manage-environments).
