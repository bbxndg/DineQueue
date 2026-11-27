import Link from 'next/link';
import { ReadinessControl } from '@/components/readiness-control';
import { CancelTicketControl } from '@/components/cancel-ticket-control';
import type { Ticket } from '@/lib/ticket-types';

const statusMessages = {
  WAITING: 'Please stay nearby. The host will call your ticket number.',
  CALLED: 'Your ticket has been called. Please go to the counter.',
  NO_SHOW: 'Your party was not seated after being called, and the held tables were released. You can join the queue again.',
  CANCELLED: 'This ticket has been cancelled. You can join the queue again.',
  COMPLETED: 'Your waitlist service has finished.',
};

export function TicketCard({ ticket, showDetails = false, csrfToken }: { ticket: Ticket; showDetails?: boolean; csrfToken: string }) {
  const joined = new Intl.DateTimeFormat('en-GB', {
    dateStyle: 'medium', timeStyle: 'short', timeZone: ticket.timezone,
  }).format(new Date(ticket.joinedAt));
  return <article aria-label={`Ticket ${ticket.ticketCode}`} className="mt-8 rounded-2xl border border-wine/30 bg-white p-6">
    <h2 className="text-5xl font-semibold tracking-tight" data-testid="ticket-code">{ticket.ticketCode}</h2>
    <dl className="mt-6 grid grid-cols-[auto_1fr] gap-x-5 gap-y-3 text-sm">
      <dt className="font-medium">Status</dt><dd data-testid="ticket-status" className="font-semibold text-wine">{ticket.status}</dd>
      <dt className="font-medium">Party size</dt><dd data-testid="party-size">{ticket.partySize}</dd>
      <dt className="font-medium">Queue</dt><dd>{ticket.queueName}</dd>
      <dt className="font-medium">Restaurant</dt><dd>{ticket.restaurantName}</dd>
      <dt className="font-medium">Service date</dt><dd>{ticket.serviceDate}</dd>
      <dt className="font-medium">Joined</dt><dd><time dateTime={ticket.joinedAt}>{joined} ({ticket.timezone})</time></dd>
    </dl>
    <p className="mt-6 leading-relaxed">{statusMessages[ticket.status]}</p>
    {ticket.status === 'WAITING' && <ReadinessControl entryId={ticket.id} isReady={ticket.isReady} csrfToken={csrfToken} />}
    {ticket.status === 'WAITING' && <CancelTicketControl key={ticket.id} entryId={ticket.id} ticketCode={ticket.ticketCode} csrfToken={csrfToken} />}
    {(ticket.status === 'CANCELLED' || ticket.status === 'NO_SHOW') && <Link href="/join" prefetch={false} className="mt-5 inline-block rounded-lg bg-wine px-5 py-3 font-semibold text-white">Join again</Link>}
    {showDetails && <Link href={`/my-queue/${ticket.id}`} prefetch={false} className="mt-5 inline-block font-semibold text-wine underline">View ticket details</Link>}
  </article>;
}
