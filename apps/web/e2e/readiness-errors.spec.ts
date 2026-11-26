import { test, expect, type Browser, type Page } from '@playwright/test';
import { createHash, randomBytes, randomUUID } from 'node:crypto';
import { execFileSync } from 'node:child_process';

// All IDs are generated locally and this helper is only for an isolated test DB.
function sql(query: string) {
  if (!process.env.TEST_DATABASE_URL) throw new Error('Isolated TEST_DATABASE_URL required.');
  execFileSync('psql', ['--no-psqlrc', '--dbname', process.env.TEST_DATABASE_URL,
    '--set', 'ON_ERROR_STOP=1', '--command', query], { stdio: 'ignore' });
}

async function withWaitingTicket(browser: Browser, run: (f: {
  customer: Page; host: Page; id: string; queue: string; csrf: string; hostCsrf: string;
}) => Promise<void>) {
  const queue = randomUUID();
  const accounts = ['CUSTOMER', 'HOST'].map(role => ({ role, id: randomUUID(), token: randomBytes(32).toString('base64url') }));
  const contexts = await Promise.all(accounts.map(() => browser.newContext()));
  try {
    sql(`BEGIN;
      INSERT INTO service_queues(id,restaurant_id,name,ticket_prefix) VALUES
      ('${queue}','30000000-0000-4000-8000-000000000001','Readiness error fixture','E');
      ${accounts.map(a => `INSERT INTO users(id,display_name,email,password_hash,role) VALUES
      ('${a.id}','Readiness fixture','${a.id}@example.test','not-a-login-hash','${a.role}');
      INSERT INTO sessions(id,user_id,token_hash,created_at,expires_at) VALUES
      ('${randomUUID()}','${a.id}','${createHash('sha256').update(a.token).digest('hex')}',now(),now()+interval '1 hour');`).join('\n')}
      INSERT INTO operator_queue_permissions(user_id,queue_id) VALUES ('${accounts[1].id}','${queue}'); COMMIT;`);
    const clients = [];
    for (let i = 0; i < accounts.length; i++) {
      await contexts[i].addCookies([{ name: 'dineq_session', value: accounts[i].token,
        url: process.env.F01_BASE_URL!, httpOnly: true, sameSite: 'Lax' }]);
      const page = await contexts[i].newPage();
      const me = await page.request.get('/api/v1/auth/me');
      expect(me.status()).toBe(200);
      clients.push({ page, csrf: (await me.json()).csrfToken as string });
    }
    const [customer, host] = clients;
    const joined = await customer.page.request.post(`/api/v1/queues/${queue}/entries`, {
      headers: { Origin: process.env.F01_BASE_URL!, 'X-CSRF-Token': customer.csrf }, data: { partySize: 2 },
    });
    expect(joined.status()).toBe(201);
    const { id } = await joined.json();
    await customer.page.goto(`/my-queue/${id}`);
    await expect(customer.page.getByTestId('ticket-readiness')).toHaveText('Not ready');
    await run({ customer: customer.page, host: host.page, id, queue, csrf: customer.csrf, hostCsrf: host.csrf });
  } finally {
    await Promise.all(contexts.map(context => context.close()));
    sql(`BEGIN; DELETE FROM queue_entries WHERE queue_id='${queue}';
      DELETE FROM service_queues WHERE id='${queue}';
      DELETE FROM users WHERE id IN ('${accounts[0].id}','${accounts[1].id}'); COMMIT;`);
  }
}

test('L7 readiness network failure preserves state and recovers after refresh', async ({ browser }) => {
  await withWaitingTicket(browser, async ({ customer, id }) => {
    const path = `**/api/v1/me/queue-entries/${id}/readiness`;
    await customer.route(path, route => route.abort('failed'));
    await customer.getByRole('button', { name: 'Mark ready', exact: true }).click();
    await expect(customer.getByRole('alert').filter({ hasText: 'Unable to connect.' })).toBeVisible();
    await expect(customer.getByTestId('ticket-readiness')).toHaveText('Not ready');
    await expect(customer.getByRole('button', { name: 'Mark ready', exact: true })).toBeEnabled();
    await customer.unroute(path);
    await customer.getByRole('button', { name: 'Refresh status', exact: true }).click();
    await expect(customer.getByTestId('ticket-readiness')).toHaveText('Not ready');
    await customer.getByRole('button', { name: 'Mark ready', exact: true }).click();
    await expect(customer.getByTestId('ticket-readiness')).toHaveText('Ready to be called');
  });
});

test('L7 stale readiness update gets real 409 and refreshes to CALLED', async ({ browser }) => {
  await withWaitingTicket(browser, async ({ customer, host, id, queue, csrf, hostCsrf }) => {
    // Change authoritative state through real APIs while the Customer view is stale.
    const ready = await customer.request.patch(`/api/v1/me/queue-entries/${id}/readiness`, {
      headers: { Origin: process.env.F01_BASE_URL!, 'X-CSRF-Token': csrf }, data: { isReady: true },
    });
    expect(ready.status()).toBe(200);
    const called = await host.request.post(`/api/v1/host/queues/${queue}/call-next`, {
      headers: { Origin: process.env.F01_BASE_URL!, 'X-CSRF-Token': hostCsrf },
    });
    expect(called.status()).toBe(200);
    const rejected = customer.waitForResponse(r => r.request().method() === 'PATCH' && r.url().endsWith(`/${id}/readiness`));
    await customer.getByRole('button', { name: 'Mark ready', exact: true }).click();
    expect((await rejected).status()).toBe(409);
    await expect(customer.getByTestId('ticket-status')).toHaveText('CALLED');
    await expect(customer.getByRole('button', { name: /Mark (not )?ready/ })).toHaveCount(0);
    await expect(customer.getByText('Your ticket has been called. Please go to the counter.', { exact: true })).toBeVisible();
  });
});

test('L7 stalled readiness request times out and permits recovery', async ({ browser }) => {
  await withWaitingTicket(browser, async ({ customer, id }) => {
    // Accelerate native timeout signals without replacing fetch or its abort behavior.
    await customer.evaluate(() => {
      const timeout = AbortSignal.timeout.bind(AbortSignal);
      let firstRequest = true;
      AbortSignal.timeout = milliseconds => {
        const delay = firstRequest ? 50 : milliseconds;
        firstRequest = false;
        return timeout(delay);
      };
    });
    const path = `**/api/v1/me/queue-entries/${id}/readiness`;
    let release!: () => void;
    const gate = new Promise<void>(resolve => { release = resolve; });
    await customer.route(path, async route => { await gate; await route.abort().catch(() => {}); });
    try {
      await customer.getByRole('button', { name: 'Mark ready', exact: true }).click();
      await expect(customer.getByRole('alert').filter({ hasText: 'Unable to connect.' })).toBeVisible();
      await expect(customer.getByTestId('ticket-readiness')).toHaveText('Not ready');
      await expect(customer.getByRole('button', { name: 'Mark ready', exact: true })).toBeEnabled();
    } finally { release(); await customer.unrouteAll({ behavior: 'wait' }); }
    await customer.getByRole('button', { name: 'Mark ready', exact: true }).click();
    await expect(customer.getByTestId('ticket-readiness')).toHaveText('Ready to be called');
  });
});
