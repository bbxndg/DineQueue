// Run only against an isolated, seeded EP3 verification database.
// Node 22+: node --env-file=.env scripts/lab5/docker-smoke.mjs [journey|create|verify]
import assert from 'node:assert/strict';
import { readFile, writeFile } from 'node:fs/promises';

const mode = process.argv[2] ?? 'journey';
const origin = `http://localhost:${process.env.WEB_HOST_PORT || '3002'}`;
const stateFile = process.env.EP3_TICKET_FILE;

async function request(path, { client, data, method = data === undefined ? 'GET' : 'POST', status = 200, originHeader = origin, csrf = true } = {}) {
  const headers = {};
  if (client) headers.Cookie = client.cookie;
  if (method === 'POST') {
    if (data !== undefined) headers['Content-Type'] = 'application/json';
    headers.Origin = originHeader;
    if (client && csrf) headers['X-CSRF-Token'] = client.csrf;
  }
  const response = await fetch(`${origin}${path}`, {
    method, headers,
    body: data === undefined ? undefined : JSON.stringify(data),
    redirect: 'manual', signal: AbortSignal.timeout(15000),
  });
  assert.equal(response.status, status, `${path}: unexpected HTTP status`);
  return response;
}

async function login(email, password) {
  if (!password) throw Error('Set the seed passwords in the local environment first.');
  const response = await request('/api/v1/auth/login', { data: { email, password } });
  const body = await response.json();
  const cookie = response.headers.getSetCookie().find(c => c.startsWith('dineq_session='))?.split(';')[0];
  if (!cookie || !body.csrfToken) throw Error('Login did not return the required session.');
  return { cookie, csrf: body.csrfToken };
}

try {
  if (!['journey', 'create', 'verify'].includes(mode)) throw Error('Use journey, create, or verify.');
  if (mode !== 'journey' && !stateFile) throw Error('Set EP3_TICKET_FILE to a private local file.');
  assert.equal((await (await request('/api/health/live')).json()).status, 'ok');
  assert.equal((await (await request('/api/health/ready')).json()).status, 'ready');
  const customer = await login('customer1@example.test', process.env.SEED_CUSTOMER_PASSWORD);

  if (mode === 'verify') {
    const saved = JSON.parse(await readFile(stateFile, 'utf8'));
    const ticket = await (await request(`/api/v1/me/queue-entries/${saved.id}`, { client: customer })).json();
    assert.equal(ticket.ticketCode, saved.ticketCode);
    assert.equal(ticket.status, saved.status);
    console.log(`PASS persisted ticket ${ticket.ticketCode} ${ticket.status} after restart (fresh login)`);
  } else {
    const queues = await (await request('/api/v1/queues', { client: customer })).json();
    const queue = queues.queues[0]?.id;
    if (!queue) throw Error('No seeded queue available.');
    const ticket = await (await request(`/api/v1/queues/${queue}/entries`, {
      client: customer, data: { partySize: 10 }, status: 201,
    })).json();
    assert.equal(ticket.status, 'WAITING');
    console.log(`PASS ticket ${ticket.ticketCode} WAITING, party size 10`);
    if (mode === 'create') {
      await writeFile(stateFile, JSON.stringify({ id: ticket.id, ticketCode: ticket.ticketCode, status: ticket.status }), { mode: 0o600, flag: 'wx' });
    } else {
      const host = await login('host@example.test', process.env.SEED_HOST_PASSWORD);
      const otherCustomer = await login('customer2@example.test', process.env.SEED_CUSTOMER2_PASSWORD);
      await request(`/api/v1/me/queue-entries/${ticket.id}`, { client: otherCustomer, status: 404 });
      await request(`/api/v1/host/queues/${queue}/entries?status=WAITING`, { client: customer, status: 403 });
      await request(`/api/v1/host/queues/${queue}/call-next`, { client: customer, method: 'POST', status: 403 });
      await request(`/api/v1/host/queues/${queue}/call-next`, { client: host, method: 'POST', csrf: false, status: 403 });
      await request(`/api/v1/host/queues/${queue}/call-next`, { client: host, method: 'POST', originHeader: 'http://invalid.example', status: 403 });
      console.log('PASS own-ticket privacy, Customer RBAC, missing CSRF and foreign Origin rejected');
      const called = await (await request(`/api/v1/host/queues/${queue}/call-next`, { client: host, method: 'POST' })).json();
      assert.equal(called.id, ticket.id);
      assert.equal(called.status, 'CALLED');
      assert.equal((await (await request(`/api/v1/me/queue-entries/${ticket.id}`, { client: customer })).json()).status, 'CALLED');
      const completed = await (await request(`/api/v1/host/queues/${queue}/entries/${ticket.id}/complete`, { client: host, method: 'POST' })).json();
      assert.equal(completed.status, 'COMPLETED');
      assert.equal((await (await request(`/api/v1/me/queue-entries/${ticket.id}`, { client: customer })).json()).status, 'COMPLETED');
      console.log('PASS Host Call Next/Complete; Customer reads CALLED and COMPLETED');
      const page = await request('/host/queue', { client: host });
      assert.match(await page.text(), /DineQueue/);
      console.log('PASS authenticated Next.js server-rendered Host page');
      await request('/api/v1/auth/logout', { client: customer, method: 'POST', status: 204 });
      await request('/api/v1/auth/me', { client: customer, status: 401 });
      console.log('PASS logout revokes the session');
    }
  }
} catch (error) {
  // Never dump responses, cookies, credentials or request options on failure.
  console.error(`FAIL EP3 ${mode}: ${error.message}`);
  process.exitCode = 1;
}
