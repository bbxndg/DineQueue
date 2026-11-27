import { test, expect } from '@playwright/test';
import { withQueue, join, headers } from './polling-fixture';

test('S2 matching: selected pair, ready FIFO, invalid connection and stale preview', async ({ browser }) => {
  await withQueue(browser, async ({ host, customer, queue }) => {
    const ticket = await join(customer, queue);
    expect((await customer.page.request.patch(`/api/v1/me/queue-entries/${ticket.id}/readiness`, { headers: headers(customer), data: { isReady: true } })).status()).toBe(200);
    const page = host.page;
    await page.goto(`/host/floor?queueId=${queue}`);
    await page.getByRole('combobox', { name: 'Zone', exact: true }).selectOption({ label: 'Terrace' });
    await page.getByRole('button', { name: 'T4, 6 seats, available', exact: true }).click();
    await page.getByRole('button', { name: 'T5, 4 seats, available', exact: true }).click();
    const preview = page.getByRole('complementary', { name: 'Matching preview' });
    await expect(preview).toContainText(ticket.ticketCode);
    await expect(preview).toContainText('3 people');
    await page.getByRole('button', { name: 'T5, 4 seats, available', exact: true }).click();
    await page.getByRole('button', { name: 'T6, 2 seats, available', exact: true }).click();
    await expect(preview.getByRole('alert')).toContainText('cannot be matched');
    await page.route(`**/api/v1/host/queues/${queue}/floor`, route => route.fulfill({ status: 503, json: {} }));
    await page.getByRole('button', { name: 'Refresh floor', exact: true }).click();
    await expect(preview).toContainText('Refresh the floor');
    await expect(preview).not.toContainText(ticket.ticketCode);
  });
});
