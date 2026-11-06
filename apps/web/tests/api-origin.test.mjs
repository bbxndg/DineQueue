import { test } from 'node:test';
import assert from 'node:assert/strict';
import { apiOrigin } from '../config/api-origin.mjs';

test('local and container upstream origins are accepted', () => {
  assert.equal(apiOrigin(), 'http://127.0.0.1:8080');
  assert.equal(apiOrigin('http://api:8080/'), 'http://api:8080');
  assert.equal(apiOrigin('https://api.example.test'), 'https://api.example.test');
});

test('invalid upstream configuration fails without echoing credentials', () => {
  for (const value of ['', 'file:///etc/passwd', 'http://user:secret@api:8080',
    'http://api:8080/private', 'http://api:8080?token=secret', 'http://api:8080/#secret']) {
    assert.throws(() => apiOrigin(value), (error) => {
      assert.match(error.message, /API_INTERNAL_URL/);
      assert.doesNotMatch(error.message, /secret/);
      return true;
    });
  }
});
