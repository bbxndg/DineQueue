import Link from 'next/link';
import { redirect } from 'next/navigation';
import { requireSession } from '@/lib/auth';
import { SignOut } from '@/components/sign-out';
import { activeTickets } from '@/lib/tickets';
import { activeQueues } from '@/lib/queues';

export default async function Home() {
  const session = await requireSession();
  if (session.user.role === 'HOST') redirect('/host/queue');
  const tickets = await activeTickets();
  const hasActiveTicket = tickets.length > 0;
  const queues = hasActiveTicket
    ? tickets.map(ticket => ({ id: ticket.queueId, name: ticket.queueName, restaurantName: ticket.restaurantName }))
    : await activeQueues();
  return (
    <main className="mx-auto flex min-h-screen max-w-5xl flex-col px-6 py-10 sm:px-12">
      <header className="border-b border-wine/20 pb-6 text-xl font-semibold tracking-tight text-wine">
        <div className="flex items-center justify-between gap-4">
          <div className="min-w-0">
            <span>DineQueue</span>
            <div data-testid="home-queue-context" className="mt-2 text-sm font-normal leading-relaxed">
              {queues.length > 0
                ? queues.map(queue => <p key={queue.id}>{queue.restaurantName} · {queue.name}</p>)
                : <p>No queue is accepting new parties right now.</p>}
            </div>
          </div>
          <div className="flex shrink-0 flex-col items-end gap-2 text-sm">
            <span>{session.user.displayName}</span>
            <nav aria-label="Customer account" className="flex flex-col items-end gap-2">
              <Link href="/my-queue" prefetch={false} className="font-semibold underline">My Queue</Link>
              <SignOut csrfToken={session.csrfToken} />
            </nav>
          </div>
        </div>
      </header>
      <section className="my-auto max-w-2xl py-20" aria-labelledby="welcome">
        <p className="mb-5 text-sm font-semibold uppercase tracking-widest text-wine">Customer account</p>
        <h1 id="welcome" className="text-4xl leading-tight font-semibold tracking-tight sm:text-6xl">
          Welcome, {session.user.displayName}
        </h1>
        <p className="mt-8 max-w-lg text-lg leading-relaxed">
          Join the walk-in waitlist and the host will call your ticket number.
        </p>
        <Link href={hasActiveTicket ? '/my-queue' : '/join'} prefetch={false}
          className="mt-8 inline-block rounded-lg bg-wine px-6 py-3 font-semibold text-white">
          {hasActiveTicket ? 'View My Queue' : 'Join Waitlist'}
        </Link>
        <Link href="/my-queue" prefetch={false} className="mt-4 ml-6 inline-block font-semibold text-wine underline">My Ticket</Link>
      </section>
      <footer className="border-t border-wine/20 pt-6 text-sm">Walk-in dining, together.</footer>
    </main>
  );
}
