'use client';

import { useState, type FormEvent } from 'react';
import { useRouter } from 'next/navigation';

export function LoginForm() {
  const router = useRouter();
  const [pending, setPending] = useState(false);
  const [error, setError] = useState('');
  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (pending) return;
    const form = new FormData(event.currentTarget);
    setPending(true); setError('');
    try {
      const response = await fetch('/api/v1/auth/login', {
        method: 'POST', credentials: 'same-origin', headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email: form.get('email'), password: form.get('password') }),
      });
      if (!response.ok) {
        setError(response.status === 401 ? 'Email or password is incorrect.' : response.status === 429 ? 'Too many attempts. Please wait a minute and try again.' : 'Unable to sign in. Please try again.');
        return;
      }
      const { user } = await response.json();
      router.replace(user.role === 'HOST' ? '/host/queue' : '/'); router.refresh();
    } catch { setError('Unable to connect. Please try again.'); }
    finally { setPending(false); }
  }
  return <form onSubmit={submit} className="mt-8 grid gap-5" aria-busy={pending}>
    <div className="grid gap-2">
      <label htmlFor="email" className="font-medium">Email</label>
      <input id="email" name="email" type="email" autoComplete="username" required maxLength={254} disabled={pending} className="rounded-lg border border-ink/30 bg-white px-4 py-3 focus:outline-2 focus:outline-wine" />
    </div>
    <div className="grid gap-2">
      <label htmlFor="password" className="font-medium">Password</label>
      <input id="password" name="password" type="password" autoComplete="current-password" required maxLength={128} disabled={pending} className="rounded-lg border border-ink/30 bg-white px-4 py-3 focus:outline-2 focus:outline-wine" />
    </div>
    {error && <p role="alert" className="rounded-lg border border-wine/30 bg-white p-3 text-wine">{error}</p>}
    <button disabled={pending} className="rounded-lg bg-wine px-5 py-3 font-semibold text-white disabled:opacity-60">{pending ? 'Signing in…' : 'Sign in'}</button>
  </form>;
}
