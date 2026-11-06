import { redirect } from 'next/navigation';
import { cookies } from 'next/headers';
import { currentSession } from '@/lib/auth';
import { LoginForm } from '@/components/login-form';

export default async function LoginPage() {
  const session = await currentSession();
  if (session) redirect(session.user.role === 'HOST' ? '/host/queue' : '/');
  const jar = await cookies();
  const endedSession = jar.has('dineq_session') || jar.has('__Host-dineq_session');
  return <main className="mx-auto flex min-h-screen max-w-md flex-col justify-center px-6 py-12">
    <p className="mb-8 text-xl font-semibold text-wine">DineQueue</p>
    <h1 className="text-3xl font-semibold">Welcome back</h1>
    <p className="mt-3 leading-relaxed">Sign in to your restaurant waitlist account.</p>
    {endedSession && <p role="status" className="mt-4 rounded-lg border border-wine/30 p-3 text-wine">Your session has ended. Please sign in again.</p>}
    <LoginForm />
    <p className="mt-6 text-sm text-ink/70">Use the account provided by your restaurant.</p>
  </main>;
}
