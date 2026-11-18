# Course starter deployment reference — EP4 only

The seven files `deploy.sh`, `release.sh`, `teardown.sh`, `smoke_test.py` and
`tests/*` were imported from the supplied `toktickit-lab5-starter/deploy` folder
for Student Guide Episode 4's offline checks. Their original content is retained
so the reference tests still describe the code they exercise.

**These are not DineQueue deployment commands yet. Do not run the cloud scripts
against a real project.** The reference uses TokTickIT resource names, a unified
Node/Prisma image, a different authentication protocol, different health routes
and a database connection format incompatible with our production validation.
Its teardown script deletes cloud data. Importing/testing these files does not
create resources or prove that DineQueue has deployed.

CI may run only:

```bash
for script in deploy/deploy.sh deploy/release.sh deploy/teardown.sh; do
  bash -n "$script"
done
bash deploy/tests/test_deploy_stub.sh
bash deploy/tests/test_release_stub.sh
python3 -m unittest discover -s deploy/tests -p 'test_smoke.py'
```

The shell tests put fake `gcloud`/`openssl` commands on PATH; the Python tests mock
the network. Their results are starter-reference test results, separate from
DineQueue's Go/browser regression counts. Shell files are not marked executable
to discourage accidental direct invocation; this does not make `bash file.sh`
safe to run against live GCP.

Before Episode 5, adapt the resource names, separate API/web/database-tool images,
build commands, runtime origin/API routing, migration job, production DB TLS and
safe account provisioning for DineQueue. Replace the reference smoke paths with
our actual health/login/session/CSRF behavior and update its offline tests. Review
the resulting dry run before any authorized live command. Release automation is
Episode 7; destructive teardown requires separate, intentional authorization.
