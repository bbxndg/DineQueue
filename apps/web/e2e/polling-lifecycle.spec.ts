import { test, expect } from '@playwright/test';
import {
  AUTO, POLL_WAIT, addQueue, addUser, controllableVisibility, countReads, hostPost, join,
  setReady, setVisibility, sql, withQueue,
} from './polling-fixture';

// Lab 7 polling lifecycle, coordination and failures (L7-T-13..17), in a real
// browser against the real API and isolated PostgreSQL. No test relies on a
// manual Refresh unless it is checking manual Refresh itself.

test('L7-T-13/15 schedules after settle, pauses while hidden, resumes once, stops on unmount', async ({ browser }) => {
  test.setTimeout(90_000); // deliberately waits through several intervals
  await withQueue(browser, async f => {
    const ticket = await join(f.customer, f.queue);
    const page = f.customer.page;
    await controllableVisibility(page);
    const reads = countReads(page, '/api/v1/me/queue-entries/active');
    await page.goto('/my-queue');
    await expect(page.getByTestId('ticket-code')).toHaveText(ticket.ticketCode);
    await expect(page.getByTestId('live-updated')).toContainText('Last updated');
    await expect.poll(() => reads.length, { timeout: POLL_WAIT }).toBe(1);
    await page.waitForTimeout(POLL_WAIT);
    expect(reads.length).toBe(2);

    await setVisibility(page, 'hidden');
    const hiddenAt = reads.length;
    await page.waitForTimeout(12_000);
    expect(reads.length).toBe(hiddenAt);
    await setVisibility(page, 'visible');
    await expect.poll(() => reads.length, { timeout: 1_500 }).toBe(hiddenAt + 1);
    await page.waitForTimeout(1_500);
    expect(reads.length).toBe(hiddenAt + 1); // one immediate read, no catch-up burst

    // Client-side navigation to the owned detail unmounts the list poller.
    await page.getByRole('link', { name: 'View ticket details' }).click();
    await expect(page).toHaveURL(new RegExp(`/my-queue/${ticket.id}$`));
    const afterUnmount = reads.length;
    await page.waitForTimeout(POLL_WAIT * 2);
    expect(reads.length).toBe(afterUnmount);
  });
});

test('L7-T-15 changing the selected Host queue stops polling the previous queue', async ({ browser }) => {
  await withQueue(browser, async (f, cleanup) => {
    const second = addQueue('Polling second queue', f.host.userId, cleanup);
    const host = f.host.page;
    const first = countReads(host, `/api/v1/host/queues/${f.queue}/`);
    const other = countReads(host, `/api/v1/host/queues/${second}/`);
    await host.goto(`/host/queue?queue=${f.queue}`);
    await expect.poll(() => first.length, { timeout: POLL_WAIT }).toBeGreaterThanOrEqual(2); // waiting + current
    await host.getByRole('navigation', { name: 'Assigned queues' }).getByRole('link', { name: 'Polling second queue' }).click();
    await expect(host).toHaveURL(new RegExp(`queue=${second}`));
    const stoppedAt = first.length;
    await expect.poll(() => other.length, { timeout: POLL_WAIT * 2 }).toBeGreaterThanOrEqual(2);
    await host.waitForTimeout(POLL_WAIT);
    expect(first.length).toBe(stoppedAt);
  });
});

test('L7-T-14 slow reads never overlap; a late pre-mutation snapshot cannot overwrite Call Next', async ({ browser }) => {
  await withQueue(browser, async f => {
    const ticket = await join(f.customer, f.queue);
    await setReady(f.customer, ticket.id, true);
    const host = f.host.page;
    // Overlap is measured from the browser's own request lifecycle: an aborted
    // request ends (requestfailed) even if the test's route handler still sleeps.
    const waitingPath = `/api/v1/host/queues/${f.queue}/entries?status=WAITING`;
    let inFlight = 0; let maxInFlight = 0; let delayed = 0;
    const isWaitingRead = (url: string) => url.endsWith(waitingPath);
    host.on('request', r => { if (isWaitingRead(r.url())) { inFlight++; maxInFlight = Math.max(maxInFlight, inFlight); } });
    host.on('requestfinished', r => { if (isWaitingRead(r.url())) inFlight--; });
    host.on('requestfailed', r => { if (isWaitingRead(r.url())) inFlight--; });
    await host.route(`**${waitingPath}`, async route => {
      // Capture the real snapshot now and deliver the first one late (> 5 s).
      const response = await route.fetch();
      if (delayed++ === 0) await new Promise(resolve => setTimeout(resolve, 8_000));
      await route.fulfill({ response }).catch(() => undefined); // may be aborted by the coordinator
    });
    await host.goto(`/host/queue?queue=${f.queue}`);
    await expect(host.getByTestId('waiting-row')).toContainText(ticket.ticketCode);
    await expect.poll(() => delayed, { timeout: POLL_WAIT }).toBe(1); // slow scheduled read in flight
    await host.getByRole('button', { name: 'Call next', exact: true }).click();
    await expect(host.getByTestId('current-ticket')).toHaveText(ticket.ticketCode);
    await expect(host.getByTestId('waiting-row')).toHaveCount(0);
    await host.waitForTimeout(9_000); // the stale WAITING snapshot is delivered meanwhile
    await expect(host.getByTestId('waiting-row')).toHaveCount(0);
    await expect(host.getByTestId('current-ticket')).toHaveText(ticket.ticketCode);
    // Manual refresh shares the same coordinator.
    await host.getByRole('button', { name: 'Refresh queue', exact: true }).click();
    await expect(host.getByTestId('current-ticket')).toHaveText(ticket.ticketCode);
    expect(maxInFlight).toBe(1);
  });
});

