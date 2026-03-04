'use client';

import { getFilters, createFilter, deleteFilter, type ServerFilter } from '@/lib/api/admin';
import { useCallback, useEffect, useState } from 'react';

export default function AdminContentPage() {
  const [filters, setFilters] = useState<ServerFilter[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [newPhrase, setNewPhrase] = useState('');
  const [submitting, setSubmitting] = useState(false);

  const load = useCallback(() => {
    getFilters()
      .then((r) => setFilters(r.filters))
      .catch((e) => setError(e instanceof Error ? e.message : 'Failed to load'));
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  const addFilter = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!newPhrase.trim()) return;
    setSubmitting(true);
    try {
      await createFilter({ phrase: newPhrase.trim(), scope: 'all', action: 'hide' });
      setNewPhrase('');
      load();
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to add filter');
    } finally {
      setSubmitting(false);
    }
  };

  const removeFilter = async (id: string) => {
    try {
      await deleteFilter(id);
      load();
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to delete');
    }
  };

  if (error) return <p className="text-red-600">{error}</p>;

  return (
    <div>
      <h1 className="text-2xl font-semibold text-gray-900">Content</h1>
      <p className="mt-1 text-sm text-gray-500">Server-side keyword filters.</p>

      <section className="mt-8">
        <h2 className="text-lg font-medium text-gray-900">Server filters</h2>
        <form onSubmit={addFilter} className="mt-4 flex gap-2">
          <input
            type="text"
            value={newPhrase}
            onChange={(e) => setNewPhrase(e.target.value)}
            placeholder="Keyword or phrase"
            className="rounded border border-gray-300 px-3 py-1.5 text-sm"
          />
          <button type="submit" disabled={submitting} className="rounded bg-indigo-600 px-3 py-1.5 text-sm text-white hover:bg-indigo-500 disabled:opacity-50">
            Add filter
          </button>
        </form>
        <div className="mt-4 overflow-hidden rounded-lg border border-gray-200 bg-white shadow">
          {filters.length === 0 ? (
            <p className="p-6 text-gray-500">No filters.</p>
          ) : (
            <table className="min-w-full divide-y divide-gray-200">
              <thead className="bg-gray-50">
                <tr>
                  <th className="px-4 py-2 text-left text-xs font-medium uppercase text-gray-500">Phrase</th>
                  <th className="px-4 py-2 text-left text-xs font-medium uppercase text-gray-500">Scope</th>
                  <th className="px-4 py-2 text-left text-xs font-medium uppercase text-gray-500">Action</th>
                  <th className="px-4 py-2 text-right text-xs font-medium uppercase text-gray-500">Actions</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-200 bg-white">
                {filters.map((f) => (
                  <tr key={f.id}>
                    <td className="px-4 py-3 text-sm text-gray-900">{f.phrase}</td>
                    <td className="px-4 py-3 text-sm text-gray-600">{f.scope}</td>
                    <td className="px-4 py-3 text-sm text-gray-600">{f.action}</td>
                    <td className="px-4 py-3 text-right">
                      <button type="button" onClick={() => removeFilter(f.id)} className="text-red-600 hover:text-red-800">
                        Delete
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
