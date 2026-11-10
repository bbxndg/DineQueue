import { test, expect } from '@playwright/test';
import { randomBytes, randomUUID, createHash } from 'node:crypto';
import { execFileSync } from 'node:child_process';

// Real PostgreSQL fixtures only. Never point this suite at a shared/live database.
function fixtureSQL(query: string) {
  if (!process.env.TEST_DATABASE_URL) throw new Error('TEST_DATABASE_URL required for isolated F-03 browser fixtures.');
  try {
    const db = new URL(process.env.TEST_DATABASE_URL);
    execFileSync('psql', ['--no-psqlrc', '--set', 'ON_ERROR_STOP=1', '--command', query], {
      env: { ...process.env, PGHOST: db.hostname, PGPORT: db.port || '5432',
        PGDATABASE: decodeURIComponent(db.pathname.slice(1)), PGUSER: decodeURIComponent(db.username),
        PGPASSWORD: decodeURIComponent(db.password), PGSSLMODE: db.searchParams.get('sslmode') ?? 'verify-full' },
      stdio: 'ignore', timeout: 10000,
    });
  } catch { throw new Error('F-03 database fixture failed; check psql and the isolated test database configuration.'); }
}

test('F-03 stored status refresh, completed detail and real read-error retry', async ({ page, context }) => {
  const customer = randomUUID();
  const token = randomBytes(32).toString('base64url');
  const hash = createHash('sha256').update(token).digest('hex');
  // Only generated UUIDs and a token digest enter SQL; raw cookie stays in memory.
  fixtureSQL(`BEGIN;
    INSERT INTO users(id,display_name,email,password_hash,role) VALUES
    ('${customer}','F03 browser fixture','${customer}@example.test','not-a-login-hash','CUSTOMER');
    INSERT INTO sessions(id,user_id,token_hash,created_at,expires_at) VALUES
    ('${randomUUID()}','${customer}','${hash}',now(),now()+interval '1 hour'); COMMIT;`);
  let renamed = false;
  try {
    await context.addCookies([{ name: 'dineq_session', value: token, url: process.env.F01_BASE_URL!, httpOnly: true, sameSite: 'Lax' }]);
    const identity = await page.request.get('/api/v1/auth/me');
    expect(identity.status()).toBe(200);
    const { csrfToken } = await identity.json();
    const queues = await page.request.get('/api/v1/queues');
    const queue = (await queues.json()).queues[0];
    const joined = await page.request.post(`/api/v1/queues/${queue.id}/entries`, {
      headers: { Origin: process.env.F01_BASE_URL!, 'X-CSRF-Token': csrfToken }, data: { partySize: 4 },
    });
    expect(joined.status()).toBe(201);
    const ticket = await joined.json();
    await page.goto('/my-queue');
    await expect(page.getByTestId('ticket-status')).toHaveText('WAITING');
    const host = '20000000-0000-4000-8000-000000000003';
    fixtureSQL(`UPDATE queue_entries SET status='CALLED',called_at=now(),called_by='${host}' WHERE customer_id='${customer}';`);
    await page.getByRole('button', { name: 'Refresh status', exact: true }).click();
    await expect(page.getByTestId('ticket-status')).toHaveText('CALLED');
    await expect(page.getByText('Your ticket has been called. Please go to the counter.', { exact: true })).toBeVisible();
    fixtureSQL(`UPDATE queue_entries SET status='COMPLETED',completed_at=now(),completed_by='${host}' WHERE customer_id='${customer}';`);
    await page.getByRole('button', { name: 'Refresh status', exact: true }).click();
    await expect(page.getByRole('status')).toContainText('You do not have an active ticket.');
    await page.goto(`/my-queue/${ticket.id}`);
    await expect(page.getByTestId('ticket-status')).toHaveText('COMPLETED');
    await expect(page.getByText('Your waitlist service has finished.', { exact: true })).toBeVisible();

    // Fail the actual ticket read while authentication remains available.
    fixtureSQL('ALTER TABLE queue_entries RENAME TO f03_temporarily_unavailable;');
    renamed = true;
    await page.getByRole('button', { name: 'Refresh status', exact: true }).click();
    await expect(page.getByRole('heading', { name: 'We cannot load your ticket right now' })).toBeVisible();
    await expect(page.getByText('You do not have an active ticket.', { exact: true })).toHaveCount(0);
    fixtureSQL('ALTER TABLE f03_temporarily_unavailable RENAME TO queue_entries;');
    renamed = false;
    await page.getByRole('button', { name: 'Try again', exact: true }).click();
    await expect(page.getByTestId('ticket-status')).toHaveText('COMPLETED');
  } finally {
    if (renamed) fixtureSQL('ALTER TABLE f03_temporarily_unavailable RENAME TO queue_entries;');
    fixtureSQL(`BEGIN; DELETE FROM queue_entries WHERE customer_id='${customer}'; DELETE FROM sessions WHERE user_id='${customer}'; DELETE FROM users WHERE id='${customer}'; COMMIT;`);
  }
});
