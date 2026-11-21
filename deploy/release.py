"""EP7 updates existing resources only. No-network plan unless --apply/--validate-only."""
import argparse
import copy
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import time
from urllib.parse import urlencode

# Also works when imported by offline tests from outside deploy/.
sys.path.insert(0, str(Path(__file__).resolve().parent))
from first_deploy import DeploymentError, GCloud


def settings(env):
    values = {name: env.get(key, default) for name, key, default in (
        ('project','PROJECT_ID',''), ('region','REGION','asia-southeast1'),
        ('tag','IMAGE_TAG',''), ('rollback','ROLLBACK','false'))}
    for key, pattern in (
        ('project',r'[a-z][a-z0-9-]{4,28}[a-z0-9]'),
        ('region',r'[a-z]+-[a-z]+[0-9]'), ('tag',r'[0-9a-f]{40}'),
        ('rollback',r'true|false')):
        if not re.fullmatch(pattern, values[key]):
            raise DeploymentError('Invalid release settings: project, region, full SHA and rollback mode are required.')
    values['rollback'] = values['rollback'] == 'true'
    return values


def target(cfg):
    return f"projects/{cfg['project']}/locations/{cfg['region']}/services/dinequeue"


def service_pattern(project, region):
    # Google may canonicalize project IDs to numeric project identifiers in responses.
    return (r'projects/(?:' + re.escape(project) + r'|[0-9]+)/locations/'
            + re.escape(region) + r'/services/dinequeue')


def public_url(service):
    value = service.get('uri', '') if isinstance(service,dict) else ''
    if not isinstance(value,str) or not re.fullmatch(r'https://[a-z0-9.-]+\.run\.app', value):
        raise DeploymentError('Existing service has no valid canonical HTTPS Cloud Run URL.')
    return value


def updated_service(service, images, sha, project, region):
    name = f'projects/{project}/locations/{region}/services/dinequeue'
    if (not isinstance(service,dict)
            or not re.fullmatch(service_pattern(project,region),service.get('name',''))
            or not isinstance(service.get('etag'),str) or not service['etag']):
        raise DeploymentError('Release requires the existing expected service and its version fingerprint.')
    if service.get('reconciling'):
        raise DeploymentError('Existing service is still reconciling; inspect it before another release.')
    public_url(service)
    template = copy.deepcopy(service.get('template', {}))
    containers = template.get('containers', [])
    if len(containers) != 2 or {c.get('name') for c in containers} != {'web','api'}:
        raise DeploymentError('Release requires exactly the existing named web and api containers.')
    template.pop('revision', None)  # A new revision must not reuse the old revision name.
    for container in containers:
        container['image'] = images[container['name']]
    template.setdefault('labels', {})['source-sha'] = sha
    labels = dict(service.get('labels', {}), **{'source-sha':sha})
    return {'name':name, 'etag':service['etag'], 'template':template, 'labels':labels,
            'traffic':[{'type':'TRAFFIC_TARGET_ALLOCATION_TYPE_LATEST','percent':100}]}


def verify_ready(service, cfg, images):
    if (not isinstance(service,dict)
            or not re.fullmatch(service_pattern(cfg['project'],cfg['region']),service.get('name',''))
            or service.get('reconciling')
            or service.get('terminalCondition', {}).get('state') != 'CONDITION_SUCCEEDED'):
        raise DeploymentError('Release operation did not produce a ready service.')
    actual = {c.get('name'):c.get('image') for c in service.get('template', {}).get('containers', [])}
    if actual != {key:images[key] for key in ('api','web')}:
        raise DeploymentError('Ready service images do not match the resolved release digests.')
    revision = service.get('latestReadyRevision', '')
    expected = '(?:' + service_pattern(cfg['project'],cfg['region']) + r'/revisions/)?[a-z][a-z0-9-]*'
    if (not re.fullmatch(expected,revision)
            or revision != service.get('latestCreatedRevision')
            or service.get('labels', {}).get('source-sha') != cfg['tag']):
        raise DeploymentError('Ready revision/source label does not match the release.')
    traffic = service.get('trafficStatuses', [])
    if (len(traffic) != 1 or traffic[0].get('percent') != 100
            or not (traffic[0].get('revision') in (revision,revision.rsplit('/',1)[-1])
                    or (not traffic[0].get('revision') and traffic[0].get('type') == 'TRAFFIC_TARGET_ALLOCATION_TYPE_LATEST'))):
        raise DeploymentError('Ready release is not receiving all traffic.')
    return public_url(service), revision


def wait_ready(g, operation, cfg, images):
    deadline = time.monotonic() + 600
    name = operation.get('name', '')
    pattern = (r'projects/(?:' + re.escape(cfg['project']) + r'|[0-9]+)/locations/'
               + re.escape(cfg['region']) + r'/operations/[A-Za-z0-9_.-]+')
    if not re.fullmatch(pattern,name):
        raise DeploymentError('Release returned an invalid operation name.')
    while True:
        if 'error' in operation:
            raise DeploymentError('Release operation failed; inspect revision conditions in Console.')
        if operation.get('done') is True:
            response = operation.get('response')
            verify_ready(response, cfg, images)
            return response
        if time.monotonic() >= deadline:
            raise DeploymentError('Release readiness wait timed out; inspect the operation before retrying.')
        time.sleep(5)
        operation = g._run_v2_request('GET', name)


