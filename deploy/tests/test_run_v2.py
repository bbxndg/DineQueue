"""Cloud Run v2 transport and native Cloud SQL volume regressions; no network."""
import contextlib
import importlib.util
import io
import json
from pathlib import Path
import subprocess
import unittest
from unittest.mock import patch

spec=importlib.util.spec_from_file_location('v2_deploy',Path(__file__).parents[1]/'first_deploy.py')
d=importlib.util.module_from_spec(spec);spec.loader.exec_module(d)
PROJECT='dinequeue-test';REGION='asia-southeast1'
NAME=f'projects/{PROJECT}/locations/{REGION}/services/dinequeue'
OP=f'projects/{PROJECT}/locations/{REGION}/operations/test-op'

class RunV2Tests(unittest.TestCase):
    def test_only_explicit_service_get_not_found_allows_creation(self):
        missing=json.dumps({'error':{'code':404,'status':'NOT_FOUND','message':'private-canary'}}).encode()
        token=subprocess.CompletedProcess([],0,stdout=b'private-token-canary',stderr=b'')
        cases=[('GET',NAME,True,404,missing,True),
               ('GET',NAME,False,404,missing,False),
               ('GET',OP,True,404,missing,False),
               ('POST',NAME,True,404,missing,False),
               ('PATCH',NAME,True,404,missing,False),
               ('GET',NAME,True,403,missing,False),
               ('GET',NAME,True,404,b'<html>Not found</html>',False),
               ('GET',NAME,True,404,b'{"error":{"code":404,"status":"PERMISSION_DENIED"}}',False)]
        for method,path,missing_ok,code,payload,absent in cases:
            error=d.urllib.error.HTTPError('https://run.googleapis.com/v2/'+path,code,'hidden',{},io.BytesIO(payload))
            with self.subTest(method=method,path=path,code=code,absent=absent),patch.object(d.subprocess,'run',return_value=token),patch.object(d.urllib.request,'build_opener') as opener,contextlib.redirect_stdout(io.StringIO()) as output:
                opener.return_value.open.side_effect=error
                if absent:
                    self.assertIsNone(d.GCloud(PROJECT)._run_v2_request(method,path,missing_ok=missing_ok))
                else:
                    with self.assertRaises(d.DeploymentError) as failure:
                        d.GCloud(PROJECT)._run_v2_request(method,path,missing_ok=missing_ok)
                    self.assertNotIn('canary',str(failure.exception))
                self.assertNotIn('canary',output.getvalue())

    def test_discovery_failure_never_attempts_a_mutation(self):
        g=d.GCloud(PROJECT)
        with patch.object(g,'_run_v2_request',side_effect=d.DeploymentError('denied')) as request:
            with self.assertRaises(d.DeploymentError):g.configure_service(self.body())
        self.assertEqual(request.call_count,1)
        self.assertEqual(request.call_args.args,('GET',NAME))

    def test_create_conflict_is_not_replayed_or_changed_into_patch(self):
        g=d.GCloud(PROJECT)
        with patch.object(g,'_run_v2_request',side_effect=[None,d.DeploymentError('HTTP 409')]) as request,contextlib.redirect_stdout(io.StringIO()):
            with self.assertRaises(d.DeploymentError):g.configure_service(self.body())
        self.assertEqual(request.call_count,2)
        self.assertEqual(request.call_args_list[1].args[0],'POST')

    def test_missing_service_is_created_with_post_not_patch(self):
        ready={'name':OP,'done':True,'response':{'name':NAME,'terminalCondition':{'state':'CONDITION_SUCCEEDED'}}}
        g=d.GCloud(PROJECT)
        with patch.object(g,'_run_v2_request',side_effect=[None,ready]) as request,contextlib.redirect_stdout(io.StringIO()):
            g.configure_service(self.body())
        self.assertEqual(request.call_args_list[0].args,('GET',NAME))
        self.assertEqual(request.call_args_list[0].kwargs,{'missing_ok':True})
        self.assertEqual(request.call_args_list[1].args[:2],('POST',f'projects/{PROJECT}/locations/{REGION}/services?serviceId=dinequeue'))
        expected={key:value for key,value in self.body().items() if key!='name'}
        self.assertEqual(request.call_args_list[1].args[2],expected)
        self.assertNotIn('name',request.call_args_list[1].args[2])

    def body(self):
        return d.service_spec_v2(PROJECT,REGION,PROJECT+':'+REGION+':dinequeue-pg',
                                 {'web':'web-digest','api':'api-digest'},'https://example.test','a'*40)

    def test_native_sql_volume_and_api_only_mount_and_secret(self):
        body=self.body();template=body['template'];containers={c['name']:c for c in template['containers']}
        self.assertEqual(template['volumes'],[{'name':'cloudsql','cloudSqlInstance':{'instances':[PROJECT+':'+REGION+':dinequeue-pg']}}])
        self.assertNotIn('csi',json.dumps(body))
        self.assertEqual(containers['api']['volumeMounts'],[{'name':'cloudsql','mountPath':'/cloudsql'}])
        self.assertNotIn('volumeMounts',containers['web'])
        self.assertNotIn('ports',containers['api'])
        self.assertEqual(containers['web']['ports'],[{'containerPort':8080}])
        self.assertEqual(containers['web']['dependsOn'],['api'])
        self.assertFalse(any(e['name']=='DATABASE_URL' for e in containers['web']['env']))
        secret=next(e for e in containers['api']['env'] if e['name']=='DATABASE_URL')
        self.assertEqual(secret['valueSource']['secretKeyRef'],{'secret':'dinequeue-database-url','version':'1'})

    def test_existing_service_is_patched_and_operation_is_awaited(self):
        g=d.GCloud(PROJECT)
        with patch.object(g,'_run_v2_request',side_effect=[{'name':NAME},{'name':OP},{'name':OP,'done':False},{'name':OP,'done':True,'response':{'name':NAME,'terminalCondition':{'state':'CONDITION_SUCCEEDED'}}}]) as request,patch.object(d.time,'sleep'),contextlib.redirect_stdout(io.StringIO()):
            g.configure_service(self.body())
        self.assertEqual(request.call_args_list[1].args[:2],('PATCH',NAME+'?updateMask=template%2Clabels%2Cingress%2Ctraffic'))
        self.assertEqual(request.call_args_list[1].args[2]['name'],NAME)
        self.assertEqual(request.call_count,4)

    def test_operation_failure_or_unsafe_name_aborts_without_raw_payload(self):
        for response in ({'name':OP,'done':True,'error':{'code':3,'message':'private-canary'}},
                         {'name':'https://evil.example/private-canary'}, {}, {'name':OP,'done':True}):
            g=d.GCloud(PROJECT)
            with self.subTest(response=response),patch.object(g,'_run_v2_request',return_value=response),contextlib.redirect_stdout(io.StringIO()) as output:
                with self.assertRaises(d.DeploymentError) as error:g.configure_service(self.body())
                self.assertNotIn('private-canary',str(error.exception)+output.getvalue())

    def test_polling_has_deadline_and_does_not_repeat_patch(self):
        g=d.GCloud(PROJECT)
        with patch.object(g,'_run_v2_request',return_value={'name':OP}) as request,patch.object(d.time,'monotonic',side_effect=[0,601]),contextlib.redirect_stdout(io.StringIO()):
            with self.assertRaises(d.DeploymentError):g.configure_service(self.body())
            self.assertEqual(request.call_count,2)

    def test_preview_performs_no_cli_or_http_request(self):
        with patch.object(d.subprocess,'run',side_effect=AssertionError('CLI called')),patch.object(d.urllib.request,'build_opener',side_effect=AssertionError('HTTP called')),contextlib.redirect_stdout(io.StringIO()) as output:
            d.Preview(PROJECT).configure_service(self.body())
        self.assertIn('cloudSqlInstance',output.getvalue())
        self.assertNotIn('cloudsql.run.googleapis.com',output.getvalue())

    def test_completed_operation_with_unready_service_is_rejected(self):
        for condition in ({}, {'state':'CONDITION_FAILED'}, {'state':'CONDITION_PENDING'}):
            with self.subTest(condition=condition),patch.object(d.GCloud,'_run_v2_request',return_value={'name':OP,'done':True,'response':{'name':NAME,'terminalCondition':condition}}),contextlib.redirect_stdout(io.StringIO()):
                with self.assertRaises(d.DeploymentError):d.GCloud(PROJECT).configure_service(self.body())

    def test_request_uses_bearer_token_in_memory_and_preserves_json(self):
        token='private-token-canary'
        response={'name':OP,'done':False}
        with patch.object(d.subprocess,'run',return_value=subprocess.CompletedProcess([],0,stdout=token.encode(),stderr=b'')),patch.object(d.urllib.request,'build_opener') as opener,contextlib.redirect_stdout(io.StringIO()) as output:
            opener.return_value.open.return_value.__enter__.return_value.read.return_value=json.dumps(response).encode()
            self.assertEqual(d.GCloud(PROJECT)._run_v2_request('PATCH',NAME,self.body()),response)
            request=opener.return_value.open.call_args.args[0]
            self.assertEqual(request.full_url,'https://run.googleapis.com/v2/'+NAME)
            self.assertEqual(request.get_header('Authorization'),'Bearer '+token)
            self.assertEqual(json.loads(request.data),self.body())
            self.assertNotIn(token,output.getvalue())

    def test_http_errors_and_bad_responses_do_not_leak_credentials_or_payload(self):
        token=subprocess.CompletedProcess([],0,stdout=b'private-token-canary',stderr=b'')
        for code in (400,401,403,429,500):
            error=d.urllib.error.HTTPError('https://run.googleapis.com/v2/'+NAME,code,'private-error-canary',{},io.BytesIO(b'private-body-canary'))
            with self.subTest(code=code),patch.object(d.subprocess,'run',return_value=token),patch.object(d.urllib.request,'build_opener') as opener,contextlib.redirect_stdout(io.StringIO()) as output:
                opener.return_value.open.side_effect=error
                with self.assertRaises(d.DeploymentError) as failure:d.GCloud(PROJECT)._run_v2_request('PATCH',NAME,self.body())
                self.assertIn('HTTP '+str(code),str(failure.exception))
                self.assertNotIn('canary',str(failure.exception)+output.getvalue())
        for payload in (b'[]',b'null',b'not-json'):
            with self.subTest(payload=payload),patch.object(d.subprocess,'run',return_value=token),patch.object(d.urllib.request,'build_opener') as opener:
                opener.return_value.open.return_value.__enter__.return_value.read.return_value=payload
                with self.assertRaises(d.DeploymentError):d.GCloud(PROJECT)._run_v2_request('GET',OP)

if __name__=='__main__':unittest.main()
