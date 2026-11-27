"""Offline tests for temporary CI configuration boundaries; no Docker/network."""
import os
from pathlib import Path
import subprocess
import tempfile
import unittest


SCRIPT = Path(__file__).resolve().parents[1] / 'prepare-env.mjs'


def values(path):
    return dict(line.split('=', 1) for line in path.read_text().splitlines() if line)


class PrepareEnvironmentTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.runner = self.root / 'runner'
        self.runner.mkdir()
        self.exports = self.root / 'exports'
        self.env = {
            **os.environ, 'RUNNER_TEMP': str(self.runner), 'CI_PG_PORT': '35432',
            'GITHUB_ENV': str(self.exports), 'GITHUB_ACTIONS': 'false',
            'GITHUB_RUN_ID': '12345', 'GITHUB_RUN_ATTEMPT': '1',
            'CI_WEB_HOST_PORT': '3042', 'CI_API_HOST_PORT': '8094', 'CI_DB_HOST_PORT': '55442',
        }

    def run_script(self, **overrides):
        return subprocess.run(['node', str(SCRIPT)], cwd=self.root,
                              env={**self.env, **overrides}, capture_output=True, text=True)

    def test_isolated_configuration_does_not_read_or_print_developer_credentials(self):
        canary = 'developer-canary-must-never-be-read'
        (self.root / '.env').write_text('DATABASE_URL=' + canary)
        result = self.run_script(DATABASE_URL=canary)
        self.assertEqual(result.returncode, 0)
        exported = values(self.exports)
        app = values(Path(exported['DINEQUEUE_COMPOSE_ENV']))
        browser = values(Path(exported['DINEQUEUE_BROWSER_ENV']))
        go = values(Path(exported['DINEQUEUE_GO_ENV']))
        self.assertNotIn(canary, result.stdout + result.stderr + repr(app) + repr(browser) + repr(go))
        self.assertIn('@127.0.0.1:35432/dinequeue_ci_test?', go['TEST_DATABASE_URL'])
        self.assertIn('@db:5432/dinequeue_browser?', app['DATABASE_URL'])
        for key in ['POSTGRES_PASSWORD', 'DATABASE_URL', 'SEED_HOST_PASSWORD']:
            self.assertNotIn(app[key], result.stdout + result.stderr)
        if os.name != 'nt':
            self.assertEqual(Path(exported['DINEQUEUE_COMPOSE_ENV']).stat().st_mode & 0o777, 0o600)
            self.assertEqual(Path(exported['DINEQUEUE_CI_DIR']).stat().st_mode & 0o777, 0o700)

    def test_local_override_ports_remain_consistent_between_app_origin_and_fixtures(self):
        self.assertEqual(self.run_script().returncode, 0)
        exported = values(self.exports)
        app = values(Path(exported['DINEQUEUE_COMPOSE_ENV']))
        browser = values(Path(exported['DINEQUEUE_BROWSER_ENV']))
        self.assertEqual(app['WEB_HOST_PORT'], '3042')
        self.assertEqual(app['API_HOST_PORT'], '8094')
        self.assertEqual(browser['F01_BASE_URL'], 'http://localhost:3042')
        self.assertIn('@127.0.0.1:55442/', browser['TEST_DATABASE_URL'])
        self.assertIn('127.0.0.1:55442:5432', (Path(exported['DINEQUEUE_CI_DIR']) / 'browser-ports.yaml').read_text())

    def test_new_invocations_do_not_reuse_credentials_or_private_files(self):
        self.assertEqual(self.run_script().returncode, 0)
        first = values(Path(values(self.exports)['DINEQUEUE_COMPOSE_ENV']))
        self.assertEqual(self.run_script().returncode, 0)
        second = values(Path(values(self.exports)['DINEQUEUE_COMPOSE_ENV']))
        self.assertNotEqual(first['POSTGRES_PASSWORD'], second['POSTGRES_PASSWORD'])
        self.assertNotEqual(first['SEED_HOST_PASSWORD'], second['SEED_HOST_PASSWORD'])

    def test_github_masks_every_generated_credential_before_exporting_paths(self):
        result = self.run_script(GITHUB_ACTIONS='true')
        self.assertEqual(result.returncode, 0)
        exported = values(self.exports)
        app = values(Path(exported['DINEQUEUE_COMPOSE_ENV']))
        browser = values(Path(exported['DINEQUEUE_BROWSER_ENV']))
        for value in [app['POSTGRES_PASSWORD'], app['DATABASE_URL'], app['SEED_HOST_PASSWORD'], browser['TEST_DATABASE_URL']]:
            self.assertIn('::add-mask::' + value, result.stdout)
        self.assertNotIn(app['POSTGRES_PASSWORD'], self.exports.read_text())

    def test_invalid_service_port_fails_before_creating_any_private_files(self):
        for port in ['', '0', '65536', '5432; echo unsafe']:
            with self.subTest(port=port):
                self.assertNotEqual(self.run_script(CI_PG_PORT=port).returncode, 0)
                self.assertEqual(list(self.runner.iterdir()), [])
                self.assertFalse(self.exports.exists())

    def test_invalid_project_identifier_fails_before_creating_any_private_files(self):
        self.assertNotEqual(self.run_script(GITHUB_RUN_ID='bad/../project').returncode, 0)
        self.assertEqual(list(self.runner.iterdir()), [])
        self.assertFalse(self.exports.exists())

    def test_invalid_or_colliding_host_ports_fail_without_creating_files(self):
        for overrides in [{'CI_WEB_HOST_PORT': '0'}, {'CI_API_HOST_PORT': 'not-a-port'},
                          {'CI_DB_HOST_PORT': '65536'}, {'CI_DB_HOST_PORT': '3042'}]:
            with self.subTest(overrides=overrides):
                self.assertNotEqual(self.run_script(**overrides).returncode, 0)
                self.assertEqual(list(self.runner.iterdir()), [])


if __name__ == '__main__':
    unittest.main()
