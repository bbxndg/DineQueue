import Link from 'next/link';
import { redirect } from 'next/navigation';
import { requireSession } from '@/lib/auth';
import { activeTickets } from '@/lib/tickets';
import { TicketCard } from '@/components/ticket-card';
import { MyTicketShell } from '@/components/my-ticket-shell';
import { RefreshTicket } from '@/components/refresh-ticket';

export default async function MyQueuePage() {
  const session = await requireSession();
  if (session.user.role !== 'CUSTOMER') redirect('/host/queue');
  const tickets = await activeTickets();
  return <MyTicketShell csrfToken={session.csrfToken}>
    <h1 className="text-3xl font-semibold sm:text-4xl">My Ticket</h1>
    <p className="mt-4">Refresh to check your current waitlist status.</p>
    <div className="mt-4"><RefreshTicket /></div>
    {tickets.length === 0 ? <div className="mt-8 rounded-2xl border border-wine/30 bg-white p-6">
      <p role="status">You do not have an active ticket.</p>
      <Link href="/join" className="mt-5 inline-block rounded-lg bg-wine px-5 py-3 font-semibold text-white">Join Waitlist</Link>
    </div> : tickets.map(ticket => <TicketCard key={ticket.id} ticket={ticket} showDetails />)}
  </MyTicketShell>;
}
