import { useState, useEffect, useCallback } from 'react'
import { queryAudit, type AuditEntry } from '../api/audit'

export default function AuditPage() {
  const [entries, setEntries] = useState<AuditEntry[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  const [filterUser, setFilterUser] = useState('')
  const [filterBucket, setFilterBucket] = useState('')
  const [limit, setLimit] = useState(100)

  const fetchAudit = useCallback(async () => {
    try {
      const data = await queryAudit({
        limit,
        user: filterUser || undefined,
        bucket: filterBucket || undefined,
      })
      setEntries(data || [])
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load audit trail')
    } finally {
      setLoading(false)
    }
  }, [limit, filterUser, filterBucket])

  useEffect(() => { fetchAudit() }, [fetchAudit])

  // auto-refresh every 10s
  useEffect(() => {
    const id = setInterval(fetchAudit, 10000)
    return () => clearInterval(id)
  }, [fetchAudit])

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-indigo-600" />
      </div>
    )
  }

  return (
    <div>
      <div className="mb-6">
        <h2 className="text-xl font-semibold text-gray-900 dark:text-white">Audit Trail</h2>
        <p className="text-sm text-gray-500 dark:text-gray-400 mt-0.5">Security and access audit log</p>
      </div>

      {error && (
        <div className="mb-4 p-3 rounded-lg bg-red-50 dark:bg-red-900/20 text-red-700 dark:text-red-400 text-sm">
          {error}
        </div>
      )}

      {/* Filter bar */}
      <div className="flex flex-wrap gap-3 mb-4">
        <input type="text" placeholder="Filter by user..." value={filterUser}
          onChange={e => setFilterUser(e.target.value)}
          className="px-3 py-2 rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-800 text-gray-900 dark:text-white text-sm w-44" />
        <input type="text" placeholder="Filter by bucket..." value={filterBucket}
          onChange={e => setFilterBucket(e.target.value)}
          className="px-3 py-2 rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-800 text-gray-900 dark:text-white text-sm w-44" />
        <select value={limit} onChange={e => setLimit(Number(e.target.value))}
          className="px-3 py-2 rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-800 text-gray-900 dark:text-white text-sm">
          <option value={50}>50 entries</option>
          <option value={100}>100 entries</option>
          <option value={500}>500 entries</option>
        </select>
      </div>

      {/* Audit table */}
      <div className="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 overflow-hidden">
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-gray-200 dark:border-gray-700">
                <th className="text-left px-4 py-3 text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Time</th>
                <th className="text-left px-4 py-3 text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">User</th>
                <th className="text-left px-4 py-3 text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Action</th>
                <th className="text-left px-4 py-3 text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Resource</th>
                <th className="text-left px-4 py-3 text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Effect</th>
                <th className="text-left px-4 py-3 text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Source IP</th>
                <th className="text-left px-4 py-3 text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Status</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-100 dark:divide-gray-700/50">
              {entries.map((e, i) => (
                <tr key={i} className="hover:bg-gray-50 dark:hover:bg-gray-700/30 transition-colors">
                  <td className="px-4 py-3 text-gray-500 dark:text-gray-400 whitespace-nowrap">
                    {new Date(e.time).toLocaleString()}
                  </td>
                  <td className="px-4 py-3 font-medium text-gray-900 dark:text-white">{e.user || '-'}</td>
                  <td className="px-4 py-3 text-gray-700 dark:text-gray-300">{e.action}</td>
                  <td className="px-4 py-3 text-gray-500 dark:text-gray-400 font-mono text-xs max-w-xs truncate">{e.resource}</td>
                  <td className="px-4 py-3">
                    <span className={`inline-flex items-center px-2 py-0.5 rounded text-xs font-medium ${
                      e.effect === 'Allow'
                        ? 'bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-400'
                        : e.effect === 'Deny'
                          ? 'bg-red-100 dark:bg-red-900/30 text-red-700 dark:text-red-400'
                          : 'bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-400'
                    }`}>
                      {e.effect || '-'}
                    </span>
                  </td>
                  <td className="px-4 py-3 text-gray-500 dark:text-gray-400 font-mono text-xs">{e.sourceIP || '-'}</td>
                  <td className="px-4 py-3 text-gray-500 dark:text-gray-400">{e.statusCode || '-'}</td>
                </tr>
              ))}
              {entries.length === 0 && (
                <tr><td colSpan={7} className="px-4 py-8 text-center text-gray-400">No audit entries</td></tr>
              )}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  )
}
