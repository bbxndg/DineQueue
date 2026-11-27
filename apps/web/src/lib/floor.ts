import type { ReadOutcome } from '@/lib/poll-coordinator';

export const tableStates = ['AVAILABLE', 'HELD', 'OCCUPIED', 'DIRTY', 'CLEANING', 'OUT_OF_SERVICE'] as const;
export type FloorTable = { id: string; code: string; capacity: number; status: typeof tableStates[number]; positionX: number; positionY: number; version: number };
export type FloorZone = { id: string; name: string; isActive: boolean; tables: FloorTable[]; connections: { tableIds: [string, string] }[] };
export type Assignment = { id: string; queueId: string; entryId: string; ticketCode: string; status: 'HELD' | 'ACTIVE' | 'CLOSED' | 'RELEASED'; turnoverPhase: 'DIRTY' | 'CLEANING' | 'CLEANED' | null; version: number; tableIds: string[]; holdExpiresAt: string };
export type FloorSnapshot = { queueId: string; restaurantId: string; operationsMode: 'MANUAL' | 'SEATING'; layoutVersion: number; serverNow: string; holdTtlSeconds: number; zones: FloorZone[]; assignments: Assignment[] };

const record = (v: unknown): v is Record<string, unknown> => typeof v === 'object' && v !== null;
const positive = (v: unknown): v is number => Number.isSafeInteger(v) && (v as number) > 0;
const coordinate = (v: unknown): v is number => typeof v === 'number' && Number.isFinite(v) && v >= 0 && v <= 100;

// Failed/malformed reads are never converted to a believable empty floor.
export function parseFloor(value: unknown, queueId: string): FloorSnapshot {
  const invalid = () => { throw new Error('Invalid floor response'); };
  if (!record(value) || value.queueId !== queueId || typeof value.restaurantId !== 'string'
    || !['MANUAL', 'SEATING'].includes(String(value.operationsMode)) || !positive(value.layoutVersion)
    || typeof value.serverNow !== 'string' || !Number.isFinite(Date.parse(value.serverNow))
    || !Number.isInteger(value.holdTtlSeconds) || Number(value.holdTtlSeconds) < 60 || Number(value.holdTtlSeconds) > 1800
    || !Array.isArray(value.zones) || !Array.isArray(value.assignments)) return invalid();
  const ids = new Set<string>();
  for (const zone of value.zones) {
    if (!record(zone) || typeof zone.id !== 'string' || ids.has(zone.id) || typeof zone.name !== 'string'
      || typeof zone.isActive !== 'boolean' || !Array.isArray(zone.tables) || !Array.isArray(zone.connections)) return invalid();
    ids.add(zone.id);
    const tableIds = new Set<string>();
    for (const table of zone.tables) {
      if (!record(table) || typeof table.id !== 'string' || ids.has(table.id) || typeof table.code !== 'string'
        || !positive(table.capacity) || table.capacity > 8 || !tableStates.includes(table.status as FloorTable['status'])
        || !coordinate(table.positionX) || !coordinate(table.positionY) || !positive(table.version)) return invalid();
      ids.add(table.id); tableIds.add(table.id);
    }
    const edges = new Set<string>();
    for (const edge of zone.connections) {
      if (!record(edge) || !Array.isArray(edge.tableIds) || edge.tableIds.length !== 2) return invalid();
      const [a, b] = edge.tableIds;
      if (typeof a !== 'string' || typeof b !== 'string' || a >= b || !tableIds.has(a) || !tableIds.has(b) || edges.has(`${a}:${b}`)) return invalid();
      edges.add(`${a}:${b}`);
    }
  }
  for (const assignment of value.assignments) {
    if (!record(assignment) || typeof assignment.id !== 'string' || assignment.queueId !== queueId
      || typeof assignment.entryId !== 'string' || typeof assignment.ticketCode !== 'string' || assignment.ticketCode === '' || !['HELD', 'ACTIVE', 'CLOSED', 'RELEASED'].includes(String(assignment.status))
      || !(assignment.turnoverPhase === null || ['DIRTY', 'CLEANING', 'CLEANED'].includes(String(assignment.turnoverPhase)))
      || !positive(assignment.version) || !Array.isArray(assignment.tableIds) || assignment.tableIds.length < 1 || assignment.tableIds.length > 3
      || new Set(assignment.tableIds).size !== assignment.tableIds.length || assignment.tableIds.some(id => typeof id !== 'string' || !ids.has(id))
      || typeof assignment.holdExpiresAt !== 'string' || !Number.isFinite(Date.parse(assignment.holdExpiresAt))) return invalid();
  }
  return value as FloorSnapshot;
}

export async function readFloor(queueId: string, signal: AbortSignal): Promise<ReadOutcome<FloorSnapshot>> {
  const response = await fetch(`/api/v1/host/queues/${encodeURIComponent(queueId)}/floor`, {
    method: 'GET', credentials: 'same-origin', cache: 'no-store', signal, headers: { Accept: 'application/json' },
  });
  if (!response.ok) {
    await response.body?.cancel().catch(() => undefined);
    return { kind: 'status', status: response.status };
  }
  return { kind: 'ok', data: parseFloor(await response.json(), queueId) };
}
