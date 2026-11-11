import { test, expect, type Page } from '@playwright/test';

// F-04 browser checks on the isolated seeded environment. Runs after the F-02/F-03
// specs (one worker): it drains earlier active tickets through real Host
// transitions and leaves no CALLED ticket behind.
const base = () => process.env.F01_BASE_URL!;

// The API allows 10 login attempts per minute per IP (T-039) and the earlier specs
// use that budget. Wait for the window to reset; this file signs in only 4 times.
test.beforeAll(async () => {
  test.setTimeout(90_000);
  await new Promise(resolve => setTimeout(resolve, 61_000));
});

async function signIn(page: Page, email: string) {
  await page.goto('/login');
  await page.getByLabel('Email', { exact: true }).fill(email);
  await page.getByLabel('Password', { exact: true }).fill(process.env.F01_TEST_PASSWORD!);
  await page.getByRole('button', { name: 'Sign in', exact: true }).click();
}

test('T-029 Host calls next and completes; customer sees CALLED then COMPLETED', async ({ browser }) => {
  test.setTimeout(90_000); // two Argon2 sign-ins, queue drain and full Host journey
  const hostContext = await browser.newContext();
  const customerContext = await browser.newContext();
  try {
    const host = await hostContext.newPage();
    await signIn(host, 'host@example.test');
    await expect(host).toHaveURL(/\/host\/queue$/);
    const { csrfToken } = await (await host.request.get('/api/v1/auth/me')).json();
    const queueId: string = (await (await host.request.get('/api/v1/host/queues')).json()).queues[0].id;
    const headers = { Origin: base(), 'X-CSRF-Token': csrfToken };
    const api = `/api/v1/host/queues/${queueId}`;
    // Drain tickets left by earlier specs so FIFO reaches this test's ticket.
    for (let i = 0; i < 100; i++) {
      const current = (await (await host.request.get(`${api}/current`)).json()).entry;
      if (current) {
        expect((await host.request.post(`${api}/entries/${current.id}/complete`, { headers })).status()).toBe(200);
        continue;
      }
      const called = await host.request.post(`${api}/call-next`, { headers });
      if (called.status() === 409) { expect((await called.json()).error.code).toBe('QUEUE_EMPTY'); break; }
      expect(called.status()).toBe(200);
    }

    const customer = await customerContext.newPage();
    await signIn(customer, 'customer2@example.test');
    await expect(customer.getByRole('link', { name: 'Join Waitlist' })).toBeVisible();
    await customer.goto('/join');
    await customer.getByLabel('Party size', { exact: true }).fill('3');
    const created = customer.waitForResponse(r => r.url().includes('/entries') && r.request().method() === 'POST');
    await customer.getByRole('button', { name: 'Join waitlist', exact: true }).click();
    const response = await created;
    expect(response.status()).toBe(201);
    const ticket = await response.json();
    await expect(customer.getByTestId('ticket-status')).toHaveText('WAITING');

    await host.reload();
    await expect(host.getByTestId('waiting-row')).toHaveCount(1);
    await expect(host.getByTestId('waiting-row')).toContainText(ticket.ticketCode);
    await expect(host.getByText('No ticket is currently called.', { exact: true })).toBeVisible();
    await host.getByRole('button', { name: 'Call next', exact: true }).click();
    await expect(host.getByRole('status')).toHaveText(`Ticket ${ticket.ticketCode} called.`);
    await expect(host.getByTestId('current-ticket')).toHaveText(ticket.ticketCode);
    await expect(host.getByText('No parties are waiting.', { exact: true })).toBeVisible();
    await expect(host.getByRole('button', { name: 'Call next', exact: true })).toBeDisabled();
    // Board shows ticket code only, never customer identity.
    await expect(host.getByText('Customer Two')).toHaveCount(0);
    await expect(host.getByText('customer2@example.test')).toHaveCount(0);

    await customer.getByRole('button', { name: 'Refresh status', exact: true }).click();
    await expect(customer.getByTestId('ticket-status')).toHaveText('CALLED');

    // Confirmation is required, and Cancel leaves the ticket CALLED.
    await host.getByRole('button', { name: 'Complete', exact: true }).click();
    await host.getByRole('button', { name: 'Cancel', exact: true }).click();
    await expect(host.getByTestId('current-ticket')).toHaveText(ticket.ticketCode);
    await host.getByRole('button', { name: 'Complete', exact: true }).click();
    await expect(host.getByText(`Confirm the waitlist service for ticket ${ticket.ticketCode} is finished?`)).toBeVisible();
    await host.getByRole('button', { name: 'Confirm complete', exact: true }).click();
    await expect(host.getByRole('status')).toHaveText(`Ticket ${ticket.ticketCode} completed.`);
    await expect(host.getByText('No ticket is currently called.', { exact: true })).toBeVisible();

    await customer.goto(`/my-queue/${ticket.id}`);
    await expect(customer.getByTestId('ticket-status')).toHaveText('COMPLETED');
    await customer.goto('/my-queue');
    await expect(customer.getByRole('status')).toContainText('You do not have an active ticket.');

    // Empty queue: server answers QUEUE_EMPTY and the UI disables Call next.
    await host.reload();
    await expect(host.getByRole('button', { name: 'Call next', exact: true })).toBeDisabled();
    const empty = await host.request.post(`${api}/call-next`, { headers });
    expect(empty.status()).toBe(409);
    expect((await empty.json()).error.code).toBe('QUEUE_EMPTY');
  } finally {
    await hostContext.close();
    await customerContext.close();
  }
});

test('T-027 unassigned Host sees no queue and is denied by the API', async ({ page }) => {
  await signIn(page, 'other-host@example.test');
  await expect(page).toHaveURL(/\/host\/queue$/);
  await expect(page.getByText('No queue is assigned to your account. Ask a manager for access.')).toBeVisible();
  await expect(page.getByRole('button', { name: 'Call next' })).toHaveCount(0);
  const queueId: string = (await (await page.request.get('/api/v1/queues')).json()).queues[0].id;
  const { csrfToken } = await (await page.request.get('/api/v1/auth/me')).json();
  expect((await page.request.get(`/api/v1/host/queues/${queueId}/entries?status=WAITING`)).status()).toBe(403);
  const call = await page.request.post(`/api/v1/host/queues/${queueId}/call-next`, { headers: { Origin: base(), 'X-CSRF-Token': csrfToken } });
  expect(call.status()).toBe(403);
  expect((await call.json()).error.code).toBe('FORBIDDEN');
});

test('T-006 Customer cannot open the Host board or call next', async ({ page }) => {
  await signIn(page, 'customer1@example.test');
  await expect(page.getByRole('link', { name: 'Join Waitlist' })).toBeVisible();
  await page.goto('/host/queue');
  await expect(page).not.toHaveURL(/\/host\/queue/);
  const queueId: string = (await (await page.request.get('/api/v1/queues')).json()).queues[0].id;
  const { csrfToken } = await (await page.request.get('/api/v1/auth/me')).json();
  const call = await page.request.post(`/api/v1/host/queues/${queueId}/call-next`, { headers: { Origin: base(), 'X-CSRF-Token': csrfToken } });
  expect(call.status()).toBe(403);
});
