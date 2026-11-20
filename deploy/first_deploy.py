"""DineQueue EP5 manual deployment. Defaults to a zero-network dry run.

Only --apply plus explicit project/public-access confirmations performs GCP calls.
No credential value is printed; subprocess errors are deliberately generic.
"""
import argparse
import io
import json
import os
import random
from pathlib import Path
import re
import secrets
import shlex
import subprocess
import tarfile
import tempfile
import time
from urllib.parse import urlencode, urlparse

ROOT = Path(__file__).resolve().parents[1]


class DeploymentError(RuntimeError):
    pass


class GCloud:
    def __init__(self, project):
        self.project = project

    def new_secret(self):
        return secrets.token_hex(24)

    def call(self, *args, data=None):
        # Arguments may contain a generated SQL password: never print raw argv,
        # stderr, successful secret output or CalledProcessError representations.
        print('GCP:', ' '.join(args[:3]))
        # Only repeat IAM binding additions, never builds/jobs/secret writes.
        for attempt in range(5):
            # Global flags must precede --container, which starts a scoped parser.
            result = subprocess.run(['gcloud', '--project', self.project, '--quiet', *args],
                                    input=data, capture_output=True)
            if not result.returncode:
                return result.stdout.decode().strip()
            if attempt == 4 or not self._retryable_binding(args, result.stderr):
                break
            print('GCP: retrying IAM binding after a transient error')
            time.sleep(2 ** attempt + random.uniform(0, 1))
        raise DeploymentError('GCP command failed at ' + ' '.join(args[:3]) + '; inspect the resource safely in Console.')

    @staticmethod
    def _retryable_binding(args, stderr):
        if 'add-iam-policy-binding' not in args:
            return False
        if re.search(rb'\b(PERMISSION_DENIED|UNAUTHENTICATED|INVALID_ARGUMENT|FORBIDDEN)\b|\b403\b', stderr):
            return False
        if re.search(rb'\b(INTERNAL|UNAVAILABLE|ABORTED)\b', stderr):
            return True
        member = next((arg.split('serviceAccount:', 1)[1] for arg in args
                       if arg.startswith('--member=serviceAccount:')), None)
        return bool(member and ('Service account ' + member + ' does not exist.').encode() in stderr)

    def describe(self, *args):
        result = subprocess.run(['gcloud', '--project', self.project, '--format=json', *args], capture_output=True)
        if result.returncode:
            # Storage uses a different 404 message from other gcloud groups.
            # Accept only the exact requested bucket, never an arbitrary 404
            # substring that could hide a permission/network failure.
            bucket_missing = (
                len(args) >= 4 and args[:3] == ('storage', 'buckets', 'describe')
                and result.stderr.strip() == (
                    'ERROR: (gcloud.storage.buckets.describe) ' + args[3] + ' not found: 404.'
                ).encode()
            )
            service_missing = (
                len(args) >= 4 and args[:3] == ('run', 'services', 'describe')
                and result.stderr.strip() == (
                    'ERROR: (gcloud.run.services.describe) Cannot find service [' + args[3] + ']'
                ).encode()
            )
            if bucket_missing or service_missing or b'NOT_FOUND' in result.stderr or re.search(rb'HTTPError[ :]+404\b', result.stderr):
                return None
            raise DeploymentError('Resource discovery failed at ' + ' '.join(args[:3]) + '; refusing to treat permission/network errors as missing resources.')
        try:
            return json.loads(result.stdout)
        except (ValueError, TypeError):
            raise DeploymentError('Invalid resource discovery response.') from None


def settings(env):
    project = env.get('PROJECT_ID', '')
    region = env.get('REGION', 'asia-southeast1')
    if not re.fullmatch(r'[a-z][a-z0-9-]{4,28}[a-z0-9]', project):
        raise DeploymentError('Set PROJECT_ID to a valid explicit GCP project ID.')
    if not re.fullmatch(r'[a-z]+-[a-z]+[0-9]', region):
        raise DeploymentError('Invalid REGION.')
    sha = env.get('SOURCE_SHA', '')
    if not re.fullmatch(r'[a-f0-9]{40}', sha):
        raise DeploymentError('Set SOURCE_SHA to the full reviewed main commit SHA.')
    return project, region, sha


