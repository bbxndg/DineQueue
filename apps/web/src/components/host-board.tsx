'use client';

import { useState, useTransition } from 'react';
import { useRouter } from 'next/navigation';

type Entry = {
  id: string; ticketCode: string; partySize: number; status: string;
  joinedAt: string; calledAt: string | null;
};

function errorMessage(status: number, code: string | undefined) {
  if (status === 401) return 'Your session has ended. Please sign in again.';
  if (code === 'QUEUE_EMPTY') return 'No waiting party to call.';
  if (code === 'CALL_IN_PROGRESS') return 'Complete the currently called ticket first.';
  if (code === 'INVALID_TRANSITION') return 'This ticket can no longer be completed. Refresh the queue.';
  if (code === 'CSRF_REJECTED') return 'This action could not be verified. Refresh the page and try again.';
  if (status === 403) return 'You are not assigned to this queue.';
  if (status === 404) return 'This ticket was not found in this queue. Refresh the queue.';
  return 'Unable to update the queue. Please try again.';
}

export function HostBoard({ queueId, timezone, csrfToken, current, waiting }: {
  queueId: string; timezone: string; csrfToken: string; current: Entry | null; waiting: Entry[];
}) {
  const router = useRouter();
  const [pending, setPending] = useState(false);
  const [refreshing, startRefresh] = useTransition();
  const [confirming, setConfirming] = useState(false);
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');
  const time = new Intl.DateTimeFormat('en-GB', { timeStyle: 'short', timeZone: timezone });
  const busy = pending || refreshing;

  async function mutate(path: string, success: (entry: Entry) => string) {
    if (pending) return;
    setPending(true); setError(''); setNotice('');
    try {
      const response = await fetch(`/api/v1/host/queues/${encodeURIComponent(queueId)}${path}`, {
        method: 'POST', credentials: 'same-origin', headers: { 'X-CSRF-Token': csrfToken },
      });
      const body = await response.json().catch(() => null);
      if (!response.ok) {
        setError(errorMessage(response.status, body?.error?.code));
        if (response.status === 401) router.replace('/login');
      } else setNotice(success(body));
      setConfirming(false);
      startRefresh(() => router.refresh());
    } catch { setError('Unable to connect. Please try again.'); }
    finally { setPending(false); }
  }

  return <div className="mt-8 grid gap-8" aria-busy={busy}>
    <div className="flex flex-wrap items-center gap-4">
      <button disabled={busy || current !== null || waiting.length === 0}
        onClick={() => mutate('/call-next', entry => `Ticket ${entry.ticketCode} called.`)}
        className="rounded-lg bg-wine px-5 py-3 font-semibold text-white disabled:opacity-60">
        {pending && !confirming ? 'Working…' : 'Call next'}
      </button>
      <button disabled={busy} onClick={() => { setError(''); setNotice(''); startRefresh(() => router.refresh()); }}
        className="font-semibold text-wine underline disabled:opacity-60">{refreshing ? 'Refreshing…' : 'Refresh queue'}</button>
    </div>
    {error && <p role="alert" className="rounded-lg border border-wine/30 bg-white p-3 text-wine">{error}</p>}
    {notice && <p role="status" className="rounded-lg border border-ink/20 bg-white p-3">{notice}</p>}

    <section aria-labelledby="current-heading" className="rounded-2xl border border-wine/30 bg-white p-6">
      <h2 id="current-heading" className="text-sm font-semibold uppercase tracking-widest text-wine">Now called</h2>
      {current === null ? <p className="mt-3">No ticket is currently called.</p> : <>
        <p className="mt-3 text-5xl font-semibold tracking-tight" data-testid="current-ticket">{current.ticketCode}</p>
        <p className="mt-3">Party of {current.partySize}{current.calledAt && <> · called at <time dateTime={current.calledAt}>{time.format(new Date(current.calledAt))}</time></>}</p>
        {!confirming
          ? <button disabled={busy} onClick={() => { setConfirming(true); setError(''); setNotice(''); }}
            className="mt-5 rounded-lg border border-wine px-5 py-3 font-semibold text-wine disabled:opacity-60">Complete</button>
          : <div className="mt-5 grid gap-3">
            <p>Confirm the waitlist service for ticket {current.ticketCode} is finished?</p>
            <div className="flex gap-4">
              <button disabled={busy} onClick={() => mutate(`/entries/${encodeURIComponent(current.id)}/complete`, entry => `Ticket ${entry.ticketCode} completed.`)}
                className="rounded-lg bg-wine px-5 py-3 font-semibold text-white disabled:opacity-60">{pending ? 'Completing…' : 'Confirm complete'}</button>
              <button disabled={pending} onClick={() => setConfirming(false)} className="font-semibold text-wine underline">Cancel</button>
            </div>
          </div>}
      </>}
    </section>

    <section aria-labelledby="waiting-heading">
      <h2 id="waiting-heading" className="text-xl font-semibold">Waiting ({waiting.length})</h2>
      {waiting.length === 0 ? <p className="mt-3">No parties are waiting.</p> :
        <table className="mt-4 w-full border-collapse text-left">
          <thead><tr className="border-b border-wine/30">
            <th scope="col" className="py-2">Position</th><th scope="col" className="py-2">Ticket</th>
            <th scope="col" className="py-2">Party size</th><th scope="col" className="py-2">Joined</th>
          </tr></thead>
          <tbody>{waiting.map((entry, index) => <tr key={entry.id} className="border-b border-ink/10" data-testid="waiting-row">
            <td className="py-2">{index + 1}</td>
            <td className="py-2 font-semibold">{entry.ticketCode}</td>
            <td className="py-2">{entry.partySize}</td>
            <td className="py-2"><time dateTime={entry.joinedAt}>{time.format(new Date(entry.joinedAt))}</time></td>
          </tr>)}</tbody>
        </table>}
    </section>
  </div>;
}
