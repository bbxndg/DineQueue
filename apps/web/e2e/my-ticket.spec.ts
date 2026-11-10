import { test, expect, type Page } from '@playwright/test';

async function signIn(page: Page, email: string) {
  await page.goto('/login');
  await page.getByLabel('Email', { exact: true }).fill(email);
  await page.getByLabel('Password', { exact: true }).fill(process.env.F01_TEST_PASSWORD!);
  await page.getByRole('button', { name: 'Sign in', exact: true }).click();
  await expect(page).not.toHaveURL(/\/login$/);
}

test('T-010/018/019/020 My Ticket empty, join, persisted reload and cross-user privacy', async ({ page, browser }) => {
  // Fresh customer1; the F-02 browser test uses customer2.
  await signIn(page, 'customer1@example.test');
  await page.getByRole('link', { name: 'My Ticket', exact: true }).click();
  await expect(page).toHaveURL(/\/my-queue$/);
  await expect(page.getByRole('status')).toContainText('You do not have an active ticket.');
  await expect(page.getByTestId('ticket-code')).toHaveCount(0);
  await page.getByRole('link', { name: 'Join Waitlist', exact: true }).click();
  await page.getByLabel('Party size', { exact: true }).fill('4');
  const created = page.waitForResponse(response => response.url().endsWith('/entries') && response.request().method() === 'POST');
  await page.getByRole('button', { name: 'Join waitlist', exact: true }).click();
  const response = await created;
  expect(response.status()).toBe(201);
  const ticket = await response.json();
  await expect(page).toHaveURL(/\/my-queue$/);
  await expect(page.getByTestId('ticket-code')).toHaveText(ticket.ticketCode);
  await expect(page.getByTestId('ticket-status')).toHaveText('WAITING');
  await expect(page.getByTestId('party-size')).toHaveText('4');
  const time = page.locator('time');
  await expect(time).toHaveAttribute('datetime', ticket.joinedAt);
  await expect(time).toContainText('Asia/Bangkok');
  const active = await page.request.get('/api/v1/me/queue-entries/active');
  expect(active.status()).toBe(200);
  expect(active.headers()['cache-control']).toBe('no-store');
  const entries = (await active.json()).entries;
  expect(entries).toHaveLength(1);
  expect(entries[0]).toMatchObject(ticket);
  expect(entries[0].serviceDate).toMatch(/^\d{4}-\d{2}-\d{2}$/);
  await page.reload();
  await expect(page.getByTestId('ticket-code')).toHaveText(ticket.ticketCode);
  await page.getByRole('button', { name: 'Refresh status' }).click();
  await expect(page.getByTestId('ticket-code')).toHaveText(ticket.ticketCode);
  await page.getByRole('link', { name: 'View ticket details' }).click();
  await expect(page).toHaveURL(new RegExp(`/my-queue/${ticket.id}$`));
  await expect(page.getByTestId('ticket-code')).toHaveText(ticket.ticketCode);
  expect((await page.request.get(`/api/v1/me/queue-entries/${ticket.id}`)).status()).toBe(200);
  expect(await page.evaluate(() => Object.keys(localStorage))).toEqual([]);

  const other = await browser.newContext();
  try {
    const otherPage = await other.newPage();
    await signIn(otherPage, 'customer2@example.test');
    const denied = await otherPage.request.get(`/api/v1/me/queue-entries/${ticket.id}`);
    expect(denied.status()).toBe(404);
    expect(await denied.json()).toMatchObject({ error: { code: 'NOT_FOUND', message: 'Resource not found.' } });
    await otherPage.goto(`/my-queue/${ticket.id}`);
    await expect(otherPage.getByRole('heading', { name: 'Ticket not found' })).toBeVisible();
    await expect(otherPage.getByTestId('ticket-code')).toHaveCount(0);
    await expect(otherPage.getByText(ticket.ticketCode, { exact: true })).toHaveCount(0);
  } finally { await other.close(); }
  await page.getByRole('button', { name: 'Sign out', exact: true }).click();
  expect((await page.request.get(`/api/v1/me/queue-entries/${ticket.id}`)).status()).toBe(401);
  await page.goto(`/my-queue/${ticket.id}`);
  await expect(page).toHaveURL(/\/login$/);
});

test('T-021 My Ticket requires a session and redirects Hosts', async ({ page, context }) => {
  await page.goto('/my-queue');
  await expect(page).toHaveURL(/\/login$/);
  expect((await page.request.get('/api/v1/me/queue-entries/active')).status()).toBe(401);
  await signIn(page, 'host@example.test');
  for (const path of ['/my-queue', '/my-queue/00000000-0000-4000-8000-000000000001']) {
    await page.goto(path);
    await expect(page).toHaveURL(/\/host\/queue$/);
  }
  expect((await page.request.get('/api/v1/me/queue-entries/active')).status()).toBe(403);
  await context.clearCookies();
  await context.addCookies([{ name: 'dineq_session', value: 'expired-ticket-test-session', url: process.env.F01_BASE_URL!, httpOnly: true, sameSite: 'Lax' }]);
  await page.goto('/my-queue');
  await expect(page).toHaveURL(/\/login$/);
  await expect(page.getByRole('status')).toHaveText('Your session has ended. Please sign in again.');
});
