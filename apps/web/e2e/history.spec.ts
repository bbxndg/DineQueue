import { execFileSync } from 'node:child_process';
import { createHash, randomBytes, randomUUID } from 'node:crypto';
import { test, expect, type Browser, type BrowserContextOptions, type Page } from '@playwright/test';
import { sql, addQueue, join as joinQueue } from './polling-fixture';

// Sprint 3 Feat 2 (#56) Customer Ticket History browser checks: H08-H12.
// Isolated TEST_DATABASE_URL fixtures and synthetic accounts only.

type Customer = { page: Page; userId: string; token: string; csrf: string };
type Cleanup = (() => Promise<void>)[];

async function customer(browser: Browser, cleanup: Cleanup, options: BrowserContextOptions = {}): Promise<Customer> {
  const id = randomUUID();
  const token = randomBytes(32).toString('base64url');
  sql(`BEGIN; INSERT INTO users(id,display_name,email,password_hash,role) VALUES ('${id}','History fixture','${id}@example.test','not-a-login-hash','CUSTOMER');
    INSERT INTO sessions(id,user_id,token_hash,created_at,expires_at) VALUES ('${randomUUID()}','${id}','${createHash('sha256').update(token).digest('hex')}',now(),now()+interval '1 hour'); COMMIT;`);
  const context = await browser.newContext(options);
  cleanup.push(async () => { await context.close(); sql(`BEGIN; DELETE FROM queue_entries WHERE customer_id='${id}'; DELETE FROM sessions WHERE user_id='${id}'; DELETE FROM users WHERE id='${id}'; COMMIT;`); });
  await context.addCookies([{ name: 'dineq_session', value: token, url: process.env.F01_BASE_URL!, httpOnly: true, sameSite: 'Lax' }]);
  const page = await context.newPage();
  const me = await page.request.get('/api/v1/auth/me');
  expect(me.status()).toBe(200);
  return { page, userId: id, token, csrf: (await me.json()).csrfToken };
}

let sequence = 500;
/** A lifecycle-valid terminal row (legacy MANUAL style: no assignment, no event). */
function terminal(queue: string, userId: string, status: 'COMPLETED' | 'CANCELLED' | 'NO_SHOW', joinedAt: string, serviceDate?: string) {
  sequence++;
  const date = serviceDate ?? joinedAt.slice(0, 10);
  const host = '20000000-0000-4000-8000-000000000003';
  const cols = { COMPLETED: `called_at,called_by,completed_at,completed_by`, CANCELLED: `cancelled_at,cancelled_by`, NO_SHOW: `called_at,called_by,no_show_at,no_show_reason` }[status];
  const vals = { COMPLETED: `ts+interval '1 min','${host}',ts+interval '2 min','${host}'`, CANCELLED: `ts+interval '1 min','${userId}'`, NO_SHOW: `ts+interval '1 min','${host}',ts+interval '6 min','HOLD_EXPIRED'` }[status];
  sql(`INSERT INTO queue_entries(id,queue_id,customer_id,service_date,sequence_number,party_size,status,joined_at,is_ready,${cols})
    SELECT gen_random_uuid(),'${queue}','${userId}','${date}',${sequence},4,'${status}',ts,true,${vals} FROM (SELECT timestamptz '${joinedAt}' AS ts) t;`);
}

function hostQueue(cleanup: Cleanup) {
  return addQueue('History fixture', '20000000-0000-4000-8000-000000000003', cleanup);
}

const items = (page: Page) => page.getByTestId('history-item');

