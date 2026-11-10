import 'server-only';
import { notFound, redirect } from 'next/navigation';
import { backendGet } from '@/lib/auth';

export type Ticket = {
  id: string; queueId: string; ticketCode: string; serviceDate: string;
  partySize: number; status: 'WAITING' | 'CALLED' | 'COMPLETED'; joinedAt: string;
  queueName: string; restaurantName: string; timezone: string;
};

async function ticketGet(path: string) {
  const response = await backendGet(path);
  if (!response || response.status === 401) redirect('/login');
  if (response.status === 404) notFound();
  if (!response.ok) throw new Error('Ticket service is temporarily unavailable.');
  return response;
}

export async function activeTickets(): Promise<Ticket[]> {
  const response = await ticketGet('/api/v1/me/queue-entries/active');
  const body: { entries: Ticket[] } = await response.json();
  return body.entries;
}

export async function ticketDetail(id: string): Promise<Ticket> {
  // Keep an arbitrary URL segment out of the backend request path.
  if (!/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i.test(id)) notFound();
  const response = await ticketGet(`/api/v1/me/queue-entries/${encodeURIComponent(id)}`);
  return response.json();
}
