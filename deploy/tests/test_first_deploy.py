"""No network/GCP: record arguments and use in-memory resource responses."""
import contextlib
import importlib.util
import io
import json
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest.mock import patch

PATH = Path(__file__).resolve().parents[1] / 'first_deploy.py'
spec = importlib.util.spec_from_file_location('first_deploy', PATH)
d = importlib.util.module_from_spec(spec)
spec.loader.exec_module(d)
SHA = 'a' * 40
PROJECT = 'dinequeue-test'


class FakeGCloud:
    project = PROJECT

    def __init__(self, mode='fresh'):
        self.mode, self.calls = mode, []
        self.deployed = False

    def new_secret(self):
        return d.secrets.token_hex(24)

    def describe(self, *args):
        self.calls.append(('describe', args, None))
        if args[:3] == ('run', 'services', 'describe'):
            return {'status': {'url': 'https://dinequeue.example.test'}} if self.deployed or self.mode == 'rerun' else None
        if args[:3] == ('sql', 'instances', 'describe') and self.mode == 'rerun':
            return {'region':'asia-southeast1', 'databaseVersion':'POSTGRES_18', 'settings': {'tier':'db-f1-micro', 'dataDiskSizeGb':'10', 'ipConfiguration': {'sslMode':'ENCRYPTED_ONLY'}}}
        if args[:3] == ('secrets', 'versions', 'describe'):
            return {'state':'ENABLED'}
        if self.mode == 'rerun':
            return {}
        return None

    def call(self, *args, data=None):
        self.calls.append(('call', args, data))
        if args[:3] == ('sql', 'users', 'list'):
            return json.dumps([{'name':'dinequeue'}] if self.mode == 'partial' else [])
        if args[:3] == ('artifacts', 'docker', 'images'):
            image = args[4]
            return image.rsplit(':', 1)[0] + '@sha256:' + 'b' * 64
        if args[:3] == ('run', 'jobs', 'execute') and self.mode == 'migration-fail':
            raise d.DeploymentError('synthetic migration failure')
        if args[:2] == ('run', 'deploy'):
            self.deployed = True
        return ''


