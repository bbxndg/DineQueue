'use client';

import { useState, type FormEvent } from 'react';
import type { ServiceQueue } from '@/lib/queues';

type Ticket = { id: string; ticketCode: string; serviceDate: string; partySize: number; status: string; joinedAt: string };

const MIN = 1;
const MAX = 16;
const LARGE_PARTY = 9; // Above the 8-seat physical table limit.

function errorMessage(status: number, code: string | undefined) {
  if (status === 400) return 'Party size must be a whole number from 1 to 16.';
  if (status === 401) return 'Your session has ended. Please sign in again.';
  if (status === 404) return 'This queue is no longer available. Refresh the page and try again.';
  if (status === 409 && code === 'ACTIVE_QUEUE_EXISTS') return 'You already have an active ticket for this queue.';
  if (status === 409) return 'This queue is not accepting new entries right now.';
  if (status === 403) return 'This action could not be verified. Refresh the page and try again.';
  return 'Unable to join the waitlist. Please try again.';
}

export function JoinForm({ queues, csrfToken }: { queues: ServiceQueue[]; csrfToken: string }) {
  const [queueId, setQueueId] = useState(queues[0].id);
  const [partySize, setPartySize] = useState('2');
  const [pending, setPending] = useState(false);
  const [error, setError] = useState('');
  const [ticket, setTicket] = useState<Ticket | null>(null);
  const size = /^\d+$/.test(partySize) ? Number(partySize) : NaN;
  const queue = queues.find(value => value.id === queueId) ?? queues[0];

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (pending) return;
    if (!Number.isInteger(size) || size < MIN || size > MAX) {
      setError('Party size must be a whole number from 1 to 16.');
      return;
    }
    setPending(true); setError('');
    try {
      const response = await fetch(`/api/v1/queues/${encodeURIComponent(queueId)}/entries`, {
        method: 'POST', credentials: 'same-origin',
        headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': csrfToken },
        body: JSON.stringify({ partySize: size }),
      });
      if (!response.ok) {
        const body = await response.json().catch(() => null);
        setError(errorMessage(response.status, body?.error?.code));
        return;
      }
      setTicket(await response.json());
    } catch { setError('Unable to connect. Please try again.'); }
    finally { setPending(false); }
  }

  if (ticket) {
    return <section aria-labelledby="ticket-heading" className="mt-8 rounded-2xl border border-wine/30 bg-white p-6">
      <h2 id="ticket-heading" className="text-sm font-semibold uppercase tracking-widest text-wine">Your ticket</h2>
      <p className="mt-3 text-5xl font-semibold tracking-tight" data-testid="ticket-code">{ticket.ticketCode}</p>
      <dl className="mt-6 grid grid-cols-2 gap-3 text-sm">
        <dt className="font-medium">Status</dt><dd data-testid="ticket-status">{ticket.status}</dd>
        <dt className="font-medium">Party size</dt><dd>{ticket.partySize}</dd>
        <dt className="font-medium">Queue</dt><dd>{queue.name}</dd>
        <dt className="font-medium">Joined</dt><dd>{new Date(ticket.joinedAt).toLocaleString()}</dd>
      </dl>
      <p className="mt-6 leading-relaxed">Please stay nearby. The host will call your ticket number.</p>
    </section>;
  }

  return <form onSubmit={submit} noValidate className="mt-8 grid gap-5" aria-busy={pending}>
    {queues.length === 1
      ? <p><span className="font-medium">Queue:</span> {queue.name} · {queue.restaurantName}</p>
      : <div className="grid gap-2">
        <label htmlFor="queue" className="font-medium">Queue</label>
        <select id="queue" value={queueId} onChange={event => setQueueId(event.target.value)} disabled={pending} className="rounded-lg border border-ink/30 bg-white px-4 py-3 focus:outline-2 focus:outline-wine">
          {queues.map(value => <option key={value.id} value={value.id}>{value.name} · {value.restaurantName}</option>)}
        </select>
      </div>}
    <div className="grid gap-2">
      <label htmlFor="partySize" className="font-medium">Party size</label>
      <input id="partySize" name="partySize" type="number" inputMode="numeric" min={MIN} max={MAX} step={1} required
        value={partySize} onChange={event => setPartySize(event.target.value)} disabled={pending} aria-describedby="partySizeHelp"
        className="rounded-lg border border-ink/30 bg-white px-4 py-3 focus:outline-2 focus:outline-wine" />
      <p id="partySizeHelp" className="text-sm">Whole number from 1 to 16.</p>
    </div>
    {size >= LARGE_PARTY && size <= MAX && <p role="note" className="rounded-lg border border-ink/20 bg-white p-3 text-sm">
      Parties of {size} may be seated across connected tables. Combined seating is not guaranteed.
    </p>}
    {error && <p role="alert" className="rounded-lg border border-wine/30 bg-white p-3 text-wine">{error}</p>}
    <button disabled={pending} className="rounded-lg bg-wine px-5 py-3 font-semibold text-white disabled:opacity-60">{pending ? 'Joining…' : 'Join waitlist'}</button>
  </form>;
}
