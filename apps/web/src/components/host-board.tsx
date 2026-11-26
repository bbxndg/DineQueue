'use client';

import { useCallback, useEffect, useState } from 'react';
import { useRouter } from 'next/navigation';
import { LiveStatus } from '@/components/live-refresh';
import { readHostBoard, type HostBoardState } from '@/lib/live-reads';
import { useLiveResource } from '@/lib/use-live-resource';
import type { HostEntry } from '@/lib/ticket-types';

type Entry = HostEntry;

function errorMessage(status: number, code: string | undefined) {
  if (status === 401) return 'Your session has ended. Please sign in again.';
  if (code === 'NO_READY_PARTIES') return 'No waiting party is ready to be called.';
  if (code === 'QUEUE_EMPTY') return 'No waiting party to call.';
  if (code === 'CALL_IN_PROGRESS') return 'Complete the currently called ticket first.';
  if (code === 'INVALID_TRANSITION') return 'This ticket can no longer be completed. Refresh the queue.';
  if (code === 'CSRF_REJECTED') return 'This action could not be verified. Refresh the page and try again.';
  if (status === 403) return 'You are not assigned to this queue.';
  if (status === 404) return 'This ticket was not found in this queue. Refresh the queue.';
  return 'Unable to update the queue. Please try again.';
}

// The parent keys this component by ticket ID. A refreshed current ticket must
// receive its own confirmation, and the mutation always uses the selected ID.
function CompleteAction({ entry, busy, pending, onComplete }: {
  entry: Entry; busy: boolean; pending: boolean; onComplete: (id: string) => Promise<void>;
}) {
  const [confirming, setConfirming] = useState(false);
  if (!confirming) return <button disabled={busy} onClick={() => setConfirming(true)}
    className="mt-5 rounded-lg border border-wine px-5 py-3 font-semibold text-wine disabled:opacity-60">Complete</button>;
  return <div className="mt-5 grid gap-3">
    <p>Confirm the waitlist service for ticket {entry.ticketCode} is finished?</p>
    <div className="flex gap-4">
      <button disabled={busy} onClick={async () => { await onComplete(entry.id); setConfirming(false); }}
        className="rounded-lg bg-wine px-5 py-3 font-semibold text-white disabled:opacity-60">{pending ? 'Completing…' : 'Confirm complete'}</button>
      <button disabled={pending} onClick={() => setConfirming(false)} className="font-semibold text-wine underline">Cancel</button>
    </div>
  </div>;
}