class FirstDeployTests(unittest.TestCase):
    def run_apply(self, mode='fresh', demo=True):
        fake = FakeGCloud(mode)
        with patch.object(d, 'source_stage'), contextlib.redirect_stdout(io.StringIO()):
            d.apply(fake, 'asia-southeast1', SHA, demo, stage_source=lambda *_: None)
        return fake

    def test_dry_run_calls_no_subprocess_or_random_generator(self):
        with patch.object(d.subprocess, 'run', side_effect=AssertionError('subprocess called')), patch.object(d.secrets, 'token_hex', side_effect=AssertionError('secret generated')), contextlib.redirect_stdout(io.StringIO()) as output:
            d.main(['--demo'], {'PROJECT_ID':PROJECT, 'SOURCE_SHA':SHA})
        plan = json.loads(output.getvalue().split('SIMULATED COMMANDS')[0])
        self.assertEqual(plan['containers']['web']['secrets'], [])
        self.assertEqual(plan['sql']['version'], 'POSTGRES_18')
        self.assertFalse(plan['sql']['storage_auto_increase'])
        self.assertEqual(plan['limits']['max_instances'], 1)

    def test_apply_needs_both_confirmations_before_commands(self):
        with patch.object(d.subprocess, 'run', side_effect=AssertionError('subprocess called')):
            with self.assertRaises(d.DeploymentError):
                d.main(['--apply'], {'PROJECT_ID':PROJECT, 'SOURCE_SHA':SHA})

    def test_invalid_settings_are_rejected(self):
        for env in ({}, {'PROJECT_ID':'bad;project', 'SOURCE_SHA':SHA}, {'PROJECT_ID':PROJECT, 'SOURCE_SHA':'latest'}, {'PROJECT_ID':PROJECT,'SOURCE_SHA':SHA,'REGION':'bad,region'}):
            with self.assertRaises(d.DeploymentError): d.settings(env)

    def test_new_deployment_orders_migration_before_service_and_public_access(self):
        f = self.run_apply()
        calls = [args for kind,args,_ in f.calls if kind=='call']
        migration = next(i for i,a in enumerate(calls) if a[:4]==('run','jobs','execute','dinequeue-migrate'))
        deployment = next(i for i,a in enumerate(calls) if a[:2]==('run','deploy'))
        public = next(i for i,a in enumerate(calls) if '--member=allUsers' in a)
        self.assertLess(migration, deployment)
        self.assertLess(deployment, public)
        self.assertTrue(any('--wait' in a for a in calls if a[:3]==('run','jobs','execute')))
        for a in calls:
            if a[:2]==('run','deploy'):
                self.assertIn('--container=api', a)
                self.assertIn('--depends-on=api', a)
                self.assertIn('--add-volume-mount=volume=cloudsql,mount-path=/cloudsql', a)
                self.assertTrue(any('API_PORT=8081' in x for x in a))
                self.assertFalse(any('PORT=8081' in x and 'API_PORT=8081' not in x for x in a))
                self.assertIn('--port=8080', a)
                self.assertFalse(any('latest' in x for x in a))

    def test_runtime_gets_database_secret_only_and_never_project_wide_secret_access(self):
        f = self.run_apply()
        for _, args, _ in f.calls:
            if '--role=roles/secretmanager.secretAccessor' in args:
                self.assertEqual(args[:2], ('secrets','add-iam-policy-binding'))
                if 'dinequeue-runtime@' in ' '.join(args): self.assertEqual(args[2], 'dinequeue-database-url')
            if args[:2]==('run','deploy'):
                before_api = args[:args.index('--container=api')]
                self.assertFalse(any('set-secrets' in x for x in before_api))
                self.assertFalse(any('DEMO_' in x for x in args))

    def test_migration_failure_aborts_service_and_public_grant(self):
        f = FakeGCloud('migration-fail')
        with patch.object(d, 'source_stage'), self.assertRaises(d.DeploymentError):
            d.apply(f, 'asia-southeast1', SHA, True, stage_source=lambda *_: None)
        self.assertFalse(any(a[:2]==('run','deploy') or '--member=allUsers' in a for _,a,_ in f.calls))

    def test_partial_credentials_never_reset_password_or_continue(self):
        f = FakeGCloud('partial')
        with self.assertRaises(d.DeploymentError): d.db_secret(f, PROJECT+':asia-southeast1:dinequeue-pg')
        self.assertFalse(any(a[:3]==('sql','users','create') or 'set-password' in a for _,a,_ in f.calls))

    def test_rerun_preserves_credentials(self):
        f = self.run_apply('rerun')
        self.assertFalse(any(a[:3] in [('sql','users','create'),('secrets','versions','add')] for _,a,_ in f.calls))

    def test_provisioning_is_opt_in_and_job_guarded(self):
        f = self.run_apply(demo=False)
        self.assertFalse(any('dinequeue-provision-demo' in a for _,a,_ in f.calls))
        f = self.run_apply(demo=True)
        provision = [a for _,a,_ in f.calls if a[:4]==('run','jobs','deploy','dinequeue-provision-demo')][0]
        self.assertIn('--args=provision-demo', provision)
        self.assertTrue(any('ENABLE_DEMO_PROVISION=true,CONFIRM_DEMO_DATABASE=dinequeue_demo' in x for x in provision))

    def test_generated_password_matches_db_url_secret(self):
        f = FakeGCloud()
        with patch.object(d.secrets, 'token_hex', return_value='test-only-password-canary'):
            d.db_secret(f, PROJECT+':asia-southeast1:dinequeue-pg')
        create = [a for _,a,_ in f.calls if a[:3]==('sql','users','create')][0]
        self.assertIn('--password=test-only-password-canary', create)
        payload = [data for _,a,data in f.calls if a[:3]==('secrets','versions','add')][0]
        self.assertIn(b':test-only-password-canary@', payload)
        self.assertIn(b'sslmode=disable', payload)
        self.assertNotIn(b'connection_limit', payload)

    def test_gcloud_errors_do_not_print_credentials(self):
        with patch.object(d.subprocess, 'run', return_value=subprocess.CompletedProcess([],1,stdout=b'secret-payload',stderr=b'password-canary')), contextlib.redirect_stdout(io.StringIO()) as output:
            with self.assertRaises(d.DeploymentError) as result:
                d.GCloud(PROJECT).call('sql','users','create','app','--password=password-canary')
        self.assertNotIn('password-canary', output.getvalue()+str(result.exception))

    def test_only_explicit_not_found_means_missing(self):
        for stderr in (b'PERMISSION_DENIED', b'network timeout'):
            with patch.object(d.subprocess, 'run', return_value=subprocess.CompletedProcess([],1,stdout=b'',stderr=stderr)):
                with self.assertRaises(d.DeploymentError): d.GCloud(PROJECT).describe('sql','instances','describe','test')
        for stderr in (b'NOT_FOUND', b'(gcloud.sql.instances.describe) HTTPError 404: instance does not exist'):
            with patch.object(d.subprocess, 'run', return_value=subprocess.CompletedProcess([],1,stdout=b'',stderr=stderr)):
                self.assertIsNone(d.GCloud(PROJECT).describe('sql','instances','describe','test'))

    def test_secret_version_must_exist(self):
        f = FakeGCloud('rerun')
        with patch.object(f, 'describe', side_effect=[{}, None]):
            with self.assertRaises(d.DeploymentError): d.db_secret(f, 'unused')

    def test_source_upload_is_git_archive_not_working_directory(self):
        with tempfile.TemporaryDirectory() as root:
            secret = Path(root)/'.env'
            secret.write_text('not-for-upload')
            with patch.object(d, 'ROOT', Path(root)), patch.object(d.subprocess, 'run') as run:
                data = io.BytesIO()
                import tarfile
                with tarfile.open(fileobj=data,mode='w') as tar:
                    body=b'tracked only'
                    info=tarfile.TarInfo('tracked.txt'); info.size=len(body)
                    tar.addfile(info,io.BytesIO(body))
                run.return_value.stdout=data.getvalue()
                target=Path(root)/'stage'; target.mkdir()
                d.source_stage(target,SHA)
                self.assertFalse((target/'.env').exists())
                self.assertEqual((target/'tracked.txt').read_text(),'tracked only')
                self.assertEqual(run.call_args.args[0],['git','archive','--format=tar',SHA])

    def test_live_source_guard_rejects_feature_branch_dirty_outdated_and_secret_files(self):
        cases = [
            ['feature/ep5'],
            ['main', SHA, ' M changed.go'],
            ['main', SHA, '', '', 'b'*40],
            ['main', SHA, '', '', SHA, '.env\napps/api/main.go'],
        ]
        for responses in cases:
            outputs = [subprocess.CompletedProcess([],0,stdout=v.encode(),stderr=b'') for v in responses]
            with patch.object(d.subprocess,'run',side_effect=outputs):
                with self.assertRaises(d.DeploymentError): d.verify_source(SHA)

    def test_existing_wrong_sql_settings_abort_before_secrets_and_build(self):
        fake = FakeGCloud('rerun')
        original = fake.describe
        def wrong(*args):
            if args[:3]==('sql','instances','describe'):
                return {'region':'other','databaseVersion':'POSTGRES_18','settings':{}}
            return original(*args)
        with patch.object(fake,'describe',side_effect=wrong), self.assertRaises(d.DeploymentError):
            d.apply(fake,'asia-southeast1',SHA,True,stage_source=lambda *_:None)
        self.assertFalse(any(a[0] in ('secrets','builds') for _,a,_ in fake.calls))

    def test_disabled_demo_secret_aborts_without_credential_replacement(self):
        fake = FakeGCloud('rerun')
        with patch.object(fake,'describe',side_effect=[{}, {'state':'DISABLED'}]):
            with self.assertRaises(d.DeploymentError):
                d.secret_value(fake,'dinequeue-demo-host-password',lambda:'never-generated')
        self.assertEqual(fake.calls,[])


if __name__ == '__main__': unittest.main()