test('L7-T-14 Customer readiness write while a slow read is pending shows the committed value', async ({ browser }) => {
  await withQueue(browser, async f => {
    const ticket = await join(f.customer, f.queue);
    const page = f.customer.page;
    let delayed = 0;
    await page.route(`**/api/v1/me/queue-entries/${ticket.id}`, async route => {
      if (route.request().method() !== 'GET') return route.fallback();
      const response = await route.fetch();
      if (delayed++ === 0) await new Promise(resolve => setTimeout(resolve, 8_000));
      await route.fulfill({ response }).catch(() => undefined);
    });
    await page.goto(`/my-queue/${ticket.id}`);
    await expect(page.getByTestId('ticket-readiness')).toHaveText('Not ready');
    await expect.poll(() => delayed, { timeout: POLL_WAIT }).toBe(1);
    await page.getByRole('button', { name: 'Mark ready', exact: true }).click();
    await expect(page.getByTestId('ticket-readiness')).toHaveText('Ready to be called');
    await page.waitForTimeout(9_000); // late "Not ready" snapshot must be ignored
    await expect(page.getByTestId('ticket-readiness')).toHaveText('Ready to be called');
  });
});

test('L7-T-16 network/server failure keeps the last state with a stale notice, then recovers', async ({ browser }) => {
  await withQueue(browser, async f => {
    const ticket = await join(f.customer, f.queue);
    await setReady(f.customer, ticket.id, true);
    const page = f.customer.page;
    await page.goto('/my-queue');
    await expect(page.getByTestId('ticket-status')).toHaveText('WAITING');
    const active = '**/api/v1/me/queue-entries/active';
    await page.route(active, route => route.abort('failed'));
    await expect(page.getByTestId('live-stale')).toBeVisible({ timeout: POLL_WAIT * 2 });
    await expect(page.getByTestId('ticket-code')).toHaveText(ticket.ticketCode); // last good state
    await expect(page.getByText('You do not have an active ticket.', { exact: true })).toHaveCount(0);
    await page.unroute(active);
    await page.route(active, route => route.fulfill({ status: 503, contentType: 'application/json', body: '{}' }));
    await page.getByRole('button', { name: 'Refresh status', exact: true }).click();
    await expect(page.getByTestId('live-stale')).toBeVisible();
    await expect(page.getByTestId('ticket-code')).toHaveText(ticket.ticketCode);
    await page.unroute(active);
    await hostPost(f.host, f.queue, '/call-next');
    await expect(page.getByTestId('ticket-status')).toHaveText('CALLED', AUTO);
    await expect(page.getByTestId('live-stale')).toHaveCount(0);

    // The Host board behaves the same way.
    const host = f.host.page;
    await host.goto(`/host/queue?queue=${f.queue}`);
    await expect(host.getByTestId('current-ticket')).toHaveText(ticket.ticketCode);
    await host.route(`**/api/v1/host/queues/${f.queue}/current`, route => route.abort('failed'));
    await expect(host.getByTestId('live-stale')).toBeVisible({ timeout: POLL_WAIT * 2 });
    await expect(host.getByTestId('current-ticket')).toHaveText(ticket.ticketCode);
    await host.unroute(`**/api/v1/host/queues/${f.queue}/current`);
    await expect(host.getByTestId('live-stale')).toHaveCount(0, AUTO);
  });
});