def release(g, cfg, *, validate_only=False):
    images = {}
    for image in (('api','web') if cfg['rollback'] else ('api','web','db-tools')):
        base = f"{cfg['region']}-docker.pkg.dev/{cfg['project']}/dinequeue/{image}"
        digest = g.call('artifacts','docker','images','describe', base + ':' + cfg['tag'],
                        '--format=value(image_summary.fully_qualified_digest)')
        if not re.fullmatch(re.escape(base) + r'@sha256:[0-9a-f]{64}',digest):
            raise DeploymentError('A required release image digest is missing or invalid.')
        images[image] = digest
    snapshot = g._run_v2_request('GET', target(cfg))  # No create fallback.
    body = updated_service(snapshot, images, cfg['tag'], cfg['project'], cfg['region'])
    query = {'updateMask':'template,labels,traffic'}
    # Server admission check before migration. validateOnly operations are not persisted/polled.
    validation = g._run_v2_request('PATCH', target(cfg) + '?' + urlencode(dict(query,validateOnly='true')),body)
    if 'error' in validation:
        raise DeploymentError('Cloud Run release validation was rejected.')
    if validate_only:
        return {'validation_only':True,'source_sha':cfg['tag'], 'images':images,
                'note':'Request validated only; no migration or deployment performed.'}
    if not cfg['rollback']:
        g.call('run','jobs','update','dinequeue-migrate','--region='+cfg['region'],
               '--image='+images['db-tools'])
        g.call('run','jobs','execute','dinequeue-migrate','--region='+cfg['region'],'--wait')
    else:
        print('Rollback/reuse: skip migration and provisioning; database stays unchanged.')
    print('GCP: Cloud Run v2 PATCH existing dinequeue; waiting for readiness')
    operation = g._run_v2_request('PATCH', target(cfg) + '?' + urlencode(query), body)
    ready = wait_ready(g, operation, cfg, images)
    url, revision = verify_ready(ready, cfg, images)
    return {'source_sha':cfg['tag'], 'rollback':cfg['rollback'], 'url':url,
            'revision':revision, 'images':images,
            'note':'Service ready; workflow smoke must pass before release success is claimed.'}


def export_result(result, env):
    # All exported values have been validated or derived from fixed names/digests.
    if env.get('GITHUB_ENV'):
        values = {'SERVICE_URL':result['url'], 'DEPLOYED_REVISION':result['revision'],
                  'DEPLOYED_API_IMAGE':result['images']['api'],
                  'DEPLOYED_WEB_IMAGE':result['images']['web'],
                  'DEPLOYED_DB_TOOLS_IMAGE':result['images'].get('db-tools','not-updated')}
        with open(env['GITHUB_ENV'],'a',encoding='utf-8') as output:
            for key, value in values.items():
                if '\n' in value or '\r' in value:
                    raise DeploymentError('Unsafe release output; no raw data is exported.')
                output.write(key+'='+value+'\n')


def main(argv=None, env=None):
    parser = argparse.ArgumentParser(description=__doc__)
    mode = parser.add_mutually_exclusive_group()
    mode.add_argument('--apply',action='store_true')
    mode.add_argument('--validate-only',action='store_true')
    args = parser.parse_args(argv)
    env = os.environ if env is None else env
    cfg = settings(env)
    if not args.apply and not args.validate_only:
        print(json.dumps(dict(cfg,mode='PLAN ONLY; no cloud calls',
                              service='dinequeue',migration='skip' if cfg['rollback'] else 'dinequeue-migrate',
                              containers=['api','web'],create_resources=False),indent=2))
        return
    if env.get('GITHUB_ACTIONS') == 'true' and (
            env.get('GITHUB_REF') != 'refs/heads/main'
            or env.get('GITHUB_REPOSITORY') != 'bbxndg/DineQueue'
            or env.get('GITHUB_EVENT_NAME') not in ('push','workflow_dispatch')):
        raise DeploymentError('Release workflow must run from the approved repository and main branch.')
    cloud = GCloud(cfg['project'])
    result = release(cloud,cfg,validate_only=args.validate_only)
    if args.apply:
        export_result(result,env)
    print(json.dumps(result,indent=2))


if __name__ == '__main__':
    try:
        main()
    except DeploymentError as error:
        # Our deployment exceptions contain fixed diagnostics, never raw provider payloads.
        sys.exit(str(error))
    except (OSError,ValueError,KeyError,TypeError,AttributeError,subprocess.SubprocessError):
        sys.exit('Release stopped safely; inspect the named service/job. Credentials and raw payloads are withheld.')
