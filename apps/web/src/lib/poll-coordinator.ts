// Framework-free refresh coordinator for Lab 7 polling (#44).
// Kept free of imports and non-erasable TypeScript so Node's test runner can
// load it directly. Mutations are never triggered from here: it only reads.

export type ReadOutcome<T> = { kind: 'ok'; data: T } | { kind: 'status'; status: number };
export type ReadFailure = { kind: 'network' | 'timeout' | 'server' | 'invalid'; status?: number };
export type AccessLoss = 401 | 403 | 404;

export type Timers = {
  set: (fn: () => void, ms: number) => unknown;
  clear: (handle: unknown) => void;
};

export type PollerOptions<T> = {
  /** Performs one read. Must honour the signal and throw on network failure. */
  read: (signal: AbortSignal) => Promise<ReadOutcome<T>>;
  onData: (data: T, at: number) => void;
  onFailure: (failure: ReadFailure) => void;
  /** 401/403/404: protected data must be dropped; polling stops permanently. */
  onAccessLost: (status: AccessLoss) => void;
  onRefreshingChange?: (refreshing: boolean) => void;
  isVisible: () => boolean;
  intervalMs?: number;
  timeoutMs?: number;
  timers?: Timers;
  now?: () => number;
};

export type Poller = {
  start: () => void;
  stop: () => void;
  /** Immediate read sharing the single in-flight slot (manual Refresh). */
  refresh: () => Promise<void>;
  /** Suspends and invalidates reads while fn runs, then refreshes once. */
  runMutation: <R>(fn: () => Promise<R>) => Promise<R>;
  visibilityChanged: () => void;
};

export const POLL_INTERVAL_MS = 5_000;
export const READ_TIMEOUT_MS = 10_000;

class TimeoutAbort extends Error {}

export function createPoller<T>(options: PollerOptions<T>): Poller {
  const interval = options.intervalMs ?? POLL_INTERVAL_MS;
  const timeout = options.timeoutMs ?? READ_TIMEOUT_MS;
  const timers: Timers = options.timers ?? {
    set: (fn, ms) => setTimeout(fn, ms),
    clear: handle => clearTimeout(handle as ReturnType<typeof setTimeout>),
  };
  const now = options.now ?? (() => Date.now());

  // Every read captures the generation it started in. Any later read, mutation,
  // hide or stop bumps it, so an older response can never be applied.
  let generation = 0;
  let inFlight: { gen: number; controller: AbortController; deadline: unknown; promise: Promise<void> } | null = null;
  let timer: unknown = null;
  let mutations = 0;
  let stopped = true;

  // Read through a function: control-flow narrowing inside the async read
  // would otherwise assume the slot is still empty.
  const currentGen = (): number | null => (inFlight ? inFlight.gen : null);
  const clearTimer = () => {
    if (timer !== null) timers.clear(timer);
    timer = null;
  };
  const invalidate = () => {
    generation += 1;
    if (inFlight) {
      timers.clear(inFlight.deadline);
      inFlight.controller.abort();
    }
    inFlight = null;
    options.onRefreshingChange?.(false);
  };
  // The next read is scheduled only after the previous one settles: no overlap
  // and no catch-up burst after slow responses or a hidden period.
  const schedule = () => {
    clearTimer();
    if (stopped || mutations > 0 || inFlight || !options.isVisible()) return;
    timer = timers.set(() => { timer = null; void read(); }, interval);
  };

  function read(): Promise<void> {
    if (stopped || mutations > 0 || !options.isVisible()) return Promise.resolve();
    if (inFlight) return inFlight.promise;
    clearTimer();
    const gen = ++generation;
    const controller = new AbortController();
    let timedOut = false;
    const deadline = timers.set(() => { timedOut = true; controller.abort(new TimeoutAbort()); }, timeout);
    options.onRefreshingChange?.(true);
    const promise = (async () => {
      try {
        const outcome = await options.read(controller.signal);
        if (gen !== generation || stopped) return;
        if (outcome.kind === 'ok') {
          options.onData(outcome.data, now());
        } else if (outcome.status === 401 || outcome.status === 403 || outcome.status === 404) {
          stopped = true;
          clearTimer();
          options.onAccessLost(outcome.status);
        } else {
          options.onFailure({ kind: 'server', status: outcome.status });
        }
      } catch (error) {
        if (gen !== generation || stopped) return;
        const invalid = error instanceof Error && error.name === 'InvalidResponse';
        options.onFailure({ kind: timedOut ? 'timeout' : invalid ? 'invalid' : 'network' });
      } finally {
        timers.clear(deadline);
        if (currentGen() === gen) {
          inFlight = null;
          options.onRefreshingChange?.(false);
        }
        if (gen === generation) schedule();
      }
    })();
    inFlight = { gen, controller, deadline, promise };
    return promise;
  }

  return {
    start() {
      stopped = false;
      schedule();
    },
    stop() {
      stopped = true;
      clearTimer();
      invalidate();
    },
    refresh: () => read(),
    async runMutation(fn) {
      mutations += 1;
      clearTimer();
      invalidate();
      try {
        return await fn();
      } finally {
        mutations -= 1;
        // Always refetch afterwards (success, conflict or failure): the
        // committed server state, not the pre-write snapshot, is authoritative.
        // Overlapping writes share one refetch, issued when the last settles;
        // a manual refresh during a write is not a mutation and is deferred to
        // that same read (see "overlapping mutations ..." unit test).
        if (mutations === 0 && !stopped) void read();
      }
    },
    visibilityChanged() {
      if (stopped) return;
      if (options.isVisible()) void read();
      else { clearTimer(); invalidate(); }
    },
  };
}
