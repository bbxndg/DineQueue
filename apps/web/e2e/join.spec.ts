import { test, expect } from '@playwright/test';

// T-010 (F-02 part): browser join with validation, large-party notice and duplicate banner.
// Uses customer2 so the F-01 auth journeys on customer1 are unaffected. Needs a fresh seed.
test('T-010 Customer joins the waitlist and sees a WAITING ticket', async ({ page }) => {
  await page.goto('/login');
  await page.getByLabel('Email', { exact: true }).fill('customer2@example.test');
  await page.getByLabel('Password', { exact: true }).fill(process.env.F01_TEST_PASSWORD!);
  await page.getByRole('button', { name: 'Sign in', exact: true }).click();
  await page.getByRole('link', { name: 'Join Waitlist' }).click();
  await expect(page).toHaveURL(/\/join$/);

  const size = page.getByLabel('Party size', { exact: true });
  const join = page.getByRole('button', { name: 'Join waitlist', exact: true });
  for (const invalid of ['0', '17']) {
    await size.fill(invalid);
    await join.click();
    await expect(page.getByRole('alert').filter({ hasText: 'Party size must be a whole number from 1 to 16.' })).toBeVisible();
  }
  await size.fill('10');
  await expect(page.getByRole('note')).toContainText('Combined seating is not guaranteed.');

  const created = page.waitForResponse(response => response.url().includes('/entries') && response.request().method() === 'POST');
  await join.click();
  const response = await created;
  expect(response.status()).toBe(201);
  const body = await response.json();
  await expect(page.getByTestId('ticket-code')).toHaveText(body.ticketCode);
  expect(body.ticketCode).toMatch(/^A-\d{3,}$/);
  await expect(page.getByTestId('ticket-status')).toHaveText('WAITING');
  expect(body.partySize).toBe(10);

  // A second join from the UI is rejected by the backend with a visible banner.
  await page.reload();
  await page.getByLabel('Party size', { exact: true }).fill('2');
  await page.getByRole('button', { name: 'Join waitlist', exact: true }).click();
  await expect(page.getByRole('alert').filter({ hasText: 'You already have an active ticket for this queue.' })).toBeVisible();
});

test('T-017 Host is redirected away from the join page', async ({ page }) => {
  await page.goto('/login');
  await page.getByLabel('Email', { exact: true }).fill('host@example.test');
  await page.getByLabel('Password', { exact: true }).fill(process.env.F01_TEST_PASSWORD!);
  await page.getByRole('button', { name: 'Sign in', exact: true }).click();
  await expect(page).toHaveURL(/\/host\/queue$/);
  await page.goto('/join');
  await expect(page).toHaveURL(/\/host\/queue$/);
});
