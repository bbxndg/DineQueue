'use client';

import { useCallback, useEffect, useState } from 'react';
import { useRouter } from 'next/navigation';
import { LiveStatus } from '@/components/live-refresh';
import { readFloor, tableStates, type FloorSnapshot, type FloorTable } from '@/lib/floor';
import { useLiveResource } from '@/lib/use-live-resource';

const label = (status: string) => status.toLowerCase().replaceAll('_', ' ');

export function FloorBoard({ queueId, initial, renderedAt }: { queueId: string; initial: FloorSnapshot | null; renderedAt: string }) {
  const router = useRouter();
  const read = useCallback((signal: AbortSignal) => readFloor(queueId, signal), [queueId]);
  const live = useLiveResource<FloorSnapshot | null>(initial, queueId, read);
  const [zoneId, setZoneId] = useState('');
  const [view, setView] = useState<'map' | 'list'>('map');
  const [selected, setSelected] = useState<{ id: string; version: number }[]>([]);
  const [notice, setNotice] = useState('');
  const [manual, setManual] = useState(false);
  useEffect(() => {
    if (live.accessLost === 401) router.replace('/login');
  }, [live.accessLost, router]);
  const floor = live.data;
  const zone = floor?.zones.find(z => z.id === zoneId) ?? floor?.zones[0];
  // A changed snapshot cannot retain an unavailable/deactivated selection.
  const chosen = zone?.isActive ? zone.tables.filter(t => selected.some(s => s.id === t.id && s.version === t.version) && t.status === 'AVAILABLE') : [];
  const selectionChanged = chosen.length !== selected.length;

  function select(table: FloorTable) {
    if (!zone?.isActive || table.status !== 'AVAILABLE') return;
    const items = chosen.map(t => ({ id: t.id, version: t.version }));
    if (items.some(t => t.id === table.id)) setSelected(items.filter(t => t.id !== table.id));
    else if (items.length < 3) setSelected([...items, { id: table.id, version: table.version }]);
    else { setNotice('Select at most 3 tables in one zone.'); return; }
    setNotice('');
  }
  async function refresh() {
    setManual(true);
    try { await live.refresh(); } finally { setManual(false); }
  }
  if (live.accessLost) return <p role="alert" className="mt-8">{live.accessLost === 401
    ? 'Your session has ended. Redirecting to sign in…' : 'This floor is no longer available to your account.'}</p>;

  return <section className="mt-8 grid min-w-0 gap-6" aria-label="Restaurant floor">
    <div>
      <button onClick={refresh} disabled={manual} className="min-h-11 font-semibold text-wine underline disabled:opacity-60">{manual ? 'Refreshing…' : 'Refresh floor'}</button>
      <LiveStatus updatedAt={live.updatedAt} renderedAt={renderedAt} failure={live.failure} />
    </div>
    {!floor ? <p role="status">{live.failure ? 'Floor could not be loaded. Retry using Refresh floor.' : 'Loading floor…'}</p> : <>
      <p>Mode: <strong>{floor.operationsMode}</strong> · Layout version {floor.layoutVersion}</p>
      {floor.operationsMode === 'MANUAL' && <p className="text-sm">Continue calling and completing tickets on the queue board. Table allocation is not enabled.</p>}
      {floor.zones.length === 0 ? <p role="status">No floor is configured for this restaurant. Ask the team to provision its approved layout.</p> : zone && <>
        <div className="flex flex-wrap items-end gap-4">
          <label className="grid gap-2 font-semibold">Zone
            <select value={zone.id} onChange={event => { setZoneId(event.target.value); setSelected([]); setNotice(''); }} className="min-h-11 rounded-lg border border-wine/30 bg-white px-3">
              {floor.zones.map(z => <option key={z.id} value={z.id}>{z.name}{z.isActive ? '' : ' (inactive)'}</option>)}
            </select>
          </label>
          <div role="group" aria-label="Floor view" className="flex gap-2">
            {(['map', 'list'] as const).map(v => <button key={v} aria-pressed={view === v} onClick={() => setView(v)} className="min-h-11 rounded-lg border border-wine/30 px-4 capitalize aria-pressed:bg-wine aria-pressed:text-white">{v} view</button>)}
          </div>
        </div>
        <h2 className="text-2xl font-semibold">{zone.name}</h2>
        {!zone.isActive && <p>This zone is inactive. Its tables cannot be selected.</p>}
        <p className="text-sm">Select up to 3 available tables in this zone. Selection only inspects the inventory; it does not hold tables or confirm a match.</p>
        {view === 'map' && <div className="relative h-72 rounded-2xl border border-wine/20 bg-white" aria-label={`${zone.name} table map`}>
          <svg className="pointer-events-none absolute inset-0 h-full w-full" viewBox="0 0 100 100" preserveAspectRatio="none" aria-hidden="true">
            {zone.connections.map(edge => {
              const a = zone.tables.find(t => t.id === edge.tableIds[0])!;
              const b = zone.tables.find(t => t.id === edge.tableIds[1])!;
              return <line key={edge.tableIds.join(':')} x1={a.positionX} y1={a.positionY} x2={b.positionX} y2={b.positionY} stroke="#8b7272" strokeWidth="0.7" strokeDasharray="2 1" />;
            })}
          </svg>
          {zone.tables.map(table => <button key={table.id} aria-label={`${table.code}, ${table.capacity} seats, ${label(table.status)}`} aria-pressed={chosen.some(t => t.id === table.id)}
            disabled={!zone.isActive || table.status !== 'AVAILABLE'} onClick={() => select(table)}
            style={{ left: `clamp(48px, ${table.positionX}%, calc(100% - 48px))`, top: `clamp(54px, ${table.positionY}%, calc(100% - 54px))` }}
            className="absolute grid min-h-24 w-24 -translate-x-1/2 -translate-y-1/2 content-center rounded-xl border border-wine/40 bg-paper p-2 text-sm shadow-sm aria-pressed:bg-wine aria-pressed:text-white disabled:bg-gray-100 disabled:text-gray-600 focus-visible:outline-2 focus-visible:outline-offset-4 focus-visible:outline-wine">
            <strong className="text-lg">{table.code}</strong><span>{table.capacity} seats</span><span className="capitalize">{label(table.status)}</span>
          </button>)}
        </div>}
        {view === 'list' && <ul className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3" aria-label={`${zone.name} table list`}>
          {zone.tables.map(table => <li key={table.id} className="rounded-xl border border-wine/20 bg-white p-4">
            <h3 className="font-semibold">{table.code} · {table.capacity} seats</h3>
            <p className="capitalize">{label(table.status)}</p><p className="text-sm">Version {table.version}</p>
            <button aria-pressed={chosen.some(t => t.id === table.id)} disabled={!zone.isActive || table.status !== 'AVAILABLE'} onClick={() => select(table)} className="mt-2 min-h-11 rounded-lg border border-wine px-4 text-wine aria-pressed:bg-wine aria-pressed:text-white disabled:opacity-50">{chosen.some(t => t.id === table.id) ? 'Deselect' : 'Select'} {table.code}</button>
          </li>)}
        </ul>}
        {zone.tables.length === 0 && <p>No tables configured in this zone.</p>}
        <div className="rounded-xl border border-wine/20 p-4">
          <h3 className="font-semibold">Approved connections</h3>
          <p>{zone.connections.map(edge => edge.tableIds.map(id => zone.tables.find(t => t.id === id)!.code).sort().join(' + ')).sort().join('; ') || 'None. Tables in this zone cannot be combined.'}</p>
          <p className="mt-2 text-sm">Positions are for display only. The approved connections above define adjacency.</p>
        </div>
        <div className="rounded-xl border border-wine/20 p-4" aria-label="Selected tables">
          <h3 className="font-semibold">Selected tables</h3>
          <p>{chosen.length ? `${chosen.map(t => t.code).join(' + ')} · ${chosen.reduce((n, t) => n + t.capacity, 0)} seats total` : 'No tables selected.'}</p>
          {chosen.length > 0 && <button className="min-h-11 font-semibold text-wine underline" onClick={() => { setSelected([]); setNotice(''); }}>Clear selection</button>}
          {selectionChanged && <p role="status">Inventory changed. Unavailable tables were removed from the selection.</p>}
          {notice && <p role="status">{notice}</p>}
        </div>
      </>}
      <p className="text-sm">Table states: {tableStates.map(label).join(' · ')}.</p>
    </>}
  </section>;
}