test('L7-T-16 losing access clears protected data: detail 404, forbidden 403, session 401', async ({ browser }) => {
  await withQueue(browser, async (f, cleanup) => {
    const ticket = await join(f.customer, f.queue);
    const page = f.customer.page;
    const detail = await page.context().newPage();
    await detail.goto(`/my-queue/${ticket.id}`);
    await expect(detail.getByTestId('ticket-code')).toHaveText(ticket.ticketCode);
    // Real backend 404: the ticket now belongs to someone else.
    const other = await addUser(browser, 'CUSTOMER', cleanup);
    sql(`UPDATE queue_entries SET customer_id='${other.userId}' WHERE id='${ticket.id}';`);
    await expect(detail.getByTestId('ticket-gone')).toContainText('Ticket not found', AUTO);
    await expect(detail.getByTestId('ticket-code')).toHaveCount(0);
    await expect(detail.getByText(ticket.ticketCode)).toHaveCount(0);
    sql(`UPDATE queue_entries SET customer_id='${f.customer.userId}' WHERE id='${ticket.id}';`);

    // Forbidden scope (403, simulated): leave the protected page.
    await page.goto('/my-queue');
    await expect(page.getByTestId('ticket-code')).toHaveText(ticket.ticketCode);
    await page.route('**/api/v1/me/queue-entries/active', route => route.fulfill({ status: 403, contentType: 'application/json', body: '{}' }), { times: 1 });
    await expect(page).toHaveURL(/\/$/, AUTO);

    // Real session revocation (401): existing sign-in flow, ticket data gone.
    await page.goto('/my-queue');
    await expect(page.getByTestId('ticket-code')).toHaveText(ticket.ticketCode);
    sql(`DELETE FROM sessions WHERE user_id='${f.customer.userId}';`);
    await expect(page).toHaveURL(/\/login$/, AUTO);
    await expect(page.getByTestId('ticket-code')).toHaveCount(0);

    // Host loses the queue grant (real 403): board leaves to the home route.
    const host = f.host.page;
    await host.goto(`/host/queue?queue=${f.queue}`);
    await expect(host.getByRole('button', { name: 'Call next', exact: true })).toBeVisible();
    const keep = addQueue('Polling remaining queue', f.host.userId, cleanup); // Host keeps some assignment
    expect(keep).toBeTruthy();
    sql(`DELETE FROM operator_queue_permissions WHERE user_id='${f.host.userId}' AND queue_id='${f.queue}';`);
    await expect(host).not.toHaveURL(new RegExp(`queue=${f.queue}`), AUTO);
    await expect(host.getByTestId('waiting-row')).toHaveCount(0);
  });
});

test('L7-T-17 confirmation and focus survive polling; a new current ticket resets confirmation', async ({ browser }) => {
  await withQueue(browser, async (f, cleanup) => {
    const a = await join(f.customer, f.queue);
    await setReady(f.customer, a.id, true);
    await hostPost(f.host, f.queue, '/call-next');
    const host = f.host.page;
    await host.goto(`/host/queue?queue=${f.queue}`);
    await expect(host.getByTestId('current-ticket')).toHaveText(a.ticketCode);
    await host.getByRole('button', { name: 'Complete', exact: true }).click();
    const cancel = host.getByRole('button', { name: 'Cancel', exact: true });
    await cancel.focus();
    const reads = countReads(host, `/api/v1/host/queues/${f.queue}/current`);
    await expect.poll(() => reads.length, { timeout: POLL_WAIT * 2 }).toBeGreaterThanOrEqual(2);
    await expect(host.getByRole('button', { name: 'Confirm complete', exact: true })).toBeVisible();
    await expect(cancel).toBeFocused();

    // A second Customer: focus on its readiness control survives polling.
    const b = await addUser(browser, 'CUSTOMER', cleanup);
    const bTicket = await join(b, f.queue);
    await b.page.goto(`/my-queue/${bTicket.id}`);
    const mark = b.page.getByRole('button', { name: 'Mark ready', exact: true });
    await mark.focus();
    const bReads = countReads(b.page, `/api/v1/me/queue-entries/${bTicket.id}`);
    await expect.poll(() => bReads.length, { timeout: POLL_WAIT * 2 }).toBeGreaterThanOrEqual(1);
    await expect(mark).toBeFocused();
    await setReady(b, bTicket.id, true);

    // Another session completes A and calls B: the confirmation belonged to A.
    await hostPost(f.host, f.queue, `/entries/${a.id}/complete`);
    await hostPost(f.host, f.queue, '/call-next');
    await expect(host.getByTestId('current-ticket')).toHaveText(bTicket.ticketCode, AUTO);
    await expect(host.getByRole('button', { name: 'Confirm complete', exact: true })).toHaveCount(0);
    await expect(host.getByRole('button', { name: 'Complete', exact: true })).toBeVisible();
    await expect(b.page.getByTestId('ticket-status')).toHaveText('CALLED', AUTO);
    await hostPost(f.host, f.queue, `/entries/${bTicket.id}/complete`);
  });
});
