import test from 'node:test';
import assert from 'node:assert/strict';

test('EP6 intentional failure: prove required CI blocks merging', () => {
  assert.fail('Intentional gate proof. Close this PR without merging.');
});
