import { useState, useEffect } from 'react'
import { useParams, Link } from 'react-router-dom'
import {
  getBucket, setBucketPolicy, setBucketQuota,
  getBucketVersioning, setBucketVersioning,
  getLifecycleRule, setLifecycleRule, deleteLifecycleRule,
  getCORSConfig, setCORSConfig, deleteCORSConfig,
  type Bucket, type LifecycleRule, type CORSRule,
} from '../api/buckets'

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

  // Versioning state
  const [versioning, setVersioning] = useState('')
  const [savingVersioning, setSavingVersioning] = useState(false)

  // Lifecycle state
  const [lifecycleRule, setLifecycleRuleState] = useState<LifecycleRule | null>(null)
  const [lcExpDays, setLcExpDays] = useState('')
  const [lcPrefix, setLcPrefix] = useState('')
  const [lcStatus, setLcStatus] = useState('Enabled')
  const [savingLifecycle, setSavingLifecycle] = useState(false)

  // CORS state
  const [corsRules, setCorsRules] = useState<CORSRule[]>([])
  const [corsText, setCorsText] = useState('')
  const [savingCors, setSavingCors] = useState(false)

  useEffect(() => {
    if (!name) return
    setLoading(true)

    Promise.all([
      getBucket(name),
      getBucketVersioning(name),
      getLifecycleRule(name),
      getCORSConfig(name),
    ])
      .then(([b, v, lc, cors]) => {
        setBucket(b)
        setMaxSizeBytes(b.maxSizeBytes ? String(b.maxSizeBytes) : '')
        setMaxObjects(b.maxObjects ? String(b.maxObjects) : '')
        setPolicyText(b.policy ? JSON.stringify(b.policy, null, 2) : '')

        setVersioning(v.versioning || '')

        if (lc.rule) {
          setLifecycleRuleState(lc.rule)
          setLcExpDays(String(lc.rule.expirationDays))
          setLcPrefix(lc.rule.prefix || '')
          setLcStatus(lc.rule.status || 'Enabled')
        }

        setCorsRules(cors.rules || [])
        if (cors.rules && cors.rules.length > 0) {
          setCorsText(JSON.stringify(cors.rules, null, 2))
        }
      })
      .catch((err) => setError(err instanceof Error ? err.message : 'Failed to load bucket'))
      .finally(() => setLoading(false))
  }, [name])

  const flash = (msg: string) => { setSuccess(msg); setTimeout(() => setSuccess(''), 3000) }

  const handleSaveQuota = async () => {
    if (!name) return
    setSavingQuota(true); setError('')
    try {
      await setBucketQuota(name, Number(maxSizeBytes) || 0, Number(maxObjects) || 0)
      flash('Quota updated')
      setBucket(await getBucket(name))
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to update quota')
    } finally { setSavingQuota(false) }
  }

  const handleSavePolicy = async () => {
    if (!name) return
    setSavingPolicy(true); setError('')
    try {
      await setBucketPolicy(name, policyText)
      flash('Policy updated')
      setBucket(await getBucket(name))
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to update policy')
    } finally { setSavingPolicy(false) }
  }

  const handleToggleVersioning = async () => {
    if (!name) return
    setSavingVersioning(true); setError('')
    const next = versioning === 'Enabled' ? 'Suspended' : 'Enabled'
    try {
      await setBucketVersioning(name, next)
      setVersioning(next)
      flash(`Versioning ${next.toLowerCase()}`)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to update versioning')
    } finally { setSavingVersioning(false) }
  }

  const handleSaveLifecycle = async () => {
    if (!name) return
    setSavingLifecycle(true); setError('')
    try {
      const rule: LifecycleRule = {
        expirationDays: Number(lcExpDays) || 30,
        prefix: lcPrefix,
        status: lcStatus,
      }
      await setLifecycleRule(name, rule)
      setLifecycleRuleState(rule)
      flash('Lifecycle rule saved')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to save lifecycle rule')
    } finally { setSavingLifecycle(false) }
  }

  const handleDeleteLifecycle = async () => {
    if (!name) return
    setSavingLifecycle(true); setError('')
    try {
      await deleteLifecycleRule(name)
      setLifecycleRuleState(null)
      setLcExpDays(''); setLcPrefix(''); setLcStatus('Enabled')
      flash('Lifecycle rule removed')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to delete lifecycle rule')
    } finally { setSavingLifecycle(false) }
  }

  const handleSaveCors = async () => {
    if (!name) return
    setSavingCors(true); setError('')
    try {
      const rules = corsText.trim() ? JSON.parse(corsText) : []
      await setCORSConfig(name, rules)
      setCorsRules(rules)
      flash('CORS configuration saved')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to save CORS config')
    } finally { setSavingCors(false) }
  }

  const handleDeleteCors = async () => {
    if (!name) return
    setSavingCors(true); setError('')
    try {
      await deleteCORSConfig(name)
      setCorsRules([]); setCorsText('')
      flash('CORS configuration removed')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to delete CORS config')
    } finally { setSavingCors(false) }
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

      <div className="flex items-center justify-between mb-6">
        <h2 className="text-xl font-semibold text-gray-900 dark:text-white">{bucket.name}</h2>
        <Link
          to={`/buckets/${bucket.name}/files`}
          className="px-4 py-2 rounded-lg bg-indigo-600 hover:bg-indigo-700 text-white text-sm font-medium transition-colors"
        >
          Browse Files
        </Link>
      </div>

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

      {/* Versioning toggle */}
      <Section title="Versioning">
        <div className="flex items-center justify-between">
          <div>
            <p className="text-sm text-gray-700 dark:text-gray-300">
              Status: <span className={`font-medium ${versioning === 'Enabled' ? 'text-green-600 dark:text-green-400' : 'text-gray-500 dark:text-gray-400'}`}>
                {versioning || 'Not configured'}
              </span>
            </p>
            <p className="text-xs text-gray-500 dark:text-gray-400 mt-1">
              When enabled, objects are versioned on each update.
            </p>
          </div>
          <button
            onClick={handleToggleVersioning}
            disabled={savingVersioning}
            className={`relative inline-flex h-6 w-11 items-center rounded-full transition-colors ${
              versioning === 'Enabled' ? 'bg-indigo-600' : 'bg-gray-300 dark:bg-gray-600'
            } ${savingVersioning ? 'opacity-50' : ''}`}
          >
            <span className={`inline-block h-4 w-4 transform rounded-full bg-white transition-transform ${
              versioning === 'Enabled' ? 'translate-x-6' : 'translate-x-1'
            }`} />
          </button>
        </div>
      </Section>

      {/* Lifecycle rule editor */}
      <Section title="Lifecycle Rule">
        <div className="grid grid-cols-3 gap-4 mb-3">
          <div>
            <label className="block text-xs font-medium text-gray-500 dark:text-gray-400 mb-1">Expiration (days)</label>
            <input
              type="number"
              value={lcExpDays}
              onChange={e => setLcExpDays(e.target.value)}
              className="w-full px-3 py-2 rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 text-gray-900 dark:text-white text-sm focus:ring-2 focus:ring-indigo-500 focus:border-transparent outline-none"
              placeholder="30"
            />
          </div>
          <div>
            <label className="block text-xs font-medium text-gray-500 dark:text-gray-400 mb-1">Key Prefix</label>
            <input
              type="text"
              value={lcPrefix}
              onChange={e => setLcPrefix(e.target.value)}
              className="w-full px-3 py-2 rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 text-gray-900 dark:text-white text-sm focus:ring-2 focus:ring-indigo-500 focus:border-transparent outline-none"
              placeholder="logs/"
            />
          </div>
          <div>
            <label className="block text-xs font-medium text-gray-500 dark:text-gray-400 mb-1">Status</label>
            <select
              value={lcStatus}
              onChange={e => setLcStatus(e.target.value)}
              className="w-full px-3 py-2 rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 text-gray-900 dark:text-white text-sm focus:ring-2 focus:ring-indigo-500 focus:border-transparent outline-none"
            >
              <option value="Enabled">Enabled</option>
              <option value="Disabled">Disabled</option>
            </select>
          </div>
        </div>
        <div className="flex gap-2">
          <button
            onClick={handleSaveLifecycle}
            disabled={savingLifecycle || !lcExpDays}
            className="px-4 py-2 rounded-lg bg-indigo-600 hover:bg-indigo-700 disabled:bg-indigo-400 text-white text-sm font-medium transition-colors"
          >
            {savingLifecycle ? 'Saving...' : 'Save Rule'}
          </button>
          {lifecycleRule && (
            <button
              onClick={handleDeleteLifecycle}
              disabled={savingLifecycle}
              className="px-4 py-2 rounded-lg border border-red-300 dark:border-red-700 text-red-600 dark:text-red-400 hover:bg-red-50 dark:hover:bg-red-900/20 text-sm font-medium transition-colors"
            >
              Remove
            </button>
          )}
        </div>
      </Section>

      {/* CORS editor */}
      <Section title="CORS Configuration">
        <textarea
          value={corsText}
          onChange={e => setCorsText(e.target.value)}
          rows={6}
          className="w-full px-3 py-2 rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 text-gray-900 dark:text-white text-sm font-mono focus:ring-2 focus:ring-indigo-500 focus:border-transparent outline-none mb-3"
          placeholder={`[{"allowed_origins":["*"],"allowed_methods":["GET","PUT"],"allowed_headers":["*"],"max_age_secs":3600}]`}
        />
        <div className="flex gap-2">
          <button
            onClick={handleSaveCors}
            disabled={savingCors}
            className="px-4 py-2 rounded-lg bg-indigo-600 hover:bg-indigo-700 disabled:bg-indigo-400 text-white text-sm font-medium transition-colors"
          >
            {savingCors ? 'Saving...' : 'Save CORS'}
          </button>
          {corsRules.length > 0 && (
            <button
              onClick={handleDeleteCors}
              disabled={savingCors}
              className="px-4 py-2 rounded-lg border border-red-300 dark:border-red-700 text-red-600 dark:text-red-400 hover:bg-red-50 dark:hover:bg-red-900/20 text-sm font-medium transition-colors"
            >
              Remove
            </button>
          )}
        </div>
      </Section>

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
