import { redirect } from 'next/navigation';
import { requireSession } from '@/lib/auth';
import { SignOut } from '@/components/sign-out';

export default async function HostPage() {
  const session = await requireSession();
  if (session.user.role !== 'HOST') redirect('/');
  return <main className="mx-auto min-h-screen max-w-5xl px-6 py-10">
    <header className="flex items-center justify-between gap-4 border-b border-wine/20 pb-6">
      <p className="text-xl font-semibold text-wine">DineQueue · Host</p>
      <SignOut csrfToken={session.csrfToken} />
    </header>
    <section className="py-16">
      <p className="mb-3 text-sm font-semibold uppercase tracking-widest text-wine">Host account</p>
      <h1 className="text-3xl font-semibold">Welcome, {session.user.displayName}</h1>
      <p className="mt-6 leading-relaxed">You are signed in. Queue operations will be available when the waitlist opens.</p>
    </section>
  </main>;
}
