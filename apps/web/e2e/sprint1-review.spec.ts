import { test, expect, type Browser, type BrowserContext, type Page, type Locator } from '@playwright/test';
import { createHash, randomBytes, randomUUID } from 'node:crypto';
import { execFileSync } from 'node:child_process';
import { mkdirSync } from 'node:fs';
import { join as pathJoin } from 'node:path';

// These fixtures belong only to the disposable integration DB, never a developer's demo DB.
function sql(query: string) {
  if (!process.env.TEST_DATABASE_URL) throw new Error('An isolated TEST_DATABASE_URL is required.');
  const db = new URL(process.env.TEST_DATABASE_URL);
  try {
    execFileSync('psql', ['--no-psqlrc', '--set', 'ON_ERROR_STOP=1', '--command', query], {
      env: { ...process.env, PGHOST: db.hostname, PGPORT: db.port || '5432',
        PGDATABASE: decodeURIComponent(db.pathname.slice(1)), PGUSER: decodeURIComponent(db.username),
        PGPASSWORD: decodeURIComponent(db.password), PGSSLMODE: db.searchParams.get('sslmode') ?? 'verify-full' },
      stdio: 'ignore', timeout: 10000,
    });
  } catch { throw new Error('Sprint 1 isolated database fixture failed.'); }
}

type Client = { page: Page; context: BrowserContext; csrf: string };
type Fixture = { queue: string; host: Client; otherHost: Client; customer: Client; secondCustomer: Client };

async function withFixture(browser: Browser, run: (fixture: Fixture) => Promise<void>) {
  const queue = randomUUID();
  const accounts = ['HOST', 'HOST', 'CUSTOMER', 'CUSTOMER'].map(role => ({ role, id: randomUUID(), token: randomBytes(32).toString('base64url') }));
  const ids = accounts.map(a => `'${a.id}'`).join(',');
  sql(`BEGIN;
    INSERT INTO service_queues(id,restaurant_id,name,ticket_prefix) VALUES
      ('${queue}','30000000-0000-4000-8000-000000000001','Sprint 1 review queue','R');
    ${accounts.map(a => `INSERT INTO users(id,display_name,email,password_hash,role) VALUES
      ('${a.id}','Review fixture','${a.id}@example.test','not-a-login-hash','${a.role}');
      INSERT INTO sessions(id,user_id,token_hash,created_at,expires_at) VALUES
      ('${randomUUID()}','${a.id}','${createHash('sha256').update(a.token).digest('hex')}',now(),now()+interval '1 hour');
      ${a.role === 'HOST' ? `INSERT INTO operator_queue_permissions(user_id,queue_id) VALUES ('${a.id}','${queue}');` : ''}`).join('\n')}
    COMMIT;`);
  const clients: Client[] = [];
  try {
    for (const a of accounts) {
      const context = await browser.newContext();
      const page = await context.newPage();
      const client = { page, context, csrf: '' };
      clients.push(client);
      await context.addCookies([{ name: 'dineq_session', value: a.token, url: process.env.F01_BASE_URL!, httpOnly: true, sameSite: 'Lax' }]);
      const identity = await page.request.get('/api/v1/auth/me');
      expect(identity.status()).toBe(200);
      client.csrf = (await identity.json()).csrfToken;
    }
    await run({ queue, host: clients[0], otherHost: clients[1], customer: clients[2], secondCustomer: clients[3] });
  } finally {
    for (const c of clients) await c.context.close();
    sql(`BEGIN; DELETE FROM queue_entries WHERE queue_id='${queue}'; DELETE FROM service_queues WHERE id='${queue}';
      DELETE FROM sessions WHERE user_id IN (${ids}); DELETE FROM users WHERE id IN (${ids}); COMMIT;`);
  }
}

async function join(client: Client, queue: string): Promise<{ id: string; ticketCode: string; joinedAt: string }> {
  const response = await client.page.request.post(`/api/v1/queues/${queue}/entries`, {
    headers: { Origin: process.env.F01_BASE_URL!, 'X-CSRF-Token': client.csrf }, data: { partySize: 2 },
  });
  expect(response.status()).toBe(201);
  return response.json();
}

