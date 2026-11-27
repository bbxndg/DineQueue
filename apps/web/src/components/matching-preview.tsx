'use client';
import { useEffect, useState } from 'react';
import { useRouter } from 'next/navigation';
import type { FloorSnapshot, FloorTable } from '@/lib/floor';

type Preview = { layoutVersion: number; entry: { ticketCode: string; partySize: number } | null; candidates: { tableIds: string[]; totalSeats: number; selectionReason: string }[]; skipped: { ticketCode: string; reasonCode: string }[]; availabilityReason: string | null };
export function MatchingPreview({ queueId, floor, tables, stale }: { queueId: string; floor: FloorSnapshot; tables: FloorTable[]; stale: boolean }) {
  const router = useRouter();
  const [outcome, setOutcome] = useState<{ key: string; data?: Preview; error?: string } | null>(null);
  const selection = tables.map(t => `${t.id}:${t.version}`).join(',');
  const requestKey = `${queueId}:${floor.serverNow}:${floor.layoutVersion}:${selection}`;
  const result = outcome?.key === requestKey ? outcome.data : null;
  const failure = outcome?.key === requestKey ? outcome.error : '';
  const pending = tables.length > 0 && !stale && outcome?.key !== requestKey;
  // Each authoritative floor refresh invalidates the advisory snapshot as parties
  // may have changed even when table versions are identical.
  useEffect(() => {
    if (!selection || stale) return;
    const abort = new AbortController();
    const query = new URLSearchParams(); selection.split(',').forEach(t => query.append('tableId', t.split(':')[0]));
    void fetch(`/api/v1/host/queues/${encodeURIComponent(queueId)}/matching?${query}`, { cache: 'no-store', credentials: 'same-origin', signal: abort.signal })
      .then(async response => {
        if (response.status === 401) { router.replace('/login'); throw new Error('Your session has ended.'); }
        if (!response.ok) throw new Error(response.status === 403 ? 'Access to matching was denied.' : 'The selected tables cannot be matched. Check availability and approved connections.');
        const value: Preview = await response.json();
        if (!Array.isArray(value.candidates) || !Array.isArray(value.skipped) || value.layoutVersion !== floor.layoutVersion) throw new Error('Floor changed. Refresh the selection.');
        if (!abort.signal.aborted) setOutcome({ key: requestKey, data: value });
      }).catch(error => { if (!abort.signal.aborted) setOutcome({ key: requestKey, error: error instanceof Error ? error.message : 'Preview could not be loaded.' }); });
    return () => abort.abort();
  // table key is stable across renders, while floor changes after settled polling.
  }, [queueId, floor.layoutVersion, requestKey, selection, stale, router]);
  return <aside aria-label="Matching preview" className="mt-4 grid gap-2" aria-live="polite">
    <h3 className="font-semibold">Matching preview</h3>
    {stale ? <p>Refresh the floor before requesting a match.</p> : pending ? <p>Finding the oldest fitting ready party…</p> : failure ? <p role="alert">{failure}</p> : result ? <>
      {result.availabilityReason ? <p>{result.availabilityReason.toLowerCase().replaceAll('_', ' ')}.</p> : result.entry && <p><strong>{result.entry.ticketCode}</strong> · {result.entry.partySize} people · Ready</p>}
      {result.candidates.map(c => <p key={c.tableIds.join(':')}>{c.totalSeats} seats · {c.selectionReason === 'RECOMMENDED' ? 'Recommended set' : 'Alternative valid set'}</p>)}
      {result.skipped.map(s => <p key={s.ticketCode}>{s.ticketCode}: {s.reasonCode.toLowerCase().replaceAll('_', ' ')}</p>)}
      <p className="text-sm">Preview only. Tables are not held.</p>
    </> : <p>Select available tables to preview a match.</p>}
  </aside>;
}
