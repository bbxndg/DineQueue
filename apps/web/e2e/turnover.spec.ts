import { test, expect, type Page } from '@playwright/test';
import { withQueue, sql, headers } from './polling-fixture';

// F-09/F-16 (#54) S2-T-025: complete ten-person journey through checkout and
// cleaning, reloading at each boundary. Isolated TEST_DATABASE_URL fixtures only.

const cleanSeating = (queue: string) => async () => sql(`BEGIN; DELETE FROM operation_events WHERE queue_id='${queue}'; DELETE FROM operation_requests WHERE queue_id='${queue}'; UPDATE dining_tables SET status='AVAILABLE',version=version+1 WHERE id IN (SELECT table_id FROM seating_assignment_tables WHERE assignment_id IN (SELECT id FROM seating_assignments WHERE queue_id='${queue}')); DELETE FROM seating_assignment_tables WHERE assignment_id IN (SELECT id FROM seating_assignments WHERE queue_id='${queue}'); DELETE FROM seating_assignments WHERE queue_id='${queue}'; COMMIT;`);

async function step(page: Page, button: string, confirm: string, expected: string) {
  const assignments = page.getByRole('region', { name: 'Current seating assignments' });
  await assignments.getByRole('button', { name: button, exact: true }).click();
  const group = page.getByRole('group', { name: confirm });
  await expect(group).toContainText('T4 + T5');
  // Backing out leaves the state unchanged.
  await group.getByRole('button', { name: 'Keep current state', exact: true }).click();
  await assignments.getByRole('button', { name: button, exact: true }).click();
  await page.getByRole('button', { name: confirm, exact: true }).click();
  await expect(assignments).toContainText(expected);
  await page.reload();
  await expect(assignments).toContainText(expected);
}

test('S2-T-025 ten-person journey: preview, hold, seat, checkout, start and finish cleaning', async ({ browser }) => {
  await withQueue(browser, async ({ host, customer, queue }, cleanup) => {
    cleanup.push(cleanSeating(queue));
    sql(`UPDATE service_queues SET operations_mode='SEATING' WHERE id='${queue}';`);
    const joined = await customer.page.request.post(`/api/v1/queues/${queue}/entries`, { headers: headers(customer), data: { partySize: 10 } });
    expect(joined.status()).toBe(201);
    const ticket = await joined.json();
    expect(ticket.isReady).toBe(false);
    expect((await customer.page.request.patch(`/api/v1/me/queue-entries/${ticket.id}/readiness`, { headers: headers(customer), data: { isReady: true } })).status()).toBe(200);

    const page = host.page;
    await page.setViewportSize({ width: 360, height: 900 });
    await page.goto(`/host/floor?queueId=${queue}`);
    await page.getByRole('combobox', { name: 'Zone', exact: true }).selectOption({ label: 'Terrace' });
    await page.getByRole('button', { name: 'T4, 6 seats, available', exact: true }).click();
    await page.getByRole('button', { name: 'T5, 4 seats, available', exact: true }).click();
    await page.getByRole('button', { name: 'Call & Hold', exact: true }).click();
    await expect(page.getByRole('group', { name: 'Confirm call and hold' })).toContainText('T4 (6 seats) + T5 (4 seats)');
    await page.getByRole('button', { name: 'Confirm Call & Hold', exact: true }).click();
    const assignments = page.getByRole('region', { name: 'Current seating assignments' });
    await expect(assignments).toContainText('T4 + T5 · Held');
    await expect(assignments).toContainText(`Ticket ${ticket.ticketCode}`);
    await page.getByRole('button', { name: 'Seat', exact: true }).click();
    await page.getByRole('button', { name: 'Confirm Seat', exact: true }).click();
    await expect(assignments).toContainText('T4 + T5 · Occupied');

    await step(page, 'Checkout', 'Confirm Checkout', 'T4 + T5 · Checked out · Dirty');
    await page.getByRole('combobox', { name: 'Zone', exact: true }).selectOption({ label: 'Terrace' });
    // Dirty tables are not selectable for a new party.
    await expect(page.getByRole('button', { name: 'T4, 6 seats, dirty', exact: true })).toBeDisabled();
    await expect(page.getByRole('button', { name: 'T5, 4 seats, dirty', exact: true })).toBeDisabled();
    await expect(assignments.getByRole('button', { name: 'Finish Cleaning' })).toHaveCount(0);

    await step(page, 'Start Cleaning', 'Confirm Start Cleaning', 'T4 + T5 · Cleaning');
    await page.getByRole('combobox', { name: 'Zone', exact: true }).selectOption({ label: 'Terrace' });
    await expect(page.getByRole('button', { name: 'T4, 6 seats, cleaning', exact: true })).toBeDisabled();

    const assignmentsRegion = page.getByRole('region', { name: 'Current seating assignments' });
    await assignmentsRegion.getByRole('button', { name: 'Finish Cleaning', exact: true }).click();
    await page.getByRole('button', { name: 'Confirm Finish Cleaning', exact: true }).click();
    await expect(assignmentsRegion).toContainText('No current seating assignments.');
    await page.reload();
    await page.getByRole('combobox', { name: 'Zone', exact: true }).selectOption({ label: 'Terrace' });
    await expect(page.getByRole('button', { name: 'T4, 6 seats, available', exact: true })).toBeEnabled();
    await expect(page.getByRole('button', { name: 'T5, 4 seats, available', exact: true })).toBeEnabled();
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);

    // The customer's waitlist journey ended at seating; no dining/cleaning state leaks to them.
    await customer.page.goto(`/my-queue/${ticket.id}`);
    await expect(customer.page.getByTestId('ticket-status')).toHaveText('COMPLETED');
    await expect(customer.page.getByText(/dirty|cleaning|T4|T5/i)).toHaveCount(0);
    // Customer cannot use Host turnover routes.
    const denied = await customer.page.request.post(`/api/v1/host/queues/${queue}/seating-assignments/${crypto.randomUUID()}/checkout`, { headers: headers(customer), data: { expectedVersion: 1, idempotencyKey: crypto.randomUUID() } });
    expect(denied.status()).toBe(403);
  });
});
