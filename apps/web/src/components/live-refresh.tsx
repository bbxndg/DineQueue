'use client';

import { createContext, useContext, useSyncExternalStore } from 'react';
import type { ReadFailure } from '@/lib/poll-coordinator';

/** Shared by Refresh buttons and readiness controls inside a polled view. */
export type LiveRefresh = {
  refresh: () => Promise<void>;
  runMutation: <R>(fn: () => Promise<R>) => Promise<R>;
};

export const LiveRefreshContext = createContext<LiveRefresh | null>(null);
export const useLiveRefresh = () => useContext(LiveRefreshContext);

const noopSubscribe = () => () => {};

function formatTime(at: number) {
  return new Intl.DateTimeFormat('en-GB', { timeStyle: 'medium' }).format(new Date(at));
}

/**
 * Last successful update and a stale notice. The server render time seeds the
 * label so the first paint already says how fresh the data is.
 */
export function LiveStatus({ updatedAt, renderedAt, failure }: {
  updatedAt: number | null; renderedAt: string; failure: ReadFailure | null;
}) {
  // Rendered only on the client so server and client markup cannot disagree on
  // the viewer's local time.
  const mounted = useSyncExternalStore(noopSubscribe, () => true, () => false);
  const last = updatedAt ?? Date.parse(renderedAt);
  return <div className="mt-3 grid gap-2 text-sm">
    <p data-testid="live-updated">Updates automatically every 5 seconds while this page is open.
      {/* Plain text: ticket views reserve <time> for ticket timestamps. */}
      {mounted && <> Last updated <span data-testid="live-updated-at">{formatTime(last)}</span>.</>}</p>
    {failure && <p role="alert" data-testid="live-stale" className="rounded-lg border border-wine/30 bg-white p-3 text-wine">
      Unable to refresh right now. Showing the last successful update; retrying automatically.
    </p>}
  </div>;
}
