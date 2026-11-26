import { test, expect, type Browser, type Page } from '@playwright/test';
import { createHash, randomBytes, randomUUID } from 'node:crypto';
import { execFileSync } from 'node:child_process';

// Lab 7 polling (#44). Generated IDs only, isolated TEST_DATABASE_URL only.
function sql(query: string) {
  if (!process.env.TEST_DATABASE_URL) throw new Error('Isolated TEST_DATABASE_URL required.');
  execFileSync('psql', ['--no-psqlrc', '--dbname', process.env.TEST_DATABASE_URL,
    '--set', 'ON_ERROR_STOP=1', '--command', query], { stdio: 'ignore' });
}

type Client = { page: Page; csrf: string };
type Fixture = { customer: Client; host: Client; queue: string; customerId: string };

// One private queue, Customer and assigned Host with generated sessions, so the
// suite never depends on or disturbs the shared demo queue.
async function withQueue(browser: Browser, run: (f: Fixture) => Promise<void>) {
  const queue = randomUUID();
  const accounts = ['CUSTOMER', 'HOST'].map(role => ({ role, id: randomUUID(), token: randomBytes(32).toString('base64url') }));
  const contexts = await Promise.all(accounts.map(() => browser.newContext()));
  try {
    sql(`BEGIN;
      INSERT INTO service_queues(id,restaurant_id,name,ticket_prefix) VALUES
      ('${queue}','30000000-0000-4000-8000-000000000001','Polling fixture','P');
      ${accounts.map(a => `INSERT INTO users(id,display_name,email,password_hash,role) VALUES
      ('${a.id}','Polling fixture','${a.id}@example.test','not-a-login-hash','${a.role}');
      INSERT INTO sessions(id,user_id,token_hash,created_at,expires_at) VALUES
      ('${randomUUID()}','${a.id}','${createHash('sha256').update(a.token).digest('hex')}',now(),now()+interval '1 hour');`).join('\n')}
      INSERT INTO operator_queue_permissions(user_id,queue_id) VALUES ('${accounts[1].id}','${queue}'); COMMIT;`);
    const clients: Client[] = [];
    for (let i = 0; i < accounts.length; i++) {
      await contexts[i].addCookies([{ name: 'dineq_session', value: accounts[i].token,
        url: process.env.F01_BASE_URL!, httpOnly: true, sameSite: 'Lax' }]);
      const page = await contexts[i].newPage();
      const me = await page.request.get('/api/v1/auth/me');
      expect(me.status()).toBe(200);
      clients.push({ page, csrf: (await me.json()).csrfToken as string });
    }
    await run({ customer: clients[0], host: clients[1], queue, customerId: accounts[0].id });
  } finally {
    await Promise.all(contexts.map(context => context.close()));
    sql(`BEGIN; DELETE FROM queue_entries WHERE queue_id='${queue}';
      DELETE FROM service_queues WHERE id='${queue}';
      DELETE FROM users WHERE id IN ('${accounts[0].id}','${accounts[1].id}'); COMMIT;`);
  }
}

const headers = (c: Client) => ({ Origin: process.env.F01_BASE_URL!, 'X-CSRF-Token': c.csrf });

async function join(f: Fixture) {
  const joined = await f.customer.page.request.post(`/api/v1/queues/${f.queue}/entries`, {
    headers: headers(f.customer), data: { partySize: 3 },
  });
  expect(joined.status()).toBe(201);
  return joined.json() as Promise<{ id: string; ticketCode: string }>;
}

// Updates arrive on the next scheduled refresh (5 s after the previous one
// settles); allow latency but well under any manual interaction.
const AUTO = { timeout: 15_000 };

async function setReady(f: Fixture, id: string, isReady: boolean) {
  const r = await f.customer.page.request.patch(`/api/v1/me/queue-entries/${id}/readiness`, {
    headers: headers(f.customer), data: { isReady },
  });
  expect(r.status()).toBe(200);
}
async function hostPost(f: Fixture, path: string) {
  const r = await f.host.page.request.post(`/api/v1/host/queues/${f.queue}${path}`, { headers: headers(f.host) });
  expect(r.status()).toBe(200);
}

// All state changes below go through the real API from another session; no
// test clicks Refresh, so only automatic updates can satisfy the assertions.
test('L7-T-19a Host call becomes CALLED on Customer list and detail without refresh', async ({ browser }) => {
  await withQueue(browser, async f => {
    const ticket = await join(f);
    await setReady(f, ticket.id, true);
    const list = f.customer.page;
    const detail = await list.context().newPage();
    await list.goto('/my-queue');
    await detail.goto(`/my-queue/${ticket.id}`);
    await expect(list.getByTestId('ticket-status')).toHaveText('WAITING');
    await expect(detail.getByTestId('ticket-status')).toHaveText('WAITING');
    await hostPost(f, '/call-next');
    await expect(list.getByTestId('ticket-status')).toHaveText('CALLED', AUTO);
    await expect(detail.getByTestId('ticket-status')).toHaveText('CALLED', AUTO);
    await expect(list.getByText('Your ticket has been called. Please go to the counter.', { exact: true })).toBeVisible();
    await expect(list.getByRole('button', { name: /Mark (not )?ready/ })).toHaveCount(0);
  });
});

test('L7-T-19b Customer readiness and call state reach the Host board without refresh', async ({ browser }) => {
  await withQueue(browser, async f => {
    const ticket = await join(f);
    const host = f.host.page;
    await host.goto(`/host/queue?queue=${f.queue}`);
    await expect(host.getByTestId('waiting-row')).toContainText('Not ready');
    await expect(host.getByRole('button', { name: 'Call next', exact: true })).toBeDisabled();
    await setReady(f, ticket.id, true);
    await expect(host.getByTestId('waiting-row').getByText('Ready', { exact: true })).toBeVisible(AUTO);
    await expect(host.getByRole('button', { name: 'Call next', exact: true })).toBeEnabled();
    // A call made by another Host session: current card fills, waiting list empties.
    await hostPost(f, '/call-next');
    await expect(host.getByTestId('current-ticket')).toHaveText(ticket.ticketCode, AUTO);
    await expect(host.getByTestId('waiting-row')).toHaveCount(0);
  });
});

test('L7-T-19c Completion leaves the active list; owned detail stays readable', async ({ browser }) => {
  await withQueue(browser, async f => {
    const ticket = await join(f);
    await setReady(f, ticket.id, true);
    await hostPost(f, '/call-next');
    const list = f.customer.page;
    const detail = await list.context().newPage();
    const host = f.host.page;
    await list.goto('/my-queue');
    await detail.goto(`/my-queue/${ticket.id}`);
    await host.goto(`/host/queue?queue=${f.queue}`);
    await expect(list.getByTestId('ticket-status')).toHaveText('CALLED');
    await expect(host.getByTestId('current-ticket')).toHaveText(ticket.ticketCode);
    await hostPost(f, `/entries/${ticket.id}/complete`);
    await expect(list.getByText('You do not have an active ticket.', { exact: true })).toBeVisible(AUTO);
    await expect(list.getByTestId('ticket-code')).toHaveCount(0);
    await expect(detail.getByTestId('ticket-status')).toHaveText('COMPLETED', AUTO);
    await expect(host.getByText('No ticket is currently called.', { exact: true })).toBeVisible(AUTO);
  });
});
