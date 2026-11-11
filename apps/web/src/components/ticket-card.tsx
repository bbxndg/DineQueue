import Link from 'next/link';
import type { Ticket } from '@/lib/tickets';

const statusMessages = {
  WAITING: 'Please stay nearby. The host will call your ticket number.',
  CALLED: 'Your ticket has been called. Please go to the counter.',
  COMPLETED: 'Your waitlist service has finished.',
};

export function TicketCard({ ticket, showDetails = false }: { ticket: Ticket; showDetails?: boolean }) {
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
    {showDetails && <Link href={`/my-queue/${ticket.id}`} prefetch={false} className="mt-5 inline-block font-semibold text-wine underline">View ticket details</Link>}
  </article>;
}
