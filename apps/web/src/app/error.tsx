'use client';

export default function ErrorPage({ reset }: { reset: () => void }) {
  return <main className="mx-auto max-w-lg px-6 py-20">
    <h1 className="text-2xl font-semibold">We cannot load your account right now</h1>
    <p className="mt-4">Please check your connection and try again.</p>
    <button onClick={reset} className="mt-6 rounded-lg bg-wine px-5 py-3 text-white">Try again</button>
  </main>;
}
