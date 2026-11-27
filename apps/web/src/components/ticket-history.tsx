'use client';

import Link from 'next/link';
import { useCallback, useEffect, useRef, useState } from 'react';
import { useRouter } from 'next/navigation';
import { readHistoryPage } from '@/lib/history';
import type { Ticket } from '@/lib/ticket-types';

type Phase = 'loading' | 'ready' | 'initial-error' | 'more-error' | 'loading-more' | 'forbidden' | 'signed-out';

const statusText: Record<string, string> = {
  COMPLETED: 'Completed',
  CANCELLED: 'Cancelled',
  NO_SHOW: 'No-show',
};

/**
 * /my-queue/history (S3 #56). No polling. Each request is tagged with a
 * generation; unmount or a reset bumps it so late responses (including from a
 * previous session) are discarded. Only one page request runs at a time.
 */
export function TicketHistory() {
  const router = useRouter();
  const [tickets, setTickets] = useState<Ticket[]>([]);
  const [cursor, setCursor] = useState<string | null>(null);
  const [phase, setPhase] = useState<Phase>('loading');
  const generation = useRef(0);
  const inFlight = useRef<AbortController | null>(null);

  const load = useCallback(async (from: string | null) => {
    if (inFlight.current) return; // no duplicate page requests
    const gen = generation.current;
    const controller = new AbortController();
    inFlight.current = controller;
    setPhase(from ? 'loading-more' : 'loading');
    try {
      const result = await readHistoryPage(from, controller.signal);
      if (gen !== generation.current) return;
      if (result.kind === 'status') {
        if (result.status === 401) {
          // Clear protected data before leaving.
          generation.current++;
          setTickets([]); setCursor(null); setPhase('signed-out');
          router.replace('/login');
          return;
        }
        if (result.status === 403) {
          generation.current++;
          setTickets([]); setCursor(null); setPhase('forbidden');
          return;
        }
        setPhase(from ? 'more-error' : 'initial-error');
        return;
      }
      setTickets(previous => {
        const seen = new Set(previous.map(t => t.id));
        return [...(from ? previous : []), ...result.page.tickets.filter(t => !from || !seen.has(t.id))];
      });
      setCursor(result.page.nextCursor);
      setPhase('ready');
    } catch {
      if (gen === generation.current && !controller.signal.aborted) setPhase(from ? 'more-error' : 'initial-error');
    } finally {
      if (inFlight.current === controller) inFlight.current = null;
    }
  }, [router]);

  useEffect(() => {
    // Counter/controller refs, not DOM nodes: capture them for the cleanup.
    const gens = generation, pending = inFlight;
    const initial = setTimeout(() => { void load(null); }, 0);
    return () => {
      clearTimeout(initial);
      gens.current++;
      pending.current?.abort();
      pending.current = null;
    };
  }, [load]);

  if (phase === 'signed-out') return <p role="status" className="mt-8">Your session has ended. Redirecting to sign in…</p>;
  if (phase === 'forbidden') return <p role="status" className="mt-8">This page is not available to your account.</p>;
  if (phase === 'loading') return <p role="status" className="mt-8">Loading your ticket history…</p>;
  if (phase === 'initial-error') return <div className="mt-8 grid gap-3">
    <p role="alert">Your ticket history could not be loaded.</p>
    <button onClick={() => void load(null)} className="justify-self-start rounded-lg border border-wine px-5 py-3 font-semibold text-wine">Retry</button>
  </div>;
  if (tickets.length === 0) return <p role="status" className="mt-8">You have no past tickets yet.</p>;

  return <div className="mt-8 grid gap-4">
    <p className="text-sm">Completed means your waitlist turn finished. It does not show whether your meal has ended.</p>
    <ul aria-label="Past tickets" className="grid gap-4">
      {tickets.map(ticket => <HistoryItem key={ticket.id} ticket={ticket} />)}
    </ul>
    {phase === 'more-error' && <div className="grid gap-3">
      <p role="alert">More tickets could not be loaded. The tickets above are still shown.</p>
      <button onClick={() => void load(cursor)} className="justify-self-start rounded-lg border border-wine px-5 py-3 font-semibold text-wine">Retry</button>
    </div>}
    {cursor && phase !== 'more-error' && <button disabled={phase === 'loading-more'} onClick={() => void load(cursor)}
      className="justify-self-start rounded-lg border border-wine px-5 py-3 font-semibold text-wine disabled:opacity-60">
      {phase === 'loading-more' ? 'Loading more…' : 'Load more'}
    </button>}
    {!cursor && <p className="text-sm">End of your ticket history.</p>}
  </div>;
}

function HistoryItem({ ticket }: { ticket: Ticket }) {
  // Joined instant in the restaurant timezone; the stored service date is shown as-is.
  const joined = new Intl.DateTimeFormat('en-GB', { dateStyle: 'medium', timeStyle: 'short', timeZone: ticket.timezone }).format(new Date(ticket.joinedAt));
  return <li data-testid="history-item" className="rounded-2xl border border-wine/30 bg-white p-5">
    <article aria-label={`Ticket ${ticket.ticketCode}`}>
      <h2 className="text-2xl font-semibold" data-testid="history-code">{ticket.ticketCode}</h2>
      <dl className="mt-3 grid grid-cols-[auto_1fr] gap-x-4 gap-y-1 text-sm">
        <dt className="font-medium">Status</dt><dd data-testid="history-status" className="font-semibold text-wine">{statusText[ticket.status] ?? ticket.status}</dd>
        <dt className="font-medium">Party size</dt><dd>{ticket.partySize}</dd>
        <dt className="font-medium">Queue</dt><dd>{ticket.queueName} · {ticket.restaurantName}</dd>
        <dt className="font-medium">Service date</dt><dd data-testid="history-service-date">{ticket.serviceDate}</dd>
        <dt className="font-medium">Joined</dt><dd><time data-testid="history-joined" dateTime={ticket.joinedAt}>{joined} ({ticket.timezone})</time></dd>
      </dl>
      <Link href={`/my-queue/${ticket.id}`} prefetch={false} className="mt-3 inline-block min-h-11 py-2 font-semibold text-wine underline">
        View ticket {ticket.ticketCode}
      </Link>
    </article>
  </li>;
}
