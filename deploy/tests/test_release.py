"""Release ordering/rollback tests: no network, cloud, Docker or real secrets."""
import copy
import importlib.util
import json
import contextlib
import io
from pathlib import Path
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location('dq_release', Path(__file__).parents[1] / 'release.py')
r = importlib.util.module_from_spec(spec)
spec.loader.exec_module(r)

PROJECT = 'release-test'
REGION = 'asia-southeast1'
SHA = 'a' * 40
NAME = f'projects/{PROJECT}/locations/{REGION}/services/dinequeue'
OP = f'projects/123456789/locations/{REGION}/operations/release-op'
URL = 'https://dinequeue-example-as.a.run.app'


def service():
    return {'name': NAME, 'etag': 'test-etag', 'uri': URL,
            'labels': {'keep': 'service-label'},
            'terminalCondition': {'state': 'CONDITION_SUCCEEDED'},
            'template': {'revision': 'dinequeue-old', 'labels': {'keep': 'revision-label'},
                         'serviceAccount': f'dinequeue-runtime@{PROJECT}.iam.gserviceaccount.com',
                         'scaling': {'minInstanceCount': 0, 'maxInstanceCount': 1},
                         'volumes': [{'name': 'cloudsql', 'cloudSqlInstance': {'instances': [f'{PROJECT}:{REGION}:dinequeue-pg']}}],
                         'containers': [
                             {'name': 'web', 'image': 'old-web', 'ports': [{'containerPort': 8080}], 'dependsOn': ['api'],
                              'env': [{'name': 'API_INTERNAL_URL', 'value': 'http://127.0.0.1:8081'}]},
                             {'name': 'api', 'image': 'old-api', 'env': [
                                 {'name': 'APP_ORIGIN', 'value': URL},
                                 {'name': 'DATABASE_URL', 'valueSource': {'secretKeyRef': {'secret': 'dinequeue-database-url', 'version': '1'}}}],
                              'volumeMounts': [{'name': 'cloudsql', 'mountPath': '/cloudsql'}]}]}}


class FakeCloud:
    def __init__(self):
        self.calls = []
        self.snapshot = service()
        self.fail_at = None
        self.patch_count = 0

    def call(self, *args, data=None):
        self.calls.append(('cli', args))
        if args[:4] == ('artifacts', 'docker', 'images', 'describe'):
            if self.fail_at == 'image':
                return 'invalid-digest'
            return args[4].rsplit(':', 1)[0] + '@sha256:' + 'b' * 64
        if args[:3] == ('run', 'jobs', 'execute') and self.fail_at == 'migration':
            raise r.DeploymentError('migration failed')
        return ''

    def _run_v2_request(self, method, path, body=None, **kwargs):
        self.calls.append((method, path, copy.deepcopy(body)))
        if method == 'GET':
            return copy.deepcopy(self.snapshot)
        if 'validateOnly=true' in path:
            if self.fail_at == 'validation':
                raise r.DeploymentError('validation failed')
            return {'name': OP}
        self.patch_count += 1
        result = copy.deepcopy(body)
        result.update(uri=URL, terminalCondition={'state': 'CONDITION_SUCCEEDED'}, reconciling=False,
                      latestReadyRevision=NAME + '/revisions/dinequeue-new',
                      latestCreatedRevision=NAME + '/revisions/dinequeue-new',
                      trafficStatuses=[{'revision': 'dinequeue-new', 'percent': 100}])
        return {'name': OP, 'done': True, 'response': result}


