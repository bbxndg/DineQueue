'use client';

import Link from 'next/link';
import { useCallback, useEffect } from 'react';
import { useRouter } from 'next/navigation';
import { TicketCard } from '@/components/ticket-card';
import { RefreshTicket } from '@/components/refresh-ticket';
import { LiveRefreshContext, LiveStatus } from '@/components/live-refresh';
import { readActiveTickets, readTicket } from '@/lib/live-reads';
import { useLiveResource } from '@/lib/use-live-resource';
import type { AccessLoss } from '@/lib/poll-coordinator';
import type { Ticket } from '@/lib/ticket-types';

// 401 follows the existing sign-in flow; a forbidden scope returns home.
function useAccessRedirect(accessLost: AccessLoss | null) {
  const router = useRouter();
  useEffect(() => {
    if (accessLost === 401) router.replace('/login');
    else if (accessLost === 403) router.replace('/');
  }, [accessLost, router]);
}

function AccessEnded({ status }: { status: AccessLoss }) {
  return <p role="status" className="mt-8">{status === 401 ? 'Your session has ended. Redirecting to sign in…' : 'This page is no longer available to your account.'}</p>;
}

/** /my-queue: active tickets, refreshed while visible. COMPLETED leaves the list. */
export function LiveMyQueue({ initial, csrfToken, renderedAt }: { initial: Ticket[]; csrfToken: string; renderedAt: string }) {
  const live = useLiveResource(initial, 'active', readActiveTickets);
  useAccessRedirect(live.accessLost);
  return <LiveRefreshContext.Provider value={live}>
    <div className="mt-4"><RefreshTicket /></div>
    <LiveStatus updatedAt={live.updatedAt} renderedAt={renderedAt} failure={live.failure} />
    {/* Protected data is dropped as soon as access is lost. */}
    {live.accessLost ? <AccessEnded status={live.accessLost} /> : live.data.length === 0 ? <div className="mt-8 rounded-2xl border border-wine/30 bg-white p-6">
      <p role="status">You do not have an active ticket.</p>
      <Link href="/join" className="mt-5 inline-block rounded-lg bg-wine px-5 py-3 font-semibold text-white">Join Waitlist</Link>
    </div> : live.data.map(ticket => <TicketCard csrfToken={csrfToken} key={ticket.id} ticket={ticket} showDetails />)}
  </LiveRefreshContext.Provider>;
}

/** /my-queue/{id}: one owned ticket, including COMPLETED, refreshed while visible. */
export function LiveTicketDetail({ initial, csrfToken, renderedAt }: { initial: Ticket; csrfToken: string; renderedAt: string }) {
  const read = useCallback((signal: AbortSignal) => readTicket(initial.id, signal), [initial.id]);
  const live = useLiveResource(initial, initial.id, read);
  useAccessRedirect(live.accessLost);
  return <LiveRefreshContext.Provider value={live}>
    <div className="mt-4 flex flex-wrap gap-5 font-semibold text-wine underline">
      <Link href="/my-queue" prefetch={false}>My Ticket</Link>
      <RefreshTicket />
    </div>
    <LiveStatus updatedAt={live.updatedAt} renderedAt={renderedAt} failure={live.failure} />
    {live.accessLost === 404 ? <div className="mt-8" data-testid="ticket-gone">
      <h2 className="text-2xl font-semibold">Ticket not found</h2>
      <p className="mt-4">This ticket is not available.</p>
    </div> : live.accessLost ? <AccessEnded status={live.accessLost} />
      : <TicketCard csrfToken={csrfToken} ticket={live.data} />}
  </LiveRefreshContext.Provider>;
}
