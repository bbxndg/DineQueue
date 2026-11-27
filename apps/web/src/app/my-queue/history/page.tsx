import Link from 'next/link';
import { redirect } from 'next/navigation';
import { requireSession } from '@/lib/auth';
import { MyTicketShell } from '@/components/my-ticket-shell';
import { TicketHistory } from '@/components/ticket-history';

// Static segment: Next resolves /my-queue/history before the dynamic [entryId] route.
export default async function TicketHistoryPage() {
  const session = await requireSession();
  if (session.user.role !== 'CUSTOMER') redirect('/host/queue');
  return <MyTicketShell csrfToken={session.csrfToken}>
    <h1 className="text-3xl font-semibold sm:text-4xl">Ticket history</h1>
    <p className="mt-4">Your past tickets, newest first.</p>
    <Link href="/my-queue" prefetch={false} className="mt-4 inline-block min-h-11 py-2 font-semibold text-wine underline">Back to My Ticket</Link>
    {/* Keyed by session: a different session remounts and discards earlier reads. */}
    <TicketHistory key={session.csrfToken} />
  </MyTicketShell>;
}