def plan(project, region, sha, demo):
    return {
        'mode': 'DRY RUN ONLY — no GCP calls or credential generation',
        'project': project, 'region': region, 'source_sha': sha,
        'registry': 'dinequeue', 'sql_instance': 'dinequeue-pg',
        'sql': {'version': 'POSTGRES_18', 'edition': 'enterprise', 'tier': 'db-f1-micro',
                'storage_gb': 10, 'storage_auto_increase': False, 'authorized_networks': []},
        'database': 'dinequeue_demo', 'source_bucket': project + '-dinequeue-build',
        'service': 'dinequeue', 'containers': {
            'web': {'image': 'web:' + sha, 'ingress_port': 8080, 'api_origin': 'http://127.0.0.1:8081', 'secrets': []},
            'api': {'image': 'api:' + sha, 'port': 8081, 'secrets': ['dinequeue-database-url:1']},
        },
        'service_identities': ['dinequeue-runtime', 'dinequeue-jobs', 'dinequeue-builder'],
        'images': ['api', 'web', 'db-tools'], 'deploy_by': 'resolved sha256 digest',
        'limits': {'min_instances': 0, 'max_instances': 1, 'concurrency': 10},
        'jobs': ['dinequeue-migrate'] + (['dinequeue-provision-demo'] if demo else []),
        'order': ['enable APIs', 'registry + SQL + database', 'identities + least-privilege bindings',
                  'generate/reuse secrets (never print)', 'build tracked main source only',
                  'resolve digests', 'migrate --wait (failure aborts)',
                  'optional guarded provisioning --wait', 'deploy web + API',
                  'set exact HTTPS origin', 'grant public web ingress'],
        'demo': demo, 'credentials': 'Secret Manager only; no values in plan/output',
        'pending': ['real deployment and HTTPS smoke evidence', 'EP6 GitHub identity/merge gates',
                    'EP7 release automation', 'EP8 restart/redeploy evidence'],
    }


def verify_source(sha):
    def git(*args):
        result = subprocess.run(['git', *args], cwd=ROOT, capture_output=True, check=True)
        return result.stdout.decode().strip()
    if git('branch', '--show-current') != 'main' or git('rev-parse', 'HEAD') != sha:
        raise DeploymentError('Live execution requires the reviewed main checkout at SOURCE_SHA.')
    if git('status', '--porcelain'):
        raise DeploymentError('Live execution requires a clean tracked/untracked working tree.')
    git('fetch', 'origin', 'main')
    if git('rev-parse', 'origin/main') != sha:
        raise DeploymentError('SOURCE_SHA does not match origin/main.')
    for path in git('ls-files').splitlines():
        parts = Path(path).parts
        if any(p in {'.aws', '.npmrc'} or (p.startswith('.env') and p != '.env.example') for p in parts):
            raise DeploymentError('Tracked secret/config file is forbidden in the build source.')


def source_stage(destination, sha):
    # Upload an archive of the reviewed commit, never the developer's directory.
    data = subprocess.run(['git', 'archive', '--format=tar', sha], cwd=ROOT,
                          check=True, capture_output=True).stdout
    with tarfile.open(fileobj=io.BytesIO(data)) as archive:
        archive.extractall(destination, filter='data')


def secret_value(g, name, value_factory):
    existing = g.describe('secrets', 'describe', name)
    if existing is not None:
        # Never reuse an empty/disabled secret or silently add a different password.
        version = g.describe('secrets', 'versions', 'describe', '1', '--secret', name)
        if not version or version.get('state') != 'ENABLED':
            _missing_secret()
        return
    value = value_factory()
    g.call('secrets', 'create', name, '--replication-policy=automatic')
    g.call('secrets', 'versions', 'add', name, '--data-file=-', data=value.encode())


def _missing_secret():
    raise DeploymentError('Expected secret version 1 is missing; reconcile credentials explicitly before retrying.')


def db_secret(g, connection):
    name = 'dinequeue-database-url'
    if g.describe('secrets', 'describe', name) is not None:
        version = g.describe('secrets', 'versions', 'describe', '1', '--secret', name)
        if not version or version.get('state') != 'ENABLED':
            _missing_secret()
        return
    users = json.loads(g.call('sql', 'users', 'list', '--instance=dinequeue-pg', '--format=json'))
    if any(user.get('name') == 'dinequeue' for user in users):
        raise DeploymentError('DB user exists without its database URL secret; refusing implicit password rotation. Reconcile the partial setup first.')
    password = g.new_secret()
    g.call('sql', 'users', 'create', 'dinequeue', '--instance=dinequeue-pg', '--password=' + password)
    value = 'postgres://dinequeue:' + password + '@localhost/dinequeue_demo?' + urlencode({
        'host': '/cloudsql/' + connection, 'sslmode': 'disable', 'connect_timeout': '10'})
    g.call('secrets', 'create', name, '--replication-policy=automatic')
    g.call('secrets', 'versions', 'add', name, '--data-file=-', data=value.encode())


