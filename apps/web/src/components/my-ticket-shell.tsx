import type { ReactNode } from 'react';
import Link from 'next/link';
import { SignOut } from '@/components/sign-out';

export function MyTicketShell({ csrfToken, children }: { csrfToken: string; children: ReactNode }) {
  return <main className="mx-auto min-h-screen max-w-5xl px-6 py-10 sm:px-12">
    <header className="flex items-center justify-between gap-4 border-b border-wine/20 pb-6">
      <Link href="/" className="text-xl font-semibold tracking-tight text-wine">DineQueue</Link>
      <SignOut csrfToken={csrfToken} />
    </header>
    <section className="max-w-xl py-14">{children}</section>
  </main>;
}