async function transition(client: Client, queue: string, suffix: string) {
  const response = await client.page.request.post(`/api/v1/host/queues/${queue}${suffix}`, {
    headers: { Origin: process.env.F01_BASE_URL!, 'X-CSRF-Token': client.csrf },
  });
  expect(response.status()).toBe(200);
  return response.json();
}

test('Sprint 1 review: Complete confirmation cannot transfer to another Host ticket', async ({ browser }) => {
  await withFixture(browser, async ({ queue, host, otherHost, customer, secondCustomer }) => {
    const first = await join(customer, queue);
    const second = await join(secondCustomer, queue);
    await transition(otherHost, queue, '/call-next');
    await host.page.goto('/host/queue');
    await expect(host.page.getByTestId('current-ticket')).toHaveText(first.ticketCode);
    await expect(host.page.getByTestId('current-joined-at')).toHaveAttribute('datetime', first.joinedAt);
    await expect(host.page.getByTestId('current-joined-at')).toContainText('Asia/Bangkok');
    await expect(host.page.getByTestId('current-called-at')).toBeVisible();
    await host.page.getByRole('button', { name: 'Complete', exact: true }).click();
    await expect(host.page.getByRole('button', { name: 'Confirm complete', exact: true })).toBeVisible();
    await transition(otherHost, queue, `/entries/${first.id}/complete`);
    await transition(otherHost, queue, '/call-next');
    await host.page.getByRole('button', { name: 'Refresh queue', exact: true }).click();
    await expect(host.page.getByTestId('current-ticket')).toHaveText(second.ticketCode);
    await expect(host.page.getByRole('button', { name: 'Confirm complete', exact: true })).toHaveCount(0);
    await expect(host.page.getByRole('button', { name: 'Complete', exact: true })).toBeVisible();
    const current = await host.page.request.get(`/api/v1/host/queues/${queue}/current`);
    expect((await current.json()).entry.id).toBe(second.id);
    await host.page.getByRole('button', { name: 'Complete', exact: true }).click();
    await expect(host.page.getByText(`Confirm the waitlist service for ticket ${second.ticketCode} is finished?`)).toBeVisible();
    await host.page.getByRole('button', { name: 'Confirm complete', exact: true }).click();
    await expect(host.page.getByText('No ticket is currently called.', { exact: true })).toBeVisible();
  });
});

test('Sprint 1 review: Customer home follows WAITING CALLED and COMPLETED state', async ({ browser }) => {
  await withFixture(browser, async ({ queue, host, customer }) => {
    await customer.page.goto('/');
    await expect(customer.page.getByTestId('home-queue-context')).toContainText('Sprint 1 review queue');
    await expect(customer.page.getByTestId('home-queue-context')).toContainText('DineQueue Demo');
    await expect(customer.page.getByRole('link', { name: 'Join Waitlist', exact: true })).toBeVisible();
    const ticket = await join(customer, queue);
    for (const status of ['WAITING', 'CALLED']) {
      if (status === 'CALLED') await transition(host, queue, '/call-next');
      await customer.page.goto('/');
      await expect(customer.page.getByTestId('home-queue-context')).toHaveText('DineQueue Demo · Sprint 1 review queue');
      await expect(customer.page.getByRole('link', { name: 'View My Queue', exact: true })).toBeVisible();
      await expect(customer.page.locator('a[href="/join"]')).toHaveCount(0);
      await customer.page.getByRole('link', { name: 'View My Queue', exact: true }).click();
      await expect(customer.page.getByTestId('ticket-status')).toHaveText(status);
    }
    await transition(host, queue, `/entries/${ticket.id}/complete`);
    await customer.page.goto('/');
    await expect(customer.page.getByRole('link', { name: 'Join Waitlist', exact: true })).toBeVisible();
    await expect(customer.page.getByRole('link', { name: 'View My Queue', exact: true })).toHaveCount(0);
  });
});