def origin_checked(value):
    parsed = urlparse(value)
    if parsed.scheme != 'https' or not parsed.hostname or parsed.username or parsed.password or parsed.path not in ('', '/') or parsed.query or parsed.fragment:
        raise DeploymentError('Cloud Run did not return a valid public HTTPS origin.')
    return value.rstrip('/')


def deploy_service(g, region, connection, images, origin, sha):
    # Only web has a container port. The API is a non-public localhost sidecar.
    g.call('run', 'deploy', 'dinequeue', '--region=' + region,
           '--service-account=dinequeue-runtime@' + g.project + '.iam.gserviceaccount.com',
           '--execution-environment=gen2', '--set-cloudsql-instances=' + connection,
           '--min-instances=0', '--max-instances=1', '--concurrency=10', '--timeout=60',
           '--labels=source-sha=' + sha,
           '--container=web', '--image=' + images['web'], '--port=8080', '--memory=512Mi', '--cpu=1',
           '--set-env-vars=NODE_ENV=production,API_INTERNAL_URL=http://127.0.0.1:8081',
           '--depends-on=api', '--startup-probe=httpGet.path=/api/health/ready,httpGet.port=8080,periodSeconds=5,timeoutSeconds=4,failureThreshold=24',
           '--container=api', '--image=' + images['api'], '--memory=512Mi', '--cpu=1',
           '--add-volume-mount=volume=cloudsql,mount-path=/cloudsql',
           '--set-env-vars=APP_ENV=production,API_PORT=8081,APP_ORIGIN=' + origin + ',CLOUD_SQL_INSTANCE=' + connection,
           '--set-secrets=DATABASE_URL=dinequeue-database-url:1',
           '--startup-probe=httpGet.path=/health/ready,httpGet.port=8081,periodSeconds=5,timeoutSeconds=4,failureThreshold=24')


