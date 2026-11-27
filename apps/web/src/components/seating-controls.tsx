'use client';
import { useEffect, useState } from 'react';
import type { Assignment, FloorSnapshot } from '@/lib/floor';
export type SeatingMutation = (path: string, body: object) => Promise<void>;
export type CallPreview = { entry: { id: string; ticketCode: string; partySize: number } | null; entryVersion: number | null; layoutVersion: number; candidates: { tableIds: string[]; tableVersions: { tableId: string; version: number }[]; totalSeats: number; selectionReason: string }[]; availabilityReason: string | null };
const button = 'min-h-11 rounded-lg border border-wine px-4 py-2 font-semibold text-wine disabled:opacity-50';
export function CallHoldControl({ preview, floor, disabled, mutate }: { preview: CallPreview; floor: FloorSnapshot; disabled: boolean; mutate: SeatingMutation }) {
  const [confirming, setConfirming] = useState(false);
  const candidate = preview.candidates[0];
  if (floor.operationsMode !== 'SEATING' || !preview.entry || !candidate || preview.availabilityReason) return null;
  const tables = candidate.tableIds.map(id => floor.zones.flatMap(z => z.tables).find(t => t.id === id));
  const zone = floor.zones.find(z => z.tables.some(t => t.id === candidate.tableIds[0]));
  return <div>
    {!confirming ? <button className={button} disabled={disabled} onClick={() => setConfirming(true)}>Call &amp; Hold</button> : <div role="group" aria-label="Confirm call and hold" className="grid gap-3 rounded-lg border border-wine/30 p-4">
      <p>Call <strong>{preview.entry.ticketCode}</strong> for {preview.entry.partySize} people?</p>
      <p>{zone?.name}: {tables.map(t => `${t?.code} (${t?.capacity} seats)`).join(' + ')}. Hold for {floor.holdTtlSeconds / 60} minutes.</p>
      <p>Confirm only when you are ready to call this party.</p>
      <button className={button} disabled={disabled} onClick={() => mutate('/match-and-call', { entryId: preview.entry!.id, tableIds: candidate.tableIds, expectedLayoutVersion: preview.layoutVersion, expectedEntryVersion: preview.entryVersion, expectedTableVersions: candidate.tableVersions, selectionReason: candidate.selectionReason, idempotencyKey: crypto.randomUUID() })}>Confirm Call &amp; Hold</button>
      <button className={button} disabled={disabled} onClick={() => setConfirming(false)}>Cancel</button>
    </div>}
  </div>;
}
function HeldAssignment({ assignment: a, floor, disabled, mutate }: { assignment: Assignment; floor: FloorSnapshot; disabled: boolean; mutate: SeatingMutation }) {
 const [confirming, setConfirming] = useState(false);
 const [now, setNow] = useState(() => Date.parse(floor.serverNow));
 useEffect(() => { const started = Date.now(); const server = Date.parse(floor.serverNow);const timer = setInterval(() => setNow(server + Date.now() - started), 1000);return () => clearInterval(timer); }, [floor.serverNow]);
 const expired = now >= Date.parse(a.holdExpiresAt);
 const names = a.tableIds.map(id => floor.zones.flatMap(z => z.tables).find(t => t.id === id)?.code).join(' + ');
 return <article className="grid gap-3 rounded-xl border border-wine/30 p-4">
   <h3 className="font-semibold">{names} · {a.status === 'HELD' ? 'Held' : 'Occupied'}</h3>
   {a.status === 'HELD' && <>
     <p>Hold deadline: {new Date(a.holdExpiresAt).toLocaleString()} · {Math.max(0, Math.ceil((Date.parse(a.holdExpiresAt) - now) / 1000))} seconds remaining</p>
     {expired ? <><p>The hold deadline has passed. Release the expired hold to update the ticket and tables.</p><button className={button} disabled={disabled} onClick={() => mutate('/expire-holds', { idempotencyKey: crypto.randomUUID() })}>Release expired holds</button></> : confirming ? <div role="group" aria-label="Confirm seating">
       <p>Confirm the party has arrived and seat them at all selected tables?</p>
       <button className={button} disabled={disabled} onClick={() => mutate(`/seating-assignments/${a.id}/seat`, { expectedVersion: a.version, idempotencyKey: crypto.randomUUID() })}>Confirm Seat</button>
       <button className={button} disabled={disabled} onClick={() => setConfirming(false)}>Cancel</button>
     </div> : <button className={button} disabled={disabled} onClick={() => setConfirming(true)}>Seat</button>}
   </>}
 </article>;
}
export function SeatingAssignments({ floor, disabled, mutate }: { floor: FloorSnapshot; disabled: boolean; mutate: SeatingMutation }) {
 return <section aria-label="Current seating assignments" className="grid gap-4"><h2 className="text-2xl font-semibold">Current seating assignments</h2>{floor.assignments.length ? floor.assignments.map(a => <HeldAssignment key={`${a.id}:${a.version}`} assignment={a} floor={floor} disabled={disabled} mutate={mutate} />) : <p>No current seating assignments.</p>}</section>;
}
