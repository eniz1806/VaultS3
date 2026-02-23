import { useState, useEffect, useCallback } from 'react'
import { getReplicationStatus, getReplicationQueue, type ReplicationStatus, type ReplicationEvent } from '../api/replication'

export default function ReplicationPage() {
  const [status, setStatus] = useState<ReplicationStatus | null>(null)
  const [queue, setQueue] = useState<ReplicationEvent[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  const fetchData = useCallback(async () => {
    try {
      const [s, q] = await Promise.all([getReplicationStatus(), getReplicationQueue(100)])
      setStatus(s)
      setQueue(q || [])
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load replication data')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { fetchData() }, [fetchData])
  useEffect(() => {
    const id = setInterval(fetchData, 10000)
    return () => clearInterval(id)
  }, [fetchData])

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
        <h2 className="text-xl font-semibold text-gray-900 dark:text-white">Replication</h2>
        <p className="text-sm text-gray-500 dark:text-gray-400 mt-0.5">Peer replication status and queue</p>
      </div>

      {error && (
        <div className="mb-4 p-3 rounded-lg bg-red-50 dark:bg-red-900/20 text-red-700 dark:text-red-400 text-sm">
          {error}
        </div>
      )}

      {/* Status badge */}
      <div className="mb-4">
        <span className={`inline-flex items-center px-3 py-1 rounded-full text-sm font-medium ${
          status?.enabled
            ? 'bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-400'
            : 'bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-400'
        }`}>
          {status?.enabled ? 'Enabled' : 'Disabled'}
        </span>
      </div>

      {/* Peer cards */}
      {(status?.peers || []).length > 0 ? (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4 mb-6">
          {(status?.peers || []).map(p => (
            <div key={p.name} className="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-4">
              <h4 className="font-medium text-gray-900 dark:text-white mb-3">{p.name}</h4>
              <dl className="space-y-2 text-sm">
                <div className="flex justify-between">
                  <dt className="text-gray-500 dark:text-gray-400">URL</dt>
                  <dd className="text-gray-700 dark:text-gray-300 font-mono text-xs truncate max-w-[180px]">{p.url}</dd>
                </div>
                <div className="flex justify-between">
                  <dt className="text-gray-500 dark:text-gray-400">Queue</dt>
                  <dd className="text-gray-700 dark:text-gray-300">{p.queueDepth}</dd>
                </div>
                <div className="flex justify-between">
                  <dt className="text-gray-500 dark:text-gray-400">Total Synced</dt>
                  <dd className="text-gray-700 dark:text-gray-300">{p.totalSynced}</dd>
                </div>
                <div className="flex justify-between">
                  <dt className="text-gray-500 dark:text-gray-400">Last Sync</dt>
                  <dd className="text-gray-700 dark:text-gray-300 text-xs">
                    {p.lastSync ? new Date(p.lastSync).toLocaleString() : 'Never'}
                  </dd>
                </div>
                {p.lastError && (
                  <div>
                    <dt className="text-red-500 text-xs">Last Error</dt>
                    <dd className="text-red-400 text-xs mt-0.5 truncate">{p.lastError}</dd>
                  </div>
                )}
              </dl>
            </div>
          ))}
        </div>
      ) : (
        <div className="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-8 text-center mb-6">
          <p className="text-gray-400">No replication peers configured</p>
        </div>
      )}

      {/* Queue table */}
      <h3 className="text-sm font-semibold text-gray-900 dark:text-white mb-3">Pending Queue</h3>
      <div className="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 overflow-hidden">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-gray-200 dark:border-gray-700">
              <th className="text-left px-4 py-3 text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Type</th>
              <th className="text-left px-4 py-3 text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Bucket</th>
              <th className="text-left px-4 py-3 text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Key</th>
              <th className="text-left px-4 py-3 text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Peer</th>
              <th className="text-left px-4 py-3 text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Retries</th>
              <th className="text-left px-4 py-3 text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Next Retry</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-100 dark:divide-gray-700/50">
            {queue.map((e, i) => (
              <tr key={i} className="hover:bg-gray-50 dark:hover:bg-gray-700/30 transition-colors">
                <td className="px-4 py-3 text-gray-700 dark:text-gray-300">{e.type}</td>
                <td className="px-4 py-3 font-medium text-gray-900 dark:text-white">{e.bucket}</td>
                <td className="px-4 py-3 text-gray-500 dark:text-gray-400 font-mono text-xs">{e.key}</td>
                <td className="px-4 py-3 text-gray-500 dark:text-gray-400">{e.peer}</td>
                <td className="px-4 py-3 text-gray-500 dark:text-gray-400">{e.retryCount}</td>
                <td className="px-4 py-3 text-gray-500 dark:text-gray-400 text-xs">
                  {e.nextRetry ? new Date(e.nextRetry).toLocaleString() : '-'}
                </td>
              </tr>
            ))}
            {queue.length === 0 && (
              <tr><td colSpan={6} className="px-4 py-8 text-center text-gray-400">Queue empty</td></tr>
            )}
          </tbody>
        </table>
      </div>
    </div>
  )
}
