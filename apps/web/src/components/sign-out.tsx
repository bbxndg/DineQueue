'use client';

import { useState } from 'react';
import { useRouter } from 'next/navigation';

export function SignOut({ csrfToken }: { csrfToken: string }) {
  const router = useRouter();
  const [pending, setPending] = useState(false);
  const [error, setError] = useState('');
  async function logout() {
    setPending(true); setError('');
    try {
      const response = await fetch('/api/v1/auth/logout', { method: 'POST', credentials: 'same-origin', headers: { 'X-CSRF-Token': csrfToken } });
      if (response.ok || response.status === 401) { router.replace('/login'); router.refresh(); }
      else setError('Unable to sign out. Refresh this page and try again.');
    } catch { setError('Unable to connect. Please try again.'); }
    finally { setPending(false); }
  }
  return <div>
    <button onClick={logout} disabled={pending} className="rounded-lg border border-wine px-4 py-2 font-medium text-wine disabled:opacity-60">{pending ? 'Signing out…' : 'Sign out'}</button>
    {error && <p role="alert" className="mt-3 text-wine">{error}</p>}
  </div>;
}
