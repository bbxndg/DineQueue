'use client';

import { useTransition } from 'react';
import { useRouter } from 'next/navigation';

export function RefreshTicket() {
  const router = useRouter();
  const [pending, startTransition] = useTransition();
  return <button disabled={pending} onClick={() => startTransition(() => router.refresh())}
    className="font-semibold text-wine underline disabled:opacity-60">
    {pending ? 'Refreshing…' : 'Refresh status'}
  </button>;
}
