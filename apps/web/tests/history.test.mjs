import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';

// Static contract checks for the history client (no TS runtime here): the reader
// targets the exact same-origin path, and the rewrite precedes the dynamic route.
test('history rewrite is exact and registered before the :entryId rewrite', () => {
  const config = readFileSync(new URL('../next.config.mjs', import.meta.url), 'utf8');
  const history = config.indexOf("source: '/api/v1/me/queue-entries/history'");
  const dynamic = config.indexOf("source: '/api/v1/me/queue-entries/:entryId'");
  assert.ok(history > 0 && dynamic > history);
});

test('history reader never polls and uses no-store same-origin requests', () => {
  const reader = readFileSync(new URL('../src/lib/history.ts', import.meta.url), 'utf8');
  assert.match(reader, /cache: 'no-store'/);
  assert.match(reader, /credentials: 'same-origin'/);
  const component = readFileSync(new URL('../src/components/ticket-history.tsx', import.meta.url), 'utf8');
  assert.doesNotMatch(component, /setInterval|useLiveResource|usePoll|createPoll/);
});
