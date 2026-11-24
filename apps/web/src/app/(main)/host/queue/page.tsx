import Link from 'next/link';
import { redirect } from 'next/navigation';
import { requireSession } from '@/lib/auth';
import { assignedQueues, queueBoard } from '@/lib/host';
import { SignOut } from '@/components/sign-out';
import { HostBoard } from '@/components/host-board';

export default async function HostPage({ searchParams }: { searchParams: Promise<{ queue?: string | string[] }> }) {
  const session = await requireSession();
  if (session.user.role !== 'HOST') redirect('/');
  const queues = await assignedQueues();
  const requested = (await searchParams).queue;
  // Only a queue from the server-provided assigned list is ever requested.
  const queue = queues.find(value => value.id === requested) ?? queues[0];
  const board = queue ? await queueBoard(queue.id) : null;
  return <main className="mx-auto min-h-screen max-w-5xl px-6 py-10">
    <header className="flex items-center justify-between gap-4 border-b border-wine/20 pb-6">
      <p className="text-xl font-semibold text-wine">DineQueue · Host</p>
      <SignOut csrfToken={session.csrfToken} />
    </header>
    <section className="py-10">
      <p className="mb-3 text-sm font-semibold uppercase tracking-widest text-wine">Host account</p>
      <h1 className="text-3xl font-semibold">Welcome, {session.user.displayName}</h1>
      {!queue || !board ? <p role="status" className="mt-6 leading-relaxed">No queue is assigned to your account. Ask a manager for access.</p> : <>
        {queues.length > 1 && <nav aria-label="Assigned queues" className="mt-6 flex flex-wrap gap-4">
          {queues.map(value => <Link key={value.id} href={`/host/queue?queue=${encodeURIComponent(value.id)}`} prefetch={false}
            aria-current={value.id === queue.id ? 'page' : undefined}
            className={value.id === queue.id ? 'font-semibold text-wine underline' : 'text-wine'}>{value.name}</Link>)}
        </nav>}
        <h2 className="mt-6 text-2xl font-semibold">{queue.name} · {queue.restaurantName}</h2>
        {!queue.isActive && <p className="mt-2 text-sm">This queue is closed to new joins. Existing tickets can still be handled.</p>}
        <HostBoard key={queue.id} queueId={queue.id} timezone={queue.timezone} csrfToken={session.csrfToken}
          current={board.current} waiting={board.waiting} />
      </>}
    </section>
  </main>;
}
