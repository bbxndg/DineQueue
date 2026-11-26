'use client';

import { useCallback, useEffect, useRef, useState } from 'react';
import { createPoller, type AccessLoss, type Poller, type ReadFailure, type ReadOutcome } from '@/lib/poll-coordinator';

export type LiveState<T> = {
  data: T;
  /** Epoch ms of the last successful read; null until the first client read. */
  updatedAt: number | null;
  failure: ReadFailure | null;
  accessLost: AccessLoss | null;
  refreshing: boolean;
};

export type LiveResource<T> = LiveState<T> & {
  refresh: () => Promise<void>;
  runMutation: <R>(fn: () => Promise<R>) => Promise<R>;
};

/**
 * Keeps server-rendered data current while the page is visible.
 * One poller per mounted view; it is stopped (timers cleared, in-flight read
 * aborted and invalidated) on unmount or when `key` changes.
 */
export function useLiveResource<T>(initial: T, key: string, read: (signal: AbortSignal) => Promise<ReadOutcome<T>>): LiveResource<T> {
  const [state, setState] = useState<LiveState<T>>({ data: initial, updatedAt: null, failure: null, accessLost: null, refreshing: false });
  const readRef = useRef(read);
  useEffect(() => { readRef.current = read; }, [read]);
  const pollerRef = useRef<Poller | null>(null);

  useEffect(() => {
    const visible = () => document.visibilityState === 'visible';
    const poller = createPoller<T>({
      read: signal => readRef.current(signal),
      isVisible: visible,
      onData: (data, at) => setState(s => ({ ...s, data, updatedAt: at, failure: null })),
      onFailure: failure => setState(s => ({ ...s, failure })),
      onAccessLost: status => setState(s => ({ ...s, accessLost: status })),
      onRefreshingChange: refreshing => setState(s => (s.refreshing === refreshing ? s : { ...s, refreshing })),
    });
    pollerRef.current = poller;
    const onVisibility = () => poller.visibilityChanged();
    document.addEventListener('visibilitychange', onVisibility);
    poller.start();
    return () => {
      document.removeEventListener('visibilitychange', onVisibility);
      poller.stop();
      if (pollerRef.current === poller) pollerRef.current = null;
    };
  }, [key]);

  const refresh = useCallback(() => pollerRef.current?.refresh() ?? Promise.resolve(), []);
  const runMutation = useCallback(<R,>(fn: () => Promise<R>) => pollerRef.current ? pollerRef.current.runMutation(fn) : fn(), []);
  return { ...state, refresh, runMutation };
}
