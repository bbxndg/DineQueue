import Link from 'next/link';
import { redirect } from 'next/navigation';
import { requireSession } from '@/lib/auth';
import { activeTickets } from '@/lib/tickets';
import { MyTicketShell } from '@/components/my-ticket-shell';
import { LiveMyQueue } from '@/components/live-tickets';

export default async function MyQueuePage() {
  const session = await requireSession();
  if (session.user.role !== 'CUSTOMER') redirect('/host/queue');
  const tickets = await activeTickets();
  return <MyTicketShell csrfToken={session.csrfToken}>
    <h1 className="text-3xl font-semibold sm:text-4xl">My Ticket</h1>
    <p className="mt-4">Your waitlist status updates on this page automatically.</p>
    <Link href="/my-queue/history" prefetch={false} className="mt-3 inline-block min-h-11 py-2 font-semibold text-wine underline">Ticket history</Link>
    <LiveMyQueue initial={tickets} csrfToken={session.csrfToken} renderedAt={new Date().toISOString()} />
  </MyTicketShell>;
}
