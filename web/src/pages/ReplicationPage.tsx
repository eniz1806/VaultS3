import { useState, useEffect, useCallback, useMemo } from 'react'
import { getReplicationStatus, getReplicationQueue, type ReplicationStatus, type ReplicationEvent } from '../api/replication'

type RSortField = 'type' | 'bucket' | 'key' | 'peer' | 'retryCount'
type RSortDir = 'asc' | 'desc'

export default function ReplicationPage() {
  const [status, setStatus] = useState<ReplicationStatus | null>(null)
  const [queue, setQueue] = useState<ReplicationEvent[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [rSortField, setRSortField] = useState<RSortField>('bucket')
  const [rSortDir, setRSortDir] = useState<RSortDir>('asc')

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

  const handleRSort = (field: RSortField) => {
    if (rSortField === field) {
      setRSortDir(d => d === 'asc' ? 'desc' : 'asc')
    } else {
      setRSortField(field)
      setRSortDir('asc')
    }
  }

  const sortedQueue = useMemo(() => {
    const s = [...queue]
    s.sort((a, b) => {
      let cmp = 0
      switch (rSortField) {
        case 'type': cmp = a.type.localeCompare(b.type); break
        case 'bucket': cmp = a.bucket.localeCompare(b.bucket); break
        case 'key': cmp = a.key.localeCompare(b.key); break
        case 'peer': cmp = a.peer.localeCompare(b.peer); break
        case 'retryCount': cmp = a.retryCount - b.retryCount; break
      }
      return rSortDir === 'asc' ? cmp : -cmp
    })
    return s
  }, [queue, rSortField, rSortDir])

  const RSortHeader = ({ field, label }: { field: RSortField; label: string }) => (
    <th
      onClick={() => handleRSort(field)}
      className="text-left px-4 py-3 text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider cursor-pointer hover:text-indigo-600 dark:hover:text-indigo-400 select-none"
    >
      <span className="inline-flex items-center gap-1">
        {label}
        {rSortField === field && (
          <span className="text-indigo-600 dark:text-indigo-400">{rSortDir === 'asc' ? '\u2191' : '\u2193'}</span>
        )}
      </span>
    </th>
  )

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

      {status && !status.enabled && (
        <div className="mb-6 p-5 rounded-xl bg-indigo-50 dark:bg-indigo-900/20 border border-indigo-200 dark:border-indigo-800">
          <h3 className="text-sm font-semibold text-indigo-900 dark:text-indigo-300 mb-2">Replication Not Enabled</h3>
          <p className="text-sm text-indigo-700 dark:text-indigo-400 mb-3">
            Async replication mirrors objects to peer VaultS3 instances automatically.
            Enable it in your <code className="px-1.5 py-0.5 rounded bg-indigo-100 dark:bg-indigo-900/40 font-mono text-xs">vaults3.yaml</code> config:
          </p>
          <pre className="text-xs font-mono bg-gray-900 text-green-400 p-3 rounded-lg overflow-x-auto">{`replication:
  enabled: true
  peers:
    - name: "dc2"
      url: "http://peer-vaults3:9000"
      access_key: "peer-admin"
      secret_key: "peer-secret"`}</pre>
        </div>
      )}

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
              <RSortHeader field="type" label="Type" />
              <RSortHeader field="bucket" label="Bucket" />
              <RSortHeader field="key" label="Key" />
              <RSortHeader field="peer" label="Peer" />
              <RSortHeader field="retryCount" label="Retries" />
              <th className="text-left px-4 py-3 text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Next Retry</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-100 dark:divide-gray-700/50">
            {sortedQueue.map((e, i) => (
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
