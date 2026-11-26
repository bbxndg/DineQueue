import { redirect } from 'next/navigation';
import { requireSession } from '@/lib/auth';
import { ticketDetail } from '@/lib/tickets';
import { MyTicketShell } from '@/components/my-ticket-shell';
import { LiveTicketDetail } from '@/components/live-tickets';

// Issue #41: keep this route outside every loading.tsx/Suspense boundary (see the (main) and
// my-queue/(list) route groups). Streaming would commit HTTP 200 before notFound() runs, so a
// foreign, missing or malformed ticket id must be rejected before the response starts.
// Lab 7: the owned ticket is read and authorized here first; only then does the client
// component take over polling, so the initial HTTP status is unchanged.
export default async function TicketDetailPage({ params }: { params: Promise<{ entryId: string }> }) {
  const session = await requireSession();
  if (session.user.role !== 'CUSTOMER') redirect('/host/queue');
  const { entryId } = await params;
  const ticket = await ticketDetail(entryId);
  return <MyTicketShell csrfToken={session.csrfToken}>
    <h1 className="text-3xl font-semibold sm:text-4xl">Ticket details</h1>
    <LiveTicketDetail key={ticket.id} initial={ticket} csrfToken={session.csrfToken} renderedAt={new Date().toISOString()} />
  </MyTicketShell>;
}
