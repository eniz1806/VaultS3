import { useState, useEffect } from 'react'
import { useParams, Link } from 'react-router-dom'
import { getBucket, setBucketPolicy, setBucketQuota, type Bucket } from '../api/buckets'

export default function BucketDetailPage() {
  const { name } = useParams<{ name: string }>()
  const [bucket, setBucket] = useState<Bucket | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [success, setSuccess] = useState('')

  // Quota state
  const [maxSizeBytes, setMaxSizeBytes] = useState('')
  const [maxObjects, setMaxObjects] = useState('')
  const [savingQuota, setSavingQuota] = useState(false)

  // Policy state
  const [policyText, setPolicyText] = useState('')
  const [savingPolicy, setSavingPolicy] = useState(false)

  useEffect(() => {
    if (!name) return
    setLoading(true)
    getBucket(name)
      .then((b) => {
        setBucket(b)
        setMaxSizeBytes(b.maxSizeBytes ? String(b.maxSizeBytes) : '')
        setMaxObjects(b.maxObjects ? String(b.maxObjects) : '')
        setPolicyText(b.policy ? JSON.stringify(b.policy, null, 2) : '')
      })
      .catch((err) => setError(err instanceof Error ? err.message : 'Failed to load bucket'))
      .finally(() => setLoading(false))
  }, [name])

  const handleSaveQuota = async () => {
    if (!name) return
    setSavingQuota(true)
    setError('')
    setSuccess('')
    try {
      await setBucketQuota(name, Number(maxSizeBytes) || 0, Number(maxObjects) || 0)
      setSuccess('Quota updated')
      const b = await getBucket(name)
      setBucket(b)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to update quota')
    } finally {
      setSavingQuota(false)
    }
  }

  const handleSavePolicy = async () => {
    if (!name) return
    setSavingPolicy(true)
    setError('')
    setSuccess('')
    try {
      await setBucketPolicy(name, policyText)
      setSuccess('Policy updated')
      const b = await getBucket(name)
      setBucket(b)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to update policy')
    } finally {
      setSavingPolicy(false)
    }
  }

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-indigo-600" />
      </div>
    )
  }

  if (!bucket) {
    return <div className="text-red-600 dark:text-red-400">{error || 'Bucket not found'}</div>
  }

  return (
    <div className="max-w-3xl">
      {/* Breadcrumb */}
      <div className="flex items-center gap-2 text-sm text-gray-500 dark:text-gray-400 mb-4">
        <Link to="/buckets" className="hover:text-indigo-600 dark:hover:text-indigo-400">Buckets</Link>
        <span>/</span>
        <span className="text-gray-900 dark:text-white font-medium">{bucket.name}</span>
      </div>

      <h2 className="text-xl font-semibold text-gray-900 dark:text-white mb-6">{bucket.name}</h2>

      {error && (
        <div className="mb-4 p-3 rounded-lg bg-red-50 dark:bg-red-900/20 text-red-700 dark:text-red-400 text-sm">
          {error}
        </div>
      )}
      {success && (
        <div className="mb-4 p-3 rounded-lg bg-green-50 dark:bg-green-900/20 text-green-700 dark:text-green-400 text-sm">
          {success}
        </div>
      )}

      {/* Info cards */}
      <div className="grid grid-cols-2 gap-4 mb-6">
        <InfoCard label="Objects" value={String(bucket.objectCount)} />
        <InfoCard label="Size" value={formatSize(bucket.size)} />
        <InfoCard label="Created" value={formatDate(bucket.createdAt)} />
        <InfoCard label="Quota" value={bucket.maxSizeBytes ? formatSize(bucket.maxSizeBytes) : 'Unlimited'} />
      </div>

      {/* Quota editor */}
      <Section title="Quota">
        <div className="grid grid-cols-2 gap-4 mb-3">
          <div>
            <label className="block text-xs font-medium text-gray-500 dark:text-gray-400 mb-1">Max Size (bytes)</label>
            <input
              type="number"
              value={maxSizeBytes}
              onChange={(e) => setMaxSizeBytes(e.target.value)}
              className="w-full px-3 py-2 rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 text-gray-900 dark:text-white text-sm focus:ring-2 focus:ring-indigo-500 focus:border-transparent outline-none"
              placeholder="0 = unlimited"
            />
          </div>
          <div>
            <label className="block text-xs font-medium text-gray-500 dark:text-gray-400 mb-1">Max Objects</label>
            <input
              type="number"
              value={maxObjects}
              onChange={(e) => setMaxObjects(e.target.value)}
              className="w-full px-3 py-2 rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 text-gray-900 dark:text-white text-sm focus:ring-2 focus:ring-indigo-500 focus:border-transparent outline-none"
              placeholder="0 = unlimited"
            />
          </div>
        </div>
        <button
          onClick={handleSaveQuota}
          disabled={savingQuota}
          className="px-4 py-2 rounded-lg bg-indigo-600 hover:bg-indigo-700 disabled:bg-indigo-400 text-white text-sm font-medium transition-colors"
        >
          {savingQuota ? 'Saving...' : 'Save Quota'}
        </button>
      </Section>

      {/* Policy editor */}
      <Section title="Bucket Policy">
        <textarea
          value={policyText}
          onChange={(e) => setPolicyText(e.target.value)}
          rows={10}
          className="w-full px-3 py-2 rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 text-gray-900 dark:text-white text-sm font-mono focus:ring-2 focus:ring-indigo-500 focus:border-transparent outline-none mb-3"
          placeholder='{"Version":"2012-10-17","Statement":[...]}'
        />
        <button
          onClick={handleSavePolicy}
          disabled={savingPolicy || !policyText.trim()}
          className="px-4 py-2 rounded-lg bg-indigo-600 hover:bg-indigo-700 disabled:bg-indigo-400 text-white text-sm font-medium transition-colors"
        >
          {savingPolicy ? 'Saving...' : 'Save Policy'}
        </button>
      </Section>
    </div>
  )
}

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div className="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-5 mb-4">
      <h3 className="text-sm font-semibold text-gray-900 dark:text-white mb-3">{title}</h3>
      {children}
    </div>
  )
}

function InfoCard({ label, value }: { label: string; value: string }) {
  return (
    <div className="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-4">
      <p className="text-xs text-gray-500 dark:text-gray-400 mb-1">{label}</p>
      <p className="text-lg font-semibold text-gray-900 dark:text-white">{value}</p>
    </div>
  )
}

function formatSize(bytes: number): string {
  if (bytes === 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(1024))
  return `${(bytes / Math.pow(1024, i)).toFixed(i > 0 ? 1 : 0)} ${units[i]}`
}

function formatDate(iso: string): string {
  return new Date(iso).toLocaleDateString(undefined, { year: 'numeric', month: 'short', day: 'numeric' })
}
