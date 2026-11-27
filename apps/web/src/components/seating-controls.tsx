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
 const [confirming, setConfirming] = useState<'seat' | 'no-show' | Turnover | null>(null);
 const [now, setNow] = useState(() => Date.parse(floor.serverNow));
 useEffect(() => {
   if (a.status !== 'HELD') return;
   const started = Date.now(); const server = Date.parse(floor.serverNow);
   let timer: ReturnType<typeof setInterval> | undefined;
   const tick = () => setNow(server + Date.now() - started);
   const visibility = () => {
     clearInterval(timer); timer = undefined;
     if (document.visibilityState === 'visible') { tick(); timer = setInterval(tick, 1000); }
   };
   // Initialize through the same external visibility event path.
   const initial = setTimeout(visibility, 0);
   document.addEventListener('visibilitychange', visibility);
   return () => { clearTimeout(initial); clearInterval(timer); document.removeEventListener('visibilitychange', visibility); };
 }, [floor.serverNow, a.status]);
 const expired = now >= Date.parse(a.holdExpiresAt);
 const names = a.tableIds.map(id => floor.zones.flatMap(z => z.tables).find(t => t.id === id)?.code).join(' + ');
 return <article className="grid gap-3 rounded-xl border border-wine/30 p-4">
   <h3 className="font-semibold">{names} · {stateLabel(a)}</h3>
   {a.ticketCode && <p className="text-sm">Ticket {a.ticketCode}</p>}
   {a.status === 'HELD' && <>
     <p>Hold deadline: {new Date(a.holdExpiresAt).toLocaleString()} · {Math.max(0, Math.ceil((Date.parse(a.holdExpiresAt) - now) / 1000))} seconds remaining</p>
     {expired ? <><p>The hold deadline has passed. Release the expired hold to update the ticket and tables.</p><button className={button} disabled={disabled} onClick={() => mutate('/expire-holds', { idempotencyKey: crypto.randomUUID() })}>Release expired holds</button></> : confirming === 'seat' ? <div role="group" aria-label="Confirm seating">
       <p>Confirm the party has arrived and seat them at all selected tables?</p>
       <button className={button} disabled={disabled} onClick={() => mutate(`/seating-assignments/${a.id}/seat`, { expectedVersion: a.version, idempotencyKey: crypto.randomUUID() })}>Confirm Seat</button>
       <button className={button} disabled={disabled} onClick={() => setConfirming(null)}>Cancel</button>
     </div> : confirming === 'no-show' ? <div role="group" aria-label="Confirm no-show">
       <p>Mark ticket <strong>{a.ticketCode}</strong> as not present? The ticket becomes No-show and every held table ({names}) is released. This cannot be undone.</p>
       <button className={button} disabled={disabled} onClick={() => mutate(`/entries/${encodeURIComponent(a.entryId)}/no-show`, { expectedVersion: a.version, reasonCode: 'CUSTOMER_ABSENT', idempotencyKey: crypto.randomUUID() })}>Confirm No-show</button>
       <button className={button} disabled={disabled} onClick={() => setConfirming(null)}>Keep hold</button>
     </div> : <div className="flex flex-wrap gap-3">
       <button className={button} disabled={disabled} onClick={() => setConfirming('seat')}>Seat</button>
       <button className={button} disabled={disabled} onClick={() => setConfirming('no-show')}>No-show</button>
     </div>}
   </>}
   {turnoverStep(a) && <TurnoverAction step={turnoverStep(a)!} assignment={a} names={names} confirming={confirming === turnoverStep(a)} setConfirming={setConfirming} disabled={disabled} mutate={mutate} />}
 </article>;
}
type Turnover = 'checkout' | 'start-cleaning' | 'finish-cleaning';
function stateLabel(a: Assignment) {
 if (a.status === 'HELD') return 'Held';
 if (a.status === 'ACTIVE') return 'Occupied';
 if (a.turnoverPhase === 'DIRTY') return 'Checked out · Dirty';
 if (a.turnoverPhase === 'CLEANING') return 'Cleaning';
 return a.status.toLowerCase();
}
// The only valid next whole-group step; unsafe shortcuts are never offered.
function turnoverStep(a: Assignment): Turnover | null {
 if (a.status === 'ACTIVE') return 'checkout';
 if (a.status === 'CLOSED' && a.turnoverPhase === 'DIRTY') return 'start-cleaning';
 if (a.status === 'CLOSED' && a.turnoverPhase === 'CLEANING') return 'finish-cleaning';
 return null;
}
const turnoverText: Record<Turnover, { button: string; confirm: string; question: (names: string, code: string) => string }> = {
 'checkout': { button: 'Checkout', confirm: 'Confirm Checkout', question: (n, c) => `Ticket ${c} has left? Every table in this group (${n}) becomes Dirty and stays unavailable until cleaning finishes.` },
 'start-cleaning': { button: 'Start Cleaning', confirm: 'Confirm Start Cleaning', question: n => `Start cleaning every table in this group (${n})? They stay unavailable while cleaning.` },
 'finish-cleaning': { button: 'Finish Cleaning', confirm: 'Confirm Finish Cleaning', question: n => `Finish cleaning and make every table in this group (${n}) Available for new parties?` },
};
function TurnoverAction({ step, assignment: a, names, confirming, setConfirming, disabled, mutate }: { step: Turnover; assignment: Assignment; names: string; confirming: boolean; setConfirming: (v: Turnover | null) => void; disabled: boolean; mutate: SeatingMutation }) {
 const text = turnoverText[step];
 if (!confirming) return <div><button className={button} disabled={disabled} onClick={() => setConfirming(step)}>{text.button}</button></div>;
 return <div role="group" aria-label={text.confirm} className="grid gap-3">
   <p>{text.question(names, a.ticketCode)}</p>
   <div className="flex flex-wrap gap-3">
     <button className={button} disabled={disabled} onClick={() => mutate(`/seating-assignments/${a.id}/${step}`, { expectedVersion: a.version, idempotencyKey: crypto.randomUUID() })}>{text.confirm}</button>
     <button className={button} disabled={disabled} onClick={() => setConfirming(null)}>Keep current state</button>
   </div>
 </div>;
}
export function SeatingAssignments({ floor, disabled, mutate }: { floor: FloorSnapshot; disabled: boolean; mutate: SeatingMutation }) {
 return <section aria-label="Current seating assignments" className="grid gap-4"><h2 className="text-2xl font-semibold">Current seating assignments</h2>{floor.assignments.length ? floor.assignments.map(a => <HeldAssignment key={`${a.id}:${a.version}`} assignment={a} floor={floor} disabled={disabled} mutate={mutate} />) : <p>No current seating assignments.</p>}</section>;
}
