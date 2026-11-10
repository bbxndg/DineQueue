import Link from 'next/link';
import { redirect } from 'next/navigation';
import { requireSession } from '@/lib/auth';
import { ticketDetail } from '@/lib/tickets';
import { TicketCard } from '@/components/ticket-card';
import { MyTicketShell } from '@/components/my-ticket-shell';
import { RefreshTicket } from '@/components/refresh-ticket';

export default async function TicketDetailPage({ params }: { params: Promise<{ entryId: string }> }) {
  const session = await requireSession();
  if (session.user.role !== 'CUSTOMER') redirect('/host/queue');
  const { entryId } = await params;
  const ticket = await ticketDetail(entryId);
  return <MyTicketShell csrfToken={session.csrfToken}>
    <h1 className="text-3xl font-semibold sm:text-4xl">Ticket details</h1>
    <div className="mt-4 flex flex-wrap gap-5 font-semibold text-wine underline">
      <Link href="/my-queue" prefetch={false}>My Ticket</Link>
      <RefreshTicket />
    </div>
    <TicketCard ticket={ticket} />
  </MyTicketShell>;
}
