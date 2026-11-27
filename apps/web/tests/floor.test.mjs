import { test } from 'node:test';
import assert from 'node:assert/strict';
import { parseFloor } from '../src/lib/floor.ts';

const snapshot = () => ({ queueId: 'q1', restaurantId: 'r1', operationsMode: 'MANUAL', layoutVersion: 1,
  holdTtlSeconds: 300, serverNow: '2026-11-27T00:00:00Z', assignments: [], zones: [{ id: 'z1', name: 'Terrace', isActive: true,
    tables: [{ id: 't1', code: 'T4', capacity: 6, status: 'AVAILABLE', positionX: 20, positionY: 50, version: 1 },
      { id: 't2', code: 'T5', capacity: 4, status: 'DIRTY', positionX: 80, positionY: 50, version: 2 }], connections: [{ tableIds: ['t1', 't2'] }] }] });

test('Floor distinguishes a configured empty floor from a malformed response', () => {
  const data = snapshot(); data.zones = [];
  assert.equal(parseFloor(data, 'q1'), data);
  for (const zones of [null, undefined, {}]) assert.throws(() => parseFloor({ ...data, zones }, 'q1'));
  assert.throws(() => parseFloor(snapshot(), 'foreign-queue'));
});

test('Floor rejects invalid metadata, table states, capacities and nonfinite coordinates', () => {
  for (const [key, value] of [['capacity', 0], ['capacity', 9], ['status', 'FREE'], ['positionX', NaN], ['positionY', Infinity], ['positionX', -1], ['version', 0]]) {
    const data = snapshot(); data.zones[0].tables[0][key] = value;
    assert.throws(() => parseFloor(data, 'q1'), key);
  }
  for (const [key, value] of [['serverNow', 'yesterday'], ['operationsMode', 'AUTO'], ['layoutVersion', 0], ['holdTtlSeconds', 1801]]) {
    assert.throws(() => parseFloor({ ...snapshot(), [key]: value }, 'q1'), key);
  }
});

test('Floor accepts explicit approved edges and rejects self, reversed, duplicate and foreign endpoints', () => {
  assert.equal(parseFloor(snapshot(), 'q1').zones[0].connections.length, 1);
  for (const tableIds of [['t1', 't1'], ['t2', 't1'], ['t1', 'other-zone-table']]) {
    const data = snapshot(); data.zones[0].connections = [{ tableIds }];
    assert.throws(() => parseFloor(data, 'q1'));
  }
  const data = snapshot(); data.zones[0].connections.push({ tableIds: ['t1', 't2'] });
  assert.throws(() => parseFloor(data, 'q1'));
});
