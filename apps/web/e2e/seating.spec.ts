import { execFileSync } from 'node:child_process';
import { test, expect, type Page } from '@playwright/test';
import { withQueue, sql, headers } from './polling-fixture';

// Explicit opt-in for a disposable API container; never infer a cloud target.
async function restartTestAPI(page: Page) {
 const container = process.env.S2_TEST_API_CONTAINER;
 if (!container) return;
 if (!/^dq-s2-[a-z0-9-]+$/.test(container)) throw new Error('Disposable S2 API container required');
 execFileSync('docker', ['restart', container], { stdio: 'ignore' });
 await expect.poll(async () => { try { return (await page.request.get('/api/health/ready')).status(); } catch { return 0; } }).toBe(200);
}

test('S2 seating: ten-person connected hold, confirmation, seat and owned persistence', async ({ browser }) => {
 await withQueue(browser, async ({ host, customer, queue }, cleanup) => {
  cleanup.push(async () => sql(`BEGIN; DELETE FROM operation_events WHERE queue_id='${queue}'; DELETE FROM operation_requests WHERE queue_id='${queue}'; UPDATE dining_tables SET status='AVAILABLE',version=version+1 WHERE id IN (SELECT table_id FROM seating_assignment_tables WHERE assignment_id IN (SELECT id FROM seating_assignments WHERE queue_id='${queue}')); DELETE FROM seating_assignment_tables WHERE assignment_id IN (SELECT id FROM seating_assignments WHERE queue_id='${queue}'); DELETE FROM seating_assignments WHERE queue_id='${queue}'; COMMIT;`));
  sql(`UPDATE service_queues SET operations_mode='SEATING' WHERE id='${queue}';`);
  const response = await customer.page.request.post(`/api/v1/queues/${queue}/entries`, { headers: headers(customer), data: { partySize: 10 } });
  expect(response.status()).toBe(201); const ticket = await response.json();
  expect((await customer.page.request.patch(`/api/v1/me/queue-entries/${ticket.id}/readiness`, { headers: headers(customer), data: { isReady: true } })).status()).toBe(200);
  const page = host.page;
  await page.setViewportSize({ width: 360, height: 900 });
  await page.goto(`/host/floor?queueId=${queue}`);
  await page.getByRole('combobox', { name: 'Zone', exact: true }).selectOption({ label: 'Terrace' });
  await page.getByRole('button', { name: 'T4, 6 seats, available', exact: true }).click();
  await page.getByRole('button', { name: 'T5, 4 seats, available', exact: true }).click();
  await page.getByRole('button', { name: 'Call & Hold', exact: true }).click();
  const confirm = page.getByRole('group', { name: 'Confirm call and hold' });
  await expect(confirm).toContainText(ticket.ticketCode);await expect(confirm).toContainText('10 people');await expect(confirm).toContainText('T4 (6 seats) + T5 (4 seats)');
  await confirm.getByRole('button', { name: 'Cancel', exact: true }).click();
  expect((await customer.page.request.get(`/api/v1/me/queue-entries/${ticket.id}`)).status()).toBe(200);
  await page.getByRole('button', { name: 'Call & Hold', exact: true }).click();
  await page.getByRole('button', { name: 'Confirm Call & Hold', exact: true }).click();
  const assignments = page.getByRole('region', { name: 'Current seating assignments' });
  await expect(assignments).toContainText('T4 + T5 · Held');
  await restartTestAPI(page);await page.reload();await expect(assignments).toContainText('T4 + T5 · Held');
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  await page.getByRole('button', { name: 'Seat', exact: true }).click();
  await page.getByRole('button', { name: 'Confirm Seat', exact: true }).click();
  await expect(assignments).toContainText('T4 + T5 · Occupied');
  await restartTestAPI(page);await page.reload();await expect(assignments).toContainText('T4 + T5 · Occupied');
  await customer.page.goto(`/my-queue/${ticket.id}`);await expect(customer.page.getByTestId('ticket-status')).toHaveText('COMPLETED');
  expect((await customer.page.request.post(`/api/v1/host/queues/${queue}/expire-holds`, { headers: headers(customer), data: { idempotencyKey: crypto.randomUUID() } })).status()).toBe(403);
 });
});