test('Sprint 1 review: Host retries a real read failure without navigating away', async ({ browser }) => {
  await withFixture(browser, async ({ queue, host, customer }) => {
    const ticket = await join(customer, queue);
    let renamed = false;
    try {
      await host.page.goto('/host/queue');
      await expect(host.page.getByTestId('waiting-row')).toContainText(ticket.ticketCode);
      sql('ALTER TABLE queue_entries RENAME TO s1_review_unavailable;');
      renamed = true;
      await host.page.getByRole('button', { name: 'Refresh queue', exact: true }).click();
      await expect(host.page.getByRole('heading', { name: 'We cannot load your account right now' })).toBeVisible();
      sql('ALTER TABLE s1_review_unavailable RENAME TO queue_entries;');
      renamed = false;
      await host.page.getByRole('button', { name: 'Try again', exact: true }).click();
      await expect(host.page.getByTestId('waiting-row')).toContainText(ticket.ticketCode);
      await expect(host.page).toHaveURL(/\/host\/queue$/);
    } finally {
      if (renamed) sql('ALTER TABLE s1_review_unavailable RENAME TO queue_entries;');
    }
  });
});

async function tabTo(page: Page, target: Locator) {
  for (let i = 0; i < 30; i++) {
    if (await target.evaluate(element => element === document.activeElement)) return;
    await page.keyboard.press('Tab');
  }
  await expect(target).toBeFocused();
}

async function checkAccessibleLayout(page: Page, width: number, name: string) {
  await expect(page.getByRole('main')).toHaveCount(1);
  await expect(page.getByRole('heading', { level: 1 })).toHaveCount(1);
  const unlabelled = await page.locator('input:not([type="hidden"]), select, textarea').evaluateAll(elements =>
    elements.filter(element => {
      const field = element as HTMLInputElement;
      return !field.labels?.length && !field.getAttribute('aria-label') && !field.getAttribute('aria-labelledby');
    }).map(element => element.id));
  expect(unlabelled).toEqual([]);
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= document.documentElement.clientWidth)).toBe(true);
  // Optional local visual evidence contains fixture UI only, never passwords/cookies.
  const directory = process.env.S1_REVIEW_SCREENSHOT_DIR;
  if (directory) {
    mkdirSync(directory, { recursive: true });
    await page.screenshot({ path: pathJoin(directory, `s1-${width}-${name}.png`), fullPage: true });
  }
}

