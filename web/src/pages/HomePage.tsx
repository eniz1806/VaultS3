import { useState, useEffect, useCallback, useMemo } from 'react'
import { Link } from 'react-router-dom'
import { getStats, type Stats } from '../api/stats'
import { getActivity, type ActivityEntry } from '../api/activity'
import Sparkline from '../components/Sparkline'

export default function HomePage() {
  const [stats, setStats] = useState<Stats | null>(null)
  const [activity, setActivity] = useState<ActivityEntry[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  const fetchData = useCallback(async () => {
    try {
      const [s, a] = await Promise.all([getStats(), getActivity(50)])
      setStats(s)
      setActivity(a || [])
      setError('')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { fetchData() }, [fetchData])

  const sparklineData = useMemo(() => {
    if (activity.length < 2) return []
    const bucketCount = 20
    const chunkSize = Math.ceil(activity.length / bucketCount)
    const buckets: number[] = []
    for (let i = 0; i < bucketCount; i++) {
      const chunk = activity.slice(i * chunkSize, (i + 1) * chunkSize)
      buckets.push(chunk.length)
    }
    return buckets.reverse()
  }, [activity])

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
        {error || 'Failed to load'}
      </div>
    )
  }

  return (
    <div>
      <h2 className="text-xl font-semibold text-gray-900 dark:text-white mb-6">Dashboard</h2>

      {/* Summary cards */}
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-4 mb-6">
        <SummaryCard label="Buckets" value={String(stats.totalBuckets)} to="/buckets" color="indigo" />
        <SummaryCard label="Objects" value={stats.totalObjects.toLocaleString()} color="blue" />
        <SummaryCard label="Storage" value={formatSize(stats.totalSize)} color="emerald" />
        <SummaryCard label="Requests" value={stats.totalRequests.toLocaleString()} to="/stats" color="amber" />
      </div>

      {/* Activity sparkline + runtime */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-4 mb-6">
        <div className="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-5">
          <h3 className="text-sm font-semibold text-gray-900 dark:text-white mb-3">Recent Activity</h3>
          {sparklineData.length > 1 ? (
            <Sparkline data={sparklineData} height={48} />
          ) : (
            <p className="text-xs text-gray-400 dark:text-gray-500">No activity data</p>
          )}
          {activity.length > 0 && (
            <div className="mt-3 space-y-1.5">
              {activity.slice(0, 5).map((a, i) => (
                <div key={i} className="flex items-center justify-between text-xs">
                  <span className="text-gray-600 dark:text-gray-400">
                    <span className={`inline-block w-10 font-mono font-medium ${methodColor(a.method)}`}>{a.method}</span>
                    {' '}{a.bucket}{a.key ? '/' + a.key : ''}
                  </span>
                  <span className="text-gray-400 dark:text-gray-500">{timeAgo(a.time)}</span>
                </div>
              ))}
            </div>
          )}
        </div>

        <div className="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-5">
          <h3 className="text-sm font-semibold text-gray-900 dark:text-white mb-3">System</h3>
          <div className="grid grid-cols-2 gap-3">
            <MiniStat label="Uptime" value={formatUptime(stats.uptimeSeconds)} />
            <MiniStat label="Memory" value={`${stats.memoryMB.toFixed(1)} MB`} />
            <MiniStat label="Goroutines" value={String(stats.goroutines)} />
            <MiniStat label="Errors" value={stats.totalErrors.toLocaleString()} />
            <MiniStat label="Bytes In" value={formatSize(stats.bytesIn)} />
            <MiniStat label="Bytes Out" value={formatSize(stats.bytesOut)} />
          </div>
        </div>
      </div>

      {/* Quick actions */}
      <div className="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-5">
        <h3 className="text-sm font-semibold text-gray-900 dark:text-white mb-3">Quick Actions</h3>
        <div className="flex flex-wrap gap-2">
          <QuickLink to="/buckets" label="Browse Buckets" />
          <QuickLink to="/search" label="Search Objects" />
          <QuickLink to="/access-keys" label="Manage Keys" />
          <QuickLink to="/iam" label="IAM Users" />
          <QuickLink to="/audit" label="Audit Trail" />
          <QuickLink to="/stats" label="Storage Stats" />
          <QuickLink to="/activity" label="Activity Log" />
          <QuickLink to="/settings" label="Settings" />
        </div>
      </div>
    </div>
  )
}

function SummaryCard({ label, value, to, color }: { label: string; value: string; to?: string; color: string }) {
  const colorMap: Record<string, string> = {
    indigo: 'bg-indigo-50 dark:bg-indigo-900/20 border-indigo-200 dark:border-indigo-800',
    blue: 'bg-blue-50 dark:bg-blue-900/20 border-blue-200 dark:border-blue-800',
    emerald: 'bg-emerald-50 dark:bg-emerald-900/20 border-emerald-200 dark:border-emerald-800',
    amber: 'bg-amber-50 dark:bg-amber-900/20 border-amber-200 dark:border-amber-800',
  }
  const content = (
    <div className={`rounded-xl border p-4 transition-colors ${colorMap[color] || colorMap.indigo} ${to ? 'hover:shadow-md cursor-pointer' : ''}`}>
      <p className="text-xs text-gray-500 dark:text-gray-400 uppercase tracking-wider font-medium mb-1">{label}</p>
      <p className="text-2xl font-semibold text-gray-900 dark:text-white">{value}</p>
    </div>
  )
  return to ? <Link to={to}>{content}</Link> : content
}

function MiniStat({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <p className="text-xs text-gray-500 dark:text-gray-400 mb-0.5">{label}</p>
      <p className="text-sm font-semibold text-gray-900 dark:text-white">{value}</p>
    </div>
  )
}

function QuickLink({ to, label }: { to: string; label: string }) {
  return (
    <Link
      to={to}
      className="px-3 py-1.5 rounded-lg border border-gray-200 dark:border-gray-700 text-sm font-medium text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors"
    >
      {label}
    </Link>
  )
}

function methodColor(method: string): string {
  switch (method) {
    case 'GET': return 'text-green-600 dark:text-green-400'
    case 'PUT': return 'text-blue-600 dark:text-blue-400'
    case 'POST': return 'text-amber-600 dark:text-amber-400'
    case 'DELETE': return 'text-red-600 dark:text-red-400'
    case 'HEAD': return 'text-purple-600 dark:text-purple-400'
    default: return 'text-gray-600 dark:text-gray-400'
  }
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

function timeAgo(iso: string): string {
  const diff = (Date.now() - new Date(iso).getTime()) / 1000
  if (diff < 60) return `${Math.floor(diff)}s ago`
  if (diff < 3600) return `${Math.floor(diff / 60)}m ago`
  if (diff < 86400) return `${Math.floor(diff / 3600)}h ago`
  return `${Math.floor(diff / 86400)}d ago`
}
