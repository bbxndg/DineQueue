'use client';

import { useState, useTransition } from 'react';
import { useRouter } from 'next/navigation';
import { useLiveRefresh } from '@/components/live-refresh';

export function RefreshTicket() {
  const router = useRouter();
  const live = useLiveRefresh();
  const [pending, startTransition] = useTransition();
  const [manual, setManual] = useState(false);
  const busy = pending || manual;

  async function refresh() {
    // Inside a polled view, share its single in-flight read instead of a second stream.
    if (!live) { startTransition(() => router.refresh()); return; }
    setManual(true);
    try { await live.refresh(); } finally { setManual(false); }
  }

  return <button disabled={busy} onClick={refresh}
    className="font-semibold text-wine underline disabled:opacity-60">
    {busy ? 'Refreshing…' : 'Refresh status'}
  </button>;
}
