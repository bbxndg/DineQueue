"""Offline grammar/schema audit with an official installed SDK; never executes GCP commands.

Usage: python3 deploy/tests/check_release_sdk.py /path/to/google-cloud-sdk
"""
import contextlib
import importlib.util
import io
import os
from pathlib import Path
import socket
import sys
import tempfile


def check(sdk_root):
    sdk_lib = Path(sdk_root).resolve()/'lib'
    if not (sdk_lib/'googlecloudsdk/gcloud_main.py').is_file():
        raise ValueError('Expected an official Google Cloud SDK root.')
    sys.path[:0] = [str(sdk_lib/'third_party'), str(sdk_lib), str(Path(__file__).parent)]
    from unittest.mock import patch
    import test_release as fixture
    from googlecloudsdk import gcloud_main
    from googlecloudsdk.core import properties
    from googlecloudsdk.generated_clients.apis.run.v2 import run_v2_messages as messages
    from apitools.base.py import encoding

    commands = 0
    bodies = 0
    with tempfile.TemporaryDirectory(prefix='dq-release-sdk-') as config, \
         patch.dict(os.environ,{'CLOUDSDK_CONFIG':config,'CLOUDSDK_CORE_DISABLE_USAGE_REPORTING':'true',
                                'CLOUDSDK_CORE_DISABLE_PROMPTS':'true','CLOUDSDK_PAGER':'cat'}), \
         patch.object(socket.socket,'connect',side_effect=RuntimeError('Network forbidden in SDK audit')), \
         patch.object(socket,'create_connection',side_effect=RuntimeError('Network forbidden in SDK audit')):
        properties.VALUES.core.project.Set(fixture.PROJECT)
        for rollback,validate_only in ((False,False),(True,False),(False,True)):
            cloud = fixture.FakeCloud()
            cfg = fixture.ReleaseTests().cfg(rollback)
            with contextlib.redirect_stdout(io.StringIO()):
                fixture.r.release(cloud,cfg,validate_only=validate_only)
            for call in cloud.calls:
                if call[0]=='cli':
                    cli = gcloud_main.CreateCLI([])
                    cli._CLI__parser.parse_args(['--project',fixture.PROJECT,'--quiet',*call[1]])
                    commands += 1
                elif call[0]=='PATCH':
                    typed = encoding.DictToMessage(call[2],messages.GoogleCloudRunV2Service)
                    assert encoding.MessageToDict(typed)==call[2], 'SDK changed a release request field'
                    request = messages.RunProjectsLocationsServicesPatchRequest(
                        name=fixture.NAME,googleCloudRunV2Service=typed,
                        updateMask='template,labels,traffic',validateOnly='validateOnly=true' in call[1])
                    assert not request.allowMissing
                    bodies += 1
        cli = gcloud_main.CreateCLI([])
        cli._CLI__parser.parse_args(['--project',fixture.PROJECT,'--quiet','auth','configure-docker',
                                    fixture.REGION+'-docker.pkg.dev'])
    print(f'PASS official SDK: {commands + 1} release/build-auth CLI commands parsed; {bodies} PATCH schemas round-tripped.')
    print('No GCP execution/network; real WIF/admission/startup remain NOT RUN.')


if __name__=='__main__':
    if len(sys.argv)!=2:
        raise SystemExit(__doc__)
    check(sys.argv[1])
