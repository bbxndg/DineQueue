"""Execute the real build script with fake Docker/gcloud and a disposable Git repo."""
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]


class BuildImagesTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory(prefix='dq-build-test-')
        self.addCleanup(self.tmp.cleanup)
        self.base = Path(self.tmp.name)
        self.repo = self.base/'repo'; self.repo.mkdir()
        self.bin = self.base/'bin'; self.bin.mkdir()
        (self.repo/'deploy').mkdir()
        shutil.copyfile(ROOT/'deploy/build-images.sh',self.repo/'deploy/build-images.sh')
        for folder in ('apps/api','apps/web'):
            path = self.repo/folder; path.mkdir(parents=True)
            (path/'Dockerfile').write_text('FROM scratch AS production\nFROM scratch AS db-tools\n')
        (self.repo/'.gitignore').write_text('.env\ngha-creds-*.json\n')
        (self.repo/'.env.example').write_text('POSTGRES_PASSWORD=\n')
        (self.repo/'tracked.txt').write_text('reviewed source')
        self.git('init','-q'); self.git('add','.')
        self.git('-c','user.name=Offline fixture','-c','user.email=fixture@example.test',
                 '-c','commit.gpgsign=false','commit','-qm','fixture')
        self.sha = self.git('rev-parse','HEAD').strip()
        (self.repo/'.env').write_text('PASSWORD=private-local-canary')
        (self.repo/'gha-creds-private.json').write_text('private-wif-canary')
        (self.repo/'untracked.txt').write_text('must not be uploaded')
        executable = '''#!/usr/bin/env python3
import json, os, pathlib, sys
args = sys.argv[1:]
kind = pathlib.Path(sys.argv[0]).name
entry = {'tool':kind,'args':args}
if kind == 'docker' and args[0] == 'build':
    context = pathlib.Path(args[-1])
    entry['context_files'] = sorted(str(p.relative_to(context)) for p in context.rglob('*') if p.is_file())
with open(os.environ['STUB_LOG'],'a') as log:
    log.write(json.dumps(entry)+'\\n')
if os.environ.get('STUB_FAIL') == kind+':'+args[0]:
    sys.exit(1)
'''
        for name in ('docker','gcloud'):
            path = self.bin/name; path.write_text(executable); path.chmod(0o700)
        self.log = self.base/'commands.jsonl'
        self.env = dict(os.environ,PATH=str(self.bin)+os.pathsep+os.environ['PATH'],
                        PROJECT_ID='release-test',REGION='asia-southeast1',IMAGE_TAG=self.sha,
                        RUNNER_TEMP=str(self.base),STUB_LOG=str(self.log))

    def git(self,*args):
        return subprocess.check_output(['git',*args],cwd=self.repo,text=True,stderr=subprocess.DEVNULL)

    def run_build(self,**env):
        result = subprocess.run(['bash','deploy/build-images.sh'],cwd=self.repo,
                                env=dict(self.env,**env),capture_output=True,text=True)
        calls = [json.loads(line) for line in self.log.read_text().splitlines()] if self.log.exists() else []
        return result, calls

    def test_three_targets_share_sha_and_context_excludes_credentials_and_untracked_files(self):
        result,calls = self.run_build()
        self.assertEqual(result.returncode,0,result.stderr)
        builds = [c for c in calls if c['tool']=='docker' and c['args'][0]=='build']
        pushes = [c for c in calls if c['tool']=='docker' and c['args'][0]=='push']
        self.assertEqual(len(builds),3); self.assertEqual(len(pushes),3)
        for call in builds:
            self.assertIn('tracked.txt',call['context_files'])
            self.assertIn('.env.example',call['context_files'])
            self.assertNotIn('.env',call['context_files'])
            self.assertNotIn('gha-creds-private.json',call['context_files'])
            self.assertNotIn('untracked.txt',call['context_files'])
            self.assertFalse(Path(call['args'][-1]).exists(),'temporary build source was not cleaned')
        images = {c['args'][c['args'].index('--tag')+1] for c in builds}
        expected = {f'asia-southeast1-docker.pkg.dev/release-test/dinequeue/{name}:{self.sha}' for name in ('api','web','db-tools')}
        self.assertEqual(images,expected)
        self.assertEqual({c['args'][1] for c in pushes},expected)
        self.assertIn('db-tools',[c['args'][c['args'].index('--target')+1] for c in builds])
        self.assertNotIn('private-',result.stdout+result.stderr)

    def test_build_failure_never_pushes_and_cleans_context(self):
        result,calls = self.run_build(STUB_FAIL='docker:build')
        self.assertNotEqual(result.returncode,0)
        self.assertFalse(any(c['tool']=='docker' and c['args'][0]=='push' for c in calls))
        self.assertFalse(list(self.base.glob('dinequeue-release-source.*')))

    def test_push_failure_stops_later_pushes(self):
        result,calls = self.run_build(STUB_FAIL='docker:push')
        self.assertNotEqual(result.returncode,0)
        self.assertEqual(sum(c['tool']=='docker' and c['args'][0]=='push' for c in calls),1)

    def test_tracked_private_file_is_rejected_before_cloud_or_docker(self):
        self.git('add','-f','.env')
        result,calls = self.run_build()
        self.assertNotEqual(result.returncode,0)
        self.assertEqual(calls,[])
        self.assertNotIn('private-local-canary',result.stdout+result.stderr)

    def test_invalid_inputs_and_wrong_source_sha_are_rejected_before_cloud(self):
        for env in ({'PROJECT_ID':'bad;project'}, {'REGION':'bad'},
                    {'IMAGE_TAG':'bad'}, {'IMAGE_TAG':'0'*40}):
            with self.subTest(env=env):
                result,calls = self.run_build(**env)
                self.assertNotEqual(result.returncode,0)
                self.assertEqual(calls,[])


if __name__ == '__main__':
    unittest.main()
