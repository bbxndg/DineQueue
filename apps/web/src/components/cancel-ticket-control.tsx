'use client';

import { useState, useTransition } from 'react';
import { useRouter } from 'next/navigation';
import { useLiveRefresh } from '@/components/live-refresh';

// F-08 (#53): owner cancels a WAITING ticket after an explicit confirmation. The
// server is authoritative; a 409 means the ticket was called/changed meanwhile.
export function CancelTicketControl({ entryId, ticketCode, csrfToken }: {
  entryId: string; ticketCode: string; csrfToken: string;
}) {
  const router = useRouter();
  const live = useLiveRefresh();
  const [confirming, setConfirming] = useState(false);
  const [pending, setPending] = useState(false);
  const [refreshing, startRefresh] = useTransition();
  const [error, setError] = useState('');
  const busy = pending || refreshing;

  async function cancel() {
    if (busy) return;
    setPending(true); setError('');
    // Same coordinator path as readiness: reads are suspended during the write
    // and the committed state is refetched afterwards.
    const write = live ? live.runMutation(submit) : submit().then(done => { if (done) startRefresh(() => router.refresh()); });
    try { await write; } finally { setPending(false); setConfirming(false); }
  }

  /** Returns false when navigation away (401) makes a refresh pointless. */
  async function submit(): Promise<boolean> {
    try {
      const response = await fetch(`/api/v1/me/queue-entries/${encodeURIComponent(entryId)}/cancel`, {
        method: 'POST', credentials: 'same-origin', cache: 'no-store',
        signal: AbortSignal.timeout(15_000),
        headers: { 'X-CSRF-Token': csrfToken },
      });
      if (response.status === 401) {
        if (live) live.reportAccessLost(401);
        else { router.replace('/login'); router.refresh(); }
        return false;
      }
      if (!response.ok) {
        setError(response.status === 409 ? 'This ticket can no longer be cancelled. It may have just been called.'
          : response.status === 404 ? 'This ticket is no longer available.'
          : response.status === 403 ? 'This action could not be verified. Refresh and try again.'
          : 'Unable to cancel the ticket. Please try again.');
      } else {
        await response.json();
      }
      return true;
    } catch {
      setError('Unable to connect. Your cancellation has not been confirmed. Refresh before trying again.');
      return true;
    }
  }

  return <div className="mt-5 grid gap-3" aria-busy={busy}>
    {!confirming ? <button disabled={busy} onClick={() => { setError(''); setConfirming(true); }}
      className="justify-self-start font-semibold text-wine underline disabled:opacity-60">Cancel ticket</button>
      : <div role="group" aria-label="Confirm ticket cancellation" className="grid gap-3 rounded-lg border border-wine/30 p-4">
        <p>Cancel ticket <strong>{ticketCode}</strong>? You will leave the queue and this number cannot be restored. You can join again later with a new number.</p>
        <div className="flex flex-wrap gap-4">
          <button disabled={busy} onClick={cancel}
            className="rounded-lg bg-wine px-5 py-3 font-semibold text-white disabled:opacity-60">{busy ? 'Cancelling…' : 'Confirm cancel'}</button>
          <button disabled={busy} onClick={() => setConfirming(false)} className="font-semibold text-wine underline disabled:opacity-60">Keep my ticket</button>
        </div>
      </div>}
    {error && <p role="alert" className="text-wine">{error}</p>}
  </div>;
}