export function HostBoard({ queueId, timezone, csrfToken, current: initialCurrent, waiting: initialWaiting, renderedAt }: {
  queueId: string; timezone: string; csrfToken: string; current: Entry | null; waiting: Entry[]; renderedAt: string;
}) {
  const router = useRouter();
  // The parent keys this board by queue ID, so a queue change remounts it and
  // the poller for the previous queue is stopped and its reads invalidated.
  const read = useCallback((signal: AbortSignal) => readHostBoard(queueId, signal), [queueId]);
  const live = useLiveResource<HostBoardState>({ current: initialCurrent, waiting: initialWaiting }, queueId, read);
  const { current, waiting } = live.data;
  const [pending, setPending] = useState(false);
  const [manual, setManual] = useState(false);
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');
  // FIFO spans service dates, so show the date too: a leftover ticket from an
  // earlier day must not look like a recent one.
  const time = new Intl.DateTimeFormat('en-GB', { dateStyle: 'medium', timeStyle: 'short', timeZone: timezone });
  // Background polls never disable controls; only writes and manual refresh do.
  const busy = pending || manual;

  useEffect(() => {
    if (live.accessLost === 401) router.replace('/login');
    else if (live.accessLost) router.replace('/');
  }, [live.accessLost, router]);

  async function mutate(path: string, success: (entry: Entry) => string) {
    if (pending) return;
    setPending(true); setError(''); setNotice('');
    try {
      // Reads are suspended/invalidated during the write and refetched after it,
      // so a pre-mutation snapshot cannot overwrite the committed result.
      await live.runMutation(async () => {
        try {
          const response = await fetch(`/api/v1/host/queues/${encodeURIComponent(queueId)}${path}`, {
            method: 'POST', credentials: 'same-origin', headers: { 'X-CSRF-Token': csrfToken },
          });
          const body = await response.json().catch(() => null);
          // 401 uses the same access-loss path as polled reads: one redirect.
          if (response.status === 401) live.reportAccessLost(401);
          else if (!response.ok) setError(errorMessage(response.status, body?.error?.code));
          else setNotice(success(body));
        } catch { setError('Unable to connect. Please try again.'); }
      });
    } finally { setPending(false); }
  }

  async function refreshQueue() {
    setError(''); setNotice(''); setManual(true);
    try { await live.refresh(); } finally { setManual(false); }
  }

  if (live.accessLost) return <p role="status" className="mt-8">{live.accessLost === 401
    ? 'Your session has ended. Redirecting to sign in…' : 'This queue is no longer available to your account.'}</p>;

  return <div className="mt-8 grid min-w-0 gap-8" aria-busy={busy}>
    <div className="grid gap-2">
      <div className="flex flex-wrap items-center gap-4">
        <button disabled={busy || current !== null || !waiting.some(entry => entry.isReady)}
          onClick={() => mutate('/call-next', entry => `Ticket ${entry.ticketCode} called.`)}
          className="rounded-lg bg-wine px-5 py-3 font-semibold text-white disabled:opacity-60">
          {pending && current === null ? 'Working…' : 'Call next'}
        </button>
        <button disabled={busy} onClick={refreshQueue}
          className="font-semibold text-wine underline disabled:opacity-60">{manual ? 'Refreshing…' : 'Refresh queue'}</button>
      </div>
      <LiveStatus updatedAt={live.updatedAt} renderedAt={renderedAt} failure={live.failure} />
    </div>
    {current === null && waiting.length > 0 && !waiting.some(entry => entry.isReady) &&
      <p className="text-sm">No waiting party is ready to be called. Not-ready parties keep their original queue order.</p>}
    {error && <p role="alert" className="rounded-lg border border-wine/30 bg-white p-3 text-wine">{error}</p>}
    {notice && <p role="status" className="rounded-lg border border-ink/20 bg-white p-3">{notice}</p>}

    <section aria-labelledby="current-heading" className="rounded-2xl border border-wine/30 bg-white p-6">
      <h2 id="current-heading" className="text-sm font-semibold uppercase tracking-widest text-wine">Now called</h2>
      {current === null ? <p className="mt-3">No ticket is currently called.</p> : <>
        <p className="mt-3 text-5xl font-semibold tracking-tight" data-testid="current-ticket">{current.ticketCode}</p>
        <p className="mt-3">Party of {current.partySize}</p>
        <dl className="mt-3 grid gap-2 text-sm">
          <div><dt className="inline font-medium">Joined: </dt><dd className="inline"><time data-testid="current-joined-at" dateTime={current.joinedAt}>{time.format(new Date(current.joinedAt))} ({timezone})</time></dd></div>
          {current.calledAt && <div><dt className="inline font-medium">Called: </dt><dd className="inline"><time data-testid="current-called-at" dateTime={current.calledAt}>{time.format(new Date(current.calledAt))} ({timezone})</time></dd></div>}
        </dl>
        <CompleteAction key={current.id} entry={current} busy={busy} pending={pending}
          onComplete={id => mutate(`/entries/${encodeURIComponent(id)}/complete`, entry => `Ticket ${entry.ticketCode} completed.`)} />
      </>}
    </section>

    <section aria-labelledby="waiting-heading" className="min-w-0">
      <h2 id="waiting-heading" className="text-xl font-semibold">Waiting ({waiting.length})</h2>
      {waiting.length === 0 ? <p className="mt-3">No parties are waiting.</p> :
        <div role="region" aria-label="Waiting tickets" tabIndex={0}
          className="mt-4 overflow-x-auto focus-visible:outline-2 focus-visible:outline-wine">
        <table className="w-full min-w-[32rem] border-collapse text-left">
          <caption className="sr-only">Waiting tickets in first-in, first-out order</caption>
          <thead><tr className="border-b border-wine/30">
            <th scope="col" className="py-2 pr-4">Position</th><th scope="col" className="py-2 pr-4">Ticket</th>
            <th scope="col" className="py-2 pr-4">Party size</th><th scope="col" className="py-2 pr-4">Readiness</th><th scope="col" className="py-2">Joined</th>
          </tr></thead>
          <tbody>{waiting.map((entry, index) => <tr key={entry.id} className="border-b border-ink/10" data-testid="waiting-row">
            <td className="py-2 pr-4">{index + 1}</td>
            <td className="py-2 pr-4 font-semibold">{entry.ticketCode}</td>
            <td className="py-2 pr-4">{entry.partySize}</td>
            <td className="py-2 pr-4">{entry.isReady ? 'Ready' : 'Not ready'}</td>
            <td className="py-2"><time dateTime={entry.joinedAt}>{time.format(new Date(entry.joinedAt))}</time></td>
          </tr>)}</tbody>
        </table></div>}
    </section>
  </div>;
}
