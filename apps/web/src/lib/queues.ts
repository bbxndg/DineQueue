import 'server-only';
import { redirect } from 'next/navigation';
import { backendGet } from '@/lib/auth';

export type ServiceQueue = { id: string; name: string; restaurantName: string };

export async function activeQueues(): Promise<ServiceQueue[]> {
  const response = await backendGet('/api/v1/queues');
  if (!response || response.status === 401) redirect('/login');
  if (!response.ok) throw new Error('Queue service is temporarily unavailable.');
  const body: { queues: ServiceQueue[] } = await response.json();
  return body.queues;
}
