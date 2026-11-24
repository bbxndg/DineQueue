import Link from 'next/link';
import { redirect } from 'next/navigation';
import { requireSession } from '@/lib/auth';
import { activeQueues } from '@/lib/queues';
import { JoinForm } from '@/components/join-form';
import { SignOut } from '@/components/sign-out';

export default async function JoinPage() {
  const session = await requireSession();
  if (session.user.role === 'HOST') redirect('/host/queue');
  const queues = await activeQueues();
  return (
    <main className="mx-auto flex min-h-screen max-w-5xl flex-col px-6 py-10 sm:px-12">
      <header className="flex items-center justify-between gap-4 border-b border-wine/20 pb-6">
        <Link href="/" className="text-xl font-semibold tracking-tight text-wine">DineQueue</Link>
        <SignOut csrfToken={session.csrfToken} />
      </header>
      <section className="min-w-0 max-w-xl py-14" aria-labelledby="join-heading">
        <p className="mb-3 text-sm font-semibold uppercase tracking-widest text-wine">Walk-in waitlist</p>
        <h1 id="join-heading" className="text-3xl font-semibold sm:text-4xl">Join the waitlist</h1>
        <p className="mt-4 leading-relaxed">Tell us how many people are dining. Your ticket number is assigned when you join. We do not promise a wait time.</p>
        {queues.length === 0
          ? <p role="status" className="mt-8 rounded-lg border border-wine/30 bg-white p-4">No queue is accepting walk-in entries right now. Please check with the host.</p>
          : <JoinForm queues={queues} csrfToken={session.csrfToken} />}
      </section>
    </main>
  );
}
