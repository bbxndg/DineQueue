import { test, expect } from '@playwright/test';
import { withQueue, sql, headers, join, setReady, addUser } from './polling-fixture';

// F-08 / issue #53 browser checks (S2-T-039, S2-T-056, S2-T-057 subset).
// Isolated TEST_DATABASE_URL fixtures only; generated users/queues are removed afterwards.

const cleanEvents = (queue: string) => async () => sql(`BEGIN; DELETE FROM operation_events WHERE queue_id='${queue}'; DELETE FROM operation_requests WHERE queue_id='${queue}'; UPDATE dining_tables SET status='AVAILABLE',version=version+1 WHERE id IN (SELECT table_id FROM seating_assignment_tables WHERE assignment_id IN (SELECT id FROM seating_assignments WHERE queue_id='${queue}')); DELETE FROM seating_assignment_tables WHERE assignment_id IN (SELECT id FROM seating_assignments WHERE queue_id='${queue}'); DELETE FROM seating_assignments WHERE queue_id='${queue}'; COMMIT;`);

test('F-08 customer cancels a waiting ticket with confirmation in a closed queue; Host board drains it', async ({ browser }) => {
  await withQueue(browser, async ({ host, customer, queue }, cleanup) => {
    cleanup.push(cleanEvents(queue));
    const ticket = await join(customer, queue);
    await setReady(customer, ticket.id, true);
    // A queue closed to new joins still lets existing WAITING tickets drain/cancel.
    sql(`UPDATE service_queues SET is_active=false WHERE id='${queue}';`);
    await host.page.goto(`/host/queue?queue=${queue}`);
    await expect(host.page.getByTestId('waiting-row')).toHaveCount(1);

    const page = customer.page;
    await page.goto('/my-queue');
    await expect(page.getByTestId('ticket-code')).toHaveText(ticket.ticketCode);
    await page.getByRole('button', { name: 'Cancel ticket', exact: true }).click();
    const confirm = page.getByRole('group', { name: 'Confirm ticket cancellation' });
    await expect(confirm).toContainText(ticket.ticketCode);
    await confirm.getByRole('button', { name: 'Keep my ticket', exact: true }).click();
    await expect(page.getByTestId('ticket-status')).toHaveText('WAITING');
    expect((await page.request.get(`/api/v1/me/queue-entries/${ticket.id}`)).status()).toBe(200);

    // A foreign customer cannot cancel it: generic 404, no state change.
    const other = await addUser(browser, 'CUSTOMER', cleanup);
    expect((await other.page.request.post(`/api/v1/me/queue-entries/${ticket.id}/cancel`, { headers: headers(other) })).status()).toBe(404);
    expect((await other.page.request.get(`/api/v1/me/queue-entries/${ticket.id}`)).status()).toBe(404);

    await page.getByRole('button', { name: 'Cancel ticket', exact: true }).click();
    await page.getByRole('button', { name: 'Confirm cancel', exact: true }).click();
    await expect(page.getByRole('status').filter({ hasText: 'You do not have an active ticket.' })).toBeVisible();
    // Host board picks up the committed cancellation through polling, without a reload.
    await expect(host.page.getByTestId('waiting-row')).toHaveCount(0, { timeout: 15_000 });

    await page.goto(`/my-queue/${ticket.id}`);
    await expect(page.getByTestId('ticket-status')).toHaveText('CANCELLED');
    await expect(page.getByTestId('ticket-code')).toHaveText(ticket.ticketCode);
    await expect(page.getByRole('button', { name: 'Cancel ticket' })).toHaveCount(0);
    await expect(page.getByRole('link', { name: 'Join again', exact: true })).toBeVisible();
    // Repeat cancel is a 409, not a second transition.
    expect((await page.request.post(`/api/v1/me/queue-entries/${ticket.id}/cancel`, { headers: headers(customer) })).status()).toBe(409);

    sql(`UPDATE service_queues SET is_active=true WHERE id='${queue}';`);
    const again = await page.request.post(`/api/v1/queues/${queue}/entries`, { headers: headers(customer), data: { partySize: 2 } });
    expect(again.status()).toBe(201);
    const fresh = await again.json();
    expect(fresh.ticketCode).not.toBe(ticket.ticketCode);
    expect(fresh.isReady).toBe(false);
    expect((await (await page.request.get(`/api/v1/me/queue-entries/${ticket.id}`)).json()).status).toBe('CANCELLED');
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  });
});