def apply(g, region, sha, demo, stage_source=source_stage):
    project = g.project
    connection = project + ':' + region + ':dinequeue-pg'
    g.call('services', 'enable', 'run.googleapis.com', 'sqladmin.googleapis.com',
           'artifactregistry.googleapis.com', 'cloudbuild.googleapis.com',
           'secretmanager.googleapis.com', 'iam.googleapis.com',
           'storage.googleapis.com', 'logging.googleapis.com')
    if g.describe('artifacts', 'repositories', 'describe', 'dinequeue', '--location=' + region) is None:
        g.call('artifacts', 'repositories', 'create', 'dinequeue', '--location=' + region, '--repository-format=docker')
    instance = g.describe('sql', 'instances', 'describe', 'dinequeue-pg')
    if instance is None:
        g.call('sql', 'instances', 'create', 'dinequeue-pg', '--database-version=POSTGRES_18',
               '--edition=enterprise', '--tier=db-f1-micro', '--region=' + region,
               '--storage-size=10GB', '--storage-type=SSD', '--no-storage-auto-increase',
               '--backup-start-time=03:00', '--ssl-mode=ENCRYPTED_ONLY')
    else:
        settings_ = instance.get('settings', {})
        if (instance.get('region') != region or instance.get('databaseVersion') != 'POSTGRES_18'
                or settings_.get('tier') != 'db-f1-micro' or str(settings_.get('dataDiskSizeGb')) != '10'
                or settings_.get('ipConfiguration', {}).get('authorizedNetworks')
                or settings_.get('ipConfiguration', {}).get('sslMode') != 'ENCRYPTED_ONLY'):
            raise DeploymentError('Existing SQL resource differs from the reviewed region/version/network plan.')
        g.call('sql', 'instances', 'patch', 'dinequeue-pg', '--no-storage-auto-increase')
    if g.describe('sql', 'databases', 'describe', 'dinequeue_demo', '--instance=dinequeue-pg') is None:
        g.call('sql', 'databases', 'create', 'dinequeue_demo', '--instance=dinequeue-pg')
    for identity in ('dinequeue-runtime', 'dinequeue-jobs', 'dinequeue-builder'):
        email = identity + '@' + project + '.iam.gserviceaccount.com'
        if g.describe('iam', 'service-accounts', 'describe', email) is None:
            g.call('iam', 'service-accounts', 'create', identity)
        if identity != 'dinequeue-builder':
            g.call('projects', 'add-iam-policy-binding', project, '--member=serviceAccount:' + email,
                   '--role=roles/cloudsql.client', '--condition=None')
    db_secret(g, connection)
    demo_keys = ('CUSTOMER', 'CUSTOMER2', 'HOST', 'OTHER_HOST')
    if demo:
        for key in demo_keys:
            secret_value(g, 'dinequeue-demo-' + key.lower().replace('_', '-') + '-password', g.new_secret)
    for identity in ('dinequeue-runtime', 'dinequeue-jobs'):
        names = ['dinequeue-database-url']
        if demo and identity == 'dinequeue-jobs':
            names += ['dinequeue-demo-' + key.lower().replace('_', '-') + '-password' for key in demo_keys]
        for name in names:
            g.call('secrets', 'add-iam-policy-binding', name, '--member=serviceAccount:' + identity + '@' + project + '.iam.gserviceaccount.com', '--role=roles/secretmanager.secretAccessor')
    builder = 'dinequeue-builder@' + project + '.iam.gserviceaccount.com'
    bucket = 'gs://' + project + '-dinequeue-build'
    # --raw uses the Storage API field names, avoiding display-format aliases.
    bucket_metadata = g.describe('storage', 'buckets', 'describe', bucket, '--raw')
    if bucket_metadata is None:
        g.call('storage', 'buckets', 'create', bucket, '--location=' + region, '--uniform-bucket-level-access', '--public-access-prevention')
    else:
        iam = bucket_metadata.get('iamConfiguration', {})
        if (bucket_metadata.get('name') != bucket.removeprefix('gs://')
                or str(bucket_metadata.get('location', '')).lower() != region
                or iam.get('uniformBucketLevelAccess', {}).get('enabled') is not True
                or iam.get('publicAccessPrevention') != 'enforced'):
            raise DeploymentError('Existing source bucket differs from the reviewed name/region/access policy; reconcile explicitly before upload.')
    g.call('storage', 'buckets', 'add-iam-policy-binding', bucket, '--member=serviceAccount:' + builder, '--role=roles/storage.objectViewer')
    g.call('artifacts', 'repositories', 'add-iam-policy-binding', 'dinequeue', '--location=' + region, '--member=serviceAccount:' + builder, '--role=roles/artifactregistry.writer')
    g.call('projects', 'add-iam-policy-binding', project, '--member=serviceAccount:' + builder, '--role=roles/logging.logWriter', '--condition=None')
    with tempfile.TemporaryDirectory(prefix='dinequeue-build-') as stage:
        stage_source(stage, sha)
        g.call('builds', 'submit', stage, '--config=' + str(Path(stage) / 'deploy/cloudbuild.yaml'),
               '--substitutions=_REGION=' + region + ',_SOURCE_SHA=' + sha,
               '--gcs-source-staging-dir=' + bucket + '/source',
               '--service-account=projects/' + project + '/serviceAccounts/' + builder)
    images = {}
    for image in ('api', 'web', 'db-tools'):
        base = region + '-docker.pkg.dev/' + project + '/dinequeue/' + image
        resolved = g.call('artifacts', 'docker', 'images', 'describe', base + ':' + sha, '--format=value(image_summary.fully_qualified_digest)')
        if not re.fullmatch(re.escape(base) + r'@sha256:[a-f0-9]{64}', resolved):
            raise DeploymentError('Could not resolve the expected immutable image digest.')
        images[image] = resolved
    existing = g.describe('run', 'services', 'describe', 'dinequeue', '--region=' + region)
    origin = origin_checked(existing['status']['url']) if existing else 'https://not-yet-public.invalid'
    common = ['--image=' + images['db-tools'], '--region=' + region,
              '--service-account=dinequeue-jobs@' + project + '.iam.gserviceaccount.com',
              '--set-cloudsql-instances=' + connection, '--max-retries=0', '--tasks=1', '--parallelism=1',
              '--task-timeout=300s', '--memory=512Mi', '--cpu=1',
              '--set-env-vars=APP_ENV=production,APP_ORIGIN=' + origin + ',CLOUD_SQL_INSTANCE=' + connection]
    g.call('run', 'jobs', 'deploy', 'dinequeue-migrate', *common,
           '--set-secrets=DATABASE_URL=dinequeue-database-url:1', '--args=migrate')
    g.call('run', 'jobs', 'execute', 'dinequeue-migrate', '--region=' + region, '--wait')
    if demo:
        provision_common = common[:-1] + [common[-1] + ',ENABLE_DEMO_PROVISION=true,CONFIRM_DEMO_DATABASE=dinequeue_demo']
        bindings = ['DATABASE_URL=dinequeue-database-url:1'] + ['DEMO_' + key + '_PASSWORD=dinequeue-demo-' + key.lower().replace('_', '-') + '-password:1' for key in demo_keys]
        g.call('run', 'jobs', 'deploy', 'dinequeue-provision-demo', *provision_common,
               '--set-secrets=' + ','.join(bindings), '--args=provision-demo')
        g.call('run', 'jobs', 'execute', 'dinequeue-provision-demo', '--region=' + region, '--wait')
    deploy_service(g, region, connection, images, origin, sha)
    service = g.describe('run', 'services', 'describe', 'dinequeue', '--region=' + region)
    public_origin = origin_checked(service['status']['url'])
    if public_origin != origin:
        deploy_service(g, region, connection, images, public_origin, sha)
    g.call('run', 'services', 'add-iam-policy-binding', 'dinequeue', '--region=' + region,
           '--member=allUsers', '--role=roles/run.invoker')
    print(json.dumps({'source_sha': sha, 'url': public_origin, 'images': images,
                      'note': 'SIMULATION ONLY; nothing deployed.' if getattr(g, 'preview', False) else 'Deployment command completed; run real smoke tests before claiming acceptance.'}, indent=2))



