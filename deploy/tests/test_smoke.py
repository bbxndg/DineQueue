"""DineQueue smoke checks, network mocked and no cloud credentials."""
import contextlib
import importlib.util
import io
from pathlib import Path
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location('smoke', Path(__file__).parents[1] / 'smoke_test.py')
smoke = importlib.util.module_from_spec(spec)
spec.loader.exec_module(smoke)


class SmokeTests(unittest.TestCase):
    def responses(self, path, data=None, csrf=None):
        return {
            '/api/health/live': (200,b'{"status":"ok"}'),
            '/api/health/ready': (200,b'{"status":"ready"}'),
            '/login': (200,b'<html>Sign in</html>'),
            '/api/v1/auth/me': (401,b'{"error":{"code":"UNAUTHENTICATED"}}'),
        }[path]

    def test_read_only_public_checks_and_anonymous_authorization(self):
        with patch.object(smoke, 'Client') as client, contextlib.redirect_stdout(io.StringIO()):
            client.return_value.request.side_effect=self.responses
            smoke.check('https://dinequeue.example.test', {})
            self.assertEqual(client.return_value.request.call_count,4)

    def test_db_not_ready_fails(self):
        def unavailable(path, **kwargs):
            return (503,b'{"status":"unavailable"}') if path=='/api/health/ready' else self.responses(path)
        with patch.object(smoke, 'Client') as client, patch.object(smoke.time,'sleep'), contextlib.redirect_stdout(io.StringIO()):
            client.return_value.request.side_effect=unavailable
            with self.assertRaisesRegex(RuntimeError,'/api/health/ready'):
                smoke.check('https://dinequeue.example.test',{})

    def test_rejects_insecure_or_credentialed_origin_before_network(self):
        for origin in ('http://example.test','https://user:secret@example.test','https://example.test/path','https://example.test?query=1'):
            with self.assertRaises(ValueError): smoke.check(origin,{})

    def test_partial_credentials_are_rejected_before_network(self):
        with patch.object(smoke,'Client',side_effect=AssertionError('network client created')):
            with self.assertRaises(ValueError): smoke.check('https://example.test',{'SMOKE_EMAIL':'test@example.test'})

    def test_logout_even_after_authenticated_identity_failure(self):
        def response(path,data=None,csrf=None):
            if path=='/api/v1/auth/login': return 200,b'{"csrfToken":"test-csrf","user":{"role":"HOST"}}'
            if path=='/api/v1/auth/logout': return 204,b''
            if path=='/api/v1/auth/me' and client.return_value.request.call_count>4: return 500,b'{}'
            return self.responses(path,data,csrf)
        with patch.object(smoke,'Client') as client, contextlib.redirect_stdout(io.StringIO()):
            client.return_value.request.side_effect=response
            client.return_value.secure_session.return_value=True
            with self.assertRaisesRegex(RuntimeError,'Authenticated'):
                smoke.check('https://example.test',{'SMOKE_EMAIL':'test@example.test','SMOKE_PASSWORD':'not-a-real-password'})
            self.assertEqual(client.return_value.request.call_args.args[0],'/api/v1/auth/logout')

    def test_missing_secure_cookie_is_rejected_and_session_revoked(self):
        def response(path,data=None,csrf=None):
            if path=='/api/v1/auth/login': return 200,b'{"csrfToken":"test-csrf","user":{"role":"CUSTOMER"}}'
            if path=='/api/v1/auth/logout': return 204,b''
            return self.responses(path,data,csrf)
        with patch.object(smoke,'Client') as client, contextlib.redirect_stdout(io.StringIO()):
            client.return_value.request.side_effect=response
            client.return_value.secure_session.return_value=False
            with self.assertRaisesRegex(RuntimeError,'Secure'):
                smoke.check('https://example.test',{'SMOKE_EMAIL':'test@example.test','SMOKE_PASSWORD':'not-a-real-password'})
            self.assertEqual(client.return_value.request.call_args.args[0],'/api/v1/auth/logout')


if __name__=='__main__': unittest.main()
