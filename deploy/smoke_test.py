"""DineQueue HTTPS smoke checks, Python standard library only.

Default: read-only health/login-page/anonymous authorization. Optional SMOKE_EMAIL
and SMOKE_PASSWORD check secure login/me/logout; no queue mutation. The complete
Customer/Host journey still requires separate evidence. No credentials/tokens or
response bodies are printed. Redirects are rejected, including cross-origin ones.
"""
import http.cookiejar
import json
import os
import sys
import time
import urllib.error
import urllib.parse
import urllib.request


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        return None


class Client:
    def __init__(self, base):
        self.base = base
        self.jar = http.cookiejar.CookieJar()
        self.opener = urllib.request.build_opener(NoRedirect(), urllib.request.HTTPCookieProcessor(self.jar))

    def request(self, path, data=None, csrf=None):
        headers = {}
        if data is not None:
            headers.update({'Content-Type':'application/json','Origin':self.base})
        if csrf:
            headers['X-CSRF-Token'] = csrf
        req = urllib.request.Request(self.base+path, headers=headers,
                                     data=json.dumps(data).encode() if data is not None else None)
        try:
            with self.opener.open(req, timeout=30) as response:
                return response.status, response.read(65536)
        except urllib.error.HTTPError as error:
            return error.code, error.read(65536)

    def secure_session(self):
        return any(c.name=='__Host-dineq_session' and c.secure and c.path=='/' and
                   c.has_nonstandard_attr('HttpOnly') and not c.domain_specified for c in self.jar)


def check(base, env=None):
    env = os.environ if env is None else env
    parsed = urllib.parse.urlparse(base)
    if parsed.scheme!='https' or not parsed.hostname or parsed.username or parsed.password or parsed.path not in ('','/') or parsed.query or parsed.fragment:
        raise ValueError('Smoke base URL must be an HTTPS origin without credentials or a path.')
    base = base.rstrip('/')
    email, password = env.get('SMOKE_EMAIL'), env.get('SMOKE_PASSWORD')
    if bool(email)!=bool(password):
        raise ValueError('Set both SMOKE_EMAIL and SMOKE_PASSWORD, or neither.')
    client = Client(base)
    for path, status, predicate in (
        ('/api/health/live',200,lambda b:json.loads(b).get('status')=='ok'),
        ('/api/health/ready',200,lambda b:json.loads(b).get('status')=='ready'),
        ('/login',200,lambda b:b'<html' in b.lower()),
        ('/api/v1/auth/me',401,lambda b:json.loads(b).get('error',{}).get('code')=='UNAUTHENTICATED'),
    ):
        for attempt in range(5):
            try:
                actual, body = client.request(path)
                if actual!=status or not predicate(body): raise ValueError('Unexpected response')
                break
            except (urllib.error.URLError,TimeoutError,ValueError,AttributeError):
                if attempt==4: raise RuntimeError('Smoke check failed: '+path) from None
                time.sleep(3)
        print('PASS '+path)
    if not email:
        print('SKIP authenticated smoke: credentials not configured; Host/Customer journey not verified')
        return
    csrf = None
    try:
        status, body = client.request('/api/v1/auth/login',data={'email':email,'password':password})
        if status!=200: raise RuntimeError('Authenticated smoke login failed')
        csrf = json.loads(body)['csrfToken']
        if not isinstance(csrf,str) or not csrf: raise ValueError('Invalid CSRF response')
        if not client.secure_session(): raise RuntimeError('Secure HttpOnly host-only session cookie missing')
        status, body = client.request('/api/v1/auth/me')
        if status!=200 or json.loads(body).get('user',{}).get('role') not in ('CUSTOMER','HOST'):
            raise RuntimeError('Authenticated identity smoke failed')
        print('PASS secure login and authenticated identity')
    finally:
        if csrf:
            status, _ = client.request('/api/v1/auth/logout',data={},csrf=csrf)
            if status!=204: raise RuntimeError('Smoke session revocation failed')
            print('PASS logout')


if __name__=='__main__':
    try:
        if len(sys.argv)!=2: raise ValueError('Usage: python3 deploy/smoke_test.py HTTPS_SERVICE_ORIGIN')
        check(sys.argv[1])
    except (ValueError,KeyError,RuntimeError,urllib.error.URLError,TimeoutError):
        sys.exit('Smoke check failed; inspect status safely. Credentials and response payloads are withheld.')
