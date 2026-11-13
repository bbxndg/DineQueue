'use client';

import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { useTransition } from 'react';

export default function TicketError({ reset }: { reset: () => void }) {
  const router = useRouter();
  const [pending, startTransition] = useTransition();
  return <main className="mx-auto max-w-lg px-6 py-20">
    <h1 className="text-2xl font-semibold">We cannot load your ticket right now</h1>
    <p role="alert" className="mt-4">Please check your connection and try again.</p>
    <button disabled={pending} onClick={() => startTransition(() => { router.refresh(); reset(); })}
      className="mt-6 rounded-lg bg-wine px-5 py-3 text-white disabled:opacity-60">{pending ? 'Retrying…' : 'Try again'}</button>
    <Link href="/" className="ml-6 font-semibold text-wine underline">Home</Link>
  </main>;
}
