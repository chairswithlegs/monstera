'use client';

import { getReports, type Report } from '@/lib/api/admin';
import Link from 'next/link';
import { useSearchParams } from 'next/navigation';
import { useEffect, useState } from 'react';
import ReportDetailView from './ReportDetailView';

export default function AdminReportsPage() {
  const searchParams = useSearchParams();
  const detailId = searchParams.get('id');

  if (detailId) {
    return <ReportDetailView id={detailId} />;
  }

  return <ReportsList />;
}

function ReportsList() {
  const [reports, setReports] = useState<Report[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [state, setState] = useState('open');

  useEffect(() => {
    getReports({ state, limit: 50 })
      .then((r) => setReports(r.reports))
      .catch((e) => setError(e instanceof Error ? e.message : 'Failed to load'));
  }, [state]);

  if (error) return <p className="text-red-600">{error}</p>;

  return (
    <div>
      <h1 className="text-2xl font-semibold text-gray-900">Reports</h1>
      <div className="mt-4 flex gap-2">
        <button
          type="button"
          onClick={() => setState('open')}
          className={`rounded px-3 py-1.5 text-sm ${state === 'open' ? 'bg-gray-800 text-white' : 'bg-gray-200 text-gray-700'}`}
        >
          Open
        </button>
        <button
          type="button"
          onClick={() => setState('resolved')}
          className={`rounded px-3 py-1.5 text-sm ${state === 'resolved' ? 'bg-gray-800 text-white' : 'bg-gray-200 text-gray-700'}`}
        >
          Resolved
        </button>
        <button
          type="button"
          onClick={() => setState('')}
          className={`rounded px-3 py-1.5 text-sm ${state === '' ? 'bg-gray-800 text-white' : 'bg-gray-200 text-gray-700'}`}
        >
          All
        </button>
      </div>
      <div className="mt-6 overflow-hidden rounded-lg border border-gray-200 bg-white shadow">
        {reports.length === 0 ? (
          <p className="p-6 text-gray-500">No reports.</p>
        ) : (
          <table className="min-w-full divide-y divide-gray-200">
            <thead className="bg-gray-50">
              <tr>
                <th className="px-4 py-2 text-left text-xs font-medium uppercase text-gray-500">ID</th>
                <th className="px-4 py-2 text-left text-xs font-medium uppercase text-gray-500">Category</th>
                <th className="px-4 py-2 text-left text-xs font-medium uppercase text-gray-500">State</th>
                <th className="px-4 py-2 text-left text-xs font-medium uppercase text-gray-500">Created</th>
                <th className="px-4 py-2 text-right text-xs font-medium uppercase text-gray-500">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-200 bg-white">
              {reports.map((r) => (
                <tr key={r.id}>
                  <td className="px-4 py-3 font-mono text-sm text-gray-900">{r.id.slice(0, 8)}…</td>
                  <td className="px-4 py-3 text-sm text-gray-600">{r.category}</td>
                  <td className="px-4 py-3 text-sm">
                    <span className={r.state === 'open' ? 'text-amber-600' : 'text-gray-600'}>{r.state}</span>
                  </td>
                  <td className="px-4 py-3 text-sm text-gray-600">{new Date(r.created_at).toLocaleString()}</td>
                  <td className="px-4 py-3 text-right">
                    <Link href={`/admin/reports?id=${encodeURIComponent(r.id)}`} className="text-indigo-600 hover:text-indigo-800">
                      View
                    </Link>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
    </div>
  );
}