class Preview(GCloud):
    preview = True
    deployed = False

    def new_secret(self):
        return 'GENERATED_AT_APPLY_ONLY'

    def describe(self, *args):
        print('[DRY RUN]', shlex.join(['gcloud', '--project', self.project, '--format=json', *args]))
        if args[:3] == ('run', 'services', 'describe') and self.deployed:
            return {'status': {'url': 'https://dinequeue-preview.example.invalid'}}
        return None

    def call(self, *args, data=None):
        redacted = ['--password=<GENERATED_AT_APPLY>' if arg.startswith('--password=') else arg for arg in args]
        print('[DRY RUN]', shlex.join(['gcloud', '--project', self.project, '--quiet', *redacted]))
        if data is not None:
            print('[DRY RUN] stdin: generated secret value withheld')
        if args[:3] == ('sql', 'users', 'list'):
            return '[]'
        if args[:3] == ('artifacts', 'docker', 'images'):
            return args[4].rsplit(':', 1)[0] + '@sha256:' + '0' * 64
        if args[:2] == ('run', 'deploy'):
            self.deployed = True
        return ''


def main(argv=None, env=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--apply', action='store_true', help='perform live billable GCP operations after review')
    parser.add_argument('--demo', action='store_true', help='explicitly provision demo/UAT accounts')
    args = parser.parse_args(argv)
    env = os.environ if env is None else env
    project, region, sha = settings(env)
    if not args.apply or env.get('DRY_RUN') == '1':
        print(json.dumps(plan(project, region, sha, args.demo), indent=2))
        print('\nSIMULATED COMMANDS (fresh resources; existing resources are reused after safe discovery):')
        apply(Preview(project), region, sha, args.demo, stage_source=lambda *_: None)
        return
    if env.get('CONFIRM_PROJECT') != project or env.get('ALLOW_PUBLIC_SERVICE') != 'true':
        raise DeploymentError('Live execution requires CONFIRM_PROJECT and ALLOW_PUBLIC_SERVICE=true after plan review.')
    verify_source(sha)
    apply(GCloud(project), region, sha, args.demo)


if __name__ == '__main__':
    try:
        main()
    except DeploymentError as error:
        print(str(error), file=__import__('sys').stderr)
        raise SystemExit(1)
    except (subprocess.SubprocessError, OSError, ValueError, KeyError):
        # No subprocess repr, response payload, password, DSN or token in output.
        print('Deployment stopped safely. Check prerequisites/resource state and the reviewed plan; no acceptance is claimed.', file=__import__('sys').stderr)
        raise SystemExit(1)
