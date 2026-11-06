import { redirect } from 'next/navigation';
import { requireSession } from '@/lib/auth';
import { SignOut } from '@/components/sign-out';

export default async function Home() {
  const session = await requireSession();
  if (session.user.role === 'HOST') redirect('/host/queue');
  return (
    <main className="mx-auto flex min-h-screen max-w-5xl flex-col px-6 py-10 sm:px-12">
      <header className="border-b border-wine/20 pb-6 text-xl font-semibold tracking-tight text-wine">
        <div className="flex items-center justify-between gap-4"><span>DineQueue</span><SignOut csrfToken={session.csrfToken} /></div>
      </header>
      <section className="my-auto max-w-2xl py-20" aria-labelledby="welcome">
        <p className="mb-5 text-sm font-semibold uppercase tracking-widest text-wine">Customer account</p>
        <h1 id="welcome" className="text-4xl leading-tight font-semibold tracking-tight sm:text-6xl">
          Welcome, {session.user.displayName}
        </h1>
        <p className="mt-8 max-w-lg text-lg leading-relaxed">
          You are signed in. Online queue entry is being prepared. Please check with your host for queue information.
        </p>
      </section>
      <footer className="border-t border-wine/20 pt-6 text-sm">Walk-in dining, together.</footer>
    </main>
  );
}
