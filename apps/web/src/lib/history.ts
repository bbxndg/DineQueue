// Client-side reader for Customer Ticket History (S3 #56). Same-origin,
// cookie-authenticated, never cached. Carries no secrets or upstream URLs.
import type { Ticket } from '@/lib/ticket-types';

export type HistoryPage = { tickets: Ticket[]; nextCursor: string | null };
export type HistoryOutcome = { kind: 'ok'; page: HistoryPage } | { kind: 'status'; status: number };

const terminal = new Set(['COMPLETED', 'CANCELLED', 'NO_SHOW']);
const isObject = (v: unknown): v is Record<string, unknown> => typeof v === 'object' && v !== null;

/** A malformed body is a failure, never an empty history. */
export function parseHistoryPage(value: unknown): HistoryPage {
  if (!isObject(value) || !Array.isArray(value.tickets) || !(value.nextCursor === null || typeof value.nextCursor === 'string')) {
    throw new Error('Unexpected history response');
  }
  for (const t of value.tickets) {
    if (!isObject(t) || typeof t.id !== 'string' || typeof t.ticketCode !== 'string' || !terminal.has(String(t.status))
      || typeof t.joinedAt !== 'string' || typeof t.serviceDate !== 'string' || typeof t.timezone !== 'string') {
      throw new Error('Unexpected history response');
    }
  }
  return { tickets: value.tickets as Ticket[], nextCursor: value.nextCursor };
}

export async function readHistoryPage(cursor: string | null, signal: AbortSignal): Promise<HistoryOutcome> {
  const query = cursor ? `?cursor=${encodeURIComponent(cursor)}` : '';
  const response = await fetch(`/api/v1/me/queue-entries/history${query}`, {
    method: 'GET', credentials: 'same-origin', cache: 'no-store', signal, headers: { Accept: 'application/json' },
  });
  if (!response.ok) {
    await response.body?.cancel().catch(() => undefined);
    return { kind: 'status', status: response.status };
  }
  return { kind: 'ok', page: parseHistoryPage(await response.json()) };
}
