'use client';

import { getReport, resolveReport, type Report } from '@/lib/api/admin';
import Link from 'next/link';
import { useCallback, useEffect, useState } from 'react';

type Props = { id: string };

export default function ReportDetailView({ id }: Props) {
  const [report, setReport] = useState<Report | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [acting, setActing] = useState(false);

  const load = useCallback(() => {
    if (!id) return;
    getReport(id)
      .then(setReport)
      .catch((e) => setError(e instanceof Error ? e.message : 'Failed to load'));
  }, [id]);

  useEffect(() => {
    load();
  }, [load]);

  const resolve = async () => {
    const resolution = prompt('Resolution note:');
    if (resolution == null) return;
    setActing(true);
    try {
      await resolveReport(id, resolution);
      load();
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed');
    } finally {
      setActing(false);
    }
  };

  if (error && !report) return <p className="text-red-600">{error}</p>;
  if (!report) return <p className="text-gray-500">Loading…</p>;

  return (
    <div>
      <p className="mb-4">
        <Link href="/admin/reports" className="text-indigo-600 hover:text-indigo-800">
          ← Back to reports
        </Link>
      </p>
      <h1 className="text-2xl font-semibold text-gray-900">Report {report.id.slice(0, 8)}…</h1>
      <dl className="mt-6 grid gap-2 sm:grid-cols-2">
        <dt className="text-sm font-medium text-gray-500">Category</dt>
        <dd className="text-sm text-gray-900">{report.category}</dd>
        <dt className="text-sm font-medium text-gray-500">State</dt>
        <dd className="text-sm text-gray-900">{report.state}</dd>
        <dt className="text-sm font-medium text-gray-500">Reporter account</dt>
        <dd className="font-mono text-sm text-gray-900">{report.account_id}</dd>
        <dt className="text-sm font-medium text-gray-500">Target account</dt>
        <dd className="font-mono text-sm text-gray-900">{report.target_id}</dd>
        {report.comment && (
          <>
            <dt className="text-sm font-medium text-gray-500">Comment</dt>
            <dd className="text-sm text-gray-900">{report.comment}</dd>
          </>
        )}
        {report.action_taken && (
          <>
            <dt className="text-sm font-medium text-gray-500">Action taken</dt>
            <dd className="text-sm text-gray-900">{report.action_taken}</dd>
          </>
        )}
      </dl>
      {error && <p className="mt-2 text-red-600">{error}</p>}
      {report.state === 'open' && (
        <div className="mt-6 flex gap-2">
          <button
            type="button"
            disabled={acting}
            onClick={resolve}
            className="rounded bg-gray-800 px-3 py-1.5 text-sm text-white hover:bg-gray-700 disabled:opacity-50"
          >
            Resolve report
          </button>
        </div>
      )}
    </div>
  );
}
