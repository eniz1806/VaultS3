import { useState, useEffect, useCallback } from 'react'
import { listBackups, getBackupStatus, triggerBackup, type BackupRecord, type BackupStatus } from '../api/backup'

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

  const fetchData = useCallback(async () => {
    try {
      const [b, s] = await Promise.all([listBackups(), getBackupStatus()])
      setBackups(b || [])
      setStatus(s)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load backup data')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { fetchData() }, [fetchData])

  const handleTrigger = async () => {
    setTriggering(true)
    setError('')
    try {
      await triggerBackup()
      fetchData()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Trigger failed')
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
                <th className="text-left px-4 py-3 text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Type</th>
                <th className="text-left px-4 py-3 text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Target</th>
                <th className="text-left px-4 py-3 text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Started</th>
                <th className="text-left px-4 py-3 text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Ended</th>
                <th className="text-left px-4 py-3 text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Objects</th>
                <th className="text-left px-4 py-3 text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Size</th>
                <th className="text-left px-4 py-3 text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Status</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-100 dark:divide-gray-700/50">
              {backups.map(b => (
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
