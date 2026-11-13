import 'server-only';
import { redirect } from 'next/navigation';
import { backendGet } from '@/lib/auth';

export type HostQueue = { id: string; name: string; restaurantName: string; timezone: string; isActive: boolean };

export type HostEntry = {
  id: string; queueId: string; ticketCode: string; serviceDate: string; partySize: number;
  status: 'WAITING' | 'CALLED' | 'COMPLETED'; joinedAt: string; calledAt: string | null; completedAt: string | null;
};

async function hostGet<T>(path: string): Promise<T> {
  const response = await backendGet(path);
  if (!response || response.status === 401) redirect('/login');
  if (response.status === 403) redirect('/');
  if (!response.ok) throw new Error('Queue service is temporarily unavailable.');
  return response.json();
}

export async function assignedQueues(): Promise<HostQueue[]> {
  return (await hostGet<{ queues: HostQueue[] }>('/api/v1/host/queues')).queues;
}

// queueId must come from assignedQueues(), never directly from the URL.
export async function queueBoard(queueId: string) {
  const id = encodeURIComponent(queueId);
  const [waiting, current] = await Promise.all([
    hostGet<{ entries: HostEntry[] }>(`/api/v1/host/queues/${id}/entries?status=WAITING`),
    hostGet<{ entry: HostEntry | null }>(`/api/v1/host/queues/${id}/current`),
  ]);
  return { waiting: waiting.entries, current: current.entry };
}
