'use client';

import { useState, useTransition } from 'react';
import { useRouter } from 'next/navigation';

export function ReadinessControl({ entryId, isReady, csrfToken }: {
  entryId: string; isReady: boolean; csrfToken: string;
}) {
  const router = useRouter();
  const [pending, setPending] = useState(false);
  const [refreshing, startRefresh] = useTransition();
  const [error, setError] = useState('');

  async function setReadiness() {
    if (pending || refreshing) return;
    setPending(true); setError('');
    try {
      const response = await fetch(`/api/v1/me/queue-entries/${encodeURIComponent(entryId)}/readiness`, {
        method: 'PATCH', credentials: 'same-origin', cache: 'no-store',
        headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': csrfToken },
        body: JSON.stringify({ isReady: !isReady }),
      });
      if (response.status === 401) { router.replace('/login'); router.refresh(); return; }
      if (!response.ok) {
        setError(response.status === 409 ? 'This ticket has changed. Readiness can change only while waiting.'
          : response.status === 404 ? 'This ticket is no longer available.'
          : response.status === 403 ? 'This action could not be verified. Refresh and try again.'
          : 'Unable to change readiness. Please try again.');
      } else {
        // Consume the committed response; the refreshed server view remains authoritative.
        await response.json();
      }
      startRefresh(() => router.refresh());
    } catch { setError('Unable to connect. Your readiness has not been confirmed. Refresh before trying again.'); }
    finally { setPending(false); }
  }

  return <div className="mt-5 grid gap-3" aria-busy={pending || refreshing}>
    <p className="font-semibold" data-testid="ticket-readiness">{isReady ? 'Ready to be called' : 'Not ready'}</p>
    <p className="text-sm">Not-ready parties may be skipped. Becoming ready keeps your original queue order for the next call.</p>
    <button disabled={pending || refreshing} onClick={setReadiness}
      className="rounded-lg border border-wine px-5 py-3 font-semibold text-wine disabled:opacity-60">
      {pending || refreshing ? 'Updating readiness…' : isReady ? 'Mark not ready' : 'Mark ready'}
    </button>
    {error && <p role="alert" className="text-wine">{error}</p>}
  </div>;
}
