import { useState, useEffect, useCallback, useMemo } from 'react'
import { listBackups, getBackupStatus, triggerBackup, type BackupRecord, type BackupStatus } from '../api/backup'
import { useToast } from '../hooks/useToast'

type BSortField = 'type' | 'target' | 'startTime' | 'objects' | 'size' | 'status'
type BSortDir = 'asc' | 'desc'

function formatSize(bytes: number): string {
  if (bytes === 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(1024))
  return `${(bytes / Math.pow(1024, i)).toFixed(i === 0 ? 0 : 1)} ${units[i]}`
}

export default function BackupPage() {
  const [backups, setBackups] = useState<BackupRecord[]>([])
  const [status, setStatus] = useState<BackupStatus | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [triggering, setTriggering] = useState(false)
  const { addToast } = useToast()
  const [bSortField, setBSortField] = useState<BSortField>('startTime')
  const [bSortDir, setBSortDir] = useState<BSortDir>('desc')

  const fetchData = useCallback(async () => {
    try {
      const s = await getBackupStatus()
      setStatus(s)
    } catch {
      setStatus({ enabled: false, running: false, targets: 0 })
    }
    try {
      const b = await listBackups()
      setBackups(b || [])
    } catch {
      setBackups([])
    }
    setLoading(false)
  }, [])

  useEffect(() => { fetchData() }, [fetchData])

  const handleTrigger = async () => {
    setTriggering(true)
    setError('')
    try {
      await triggerBackup()
      addToast('success', 'Backup triggered')
      fetchData()
    } catch (err) {
      addToast('error', err instanceof Error ? err.message : 'Trigger failed')
    } finally {
      setTriggering(false)
    }
  }

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-indigo-600" />
      </div>
    )
  }

  const handleBSort = (field: BSortField) => {
    if (bSortField === field) {
      setBSortDir(d => d === 'asc' ? 'desc' : 'asc')
    } else {
      setBSortField(field)
      setBSortDir('asc')
    }
  }

  const sortedBackups = useMemo(() => {
    const s = [...backups]
    s.sort((a, b) => {
      let cmp = 0
      switch (bSortField) {
        case 'type': cmp = a.type.localeCompare(b.type); break
        case 'target': cmp = a.target.localeCompare(b.target); break
        case 'startTime': cmp = (a.startTime || '').localeCompare(b.startTime || ''); break
        case 'objects': cmp = a.objects - b.objects; break
        case 'size': cmp = a.size - b.size; break
        case 'status': cmp = a.status.localeCompare(b.status); break
      }
      return bSortDir === 'asc' ? cmp : -cmp
    })
    return s
  }, [backups, bSortField, bSortDir])

  const BSortHeader = ({ field, label }: { field: BSortField; label: string }) => (
    <th
      onClick={() => handleBSort(field)}
      className="text-left px-4 py-3 text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider cursor-pointer hover:text-indigo-600 dark:hover:text-indigo-400 select-none"
    >
      <span className="inline-flex items-center gap-1">
        {label}
        {bSortField === field && (
          <span className="text-indigo-600 dark:text-indigo-400">{bSortDir === 'asc' ? '\u2191' : '\u2193'}</span>
        )}
      </span>
    </th>
  )

  const statusColor = (s: string) => {
    switch (s) {
      case 'completed': return 'bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-400'
      case 'running': return 'bg-amber-100 dark:bg-amber-900/30 text-amber-700 dark:text-amber-400'
      case 'failed': return 'bg-red-100 dark:bg-red-900/30 text-red-700 dark:text-red-400'
      default: return 'bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-400'
    }
  }

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <div>
          <h2 className="text-xl font-semibold text-gray-900 dark:text-white">Backups</h2>
          <p className="text-sm text-gray-500 dark:text-gray-400 mt-0.5">Backup history and management</p>
        </div>
        <button onClick={handleTrigger} disabled={triggering}
          className="px-4 py-2 rounded-lg bg-indigo-600 hover:bg-indigo-700 disabled:bg-indigo-400 text-white text-sm font-medium transition-colors">
          {triggering ? 'Starting...' : 'Trigger Backup'}
        </button>
      </div>

      {error && (
        <div className="mb-4 p-3 rounded-lg bg-red-50 dark:bg-red-900/20 text-red-700 dark:text-red-400 text-sm">
          {error}
        </div>
      )}

      {status && !status.enabled && (
        <div className="mb-6 p-5 rounded-xl bg-indigo-50 dark:bg-indigo-900/20 border border-indigo-200 dark:border-indigo-800">
          <h3 className="text-sm font-semibold text-indigo-900 dark:text-indigo-300 mb-2">Backups Not Enabled</h3>
          <p className="text-sm text-indigo-700 dark:text-indigo-400 mb-3">
            Schedule automatic full or incremental backups to local directories.
            Enable them in your <code className="px-1.5 py-0.5 rounded bg-indigo-100 dark:bg-indigo-900/40 font-mono text-xs">vaults3.yaml</code> config:
          </p>
          <pre className="text-xs font-mono bg-gray-900 text-green-400 p-3 rounded-lg overflow-x-auto">{`backup:
  enabled: true
  targets:
    - name: "local-backup"
      type: "local"
      path: "/backups/vaults3"
  schedule_cron: "0 2 * * *"
  retention_days: 30`}</pre>
        </div>
      )}

      {/* Status cards */}
      <div className="grid grid-cols-3 gap-4 mb-6">
        {[
          { label: 'Status', value: status?.enabled ? 'Enabled' : 'Disabled', color: status?.enabled ? 'text-green-600 dark:text-green-400' : 'text-gray-500' },
          { label: 'Running', value: status?.running ? 'Yes' : 'No', color: status?.running ? 'text-amber-600 dark:text-amber-400' : '' },
          { label: 'Targets', value: status?.targets ?? 0 },
        ].map(card => (
          <div key={card.label} className="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-4">
            <p className="text-xs text-gray-500 dark:text-gray-400">{card.label}</p>
            <p className={`text-2xl font-bold mt-1 ${card.color || 'text-gray-900 dark:text-white'}`}>{card.value}</p>
          </div>
        ))}
      </div>

      {/* Backup history table */}
      <div className="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 overflow-hidden">
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-gray-200 dark:border-gray-700">
                <th className="text-left px-4 py-3 text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">ID</th>
                <BSortHeader field="type" label="Type" />
                <BSortHeader field="target" label="Target" />
                <BSortHeader field="startTime" label="Started" />
                <th className="text-left px-4 py-3 text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Ended</th>
                <BSortHeader field="objects" label="Objects" />
                <BSortHeader field="size" label="Size" />
                <BSortHeader field="status" label="Status" />
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-100 dark:divide-gray-700/50">
              {sortedBackups.map(b => (
                <tr key={b.id} className="hover:bg-gray-50 dark:hover:bg-gray-700/30 transition-colors">
                  <td className="px-4 py-3 font-mono text-xs text-gray-900 dark:text-white">{b.id.slice(0, 8)}</td>
                  <td className="px-4 py-3 text-gray-700 dark:text-gray-300">{b.type}</td>
                  <td className="px-4 py-3 text-gray-500 dark:text-gray-400">{b.target}</td>
                  <td className="px-4 py-3 text-gray-500 dark:text-gray-400 text-xs whitespace-nowrap">
                    {b.startTime ? new Date(b.startTime).toLocaleString() : '-'}
                  </td>
                  <td className="px-4 py-3 text-gray-500 dark:text-gray-400 text-xs whitespace-nowrap">
                    {b.endTime ? new Date(b.endTime).toLocaleString() : '-'}
                  </td>
                  <td className="px-4 py-3 text-gray-500 dark:text-gray-400">{b.objects}</td>
                  <td className="px-4 py-3 text-gray-500 dark:text-gray-400">{formatSize(b.size)}</td>
                  <td className="px-4 py-3">
                    <span className={`inline-flex items-center px-2 py-0.5 rounded text-xs font-medium ${statusColor(b.status)}`}>
                      {b.status}
                    </span>
                  </td>
                </tr>
              ))}
              {backups.length === 0 && (
                <tr><td colSpan={8} className="px-4 py-8 text-center text-gray-400">No backups yet</td></tr>
              )}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  )
}