test('H08 navigation, Load more without duplicates, owned legacy detail and back to My Ticket', async ({ browser }) => {
  const cleanup: Cleanup = [];
  try {
    const queue = hostQueue(cleanup);
    const c = await customer(browser, cleanup);
    const statuses = ['COMPLETED', 'CANCELLED', 'NO_SHOW'] as const;
    for (let i = 0; i < 23; i++) terminal(queue, c.userId, statuses[i % 3], new Date(Date.UTC(2026, 10, 1, 8, 0, 0) - i * 3_600_000).toISOString());
    const active = await joinQueue({ page: c.page, csrf: c.csrf, userId: c.userId }, queue);
    const expected: string[] = JSON.parse(execFileSync('psql', ['--no-psqlrc', '--dbname', process.env.TEST_DATABASE_URL!, '-tA', '-c',
      `SELECT json_agg(code) FROM (SELECT q.ticket_prefix||'-'||lpad(e.sequence_number::text,3,'0') AS code FROM queue_entries e JOIN service_queues q ON q.id=e.queue_id WHERE e.customer_id='${c.userId}' AND e.status<>'WAITING' ORDER BY e.joined_at DESC, e.id DESC) s`]).toString().trim());

    const page = c.page;
    await page.setViewportSize({ width: 360, height: 900 });
    await page.goto('/');
    await page.getByRole('navigation', { name: 'Customer account' }).getByRole('link', { name: 'Ticket history', exact: true }).click();
    await expect(page).toHaveURL(/\/my-queue\/history$/);
    await expect(page.getByRole('heading', { name: 'Ticket history' })).toBeVisible();
    await expect(items(page)).toHaveCount(20);
    // Review #68: the initial load is not announced; Load more is, politely.
    const announce = page.getByTestId('history-announcement');
    await expect(announce).toHaveAttribute('role', 'status');
    await expect(announce).toHaveText('');
    await page.getByRole('button', { name: 'Load more', exact: true }).click();
    await expect(items(page)).toHaveCount(23);
    await expect(announce).toHaveText('Loaded 3 more tickets. You have reached the end of your history.');
    await expect(page.getByText('End of your ticket history.')).toBeVisible();
    await expect(page.getByRole('button', { name: 'Load more' })).toHaveCount(0);
    expect(await page.getByTestId('history-code').allTextContents()).toEqual(expected);
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);

    // Keyboard reaches the item link; the owned read-only detail opens.
    const last = expected[22];
    const link = page.getByRole('link', { name: `View ticket ${last}`, exact: true });
    await link.focus();
    await page.keyboard.press('Enter');
    await expect(page.getByTestId('ticket-code')).toHaveText(last);
    await expect(page.getByRole('button', { name: /Cancel ticket|Mark ready|Mark not ready/ })).toHaveCount(0);
    await page.goBack();
    await page.getByRole('link', { name: 'Back to My Ticket', exact: true }).click();
    await expect(page).toHaveURL(/\/my-queue$/);
    await expect(page.getByTestId('ticket-code')).toHaveText(active.ticketCode);
    await expect(page.getByTestId('ticket-status')).toHaveText('WAITING');
  } finally { for (const step of cleanup.reverse()) await step().catch(() => undefined); }
});

test('H09 empty history, initial failure with Retry, later-page failure keeps rows and retries once', async ({ browser }) => {
  const cleanup: Cleanup = [];
  try {
    const queue = hostQueue(cleanup);
    const empty = await customer(browser, cleanup);
    await empty.page.goto('/my-queue/history');
    await expect(empty.page.getByRole('status').filter({ hasText: 'You have no past tickets yet.' })).toBeVisible();
    // Empty is distinct from failure (filter by text: Next's route announcer also has role=alert).
    await expect(empty.page.getByRole('alert').filter({ hasText: 'could not be loaded' })).toHaveCount(0);

    const c = await customer(browser, cleanup);
    for (let i = 0; i < 21; i++) terminal(queue, c.userId, 'COMPLETED', new Date(Date.UTC(2026, 9, 1) - i * 60_000).toISOString());
    const page = c.page;
    let firstFails = true;
    await page.route('**/api/v1/me/queue-entries/history', async route => {
      if (firstFails) { firstFails = false; await route.fulfill({ status: 503, json: { error: { code: 'UNAVAILABLE' } } }); } else await route.continue();
    });
    await page.goto('/my-queue/history');
    await expect(page.getByRole('alert').filter({ hasText: 'could not be loaded' })).toHaveText('Your ticket history could not be loaded.');
    await expect(page.getByText('You have no past tickets yet.')).toHaveCount(0);
    await page.getByRole('button', { name: 'Retry', exact: true }).click();
    await expect(items(page)).toHaveCount(20);

    const cursorRequests: string[] = [];
    let laterFails = true;
    await page.route('**/api/v1/me/queue-entries/history?cursor=*', async route => {
      cursorRequests.push(route.request().url());
      if (laterFails) { laterFails = false; await route.fulfill({ status: 500, json: { error: { code: 'INTERNAL' } } }); } else await route.continue();
    });
    await page.getByRole('button', { name: 'Load more', exact: true }).dblclick();
    await expect(page.getByRole('alert').filter({ hasText: 'could not be loaded' })).toHaveText('More tickets could not be loaded. The tickets above are still shown.');
    await expect(items(page)).toHaveCount(20);
    await page.getByRole('button', { name: 'Retry', exact: true }).click();
    await expect(items(page)).toHaveCount(21);
    expect(cursorRequests).toHaveLength(2);
    expect(cursorRequests[1]).toBe(cursorRequests[0]);
  } finally { for (const step of cleanup.reverse()) await step().catch(() => undefined); }
});

