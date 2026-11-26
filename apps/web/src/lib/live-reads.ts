// Client-side read-only fetchers for polling. Same-origin, cookie-authenticated,
// never cached. Server-only helpers (lib/tickets, lib/host) must not be imported
// into client components; this module carries no secrets or upstream URLs.
import type { ReadOutcome } from '@/lib/poll-coordinator';
import type { HostEntry, Ticket } from '@/lib/ticket-types';

class InvalidResponse extends Error {
  constructor() { super('Unexpected response shape'); this.name = 'InvalidResponse'; }
}

async function getJSON(path: string, signal: AbortSignal): Promise<ReadOutcome<unknown>> {
  const response = await fetch(path, {
    method: 'GET', credentials: 'same-origin', cache: 'no-store', signal,
    headers: { Accept: 'application/json' },
  });
  if (!response.ok) {
    // Drain without trusting the body; status alone drives handling.
    await response.body?.cancel().catch(() => undefined);
    return { kind: 'status', status: response.status };
  }
  return { kind: 'ok', data: await response.json() };
}

const isObject = (value: unknown): value is Record<string, unknown> => typeof value === 'object' && value !== null;
// A malformed body is a failure, never an empty list.
function entity<T>(value: unknown): T {
  if (!isObject(value) || typeof value.id !== 'string' || typeof value.status !== 'string' || typeof value.isReady !== 'boolean') {
    throw new InvalidResponse();
  }
  return value as T;
}
function entityList<T>(value: unknown): T[] {
  if (!isObject(value) || !Array.isArray(value.entries)) throw new InvalidResponse();
  return value.entries.map(item => entity<T>(item));
}

export async function readActiveTickets(signal: AbortSignal): Promise<ReadOutcome<Ticket[]>> {
  const result = await getJSON('/api/v1/me/queue-entries/active', signal);
  return result.kind === 'ok' ? { kind: 'ok', data: entityList<Ticket>(result.data) } : result;
}

export async function readTicket(id: string, signal: AbortSignal): Promise<ReadOutcome<Ticket>> {
  const result = await getJSON(`/api/v1/me/queue-entries/${encodeURIComponent(id)}`, signal);
  return result.kind === 'ok' ? { kind: 'ok', data: entity<Ticket>(result.data) } : result;
}

export type HostBoardState = { current: HostEntry | null; waiting: HostEntry[] };

// Waiting and current are two independent reads, not an atomic snapshot. A
// ticket seen in both (called between the reads) is shown only as CALLED.
export function reconcileBoard(waiting: HostEntry[], current: HostEntry | null): HostBoardState {
  if (!current) return { current, waiting };
  return { current, waiting: waiting.filter(entry => entry.id !== current.id) };
}

export async function readHostBoard(queueId: string, signal: AbortSignal): Promise<ReadOutcome<HostBoardState>> {
  const base = `/api/v1/host/queues/${encodeURIComponent(queueId)}`;
  const [waiting, current] = await Promise.all([
    getJSON(`${base}/entries?status=WAITING`, signal),
    getJSON(`${base}/current`, signal),
  ]);
  // Report the most serious access status first so protected state is cleared.
  for (const status of [401, 403, 404]) {
    if ((waiting.kind === 'status' && waiting.status === status) || (current.kind === 'status' && current.status === status)) {
      return { kind: 'status', status };
    }
  }
  if (waiting.kind === 'status') return waiting;
  if (current.kind === 'status') return current;
  if (!isObject(current.data) || !('entry' in current.data)) throw new InvalidResponse();
  const entry = current.data.entry === null ? null : entity<HostEntry>(current.data.entry);
  return { kind: 'ok', data: reconcileBoard(entityList<HostEntry>(waiting.data), entry) };
}