for (const width of [360, 1280]) {
  test(`Sprint 1 completion: labelled keyboard journey and layout at ${width}px`, async ({ browser }) => {
    test.setTimeout(60_000);
    const guest = await browser.newContext({ viewport: { width, height: 900 } });
    try {
      const page = await guest.newPage();
      await page.goto('/login');
      await checkAccessibleLayout(page, width, 'login');
      await tabTo(page, page.getByLabel('Email', { exact: true }));
      await page.keyboard.type('keyboard-fixture@example.test');
      await page.keyboard.press('Tab');
      await expect(page.getByLabel('Password', { exact: true })).toBeFocused();
      await page.keyboard.type('not-a-real-account-password');
      await page.keyboard.press('Tab');
      await expect(page.getByRole('button', { name: 'Sign in', exact: true })).toBeFocused();
    } finally { await guest.close(); }

    await withFixture(browser, async ({ host, customer }) => {
      for (const client of [host, customer]) await client.page.setViewportSize({ width, height: 900 });
      await customer.page.goto('/');
      await checkAccessibleLayout(customer.page, width, 'home');
      await tabTo(customer.page, customer.page.getByRole('link', { name: 'Join Waitlist', exact: true }));
      await customer.page.keyboard.press('Enter');
      await expect(customer.page.getByRole('heading', { name: 'Join the waitlist', exact: true })).toBeVisible();
      await checkAccessibleLayout(customer.page, width, 'join');
      const size = customer.page.getByLabel('Party size', { exact: true });
      await tabTo(customer.page, size);
      await customer.page.keyboard.press('ControlOrMeta+A');
      await customer.page.keyboard.type('0');
      await tabTo(customer.page, customer.page.getByRole('button', { name: 'Join waitlist', exact: true }));
      await customer.page.keyboard.press('Enter');
      await expect(customer.page.getByRole('alert').filter({ hasText: 'whole number from 1 to 16' })).toBeVisible();
      await tabTo(customer.page, size);
      await customer.page.keyboard.press('ControlOrMeta+A');
      await customer.page.keyboard.type('4');
      await tabTo(customer.page, customer.page.getByRole('button', { name: 'Join waitlist', exact: true }));
      await customer.page.keyboard.press('Enter');
      await expect(customer.page.getByTestId('ticket-status')).toHaveText('WAITING');
      await checkAccessibleLayout(customer.page, width, 'ticket-waiting');
      await tabTo(customer.page, customer.page.getByRole('link', { name: 'View ticket details', exact: true }));
      await customer.page.keyboard.press('Enter');
      await expect(customer.page.getByRole('heading', { name: 'Ticket details', exact: true })).toBeVisible();

      await host.page.goto('/host/queue');
      await expect(host.page.getByTestId('waiting-row')).toHaveCount(1);
      await checkAccessibleLayout(host.page, width, 'host-waiting');
      const headerText = await host.page.getByRole('columnheader').evaluateAll(cells => cells.map(cell => {
        const range = document.createRange(); range.selectNodeContents(cell);
        const rect = range.getBoundingClientRect(); return { left: rect.left, right: rect.right };
      }));
      for (let i = 1; i < headerText.length; i++) expect(headerText[i].left - headerText[i - 1].right).toBeGreaterThanOrEqual(12);
      await tabTo(host.page, host.page.getByRole('region', { name: 'Waiting tickets', exact: true }));
      if (width === 360) {
        await host.page.keyboard.press('ArrowRight');
        await expect.poll(() => host.page.getByRole('region', { name: 'Waiting tickets', exact: true }).evaluate(element => element.scrollLeft)).toBeGreaterThan(0);
      }
      await tabTo(host.page, host.page.getByRole('button', { name: 'Call next', exact: true }));
      await host.page.keyboard.press('Enter');
      await expect(host.page.getByRole('status')).toContainText('called.');
      await expect(host.page.getByTestId('current-joined-at')).toBeVisible();
      await checkAccessibleLayout(host.page, width, 'host-called');
      await tabTo(customer.page, customer.page.getByRole('button', { name: 'Refresh status', exact: true }));
      await customer.page.keyboard.press('Enter');
      await expect(customer.page.getByTestId('ticket-status')).toHaveText('CALLED');
      await expect(customer.page.getByText('Your ticket has been called. Please go to the counter.', { exact: true })).toBeVisible();

      await tabTo(host.page, host.page.getByRole('button', { name: 'Complete', exact: true }));
      await host.page.keyboard.press('Enter');
      await tabTo(host.page, host.page.getByRole('button', { name: 'Cancel', exact: true }));
      await host.page.keyboard.press('Enter');
      await expect(host.page.getByRole('button', { name: 'Confirm complete', exact: true })).toHaveCount(0);
      await tabTo(host.page, host.page.getByRole('button', { name: 'Complete', exact: true }));
      await host.page.keyboard.press('Enter');
      await tabTo(host.page, host.page.getByRole('button', { name: 'Confirm complete', exact: true }));
      await host.page.keyboard.press('Enter');
      await expect(host.page.getByRole('status')).toContainText('completed.');
      await tabTo(customer.page, customer.page.getByRole('button', { name: 'Refresh status', exact: true }));
      await customer.page.keyboard.press('Enter');
      await expect(customer.page.getByTestId('ticket-status')).toHaveText('COMPLETED');
      await expect(customer.page.getByText('Your waitlist service has finished.', { exact: true })).toBeVisible();
      await checkAccessibleLayout(customer.page, width, 'ticket-completed');
    });
  });
}
