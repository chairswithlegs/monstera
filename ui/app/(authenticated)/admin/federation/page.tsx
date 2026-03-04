'use client';

import { getInstances, getDomainBlocks, createDomainBlock, deleteDomainBlock, type KnownInstance, type DomainBlock } from '@/lib/api/admin';
import { useCallback, useEffect, useState } from 'react';

export default function AdminFederationPage() {
  const [instances, setInstances] = useState<KnownInstance[]>([]);
  const [blocks, setBlocks] = useState<DomainBlock[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [newDomain, setNewDomain] = useState('');
  const [newReason, setNewReason] = useState('');
  const [submitting, setSubmitting] = useState(false);

  const load = useCallback(() => {
    getInstances({ limit: 50 })
      .then((r) => setInstances(r.instances))
      .catch((e) => setError(e instanceof Error ? e.message : 'Failed to load instances'));
    getDomainBlocks()
      .then((r) => setBlocks(r.domain_blocks))
      .catch((e) => setError(e instanceof Error ? e.message : 'Failed to load blocks'));
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  const addBlock = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!newDomain.trim()) return;
    setSubmitting(true);
    try {
      await createDomainBlock({ domain: newDomain.trim(), severity: 'silence', reason: newReason.trim() || undefined });
      setNewDomain('');
      setNewReason('');
      load();
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to add block');
    } finally {
      setSubmitting(false);
    }
  };

  const removeBlock = async (domain: string) => {
    try {
      await deleteDomainBlock(domain);
      load();
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to remove block');
    }
  };

  if (error) return <p className="text-red-600">{error}</p>;

  return (
    <div>
      <h1 className="text-2xl font-semibold text-gray-900">Federation</h1>

      <section className="mt-8">
        <h2 className="text-lg font-medium text-gray-900">Known instances</h2>
        <div className="mt-4 overflow-hidden rounded-lg border border-gray-200 bg-white shadow">
          {instances.length === 0 ? (
            <p className="p-6 text-gray-500">None yet.</p>
          ) : (
            <table className="min-w-full divide-y divide-gray-200">
              <thead className="bg-gray-50">
                <tr>
                  <th className="px-4 py-2 text-left text-xs font-medium uppercase text-gray-500">Domain</th>
                  <th className="px-4 py-2 text-left text-xs font-medium uppercase text-gray-500">Accounts</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-200 bg-white">
                {instances.map((i) => (
                  <tr key={i.id}>
                    <td className="px-4 py-3 text-sm text-gray-900">{i.domain}</td>
                    <td className="px-4 py-3 text-sm text-gray-600">{i.accounts_count}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
      </section>

      <section className="mt-8">
        <h2 className="text-lg font-medium text-gray-900">Domain blocks</h2>
        <form onSubmit={addBlock} className="mt-4 flex flex-wrap items-end gap-2">
          <input
            type="text"
            value={newDomain}
            onChange={(e) => setNewDomain(e.target.value)}
            placeholder="domain.example.com"
            className="rounded border border-gray-300 px-3 py-1.5 text-sm"
          />
          <input
            type="text"
            value={newReason}
            onChange={(e) => setNewReason(e.target.value)}
            placeholder="Reason (optional)"
            className="rounded border border-gray-300 px-3 py-1.5 text-sm"
          />
          <button type="submit" disabled={submitting} className="rounded bg-indigo-600 px-3 py-1.5 text-sm text-white hover:bg-indigo-500 disabled:opacity-50">
            Add block
          </button>
        </form>
        <div className="mt-4 overflow-hidden rounded-lg border border-gray-200 bg-white shadow">
          {blocks.length === 0 ? (
            <p className="p-6 text-gray-500">No domain blocks.</p>
          ) : (
            <table className="min-w-full divide-y divide-gray-200">
              <thead className="bg-gray-50">
                <tr>
                  <th className="px-4 py-2 text-left text-xs font-medium uppercase text-gray-500">Domain</th>
                  <th className="px-4 py-2 text-left text-xs font-medium uppercase text-gray-500">Severity</th>
                  <th className="px-4 py-2 text-right text-xs font-medium uppercase text-gray-500">Actions</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-200 bg-white">
                {blocks.map((b) => (
                  <tr key={b.id}>
                    <td className="px-4 py-3 text-sm text-gray-900">{b.domain}</td>
                    <td className="px-4 py-3 text-sm text-gray-600">{b.severity}</td>
                    <td className="px-4 py-3 text-right">
                      <button type="button" onClick={() => removeBlock(b.domain)} className="text-red-600 hover:text-red-800">
                        Remove
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
      </section>
    </div>
  );
}
