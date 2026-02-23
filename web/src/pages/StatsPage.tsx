import { useState, useEffect } from 'react'
import { getStats, type Stats } from '../api/stats'

export default function StatsPage() {
  const [stats, setStats] = useState<Stats | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  useEffect(() => {
    getStats()
      .then(setStats)
      .catch((err) => setError(err instanceof Error ? err.message : 'Failed to load stats'))
      .finally(() => setLoading(false))
  }, [])

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-indigo-600" />
      </div>
    )
  }

  if (error || !stats) {
    return (
      <div className="p-3 rounded-lg bg-red-50 dark:bg-red-900/20 text-red-700 dark:text-red-400 text-sm">
        {error || 'Failed to load stats'}
      </div>
    )
  }

  const maxSize = Math.max(...stats.buckets.map((b) => b.size), 1)

  return (
    <div>
      <h2 className="text-xl font-semibold text-gray-900 dark:text-white mb-6">Storage Stats</h2>

      {/* Stat cards */}
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-4 mb-8">
        <StatCard label="Total Storage" value={formatSize(stats.totalSize)} />
        <StatCard label="Total Objects" value={String(stats.totalObjects)} />
        <StatCard label="Buckets" value={String(stats.totalBuckets)} />
        <StatCard label="Uptime" value={formatUptime(stats.uptimeSeconds)} />
      </div>

      {/* Runtime info */}
      <div className="grid grid-cols-2 gap-4 mb-8">
        <div className="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-4">
          <p className="text-xs text-gray-500 dark:text-gray-400 uppercase tracking-wider font-medium mb-1">Goroutines</p>
          <p className="text-2xl font-semibold text-gray-900 dark:text-white">{stats.goroutines}</p>
        </div>
        <div className="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-4">
          <p className="text-xs text-gray-500 dark:text-gray-400 uppercase tracking-wider font-medium mb-1">Memory</p>
          <p className="text-2xl font-semibold text-gray-900 dark:text-white">{stats.memoryMB.toFixed(1)} MB</p>
        </div>
      </div>

      {/* Per-bucket breakdown */}
      {stats.buckets.length > 0 && (
        <div className="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-6">
          <h3 className="text-sm font-semibold text-gray-900 dark:text-white mb-4">Per-Bucket Storage</h3>
          <div className="space-y-3">
            {stats.buckets.map((b) => (
              <div key={b.name}>
                <div className="flex items-center justify-between text-sm mb-1">
                  <span className="text-gray-700 dark:text-gray-300 font-medium">{b.name}</span>
                  <span className="text-gray-500 dark:text-gray-400">
                    {formatSize(b.size)} &middot; {b.objectCount} object{b.objectCount !== 1 ? 's' : ''}
                  </span>
                </div>
                <div className="w-full bg-gray-100 dark:bg-gray-700 rounded-full h-2">
                  <div
                    className="bg-indigo-600 h-2 rounded-full transition-all"
                    style={{ width: `${Math.max((b.size / maxSize) * 100, 1)}%` }}
                  />
                </div>
                {(b.maxSizeBytes || b.maxObjects) && (
                  <p className="text-xs text-gray-400 dark:text-gray-500 mt-0.5">
                    Quota: {b.maxSizeBytes ? formatSize(b.maxSizeBytes) : 'unlimited'} / {b.maxObjects ? `${b.maxObjects} objects` : 'unlimited'}
                  </p>
                )}
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  )
}

function StatCard({ label, value }: { label: string; value: string }) {
  return (
    <div className="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-4">
      <p className="text-xs text-gray-500 dark:text-gray-400 uppercase tracking-wider font-medium mb-1">{label}</p>
      <p className="text-2xl font-semibold text-gray-900 dark:text-white">{value}</p>
    </div>
  )
}

function formatSize(bytes: number): string {
  if (bytes === 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(1024))
  return `${(bytes / Math.pow(1024, i)).toFixed(i > 0 ? 1 : 0)} ${units[i]}`
}

function formatUptime(seconds: number): string {
  const d = Math.floor(seconds / 86400)
  const h = Math.floor((seconds % 86400) / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  if (d > 0) return `${d}d ${h}h`
  if (h > 0) return `${h}h ${m}m`
  return `${m}m`
}