class ReleaseTests(unittest.TestCase):
    def cfg(self, rollback=False):
        return r.settings({'PROJECT_ID': PROJECT, 'REGION': REGION,
                           'IMAGE_TAG': SHA, 'ROLLBACK': str(rollback).lower()})

    def test_invalid_settings_rejected_before_cloud(self):
        for key, value in [('PROJECT_ID', ''), ('PROJECT_ID', 'bad;project'),
                           ('REGION', 'invalid'), ('IMAGE_TAG', 'short'),
                           ('IMAGE_TAG', SHA + '\n'), ('ROLLBACK', 'yes')]:
            env = {'PROJECT_ID': PROJECT, 'REGION': REGION, 'IMAGE_TAG': SHA, 'ROLLBACK': 'false'}
            env[key] = value
            with self.subTest(key=key, value=value), self.assertRaises(r.DeploymentError):
                r.settings(env)

    def test_normal_release_validates_then_waits_for_migration_before_patch(self):
        g = FakeCloud()
        result = r.release(g, self.cfg())
        kinds = [c[0] for c in g.calls]
        self.assertNotIn('POST', kinds)
        validation = next(i for i,c in enumerate(g.calls) if c[0] == 'PATCH' and 'validateOnly=true' in c[1])
        migration = next(i for i,c in enumerate(g.calls) if c[:1] == ('cli',) and c[1][:3] == ('run','jobs','execute'))
        rollout = next(i for i,c in enumerate(g.calls) if c[0] == 'PATCH' and 'validateOnly' not in c[1])
        self.assertLess(validation, migration)
        self.assertLess(migration, rollout)
        self.assertIn('--wait', g.calls[migration][1])
        self.assertEqual(result['source_sha'], SHA)
        self.assertEqual(result['url'], URL)
        self.assertEqual(set(result['images']), {'web','api','db-tools'})

    def test_failures_never_mutate_service(self):
        for failure in ('image','validation','migration'):
            g = FakeCloud(); g.fail_at = failure
            with self.subTest(failure=failure), self.assertRaises(r.DeploymentError):
                r.release(g, self.cfg())
            self.assertEqual(g.patch_count, 0)
            if failure != 'migration':
                self.assertFalse(any(c[0] == 'cli' and c[1][:2] == ('run','jobs') for c in g.calls))

    def test_rollback_does_not_touch_jobs_or_require_dbtools_image(self):
        g = FakeCloud(); result = r.release(g, self.cfg(True))
        self.assertFalse(any(c[0] == 'cli' and c[1][:2] == ('run','jobs') for c in g.calls))
        self.assertEqual(set(result['images']), {'api','web'})
        self.assertTrue(result['rollback'])
        self.assertEqual(g.patch_count, 1)

    def test_validation_only_never_runs_jobs_or_real_patch(self):
        g = FakeCloud(); result = r.release(g, self.cfg(), validate_only=True)
        self.assertEqual(g.patch_count, 0)
        self.assertTrue(result['validation_only'])
        self.assertFalse(any(c[0] == 'cli' and c[1][:2] == ('run','jobs') for c in g.calls))

    def test_update_preserves_configuration_and_only_changes_named_images_and_labels(self):
        original = service(); images = {'web':'new-web', 'api':'new-api'}
        body = r.updated_service(original, images, SHA, PROJECT, REGION)
        self.assertEqual(original, service())
        self.assertNotIn('revision', body['template'])
        self.assertEqual(body['etag'], original['etag'])
        self.assertNotIn('uri', body)
        self.assertEqual(body['traffic'], [{'type':'TRAFFIC_TARGET_ALLOCATION_TYPE_LATEST','percent':100}])
        for previous, current in zip(original['template']['containers'], body['template']['containers']):
            self.assertEqual(current['image'], images[current['name']])
            self.assertEqual({k:v for k,v in previous.items() if k!='image'}, {k:v for k,v in current.items() if k!='image'})
        for key in ('serviceAccount','scaling','volumes'):
            self.assertEqual(body['template'][key], original['template'][key])
        self.assertEqual(body['labels'], {'keep':'service-label','source-sha':SHA})

    def test_missing_or_wrong_service_rejected_before_job_changes(self):
        for value in (None, {}, dict(service(), name=NAME+'-wrong'),
                      dict(service(), uri='https://evil.example'), dict(service(), etag=''),
                      dict(service(),reconciling=True)):
            g = FakeCloud(); g.snapshot = value
            with self.subTest(value=value), self.assertRaises(r.DeploymentError):
                r.release(g, self.cfg())
            self.assertEqual(g.patch_count, 0)

    def test_unknown_or_duplicate_containers_rejected(self):
        for containers in ([], [service()['template']['containers'][0]] * 2,
                           service()['template']['containers'] + [{'name':'extra'}]):
            value = service(); value['template']['containers'] = containers
            with self.subTest(containers=containers), self.assertRaises(r.DeploymentError):
                r.updated_service(value, {'api':'a','web':'w'}, SHA, PROJECT, REGION)

    def test_operation_errors_wrong_scope_and_deadline_rejected(self):
        for op in ({'name':OP,'done':True,'error':{'message':'private-canary'}},
                   {'name':'projects/evil/locations/elsewhere/operations/op'},
                   {'name':OP,'done':True}, {'name':OP}):
            with self.subTest(op=op), patch.object(r.time,'monotonic',side_effect=[0,601]), self.assertRaises(r.DeploymentError) as error:
                r.wait_ready(FakeCloud(), op, self.cfg(), {'api':'a','web':'w'})
            self.assertNotIn('private-canary', str(error.exception))

    def test_unready_or_wrong_images_or_traffic_is_not_success(self):
        g = FakeCloud(); cfg = self.cfg()
        body = r.updated_service(service(), {'api':'a','web':'w'}, SHA, PROJECT, REGION)
        op = g._run_v2_request('PATCH', NAME, body)
        for key, value in [('reconciling',True), ('terminalCondition',{'state':'CONDITION_FAILED'}),
                           ('trafficStatuses',[{'revision':'dinequeue-old','percent':100}]),
                           ('latestReadyRevision', NAME+'/revisions/dinequeue-old')]:
            broken = copy.deepcopy(op); broken['response'][key] = value
            with self.subTest(key=key), self.assertRaises(r.DeploymentError):
                r.wait_ready(g, broken, cfg, {'api':'a','web':'w'})

    def test_wrong_image_in_operation_result_is_rejected(self):
        g = FakeCloud()
        body = r.updated_service(service(), {'api':'a','web':'w'}, SHA, PROJECT, REGION)
        op = g._run_v2_request('PATCH', NAME, body)
        op['response']['template']['containers'][0]['image'] = 'wrong-image'
        with self.assertRaises(r.DeploymentError):
            r.wait_ready(g,op,self.cfg(),{'api':'a','web':'w'})

    def test_default_plan_never_constructs_cloud_client(self):
        env = {'PROJECT_ID':PROJECT,'REGION':REGION,'IMAGE_TAG':SHA,'ROLLBACK':'false'}
        with patch.object(r,'GCloud',side_effect=AssertionError('cloud called')), contextlib.redirect_stdout(io.StringIO()) as output:
            r.main([],env)
        self.assertIn('PLAN ONLY',output.getvalue())

    def test_untrusted_workflow_cannot_apply(self):
        env = {'PROJECT_ID':PROJECT,'REGION':REGION,'IMAGE_TAG':SHA,'ROLLBACK':'false',
               'GITHUB_ACTIONS':'true','GITHUB_REF':'refs/heads/main',
               'GITHUB_REPOSITORY':'bbxndg/DineQueue','GITHUB_EVENT_NAME':'push'}
        for key,value in [('GITHUB_REF','refs/heads/gcp-staging'),
                          ('GITHUB_EVENT_NAME','pull_request'),('GITHUB_REPOSITORY','other/DineQueue')]:
            with self.subTest(key=key), patch.object(r,'GCloud',side_effect=AssertionError('cloud called')),self.assertRaises(r.DeploymentError):
                r.main(['--apply'],dict(env,**{key:value}))

    def test_operation_polling_uses_exact_operation_without_replaying_patch(self):
        g = FakeCloud(); cfg = self.cfg()
        body = r.updated_service(service(), {'api':'a','web':'w'}, SHA, PROJECT, REGION)
        ready_op = g._run_v2_request('PATCH',NAME,body)
        with patch.object(g,'_run_v2_request',return_value=ready_op) as request,patch.object(r.time,'sleep'):
            r.wait_ready(g,{'name':OP},cfg,{'api':'a','web':'w'})
        request.assert_called_once_with('GET',OP)

    def test_numeric_project_and_short_revision_forms_are_accepted(self):
        g = FakeCloud(); cfg = self.cfg()
        body = r.updated_service(service(), {'api':'a','web':'w'}, SHA, PROJECT, REGION)
        ready = g._run_v2_request('PATCH',NAME,body)['response']
        ready['name'] = NAME.replace(PROJECT,'123456789')
        ready['latestReadyRevision'] = ready['latestCreatedRevision'] = 'dinequeue-new'
        self.assertEqual(r.verify_ready(ready,cfg,{'api':'a','web':'w'}),(URL,'dinequeue-new'))
        self.assertEqual(r.updated_service(ready,{'api':'a','web':'w'},SHA,PROJECT,REGION)['name'],NAME)

    def test_etag_conflict_stops_without_retrying_the_mutation(self):
        g = FakeCloud()
        original = g._run_v2_request
        def request(method,path,body=None,**kwargs):
            if method=='PATCH' and 'validateOnly' not in path:
                g.patch_count += 1
                raise r.DeploymentError('HTTP 409')
            return original(method,path,body,**kwargs)
        with patch.object(g,'_run_v2_request',side_effect=request),self.assertRaises(r.DeploymentError):
            r.release(g,self.cfg())
        self.assertEqual(g.patch_count,1)


if __name__ == '__main__':
    unittest.main()
