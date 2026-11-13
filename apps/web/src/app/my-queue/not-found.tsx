import Link from 'next/link';

export default function TicketNotFound() {
  return <main className="mx-auto max-w-lg px-6 py-20">
    <h1 className="text-2xl font-semibold">Ticket not found</h1>
    <p className="mt-4">This ticket is not available.</p>
    <Link href="/my-queue" className="mt-6 inline-block font-semibold text-wine underline">My Ticket</Link>
  </main>;
}
