"""Optional offline audit against an installed official Google Cloud SDK.

Usage: python3 deploy/tests/check_gcloud_sdk.py /path/to/google-cloud-sdk
Uses SDK internals intentionally; this is an audit tool, not the deployment path.
No credentials, command Run methods, or cloud connections are used.
"""
import contextlib
import importlib.util
import io
import os
from pathlib import Path
import shlex
import socket
import sys
import tempfile
from types import SimpleNamespace


def check(sdk_root):
    sdk_lib = Path(sdk_root).resolve() / 'lib'
    if not (sdk_lib / 'googlecloudsdk/gcloud_main.py').is_file():
        raise ValueError('Expected the root directory of an official Google Cloud SDK.')
    sys.path[:0] = [str(sdk_lib / 'third_party'), str(sdk_lib)]
    path = Path(__file__).resolve().parents[1] / 'first_deploy.py'
    spec = importlib.util.spec_from_file_location('audited_deploy', path)
    deploy = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(deploy)
    # Import after SDK paths and argparse: Calliope uses its bundled argparse.
    from unittest.mock import patch
    with tempfile.TemporaryDirectory(prefix='dq-sdk-check-') as config:
        env = {'CLOUDSDK_CONFIG': config, 'CLOUDSDK_CORE_DISABLE_USAGE_REPORTING': 'true',
               'CLOUDSDK_CORE_DISABLE_PROMPTS': 'true', 'CLOUDSDK_PAGER': 'cat'}
        with patch.dict(os.environ, env), \
             patch.object(socket.socket, 'connect', side_effect=RuntimeError('Network forbidden during SDK audit')), \
             patch.object(socket, 'create_connection', side_effect=RuntimeError('Network forbidden during SDK audit')):
            from googlecloudsdk import gcloud_main
            from googlecloudsdk.calliope import base
            from googlecloudsdk.command_lib.run import flags
            from googlecloudsdk.api_lib.run import service
            from googlecloudsdk.generated_clients.apis.run.v1 import run_v1_messages as messages
            from googlecloudsdk.core import properties
            from apitools.base.py import encoding

            with contextlib.redirect_stdout(io.StringIO()) as output:
                deploy.apply(deploy.Preview('dinequeue-test'), 'asia-southeast1', 'a' * 40,
                             True, stage_source=lambda *_: None)
            commands = [shlex.split(line.removeprefix('[DRY RUN] '))
                        for line in output.getvalue().splitlines()
                        if line.startswith('[DRY RUN] gcloud ')]
            properties.VALUES.core.project.Set('dinequeue-test')
            service_specs = []
            for command in commands:
                cli = gcloud_main.CreateCLI([])
                # Parse only; never call CLI.Execute or a command's Run method.
                args = cli._CLI__parser.parse_args(command[1:])
                if command[4:7] != ['alpha', 'run', 'deploy']:
                    continue
                changes = flags.GetServiceConfigurationChanges(args, base.ReleaseTrack.ALPHA)
                resource = service.Service.New(
                    SimpleNamespace(_VERSION='v1', MESSAGES_MODULE=messages), 'dinequeue-test')
                for change in changes:
                    resource = change.Adjust(resource)
                data = encoding.MessageToDict(resource.Message())
                pod = data['spec']['template']['spec']
                containers = {c['name']: c for c in pod['containers']}
                assert set(containers) == {'api', 'web'}
                assert containers['web']['ports'] == [{'containerPort': 8080}]
                assert not containers['api'].get('ports')
                assert not containers['web'].get('volumeMounts')
                assert containers['api']['volumeMounts'] == [{'name':'db-sockets', 'mountPath':'/cloudsql'}]
                assert pod['volumes'] == [{'name':'db-sockets', 'csi': {
                    'driver':'cloudsql.run.googleapis.com',
                    'volumeAttributes':{'instances':'dinequeue-test:asia-southeast1:dinequeue-pg'}}}]
                assert all(v['name'] != 'cloudsql' for v in pod['volumes'])
                assert not any(e['name']=='DATABASE_URL' for e in containers['web']['env'])
                service_specs.append(data)
            assert len(service_specs) == 2, 'Expected bootstrap and exact-origin service specs'
            print('PASS real SDK parser:', len(commands), 'planned fresh-deployment commands')
            print('PASS real SDK configuration transforms:', len(service_specs), 'service specs')
            print('No command execution or network access; server-side admission remains unverified.')


if __name__ == '__main__':
    if len(sys.argv) != 2:
        raise SystemExit(__doc__)
    check(sys.argv[1])