test('F-08 Host marks a held ten-person group no-show; whole group released and owner view updates', async ({ browser }) => {
  await withQueue(browser, async ({ host, customer, queue }, cleanup) => {
    cleanup.push(cleanEvents(queue));
    sql(`UPDATE service_queues SET operations_mode='SEATING' WHERE id='${queue}';`);
    const response = await customer.page.request.post(`/api/v1/queues/${queue}/entries`, { headers: headers(customer), data: { partySize: 10 } });
    expect(response.status()).toBe(201);
    const ticket = await response.json();
    await setReady(customer, ticket.id, true);

    const page = host.page;
    await page.setViewportSize({ width: 360, height: 900 });
    await page.goto(`/host/floor?queueId=${queue}`);
    await page.getByRole('combobox', { name: 'Zone', exact: true }).selectOption({ label: 'Terrace' });
    await page.getByRole('button', { name: 'T4, 6 seats, available', exact: true }).click();
    await page.getByRole('button', { name: 'T5, 4 seats, available', exact: true }).click();
    await page.getByRole('button', { name: 'Call & Hold', exact: true }).click();
    await page.getByRole('button', { name: 'Confirm Call & Hold', exact: true }).click();
    const assignments = page.getByRole('region', { name: 'Current seating assignments' });
    await expect(assignments).toContainText('T4 + T5 · Held');

    // Owner detail is open and visible while the Host acts.
    await customer.page.goto(`/my-queue/${ticket.id}`);
    await expect(customer.page.getByTestId('ticket-status')).toHaveText('CALLED');
    await expect(customer.page.getByRole('button', { name: 'Cancel ticket' })).toHaveCount(0);

    await assignments.getByRole('button', { name: 'No-show', exact: true }).click();
    const confirm = page.getByRole('group', { name: 'Confirm no-show' });
    await expect(confirm).toContainText(ticket.ticketCode);
    await expect(confirm).toContainText('T4 + T5');
    await confirm.getByRole('button', { name: 'Keep hold', exact: true }).click();
    await expect(assignments).toContainText('T4 + T5 · Held');
    await assignments.getByRole('button', { name: 'No-show', exact: true }).click();
    await page.getByRole('button', { name: 'Confirm No-show', exact: true }).click();
    await expect(assignments).toContainText('No current seating assignments.');
    await expect(page.getByRole('button', { name: 'T4, 6 seats, available', exact: true })).toBeVisible();
    await expect(page.getByRole('button', { name: 'T5, 4 seats, available', exact: true })).toBeVisible();
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);

    // Committed terminal state reaches the visible owner view through polling.
    await expect(customer.page.getByTestId('ticket-status')).toHaveText('NO_SHOW', { timeout: 15_000 });
    await expect(customer.page.getByRole('link', { name: 'Join again', exact: true })).toBeVisible();
    await customer.page.goto('/my-queue');
    await expect(customer.page.getByRole('status').filter({ hasText: 'You do not have an active ticket.' })).toBeVisible();
    // A customer session cannot use the Host action.
    expect((await customer.page.request.post(`/api/v1/host/queues/${queue}/entries/${ticket.id}/no-show`, { headers: headers(customer), data: { expectedVersion: 1, idempotencyKey: crypto.randomUUID(), reasonCode: 'CUSTOMER_ABSENT' } })).status()).toBe(403);
  });
});
