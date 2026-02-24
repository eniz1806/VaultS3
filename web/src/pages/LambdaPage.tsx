import { useState, useEffect, useCallback, useMemo } from 'react'
import { getLambdaStatus, listLambdaTriggers, deleteBucketTriggers, type LambdaStatus, type BucketTriggers } from '../api/lambda'
import { useToast } from '../hooks/useToast'

type LSortField = 'bucket' | 'functionURL' | 'events' | 'keyFilter'
type LSortDir = 'asc' | 'desc'

export default function LambdaPage() {
  const [status, setStatus] = useState<LambdaStatus | null>(null)
  const [triggers, setTriggers] = useState<BucketTriggers[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [deleteTarget, setDeleteTarget] = useState<string | null>(null)
  const { addToast } = useToast()
  const [lSortField, setLSortField] = useState<LSortField>('bucket')
  const [lSortDir, setLSortDir] = useState<LSortDir>('asc')

  const fetchData = useCallback(async () => {
    try {
      const s = await getLambdaStatus()
      setStatus(s)
    } catch {
      setStatus({ enabled: false, totalTriggers: 0, buckets: 0, queueDepth: 0 })
    }
    try {
      const t = await listLambdaTriggers()
      setTriggers(t || [])
    } catch {
      setTriggers([])
    }
    setLoading(false)
  }, [])

  useEffect(() => { fetchData() }, [fetchData])

  const handleDeleteTriggers = async (bucket: string) => {
    setError('')
    try {
      await deleteBucketTriggers(bucket)
      setDeleteTarget(null)
      addToast('success', `Triggers for "${bucket}" deleted`)
      fetchData()
    } catch (err) {
      addToast('error', err instanceof Error ? err.message : 'Delete failed')
    }
  }

  const handleLSort = (field: LSortField) => {
    if (lSortField === field) {
      setLSortDir(d => d === 'asc' ? 'desc' : 'asc')
    } else {
      setLSortField(field)
      setLSortDir('asc')
    }
  }

  // Flatten triggers for sorting
  const flatTriggers = useMemo(() => {
    const flat = triggers.flatMap(bt => (bt.triggers || []).map(t => ({ bucket: bt.bucket, ...t })))
    flat.sort((a, b) => {
      let cmp = 0
      switch (lSortField) {
        case 'bucket': cmp = a.bucket.localeCompare(b.bucket); break
        case 'functionURL': cmp = (a.functionURL || '').localeCompare(b.functionURL || ''); break
        case 'events': cmp = (a.events?.length || 0) - (b.events?.length || 0); break
        case 'keyFilter': cmp = (a.keyFilter || '').localeCompare(b.keyFilter || ''); break
      }
      return lSortDir === 'asc' ? cmp : -cmp
    })
    return flat
  }, [triggers, lSortField, lSortDir])

  const LSortHeader = ({ field, label }: { field: LSortField; label: string }) => (
    <th
      onClick={() => handleLSort(field)}
      className="text-left px-4 py-3 text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider cursor-pointer hover:text-indigo-600 dark:hover:text-indigo-400 select-none"
    >
      <span className="inline-flex items-center gap-1">
        {label}
        {lSortField === field && (
          <span className="text-indigo-600 dark:text-indigo-400">{lSortDir === 'asc' ? '\u2191' : '\u2193'}</span>
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
        <h2 className="text-xl font-semibold text-gray-900 dark:text-white">Lambda Triggers</h2>
        <p className="text-sm text-gray-500 dark:text-gray-400 mt-0.5">Event-driven function triggers</p>
      </div>

      {error && (
        <div className="mb-4 p-3 rounded-lg bg-red-50 dark:bg-red-900/20 text-red-700 dark:text-red-400 text-sm">
          {error}
        </div>
      )}

      {status && !status.enabled && (
        <div className="mb-6 p-5 rounded-xl bg-indigo-50 dark:bg-indigo-900/20 border border-indigo-200 dark:border-indigo-800">
          <h3 className="text-sm font-semibold text-indigo-900 dark:text-indigo-300 mb-2">Lambda Triggers Not Enabled</h3>
          <p className="text-sm text-indigo-700 dark:text-indigo-400 mb-3">
            Lambda triggers call external webhook URLs when S3 events occur (object created, deleted, etc.).
            Enable them in your <code className="px-1.5 py-0.5 rounded bg-indigo-100 dark:bg-indigo-900/40 font-mono text-xs">vaults3.yaml</code> config:
          </p>
          <pre className="text-xs font-mono bg-gray-900 text-green-400 p-3 rounded-lg overflow-x-auto">{`lambda:
  enabled: true
  timeout_secs: 30
  max_workers: 4
  queue_size: 256`}</pre>
        </div>
      )}

      {/* Status cards */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4 mb-6">
        {[
          { label: 'Status', value: status?.enabled ? 'Enabled' : 'Disabled', color: status?.enabled ? 'text-green-600 dark:text-green-400' : 'text-gray-500' },
          { label: 'Total Triggers', value: status?.totalTriggers ?? 0 },
          { label: 'Buckets', value: status?.buckets ?? 0 },
          { label: 'Queue Depth', value: status?.queueDepth ?? 0 },
        ].map(card => (
          <div key={card.label} className="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-4">
            <p className="text-xs text-gray-500 dark:text-gray-400">{card.label}</p>
            <p className={`text-2xl font-bold mt-1 ${card.color || 'text-gray-900 dark:text-white'}`}>{card.value}</p>
          </div>
        ))}
      </div>

      {/* Triggers table */}
      <div className="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 overflow-hidden">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-gray-200 dark:border-gray-700">
              <LSortHeader field="bucket" label="Bucket" />
              <LSortHeader field="functionURL" label="Function URL" />
              <LSortHeader field="events" label="Events" />
              <LSortHeader field="keyFilter" label="Key Filter" />
              <th className="text-right px-4 py-3 text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Actions</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-100 dark:divide-gray-700/50">
            {flatTriggers.map((t, i) => (
              <tr key={`${t.bucket}-${i}`} className="hover:bg-gray-50 dark:hover:bg-gray-700/30 transition-colors">
                <td className="px-4 py-3 font-medium text-gray-900 dark:text-white">{t.bucket}</td>
                <td className="px-4 py-3 text-gray-500 dark:text-gray-400 font-mono text-xs max-w-xs truncate">{t.functionURL}</td>
                <td className="px-4 py-3">
                  <div className="flex flex-wrap gap-1">
                    {(t.events || []).map(ev => (
                      <span key={ev} className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-purple-100 dark:bg-purple-900/30 text-purple-700 dark:text-purple-400">
                        {ev}
                      </span>
                    ))}
                  </div>
                </td>
                <td className="px-4 py-3 text-gray-500 dark:text-gray-400 font-mono text-xs">{t.keyFilter || '*'}</td>
                <td className="px-4 py-3 text-right">
                  <button onClick={() => setDeleteTarget(t.bucket)}
                    className="text-gray-400 hover:text-red-600 dark:hover:text-red-400 transition-colors" title="Delete triggers">
                    <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                      <path strokeLinecap="round" strokeLinejoin="round" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                    </svg>
                  </button>
                </td>
              </tr>
            ))}
            {flatTriggers.length === 0 && (
              <tr><td colSpan={5} className="px-4 py-8 text-center text-gray-400">No lambda triggers configured</td></tr>
            )}
          </tbody>
        </table>
      </div>

      {/* Delete confirmation */}
      {deleteTarget && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40">
          <div className="bg-white dark:bg-gray-800 rounded-xl shadow-xl border border-gray-200 dark:border-gray-700 p-6 w-full max-w-sm mx-4">
            <h3 className="text-lg font-semibold text-gray-900 dark:text-white mb-2">Delete Triggers</h3>
            <p className="text-sm text-gray-600 dark:text-gray-400 mb-4">
              Remove all triggers for bucket <strong>{deleteTarget}</strong>?
            </p>
            <div className="flex gap-2 justify-end">
              <button onClick={() => setDeleteTarget(null)}
                className="px-4 py-2 rounded-lg text-sm text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors">Cancel</button>
              <button onClick={() => handleDeleteTriggers(deleteTarget)}
                className="px-4 py-2 rounded-lg bg-red-600 hover:bg-red-700 text-white text-sm font-medium transition-colors">Delete</button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
