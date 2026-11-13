import 'server-only';
import { cookies } from 'next/headers';
import { redirect } from 'next/navigation';

export type CurrentSession = {
  user: { id: string; displayName: string; role: 'CUSTOMER' | 'HOST' };
  csrfToken: string;
};

// Server-side GET to the API, forwarding only the session cookie. Returns null without a session.
export async function backendGet(path: string): Promise<Response | null> {
  const jar = await cookies();
  const session = jar.get('__Host-dineq_session') ?? jar.get('dineq_session');
  if (!session) return null;
  const origin = process.env.API_INTERNAL_URL ?? 'http://127.0.0.1:8080';
  return fetch(`${origin}${path}`, {
    headers: { Cookie: `${session.name}=${encodeURIComponent(session.value)}` },
    cache: 'no-store', signal: AbortSignal.timeout(5000),
  });
}

export async function currentSession(): Promise<CurrentSession | null> {
  const response = await backendGet('/api/v1/auth/me');
  if (!response || response.status === 401) return null;
  if (!response.ok) throw new Error('Sign-in service is temporarily unavailable.');
  return response.json();
}

export async function requireSession() {
  const session = await currentSession();
  if (!session) redirect('/login');
  return session;
}
