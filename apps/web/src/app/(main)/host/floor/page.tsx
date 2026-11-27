import Link from 'next/link';
import { redirect } from 'next/navigation';
import { requireSession, backendGet } from '@/lib/auth';
import { assignedQueues } from '@/lib/host';
import { parseFloor, type FloorSnapshot } from '@/lib/floor';
import { FloorBoard } from '@/components/floor-board';
import { SignOut } from '@/components/sign-out';

export default async function FloorPage({ searchParams }: { searchParams: Promise<{ queueId?: string | string[] }> }) {
  const session = await requireSession();
  if (session.user.role !== 'HOST') redirect('/');
  const queues = await assignedQueues();
  const requested = (await searchParams).queueId;
  const queue = requested === undefined ? queues[0] : queues.find(q => q.id === requested);
  let initial: FloorSnapshot | null = null;
  let denied = false;
  if (queue) {
    let response: Response | null = null;
    try { response = await backendGet(`/api/v1/host/queues/${encodeURIComponent(queue.id)}/floor`); }
    catch { /* Keep the page usable when only the floor read fails. */ }
    if (response?.status === 401) redirect('/login');
    if (response?.status === 403 || response?.status === 404) denied = true;
    else if (response?.ok) {
      try { initial = parseFloor(await response.json(), queue.id); } catch { /* Client coordinator retries a failed initial read. */ }
    }
  }
  return <main className="mx-auto min-h-screen max-w-5xl px-4 py-8 sm:px-6">
    <header className="flex items-center justify-between gap-4 border-b border-wine/20 pb-6"><p className="text-xl font-semibold text-wine">DineQueue · Host</p><SignOut csrfToken={session.csrfToken} /></header>
    <h1 className="mt-8 text-3xl font-semibold">Restaurant floor</h1>
    <nav aria-label="Host navigation" className="mt-4 flex flex-wrap gap-4">
      <Link href={queue ? `/host/queue?queue=${encodeURIComponent(queue.id)}` : '/host/queue'} prefetch={false} className="min-h-11 py-2 text-wine underline">Queue board</Link>
      {queues.length > 1 && queues.map(q => <Link key={q.id} href={`/host/floor?queueId=${encodeURIComponent(q.id)}`} prefetch={false} aria-current={q.id === queue?.id ? 'page' : undefined} className="min-h-11 py-2 text-wine underline">{q.name}</Link>)}
    </nav>
    {!queue || denied ? <p role="alert" className="mt-6">{requested ? 'This queue is not available to your account.' : 'No queue is assigned to your account.'}</p> : <>
      <p className="mt-4 text-lg">{queue.restaurantName} · {queue.name}</p>
      <FloorBoard key={queue.id} queueId={queue.id} initial={initial} renderedAt={new Date().toISOString()} />
    </>}
  </main>;
}
