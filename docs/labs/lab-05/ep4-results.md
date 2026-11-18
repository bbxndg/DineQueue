# Lab 5 Episode 4 — Local CI validation and pending hosted evidence

Date: 18 November 2026, Asia/Bangkok. [Issue #15](https://github.com/bbxndg/DineQueue/issues/15).

**Status: local checks passed; hosted Actions and reviewed release evidence are pending.**
The user will open the PR after local verification. GitHub's connected integration
returned `403 Resource not accessible by integration` when creating a draft PR;
that did not prevent pushing the feature branch. No hosted CI PASS, peer review,
enforced branch protection or merge is claimed.

## Snapshots and changes

- Branch: `ci/lab5-15-ep4-pr-checks`, based on EP3 merge `3c80163` in `gcp-staging`.
- Pre-implementation contract commit: `47d881e` ([contract](ep4-contract.md)).
- Full app/Go/web/Docker/browser validation used an independent clean clone of
  `28f0a2bd72526a05deb2ad3395fa3b15a0912d99`.
- `bd3880c0ae68bb9f30b974c469df2649b4774a02` subsequently corrected the workflow's
  Bash syntax-check loop and two matching documentation examples. Its workflow
  validation and entire offline-check step were rerun successfully. A direct diff
  confirmed no change to app/API, web/browser tests, CI helper, dependencies,
  production Compose or image inputs from the fully tested snapshot.
- Later results/log documentation commits do not change those tested inputs.

The workflow is `CI & Docker Build`, job `test-and-docker`, targeting PRs into
`gcp-staging` and `main`. It retains reusable/manual triggers, `contents: read`,
checkout without persisted credentials, pinned official Actions, a bounded
timeout and cancellation of superseded runs. It never authenticates to GCP.

The supplied course `deploy/` files were reviewed and imported byte-for-byte,
with a separate [provenance/limitations README](../../../deploy/README.md).
They are **TokTickIT reference scripts for offline tests**, not DineQueue cloud
deployment tools yet. Student Guide EP5 section 5.0 requires stack-specific
adaptation before live use. The CI invokes syntax checks, stubbed commands and
mocked network tests only; no cloud script was run against real GCP.

## Local execution

The independent checkout had no copied developer `.env`, build output or host
dependencies. A fresh `npm ci --ignore-scripts --no-audit --no-fund` installed the
existing lockfile without version changes. Host Node was 22.17.0 (within the repo
engine range); production web images used Node 22.23.4. The hosted workflow will
use `.nvmrc` (22.23.4); hosted runtime setup has not yet been exercised.

Go checks used the cloned source in the development/toolchain image, reporting
`go version go1.26.10 linux/amd64`. A dedicated PostgreSQL 18.6 container with
tmpfs data, synthetic service-only credentials and a new network served the empty
Go test database. Docker-network access replaced only the loopback hostname/port
in the private test URL. Application schema fixtures remain isolated by the tests.

The production-built browser app ran as a separate CI-created Compose project
on host ports 3042/8094, with fixture PostgreSQL published only on loopback 55442.
Those local overrides avoid the developer's active 3000/8080 and EP3 3002/8082
projects. The Go test database was separately published on loopback 35432.

`prepare-env.mjs` generated private per-run config under `/tmp/dinequeue-ep4`
without reading `.env` or GCP credentials. Its local run used:

```bash
RUNNER_TEMP=/tmp/dinequeue-ep4 CI_PG_PORT=35432 CI_WEB_HOST_PORT=3042 CI_API_HOST_PORT=8094 CI_DB_HOST_PORT=55442 GITHUB_ENV=/tmp/dinequeue-ep4/exports GITHUB_ACTIONS=false node scripts/ci/prepare-env.mjs
```

The exports file contains only private file paths and a unique Compose project
name. Generated credential files are not included in this report or Git.

| Check | Actual result | Evidence |
|---|---|---|
| actionlint 1.7.12 on final workflow | Exit 0; official release archive's published SHA-256 verified before use | Checksum `8aca8db96f1b94770f1b0d72b6dddcb1ebb8123cb3712530b08cc387b349a3d8` |
| Bash syntax | Each of the three reference scripts checked individually, exit 0 | [Offline output](evidence/ep4/offline-checks.txt) |
| Course deploy stub tests | Six scenarios passed, including failed migration and dry-run behavior | [Offline output](evidence/ep4/offline-checks.txt) |
| Course release stub tests | Five scenarios passed, including rollback and failure gates | [Offline output](evidence/ep4/offline-checks.txt) |
| Course Python smoke unit tests | 3/3 passed; network mocked | [Offline output](evidence/ep4/offline-checks.txt) |
| CI configuration/privacy unit tests | 7/7 passed, including invalid ports/identifiers, private file permissions, mask commands, fresh credentials and ignoring developer config | [Offline output](evidence/ep4/offline-checks.txt) |
| Go formatting, module download, vet | All exited 0 | [Go output](evidence/ep4/go-checks.txt) |
| Go integration/race suite | 53 top-level functions passed, zero failures, no race reports; excludes subtests from that count | [Go output](evidence/ep4/go-checks.txt) |
| Web tests | 2/2 passed | [Web output](evidence/ep4/web-checks.txt) |
| Web lint/build/typecheck | All exited 0; build generated route types before typecheck | [Web output](evidence/ep4/web-checks.txt) |
| Production Compose config and all image targets | Exit 0; no registry push | [Build output](evidence/ep4/docker-build.txt) |
| Migration/explicit seed/start | Migration exited 0; DB/web healthy; API running; seed completed | [Startup output](evidence/ep4/app-start.txt) |
| Existing browser suite | 20/20 passed, one worker, matching Playwright Chromium v1254 (Chrome for Testing 158.0.8127.18), about 1.4 minutes | [Browser output](evidence/ep4/browser-checks.txt) |
| `git diff --check` | Exit 0 after whitespace normalization of copied text logs | Git inspection |

The Go command sequence was `gofmt -l .` (fail if nonempty), `go mod download`,
`go vet -tags=integration ./...`, and
`go test -tags=integration -race -count=1 -v ./...`. It used the independent clone
inside a Go toolchain container and the disposable Go PostgreSQL service.

The web sequence was `npm ci --ignore-scripts --no-audit --no-fund`, `npm test`,
`npm run lint`, `npm run build`, `npm run typecheck`. Installation emitted a
transitive ESLint deprecation warning but completed successfully; this is not a
dependency vulnerability audit.

Production images were built and started using the generated environment path,
unique project name and `compose.production.yaml`; startup additionally used the
generated loopback DB-port override for existing browser fixtures. The same
options were used for the explicit `run --rm seed`. No app source mounts were
used. As documented in EP3, this is local production-image verification with
API `APP_ENV=development` for HTTP/non-TLS local PostgreSQL; production HTTPS/TLS
and disabled-production-seed validation were not relaxed.

The matching browser was downloaded by the pinned Playwright package to temporary
storage using `npx --no-install playwright install chromium`, without installing
host OS packages. The hosted workflow additionally uses `--with-deps` on its
disposable Ubuntu runner. Browser execution used the actual repository config:

```bash
PLAYWRIGHT_BROWSERS_PATH=/tmp/dinequeue-ep4/browsers CI=true node --env-file="$DINEQUEUE_BROWSER_ENV" node_modules/@playwright/test/cli.js test --config apps/web/playwright.config.ts
```

No browser trace/video/screenshot or private environment artifact is uploaded by
the workflow. Go, browser, CI-helper and course-reference counts above are
separate categories and must not be presented as a single unit-test count.

## Credential checks and cleanup

A known-value scan checked configured developer/generated browser credentials
against tracked/pending files, Git patch history, 49 saved-image archive members,
the disposable app logs and these evidence files. No matching values were found.
Selected high-confidence private-key/token patterns were also absent in the
tracked/pending files. Synthetic service-only/reference test values are explicitly
identified in code. This limited scan does not establish that all possible secrets
or vulnerabilities are absent.

The disposable EP4 Compose app, its test-only volume, Go PostgreSQL container and
network, and generated private configuration were removed after verification.
The developer's dev and EP3 containers remained running. Container status before
cleanup is preserved in [the actual status output](evidence/ep4/container-status.txt).

## RED cases and corrections

- The first helper-test run observed two real failures: invalid project identifiers
  left a generated directory, and local port overrides were not propagated. After
  moving validation before filesystem writes and adding consistent overrides,
  all seven final helper tests passed. No production app rule was changed.
- actionlint initially rejected the numeric service-port property access. It was
  changed to the string key `ports['5432']`; the final workflow validates.
- The imported example `bash -n file1 file2 ...` parses only the first script.
  The final workflow loops over each file; the corrected step was executed.

These are local setup/code-check results, not the separate hosted intentional
failure demonstration required by Issue #15.

## Pending GitHub evidence and completion

1. The user opens the implementation PR into `gcp-staging` with `Part of #15`.
2. Capture the real automatically triggered `test-and-docker` run URL, conclusion,
   step results, PR head SHA and checked-out merge SHA. Fix any hosted-only failure
   before marking it passed.
3. Use a separate temporary branch/PR containing an intentionally failing
   synthetic Go test; capture the actual red check and close without merging.
   The implementation branch contains no deliberately failing file.
4. Obtain teammate review and merge into `gcp-staging`, then open the reviewed
   release PR to `main` and capture the same passing check before merge, as the
   Student Guide requires for EP4 completion. Keep Issue #15 open until this is
   evidenced. No merge or author approval is performed on the teammate's behalf.

Actual branch-protection enforcement belongs to EP6. A red run alone does not
prove a merge is blocked. GCP, deployment/release automation, native Windows
execution and a full dependency vulnerability audit are not performed in EP4.

References: [actionlint official release](https://github.com/rhysd/actionlint/releases/tag/v1.7.12),
[GitHub workflow syntax](https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-syntax).
