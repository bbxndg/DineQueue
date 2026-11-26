import { expect, type Browser, type Page } from '@playwright/test';
import { createHash, randomBytes, randomUUID } from 'node:crypto';
import { execFileSync } from 'node:child_process';

// Lab 7 polling (#44) lifecycle fixtures. Generated IDs only; isolated
// TEST_DATABASE_URL only, never a developer or cloud database.
export function sql(query: string) {
  if (!process.env.TEST_DATABASE_URL) throw new Error('Isolated TEST_DATABASE_URL required.');
  execFileSync('psql', ['--no-psqlrc', '--dbname', process.env.TEST_DATABASE_URL,
    '--set', 'ON_ERROR_STOP=1', '--command', query], { stdio: 'ignore' });
}

export type Client = { page: Page; csrf: string; userId: string };
export type Fixture = { customer: Client; host: Client; queue: string; browser: Browser };

export const headers = (c: Client) => ({ Origin: process.env.F01_BASE_URL!, 'X-CSRF-Token': c.csrf });

/** A generated user with a session in its own browser context. */
export async function addUser(browser: Browser, role: 'CUSTOMER' | 'HOST', cleanup: (() => Promise<void>)[]): Promise<Client> {
  const id = randomUUID();
  const token = randomBytes(32).toString('base64url');
  sql(`BEGIN; INSERT INTO users(id,display_name,email,password_hash,role) VALUES
    ('${id}','Polling fixture','${id}@example.test','not-a-login-hash','${role}');
    INSERT INTO sessions(id,user_id,token_hash,created_at,expires_at) VALUES
    ('${randomUUID()}','${id}','${createHash('sha256').update(token).digest('hex')}',now(),now()+interval '1 hour'); COMMIT;`);
  const context = await browser.newContext();
  cleanup.push(async () => {
    await context.close();
    sql(`BEGIN; DELETE FROM queue_entries WHERE customer_id='${id}'; DELETE FROM users WHERE id='${id}'; COMMIT;`);
  });
  await context.addCookies([{ name: 'dineq_session', value: token, url: process.env.F01_BASE_URL!, httpOnly: true, sameSite: 'Lax' }]);
  const page = await context.newPage();
  const me = await page.request.get('/api/v1/auth/me');
  expect(me.status()).toBe(200);
  return { page, csrf: (await me.json()).csrfToken as string, userId: id };
}

export function addQueue(name: string, hostId: string, cleanup: (() => Promise<void>)[]) {
  const queue = randomUUID();
  sql(`BEGIN; INSERT INTO service_queues(id,restaurant_id,name,ticket_prefix) VALUES
    ('${queue}','30000000-0000-4000-8000-000000000001','${name}','P');
    INSERT INTO operator_queue_permissions(user_id,queue_id) VALUES ('${hostId}','${queue}'); COMMIT;`);
  // Registered first so it runs last, after users' tickets are removed.
  cleanup.unshift(async () => sql(`BEGIN; DELETE FROM queue_entries WHERE queue_id='${queue}';
    DELETE FROM operator_queue_permissions WHERE queue_id='${queue}'; DELETE FROM service_queues WHERE id='${queue}'; COMMIT;`));
  return queue;
}

export async function withQueue(browser: Browser, run: (f: Fixture, cleanup: (() => Promise<void>)[]) => Promise<void>) {
  const cleanup: (() => Promise<void>)[] = [];
  try {
    const host = await addUser(browser, 'HOST', cleanup);
    const customer = await addUser(browser, 'CUSTOMER', cleanup);
    const queue = addQueue('Polling fixture', host.userId, cleanup);
    await run({ customer, host, queue, browser }, cleanup);
  } finally {
    for (const step of cleanup.reverse()) await step().catch(() => undefined);
  }
}

export async function join(customer: Client, queue: string) {
  const joined = await customer.page.request.post(`/api/v1/queues/${queue}/entries`, { headers: headers(customer), data: { partySize: 3 } });
  expect(joined.status()).toBe(201);
  return joined.json() as Promise<{ id: string; ticketCode: string }>;
}

export async function setReady(customer: Client, id: string, isReady: boolean) {
  const r = await customer.page.request.patch(`/api/v1/me/queue-entries/${id}/readiness`, { headers: headers(customer), data: { isReady } });
  expect(r.status()).toBe(200);
}

export async function hostPost(host: Client, queue: string, path: string) {
  const r = await host.page.request.post(`/api/v1/host/queues/${queue}${path}`, { headers: headers(host) });
  expect(r.status()).toBe(200);
}

/** Client-side fetch GETs whose URL contains the fragment (server renders are excluded). */
export function countReads(page: Page, fragment: string) {
  const seen: string[] = [];
  page.on('request', request => {
    if (request.resourceType() === 'fetch' && request.method() === 'GET' && request.url().includes(fragment)) seen.push(request.url());
  });
  return seen;
}

/** Headless pages are always visible; this lets a test hide/show the document. */
export async function controllableVisibility(page: Page) {
  await page.addInitScript(() => {
    let state: DocumentVisibilityState = 'visible';
    Object.defineProperty(Document.prototype, 'visibilityState', { configurable: true, get: () => state });
    Object.defineProperty(Document.prototype, 'hidden', { configurable: true, get: () => state === 'hidden' });
    (window as unknown as { setVisibility: (v: DocumentVisibilityState) => void }).setVisibility = v => {
      state = v; document.dispatchEvent(new Event('visibilitychange'));
    };
  });
}

export const setVisibility = (page: Page, v: 'visible' | 'hidden') =>
  page.evaluate(value => (window as unknown as { setVisibility: (v: string) => void }).setVisibility(value), v);

export const AUTO = { timeout: 15_000 };
export const POLL_WAIT = 6_500; // one 5 s interval plus scheduling slack