test('H10 stored service date unchanged and joined time shown in the restaurant timezone', async ({ browser }) => {
  const cleanup: Cleanup = [];
  try {
    const queue = hostQueue(cleanup);
    // Browser in Los Angeles; restaurant in Asia/Bangkok. 17:30Z on 20 Nov is 00:30 on 21 Nov in Bangkok.
    const c = await customer(browser, cleanup, { timezoneId: 'America/Los_Angeles' });
    terminal(queue, c.userId, 'COMPLETED', '2026-11-20T17:30:00.123456Z', '2026-11-21');
    await c.page.goto('/my-queue/history');
    await expect(items(c.page)).toHaveCount(1);
    await expect(c.page.getByTestId('history-service-date')).toHaveText('2026-11-21');
    const expected = new Intl.DateTimeFormat('en-GB', { dateStyle: 'medium', timeStyle: 'short', timeZone: 'Asia/Bangkok' }).format(new Date('2026-11-20T17:30:00.123Z'));
    await expect(c.page.getByTestId('history-joined')).toHaveText(`${expected} (Asia/Bangkok)`);
    await expect(c.page.getByTestId('history-joined')).toContainText('21 Nov 2026');
  } finally { for (const step of cleanup.reverse()) await step().catch(() => undefined); }
});

test('H11 expired session clears to login; a delayed previous-user response is ignored; no-store', async ({ browser }) => {
  const cleanup: Cleanup = [];
  try {
    const queue = hostQueue(cleanup);
    const a = await customer(browser, cleanup);
    const b = await customer(browser, cleanup);
    terminal(queue, a.userId, 'CANCELLED', '2026-11-10T10:00:00Z');
    terminal(queue, b.userId, 'NO_SHOW', '2026-11-11T10:00:00Z');
    const codeOf = (user: string) => execFileSync('psql', ['--no-psqlrc', '--dbname', process.env.TEST_DATABASE_URL!, '-tA', '-c',
      `SELECT q.ticket_prefix||'-'||lpad(e.sequence_number::text,3,'0') FROM queue_entries e JOIN service_queues q ON q.id=e.queue_id WHERE e.customer_id='${user}'`]).toString().trim();
    const aCode = codeOf(a.userId), bCode = codeOf(b.userId);

    const direct = await a.page.request.get('/api/v1/me/queue-entries/history');
    expect(direct.status()).toBe(200);
    expect(direct.headers()['cache-control']).toContain('no-store');

    // A's request is held; the tab then switches to B's session and reloads history.
    const page = a.page;
    let release!: () => void;
    const held = new Promise<void>(resolve => { release = resolve; });
    let delayedOnce = true;
    await page.route('**/api/v1/me/queue-entries/history', async route => {
      if (delayedOnce) {
        delayedOnce = false;
        const response = await route.fetch();
        await held;
        await route.fulfill({ response }).catch(() => undefined);
      } else await route.continue();
    });
    await page.goto('/my-queue/history');
    await expect(page.getByText('Loading your ticket history…')).toBeVisible();
    await page.context().addCookies([{ name: 'dineq_session', value: b.token, url: process.env.F01_BASE_URL!, httpOnly: true, sameSite: 'Lax' }]);
    await page.goto('/my-queue/history');
    await expect(page.getByTestId('history-code')).toHaveText([bCode]);
    release();
    await page.waitForTimeout(500);
    await expect(page.getByText(aCode)).toHaveCount(0);
    await expect(page.getByTestId('history-code')).toHaveText([bCode]);

    // Session expires while viewing: the next request is a real 401, data is cleared, route to login.
    sql(`UPDATE sessions SET created_at=now()-interval '2 hours', expires_at=now()-interval '1 minute' WHERE user_id='${b.userId}';`);
    await page.reload();
    await expect(page).toHaveURL(/\/login/);
    await expect(page.getByText(bCode)).toHaveCount(0);
  } finally { for (const step of cleanup.reverse()) await step().catch(() => undefined); }
});

test('H12 same database after an API restart keeps history and owned detail', async ({ browser }) => {
  const script = process.env.S3_RESTART_API_SCRIPT;
  test.skip(!script, 'S3_RESTART_API_SCRIPT not set: restart cannot be exercised in this environment');
  const cleanup: Cleanup = [];
  try {
    const queue = hostQueue(cleanup);
    const c = await customer(browser, cleanup);
    terminal(queue, c.userId, 'COMPLETED', '2026-11-12T09:00:00.654321Z');
    terminal(queue, c.userId, 'NO_SHOW', '2026-11-12T08:00:00Z');
    await c.page.goto('/my-queue/history');
    await expect(items(c.page)).toHaveCount(2);
    const before = await c.page.getByTestId('history-code').allTextContents();
    execFileSync('bash', [script!], { stdio: 'ignore' });
    await expect.poll(async () => { try { return (await c.page.request.get('/api/health/ready')).status(); } catch { return 0; } }, { timeout: 30_000 }).toBe(200);
    await c.page.reload();
    await expect(c.page.getByTestId('history-code')).toHaveText(before);
    await c.page.getByRole('link', { name: `View ticket ${before[0]}`, exact: true }).click();
    await expect(c.page.getByTestId('ticket-status')).toHaveText('COMPLETED');
  } finally { for (const step of cleanup.reverse()) await step().catch(() => undefined); }
});
