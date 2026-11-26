import { test, expect, type Page } from '@playwright/test';

async function signIn(page: Page, email: string) {
  await page.goto('/login');
  await page.getByLabel('Email', { exact: true }).fill(email);
  await page.getByLabel('Password', { exact: true }).fill(process.env.F01_TEST_PASSWORD!);
  await page.getByRole('button', { name: 'Sign in', exact: true }).click();
  await expect(page).not.toHaveURL(/\/login/);
}

test('L7 readiness: skip unavailable party, restore original FIFO, lock readiness after call', async ({ browser }) => {
  const contexts = await Promise.all([browser.newContext(), browser.newContext(), browser.newContext()]);
  try {
    const [a, b, host] = await Promise.all(contexts.map(context => context.newPage()));
    await signIn(a, 'customer1@example.test');
    await signIn(b, 'customer2@example.test');
    await signIn(host, 'host@example.test');
    async function join(page: Page) {
      await page.goto('/join');
      await page.getByLabel('Party size', { exact: true }).fill('4');
      await page.getByRole('button', { name: 'Join waitlist', exact: true }).click();
      await expect(page.getByTestId('ticket-readiness')).toHaveText('Not ready');
      return (await page.getByTestId('ticket-code').textContent())!;
    }
    const aCode = await join(a);
    const bCode = await join(b);
    // Toggle both ways on the active card, then use the owned detail control.
    await a.getByRole('button', { name: 'Mark ready', exact: true }).click();
    await expect(a.getByTestId('ticket-readiness')).toHaveText('Ready to be called');
    await a.getByRole('button', { name: 'Mark not ready', exact: true }).click();
    await expect(a.getByTestId('ticket-readiness')).toHaveText('Not ready');
    await b.getByRole('link', { name: 'View ticket details' }).click();
    // Act on the detail view, not the list control that is still being replaced.
    await expect(b.getByRole('heading', { name: 'Ticket details' })).toBeVisible();
    await b.getByRole('button', { name: 'Mark ready', exact: true }).click();
    await expect(b.getByTestId('ticket-readiness')).toHaveText('Ready to be called');
    await host.getByRole('button', { name: 'Refresh queue', exact: true }).click();
    await expect(host.getByTestId('waiting-row')).toHaveCount(2);
    await expect(host.getByTestId('waiting-row').first()).toContainText(aCode);
    await expect(host.getByTestId('waiting-row').first()).toContainText('Not ready');
    await host.getByRole('button', { name: 'Call next', exact: true }).click();
    await expect(host.getByTestId('current-ticket')).toHaveText(bCode);
    await b.getByRole('button', { name: 'Refresh status', exact: true }).click();
    await expect(b.getByTestId('ticket-status')).toHaveText('CALLED');
    await expect(b.getByRole('button', { name: /Mark (not )?ready/ })).toHaveCount(0);
    await a.getByRole('button', { name: 'Mark ready', exact: true }).click();
    await expect(a.getByTestId('ticket-readiness')).toHaveText('Ready to be called');
    await host.getByRole('button', { name: 'Refresh queue', exact: true }).click();
    await expect(host.getByTestId('current-ticket')).toHaveText(bCode);
    await host.getByRole('button', { name: 'Complete', exact: true }).click();
    await host.getByRole('button', { name: 'Confirm complete', exact: true }).click();
    await expect(host.getByText('No ticket is currently called.', { exact: true })).toBeVisible();
    await host.getByRole('button', { name: 'Call next', exact: true }).click();
    await expect(host.getByTestId('current-ticket')).toHaveText(aCode);
    await a.getByRole('button', { name: 'Refresh status', exact: true }).click();
    await expect(a.getByTestId('ticket-status')).toHaveText('CALLED');
    // Leave no active called ticket for later regression fixtures on the demo queue.
    await host.getByRole('button', { name: 'Complete', exact: true }).click();
    await host.getByRole('button', { name: 'Confirm complete', exact: true }).click();
    await expect(host.getByText('No ticket is currently called.', { exact: true })).toBeVisible();
  } finally { await Promise.all(contexts.map(context => context.close())); }
});
