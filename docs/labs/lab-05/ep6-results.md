# EP6 — main merge gate and deployment identity setup evidence

Recorded: 21 November 2026 (Asia/Bangkok).
Tracking: [Issue #17](https://github.com/bbxndg/DineQueue/issues/17).
Documentation branch: `feat/lab5-ep6-identity-merge-gates`.
Baseline: main/staging `7269e9ed28b10293a80f6f2c58c9c8a64937da99`.

## Scope and evidence sources

The user completed the Console/GitHub walkthrough and moved Issue #17 to Done.
The GitHub connector confirms it is closed as completed. This record combines
the user's report, six locally inspected screenshots already attached by the
user to that issue, and read-only GitHub PR/Actions metadata. It does not claim
the agent executed Google Cloud IAM commands or an OIDC authentication test.

On 21 November the user chose the course's **main-only** ruleset scope. The
earlier suggestion to protect `gcp-staging` is superseded. The original Issue #17
body still mentions both branches and has all checkboxes selected; that historical
issue text does not override this explicit scope decision or prove a token exchange.
No issue edits/reopening or extra ruleset were performed by this documentation work.

## Verified GitHub configuration

| Check | Observed result | Evidence |
|---|---|---|
| Environment deployment restriction | Selected branches/tags; one branch `main`, zero tags | [6.4 screenshot](https://github.com/bbxndg/DineQueue/issues/17#issuecomment-7059841621) |
| Environment variable inventory | `GCP_PROJECT_ID`, `GCP_REGION`, `WIF_PROVIDER`, `WIF_SERVICE_ACCOUNT` present; region `asia-southeast1`; parts of values redacted | [6.4 screenshot](https://github.com/bbxndg/DineQueue/issues/17#issuecomment-7059841621) |
| No environment secrets at capture time | Screenshot says this environment has no secrets | Same 6.4 screenshot; applies only to this environment at capture time |
| Ruleset enforcement/target | `protect-main`, Active, default branch target resolves to `main` | [6.5 screenshots/export](https://github.com/bbxndg/DineQueue/issues/17#issuecomment-7059856918) |
| Bypass | Empty list, also `bypass_actors: []` in captured ruleset export | Same 6.5 evidence |
| Required review | One approving review; PR rule enabled | Same 6.5 evidence |
| Required CI | `test-and-docker`; strict up-to-date policy true | Same 6.5 evidence |
| Branch protections | `deletion` and `non_fast_forward` rules present | Same 6.5 evidence |

The environment screenshot's path/name is partially redacted. The walkthrough
specified `demo`; the exact full stored variable values and environment name
were not separately queried through an authenticated environment API in this audit.

## Actual intentional-failure proof

- PR: [#35](https://github.com/bbxndg/DineQueue/pull/35), base `main`, branch
  `test/lab5-ep6-prove-gate`, closed **without merging**, confirmed by GitHub metadata.
- Head SHA: `a9c7faad7cd20b99e1292e40bcb2270103aad817`.
- Actions run: [CI & Docker Build #15](https://github.com/bbxndg/DineQueue/actions/runs/44787579607).
- Actual job: [`test-and-docker`](https://github.com/bbxndg/DineQueue/actions/runs/44787579607/job/136466392737), completed with conclusion `failure`.
- The user supplied local Node test output: one test, zero pass, one intentional
  failure in `apps/web/tests/ep6-gate-proof.test.mjs`.
- Hosted screenshot shows the web step running three tests: two pass, one fail,
  with exit code 1. Connector metadata confirms the web check step failed and
  subsequent production build/browser steps were skipped. These skipped checks
  are not fresh regression PASS results.
- [6.6 screenshots](https://github.com/bbxndg/DineQueue/issues/17#issuecomment-7059870565)
  show a red `test-and-docker` check explicitly labelled **Required**, a disabled
  merge button, and **Merging is blocked**. The displayed explanatory text also
  cites the missing approval; the captured ruleset independently requires both
  that approval and the failing check. No approval-only negative test is claimed.
- User-supplied terminal output confirms removal of the temporary worktree and
  local branch. Remote branch deletion was not separately verified by this record.

No intentional-failure test belongs to this documentation branch or the accepted
release. The evidence above is from the already completed temporary proof, not
a new failed CI run fabricated for documentation.

## IAM/WIF status and EP7 handoff

The user reports completing the guided setup: deployer account, project Cloud
Run Developer, Artifact Registry Writer limited to `dinequeue`, Service Account
User on runtime/jobs identities, and pool/provider impersonation for the repo.
The runbook records the exact intended condition and role scopes. No Console
provider/IAM export or key-list screenshot is attached to Issue #17's three
evidence comments, so these settings are user-reported rather than independently
verified here. No service-account key was created or added by this work.

**Real WIF authentication success and denial: NOT RUN in EP6.**
Issue #17's checked authentication boxes must not be cited as an Actions auth log.
EP7 will run the reviewed `deploy.yml` on main in environment `demo` and record
actual exchange/deployment results; confirm the selected immutable subject format
then. Do not broaden the provider condition to make an unrelated PR authenticate.

Positive main-gate proof (green required check plus teammate approval and reviewed
merge) remains to be recorded on the combined EP7 release PR. This documentation
PR targets `gcp-staging` only; the user explicitly deferred a separate EP6 release
PR into main. After its teammate review/merge, EP7 can branch from updated staging.
Do not mark this PR or the later main release merged before that actually occurs.

## Documentation verification

Only Markdown changed. Local checks for this update cover whitespace, local
Markdown link targets, accidental credential markers and staged file scope.
No app tests, Docker build, cloud mutation, redeployment or new OIDC workflow
was needed for this documentation update. Hosted CI for this PR is a separate
result to be linked after opening it.

The deferred EP5 live deployment/public smoke and reported manual browser journey
are recorded separately in [EP5 cloud results](ep5-cloud-results.md), retaining
the deployed EP5 SHA rather than this later documentation commit.
