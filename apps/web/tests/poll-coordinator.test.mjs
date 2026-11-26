// Lab 7 #44 polling coordinator: deterministic scheduling with fake timers.
// Node >= 22.18 strips the erasable TypeScript in this dependency-free module.
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { createPoller } from '../src/lib/poll-coordinator.ts';

function fakeTimers() {
  let now = 0; let seq = 0;
  const pending = new Map();
  return {
    timers: {
      set(fn, ms) { const id = ++seq; pending.set(id, { at: now + ms, fn }); return id; },
      clear(id) { pending.delete(id); },
    },
    now: () => now,
    count: () => pending.size,
    // Fire due timers in time order; microtasks settle between them.
    async advance(ms) {
      const end = now + ms;
      for (;;) {
        const next = [...pending.entries()].filter(([, t]) => t.at <= end).sort((a, b) => a[1].at - b[1].at)[0];
        if (!next) break;
        pending.delete(next[0]); now = next[1].at; next[1].fn();
        await flush();
      }
      now = end; await flush();
    },
  };
}
const flush = async () => { for (let i = 0; i < 20; i++) await Promise.resolve(); };

function deferred() {
  let resolve, reject;
  const promise = new Promise((res, rej) => { resolve = res; reject = rej; });
  return { promise, resolve, reject };
}

function setup(overrides = {}) {
  const clock = fakeTimers();
  const log = { reads: 0, data: [], failures: [], lost: [] };
  let visible = true;
  const reads = [];
  const poller = createPoller({
    read: signal => { log.reads++; const d = deferred(); reads.push({ ...d, signal });
      signal.addEventListener('abort', () => d.reject(new DOMException('aborted', 'AbortError'))); return d.promise; },
    onData: data => log.data.push(data),
    onFailure: f => log.failures.push(f.kind),
    onAccessLost: s => log.lost.push(s),
    isVisible: () => visible,
    timers: clock.timers, now: clock.now, intervalMs: 5000, timeoutMs: 10000,
    ...overrides,
  });
  return { clock, log, poller, reads, setVisible(v) { visible = v; poller.visibilityChanged(); } };
}
const ok = data => ({ kind: 'ok', data });

test('reads every 5 s after the previous read settles, never overlapping', async () => {
  const { clock, log, poller, reads } = setup();
  poller.start();
  await clock.advance(4999); assert.equal(log.reads, 0);
  await clock.advance(1); assert.equal(log.reads, 1);
  // A slow read (8 s) must not start a second request or a catch-up burst.
  await clock.advance(8000); assert.equal(log.reads, 1);
  reads[0].resolve(ok('A')); await flush();
  assert.deepEqual(log.data, ['A']);
  await clock.advance(4999); assert.equal(log.reads, 1);
  await clock.advance(1); assert.equal(log.reads, 2);
});

test('manual refresh shares the in-flight read and ignores a superseded response', async () => {
  const { clock, log, poller, reads } = setup();
  poller.start();
  await clock.advance(5000);
  const manual = poller.refresh();
  assert.equal(log.reads, 1, 'manual refresh joins the in-flight read');
  reads[0].resolve(ok('B')); await manual;
  assert.deepEqual(log.data, ['B']);
  // Fresh manual refresh replaces the pending timer instead of adding a stream.
  const second = poller.refresh(); assert.equal(log.reads, 2);
  reads[1].resolve(ok('C')); await second;
  await clock.advance(4999); assert.equal(log.reads, 2);
});

test('a mutation invalidates the pre-write read and refreshes after commit', async () => {
  const { clock, log, poller, reads } = setup();
  poller.start();
  await clock.advance(5000); // read #1 (pre-mutation snapshot) in flight
  const write = deferred();
  const mutation = poller.runMutation(() => write.promise);
  assert.equal(reads[0].signal.aborted, true);
  await clock.advance(20000);
  assert.equal(log.reads, 1, 'no timer reads while the write is pending');
  assert.equal(poller.refresh() instanceof Promise, true);
  assert.equal(log.reads, 1, 'manual refresh is deferred during the write');
  write.resolve('done');
  assert.equal(await mutation, 'done');
  await flush();
  assert.equal(log.reads, 2, 'one refetch after the write');
  reads[1].resolve(ok('after-write')); await flush();
  assert.deepEqual(log.data, ['after-write']);
});

test('a stale response that resolves after a mutation is never applied', async () => {
  // Simulate a fetch implementation that ignores abort and resolves late.
  const clock = fakeTimers();
  const data = []; const slots = [];
  const poller = createPoller({
    read: () => { const d = deferred(); slots.push(d); return d.promise; },
    onData: d => data.push(d), onFailure() {}, onAccessLost() {}, isVisible: () => true,
    timers: clock.timers, now: clock.now,
  });
  poller.start();
  await clock.advance(5000);
  await poller.runMutation(async () => 'ok');
  slots[1].resolve(ok('committed')); await flush();
  slots[0].resolve(ok('pre-mutation')); await flush();
  assert.deepEqual(data, ['committed']);
});

test('pauses while hidden and reads once immediately when visible again', async () => {
  const { clock, log, poller, reads, setVisible } = setup();
  poller.start();
  setVisible(false);
  await clock.advance(60000); assert.equal(log.reads, 0);
  setVisible(true); assert.equal(log.reads, 1, 'immediate read, no burst');
  reads[0].resolve(ok('V')); await flush();
  await clock.advance(4999); assert.equal(log.reads, 1);
  // Hiding during a read aborts and invalidates it.
  await clock.advance(1); assert.equal(log.reads, 2);
  setVisible(false); assert.equal(reads[1].signal.aborted, true);
  await clock.advance(60000); assert.equal(log.reads, 2);
});

test('stop clears timers and aborts the in-flight read', async () => {
  const { clock, log, poller, reads } = setup();
  poller.start();
  await clock.advance(5000);
  poller.stop();
  assert.equal(reads[0].signal.aborted, true);
  assert.equal(clock.count(), 0, 'no leaked timer');
  await clock.advance(60000); assert.equal(log.reads, 1);
  assert.deepEqual(log.data, []);
});

test('a stalled read times out, keeps polling and recovers', async () => {
  const { clock, log, poller, reads } = setup();
  poller.start();
  await clock.advance(5000);
  await clock.advance(10000);
  assert.equal(reads[0].signal.aborted, true);
  assert.deepEqual(log.failures, ['timeout']);
  await clock.advance(5000); assert.equal(log.reads, 2);
  reads[1].resolve(ok('recovered')); await flush();
  assert.deepEqual(log.data, ['recovered']);
});

test('network and server failures keep polling; access loss stops it', async () => {
  const { clock, log, poller, reads } = setup();
  poller.start();
  await clock.advance(5000); reads[0].reject(new TypeError('Failed to fetch')); await flush();
  await clock.advance(5000); reads[1].resolve({ kind: 'status', status: 503 }); await flush();
  assert.deepEqual(log.failures, ['network', 'server']);
  assert.deepEqual(log.data, [], 'a failure is never turned into data');
  await clock.advance(5000); reads[2].resolve({ kind: 'status', status: 401 }); await flush();
  assert.deepEqual(log.lost, [401]);
  await clock.advance(60000); assert.equal(log.reads, 3, 'no reads after access is lost');
  assert.equal(clock.count(), 0);
});
