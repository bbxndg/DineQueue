import { test, expect } from '@playwright/test';
import { withQueue, sql, controllableVisibility, countReads, setVisibility } from './polling-fixture';
import { randomUUID } from 'node:crypto';

// A dedicated seeded floor test database is required, as with the other E2E suites.
test('S2 floor: capacities, approved pair, keyboard/list selection and responsive layout', async ({ browser }) => {
  await withQueue(browser, async ({ host, queue }) => {
    const page = host.page;
    await page.setViewportSize({ width: 1280, height: 900 });
    await page.goto(`/host/floor?queueId=${queue}`);
    await expect(page.getByRole('heading', { name: 'Restaurant floor', exact: true })).toBeVisible();
    await page.getByRole('combobox', { name: 'Zone', exact: true }).selectOption({ label: 'Terrace' });
    await expect(page.getByText('T4 + T5; T5 + T6', { exact: true })).toBeVisible();
    await page.getByRole('button', { name: 'T4, 6 seats, available', exact: true }).click();
    await page.getByRole('button', { name: 'T5, 4 seats, available', exact: true }).click();
    await expect(page.getByLabel('Selected tables', { exact: true })).toContainText('T4 + T5 · 10 seats total');
    await page.getByRole('button', { name: 'list view', exact: true }).click();
    const t4 = page.getByRole('button', { name: 'Deselect T4', exact: true });
    await expect(t4).toHaveAttribute('aria-pressed', 'true');
    await t4.focus(); await page.keyboard.press('Space');
    await expect(page.getByRole('button', { name: 'Select T4', exact: true })).toBeFocused();
    await page.keyboard.press('Enter');
    await expect(page.getByLabel('Selected tables', { exact: true })).toContainText('10 seats total');
    await page.screenshot({ path: '/tmp/dq-s2-floor-evidence/floor-desktop.png', fullPage: true });
    await page.setViewportSize({ width: 360, height: 800 });
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
    await page.getByRole('button', { name: 'map view', exact: true }).click();
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
    await page.screenshot({ path: '/tmp/dq-s2-floor-evidence/floor-mobile.png', fullPage: true });
    await page.getByRole('combobox', { name: 'Zone', exact: true }).selectOption({ label: 'Private' });
    await expect(page.getByLabel('Selected tables', { exact: true })).toContainText('No tables selected');
    await expect(page.getByText('None. Tables in this zone cannot be combined.', { exact: true })).toBeVisible();
  });
});

test('S2 floor: failed and malformed reads retain snapshot, recover and hide protected data on denial', async ({ browser }) => {
  await withQueue(browser, async ({ host, queue }) => {
    const page = host.page;
    await page.goto(`/host/floor?queueId=${queue}`);
    await page.getByRole('combobox', { name: 'Zone', exact: true }).selectOption({ label: 'Terrace' });
    let mode = 'fail';
    await page.route(`**/api/v1/host/queues/${queue}/floor`, async route => {
      if (mode === 'fail') await route.fulfill({ status: 503, json: { error: 'test outage' } });
      else if (mode === 'malformed') await route.fulfill({ status: 200, json: { zones: [] } });
      else if (mode === 'denied') await route.fulfill({ status: 403, json: { error: { code: 'FORBIDDEN' } } });
      else await route.continue();
    });
    await page.getByRole('button', { name: 'Refresh floor', exact: true }).click();
    await expect(page.getByTestId('live-stale')).toBeVisible();
    await expect(page.getByRole('button', { name: 'T4, 6 seats, available', exact: true })).toBeVisible();
    mode = 'malformed';
    await page.getByRole('button', { name: 'Refresh floor', exact: true }).click();
    await expect(page.getByTestId('live-stale')).toBeVisible();
    await expect(page.getByRole('button', { name: 'T4, 6 seats, available', exact: true })).toBeVisible();
    mode = 'ok';
    await page.getByRole('button', { name: 'Refresh floor', exact: true }).click();
    await expect(page.getByTestId('live-stale')).toHaveCount(0);
    mode = 'denied';
    await page.getByRole('button', { name: 'Refresh floor', exact: true }).click();
    await expect(page.getByRole('alert').filter({ hasText: 'This floor' })).toHaveText('This floor is no longer available to your account.');
    await expect(page.getByRole('combobox', { name: 'Zone', exact: true })).toHaveCount(0);
  });
});

test('S2 floor: queue scope, customer denial and configured empty floor', async ({ browser }) => {
  await withQueue(browser, async ({ host, customer, queue }) => {
    expect((await customer.page.request.get(`/api/v1/host/queues/${queue}/floor`)).status()).toBe(403);
    await host.page.goto(`/host/floor?queueId=${randomUUID()}`);
    await expect(host.page.getByRole('alert').filter({ hasText: 'This queue' })).toHaveText('This queue is not available to your account.');
    const restaurant = randomUUID();
    sql(`INSERT INTO restaurants(id,name) VALUES ('${restaurant}','Empty floor fixture'); UPDATE service_queues SET restaurant_id='${restaurant}' WHERE id='${queue}';`);
    try {
      await host.page.goto(`/host/floor?queueId=${queue}`);
      await expect(host.page.getByRole('status').filter({ hasText: 'No floor is configured' })).toBeVisible();
    } finally {
      sql(`UPDATE service_queues SET restaurant_id='30000000-0000-4000-8000-000000000001' WHERE id='${queue}'; DELETE FROM restaurants WHERE id='${restaurant}';`);
    }
  });
});

test('S2 floor: selection invalidation and shared visible-only coordinator', async ({ browser }) => {
  test.setTimeout(45000);
  await withQueue(browser, async ({ host, queue }) => {
    const page = host.page;
    await controllableVisibility(page);
    const reads = countReads(page, `/api/v1/host/queues/${queue}/floor`);
    await page.goto(`/host/floor?queueId=${queue}`);
    await page.getByRole('combobox', { name: 'Zone', exact: true }).selectOption({ label: 'Terrace' });
    await page.getByRole('button', { name: 'T4, 6 seats, available', exact: true }).click();
    sql("UPDATE dining_tables SET status='DIRTY',version=version+1 WHERE code='T4'");
    try {
      await expect(page.getByRole('button', { name: 'T4, 6 seats, dirty', exact: true })).toBeDisabled({ timeout: 10000 });
      await expect(page.getByLabel('Selected tables', { exact: true })).toContainText('No tables selected');
      await setVisibility(page, 'hidden');
      const before = reads.length;
      await page.waitForTimeout(5500);
      expect(reads.length).toBe(before);
      await setVisibility(page, 'visible');
      await expect.poll(() => reads.length).toBe(before + 1);
      await page.getByRole('link', { name: 'Queue board', exact: true }).click();
      await expect(page.getByRole('heading', { name: /Welcome/ })).toBeVisible();
      const stopped = reads.length;
      await page.waitForTimeout(5500);
      expect(reads.length).toBe(stopped);
    } finally { sql("UPDATE dining_tables SET status='AVAILABLE',version=version+1 WHERE code='T4'"); }
  });
});
