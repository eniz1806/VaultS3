import { useState, useEffect, useCallback, useMemo } from 'react'
import { listNotifications, type NotificationConfig } from '../api/notifications'

type SortField = 'bucket' | 'webhookURL' | 'events'
type SortDir = 'asc' | 'desc'

export default function NotificationsPage() {
  const [configs, setConfigs] = useState<NotificationConfig[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [sortField, setSortField] = useState<SortField>('bucket')
  const [sortDir, setSortDir] = useState<SortDir>('asc')

  const fetchData = useCallback(async () => {
    try {
      const data = await listNotifications()
      setConfigs(data || [])
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load notifications')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { fetchData() }, [fetchData])

  const handleSort = (field: SortField) => {
    if (sortField === field) {
      setSortDir(d => d === 'asc' ? 'desc' : 'asc')
    } else {
      setSortField(field)
      setSortDir('asc')
    }
  }

  const sorted = useMemo(() => {
    const s = [...configs]
    s.sort((a, b) => {
      let cmp = 0
      switch (sortField) {
        case 'bucket': cmp = a.bucket.localeCompare(b.bucket); break
        case 'webhookURL': cmp = (a.webhookURL || '').localeCompare(b.webhookURL || ''); break
        case 'events': cmp = (a.events?.length || 0) - (b.events?.length || 0); break
      }
      return sortDir === 'asc' ? cmp : -cmp
    })
    return s
  }, [configs, sortField, sortDir])

  const SortHeader = ({ field, label }: { field: SortField; label: string }) => (
    <th
      onClick={() => handleSort(field)}
      className="text-left px-4 py-3 text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider cursor-pointer hover:text-indigo-600 dark:hover:text-indigo-400 select-none"
    >
      <span className="inline-flex items-center gap-1">
        {label}
        {sortField === field && (
          <span className="text-indigo-600 dark:text-indigo-400">{sortDir === 'asc' ? '\u2191' : '\u2193'}</span>
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
        <h2 className="text-xl font-semibold text-gray-900 dark:text-white">Notifications</h2>
        <p className="text-sm text-gray-500 dark:text-gray-400 mt-0.5">Event notification configurations</p>
      </div>

      {error && (
        <div className="mb-4 p-3 rounded-lg bg-red-50 dark:bg-red-900/20 text-red-700 dark:text-red-400 text-sm">
          {error}
        </div>
      )}

      {configs.length === 0 ? (
        <div className="mb-6 p-5 rounded-xl bg-indigo-50 dark:bg-indigo-900/20 border border-indigo-200 dark:border-indigo-800">
          <h3 className="text-sm font-semibold text-indigo-900 dark:text-indigo-300 mb-2">No Notifications Configured</h3>
          <p className="text-sm text-indigo-700 dark:text-indigo-400 mb-3">
            Send webhook notifications when objects are created or deleted. Configure per-bucket via the S3 API
            or enable global backends in your <code className="px-1.5 py-0.5 rounded bg-indigo-100 dark:bg-indigo-900/40 font-mono text-xs">vaults3.yaml</code> config:
          </p>
          <pre className="text-xs font-mono bg-gray-900 text-green-400 p-3 rounded-lg overflow-x-auto">{`notifications:
  max_workers: 4
  queue_size: 256
  timeout_secs: 10
  # Optional global backends:
  # kafka:
  #   enabled: true
  #   brokers: ["localhost:9092"]
  #   topic: "vaults3-events"`}</pre>
        </div>
      ) : (
        <div className="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 overflow-hidden">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-gray-200 dark:border-gray-700">
                <SortHeader field="bucket" label="Bucket" />
                <SortHeader field="webhookURL" label="Webhook URL" />
                <SortHeader field="events" label="Events" />
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-100 dark:divide-gray-700/50">
              {sorted.map((c, i) => (
                <tr key={i} className="hover:bg-gray-50 dark:hover:bg-gray-700/30 transition-colors">
                  <td className="px-4 py-3 font-medium text-gray-900 dark:text-white">{c.bucket}</td>
                  <td className="px-4 py-3 text-gray-500 dark:text-gray-400 font-mono text-xs max-w-sm truncate">{c.webhookURL}</td>
                  <td className="px-4 py-3">
                    <div className="flex flex-wrap gap-1">
                      {(c.events || []).map(ev => (
                        <span key={ev} className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-indigo-100 dark:bg-indigo-900/30 text-indigo-700 dark:text-indigo-400">
                          {ev}
                        </span>
                      ))}
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}
