// DTO types shared by server-only read helpers and client polling components.
// No runtime code: safe to import from both sides of the boundary.

export type Ticket = {
  id: string; queueId: string; ticketCode: string; serviceDate: string;
  partySize: number; isReady: boolean; status: 'WAITING' | 'CALLED' | 'COMPLETED' | 'CANCELLED' | 'NO_SHOW'; joinedAt: string;
  queueName: string; restaurantName: string; timezone: string;
};

export type HostEntry = {
  id: string; queueId: string; ticketCode: string; serviceDate: string; partySize: number;
  isReady: boolean; status: 'WAITING' | 'CALLED' | 'COMPLETED' | 'CANCELLED' | 'NO_SHOW'; joinedAt: string; calledAt: string | null; completedAt: string | null;
};
