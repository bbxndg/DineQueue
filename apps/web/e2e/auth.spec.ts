import { test, expect, type Page } from '@playwright/test';

async function signIn(page: Page, email: string) {
  await page.goto('/login');
  await page.getByLabel('Email', { exact: true }).fill(email);
  await page.getByLabel('Password', { exact: true }).fill(process.env.F01_TEST_PASSWORD!);
  await page.getByRole('button', { name: 'Sign in', exact: true }).click();
}

test('T-041 Customer login, refresh, role redirect, CSRF rejection and logout', async ({ page, context }) => {
  await signIn(page, 'customer1@example.test');
  await expect(page.getByRole('heading', { name: 'Welcome, Customer One' })).toBeVisible();
  await page.reload();
  await expect(page.getByRole('heading', { name: 'Welcome, Customer One' })).toBeVisible();
  const cookie = (await context.cookies()).find(value => value.name === 'dineq_session');
  expect(cookie?.httpOnly).toBe(true);
  expect(cookie?.sameSite).toBe('Lax');
  const rejected = await page.request.post('/api/v1/auth/logout', { headers: { Origin: 'https://evil.example' } });
  expect(rejected.status()).toBe(403);
  expect((await page.request.get('/api/v1/auth/me')).status()).toBe(200);
  await page.goto('/host/queue');
  await expect(page).toHaveURL(new URL('/', process.env.F01_BASE_URL!).href);
  expect(await page.evaluate(() => Object.keys(localStorage))).toEqual([]);
  await page.getByRole('button', { name: 'Sign out', exact: true }).click();
  await expect(page).toHaveURL(/\/login$/);
  const revoked = await page.request.get('/api/v1/auth/me', { headers: { Cookie: `dineq_session=${cookie!.value}` } });
  expect(revoked.status()).toBe(401);
  await page.goto('/');
  await expect(page).toHaveURL(/\/login$/);
});

test('T-041 Host identity survives reload and signs out', async ({ page }) => {
  await signIn(page, 'host@example.test');
  await expect(page).toHaveURL(/\/host\/queue$/);
  await expect(page.getByRole('heading', { name: 'Welcome, Restaurant Host' })).toBeVisible();
  await page.reload();
  await expect(page.getByRole('heading', { name: 'Welcome, Restaurant Host' })).toBeVisible();
  await page.getByRole('button', { name: 'Sign out', exact: true }).click();
  await expect(page).toHaveURL(/\/login$/);
});

test('T-041 Invalid credentials and invalid session recover to login', async ({ page, context }) => {
  await page.goto('/');
  await expect(page).toHaveURL(/\/login$/);
  await page.getByLabel('Email', { exact: true }).fill('customer1@example.test');
  await page.getByLabel('Password', { exact: true }).fill('incorrect-test-password');
  await page.getByRole('button', { name: 'Sign in', exact: true }).click();
  await expect(page.getByRole('alert').filter({ hasText: 'Email or password is incorrect.' })).toHaveText('Email or password is incorrect.');
  expect((await context.cookies()).some(cookie => cookie.name === 'dineq_session')).toBe(false);
  await context.addCookies([{ name: 'dineq_session', value: 'invalid-expired-session', url: process.env.F01_BASE_URL!, httpOnly: true, sameSite: 'Lax' }]);
  await page.goto('/');
  await expect(page).toHaveURL(/\/login$/);
  await expect(page.getByRole('status')).toHaveText('Your session has ended. Please sign in again.');
});
