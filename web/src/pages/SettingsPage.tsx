import { useState, useEffect, useCallback } from 'react'
import { getSettings, changeCredentials, type Settings } from '../api/settings'

export default function SettingsPage() {
  const [settings, setSettings] = useState<Settings | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  const fetchSettings = useCallback(async () => {
    try {
      const s = await getSettings()
      setSettings(s)
      setError('')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load settings')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { fetchSettings() }, [fetchSettings])

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-indigo-600" />
      </div>
    )
  }

  if (error || !settings) {
    return (
      <div className="p-3 rounded-lg bg-red-50 dark:bg-red-900/20 text-red-700 dark:text-red-400 text-sm">
        {error || 'Failed to load settings'}
      </div>
    )
  }

  const features = settings.features
  const featureList = [
    { label: 'Encryption at Rest', enabled: features.encryption },
    { label: 'Compression', enabled: features.compression },
    { label: 'Access Logging', enabled: features.accessLog },
    { label: 'Rate Limiting', enabled: features.rateLimit },
    { label: 'Replication', enabled: features.replication },
    { label: 'Virus Scanner', enabled: features.scanner },
    { label: 'Data Tiering', enabled: features.tiering },
    { label: 'Backup Scheduler', enabled: features.backup },
    { label: 'OIDC / SSO', enabled: features.oidc },
    { label: 'Lambda Triggers', enabled: features.lambda },
    { label: 'Debug Mode', enabled: features.debug },
  ]

  return (
    <div>
      <h2 className="text-xl font-semibold text-gray-900 dark:text-white mb-6">Settings</h2>
      <p className="text-sm text-gray-500 dark:text-gray-400 mb-6">
        Read-only view of the server configuration. Edit <code className="px-1 py-0.5 rounded bg-gray-100 dark:bg-gray-700 text-xs">configs/vaults3.yaml</code> and restart the server to change settings.
      </p>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
        {/* Server */}
        <Section title="Server">
          <Row label="Listen Address" value={`${settings.server.address}:${settings.server.port}`} />
          <Row label="Domain" value={settings.server.domain || '(not set)'} />
          <Row label="TLS" value={settings.server.tlsEnabled ? 'Enabled' : 'Disabled'} />
          <Row label="Shutdown Timeout" value={`${settings.server.shutdownTimeoutSecs}s`} />
        </Section>

        {/* Storage */}
        <Section title="Storage">
          <Row label="Data Directory" value={settings.storage.dataDir} mono />
          <Row label="Metadata Directory" value={settings.storage.metadataDir} mono />
        </Section>

        {/* Features */}
        <Section title="Features">
          <div className="grid grid-cols-2 gap-2">
            {featureList.map(f => (
              <div key={f.label} className="flex items-center gap-2 text-sm">
                <span className={`inline-block w-2 h-2 rounded-full ${f.enabled ? 'bg-green-500' : 'bg-gray-300 dark:bg-gray-600'}`} />
                <span className={f.enabled ? 'text-gray-900 dark:text-white' : 'text-gray-400 dark:text-gray-500'}>{f.label}</span>
              </div>
            ))}
          </div>
        </Section>

        {/* Lifecycle */}
        <Section title="Lifecycle">
          <Row label="Scan Interval" value={`${settings.lifecycle.scanIntervalSecs}s`} />
          <Row label="Audit Retention" value={`${settings.lifecycle.auditRetentionDays} days`} />
        </Section>

        {/* Rate Limit */}
        {settings.features.rateLimit && settings.rateLimit && (
          <Section title="Rate Limiting">
            <Row label="Requests/sec" value={String(settings.rateLimit.requestsPerSec)} />
            <Row label="Burst Size" value={String(settings.rateLimit.burstSize)} />
            <Row label="Per-Key RPS" value={String(settings.rateLimit.perKeyRps)} />
            <Row label="Per-Key Burst" value={String(settings.rateLimit.perKeyBurst)} />
          </Section>
        )}

        {/* Memory */}
        <Section title="Memory">
          <Row label="Max Search Entries" value={settings.memory.maxSearchEntries.toLocaleString()} />
          {settings.memory.goMemLimitMb ? (
            <Row label="Go Memory Limit" value={`${settings.memory.goMemLimitMb} MB`} />
          ) : (
            <Row label="Go Memory Limit" value="(not set)" />
          )}
        </Section>
      </div>

      <div className="mt-6">
        <ChangeCredentialsForm />
      </div>
    </div>
  )
}

function ChangeCredentialsForm() {
  const [currentSecretKey, setCurrentSecretKey] = useState('')
  const [newAccessKey, setNewAccessKey] = useState('')
  const [newSecretKey, setNewSecretKey] = useState('')
  const [confirmSecretKey, setConfirmSecretKey] = useState('')
  const [saving, setSaving] = useState(false)
  const [message, setMessage] = useState<{ type: 'success' | 'error'; text: string } | null>(null)

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setMessage(null)

    if (newSecretKey !== confirmSecretKey) {
      setMessage({ type: 'error', text: 'New secret keys do not match' })
      return
    }

    if (newSecretKey.length < 8) {
      setMessage({ type: 'error', text: 'Secret key must be at least 8 characters' })
      return
    }

    setSaving(true)
    try {
      await changeCredentials(currentSecretKey, newAccessKey, newSecretKey)
      setMessage({ type: 'success', text: 'Credentials updated. Please log in again with your new credentials.' })
      setCurrentSecretKey('')
      setNewAccessKey('')
      setNewSecretKey('')
      setConfirmSecretKey('')
      // Clear token and redirect to login after short delay
      setTimeout(() => {
        localStorage.removeItem('token')
        window.location.href = '/dashboard/'
      }, 2000)
    } catch (err) {
      setMessage({ type: 'error', text: err instanceof Error ? err.message : 'Failed to update credentials' })
    } finally {
      setSaving(false)
    }
  }

  const inputClass = 'w-full px-3 py-2 text-sm rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 text-gray-900 dark:text-white focus:ring-2 focus:ring-indigo-500 focus:border-transparent'

  return (
    <div className="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-5">
      <h3 className="text-sm font-semibold text-gray-900 dark:text-white mb-1">Change Admin Credentials</h3>
      <p className="text-xs text-gray-500 dark:text-gray-400 mb-4">Update the admin access key and secret key. You will be logged out after changing credentials.</p>

      {message && (
        <div className={`p-3 rounded-lg text-sm mb-4 ${message.type === 'success' ? 'bg-green-50 dark:bg-green-900/20 text-green-700 dark:text-green-400' : 'bg-red-50 dark:bg-red-900/20 text-red-700 dark:text-red-400'}`}>
          {message.text}
        </div>
      )}

      <form onSubmit={handleSubmit} className="space-y-4 max-w-md">
        <div>
          <label className="block text-sm text-gray-600 dark:text-gray-400 mb-1">Current Secret Key</label>
          <input type="password" value={currentSecretKey} onChange={e => setCurrentSecretKey(e.target.value)} required className={inputClass} />
        </div>
        <div>
          <label className="block text-sm text-gray-600 dark:text-gray-400 mb-1">New Access Key</label>
          <input type="text" value={newAccessKey} onChange={e => setNewAccessKey(e.target.value)} required className={inputClass} />
        </div>
        <div>
          <label className="block text-sm text-gray-600 dark:text-gray-400 mb-1">New Secret Key</label>
          <input type="password" value={newSecretKey} onChange={e => setNewSecretKey(e.target.value)} required className={inputClass} />
        </div>
        <div>
          <label className="block text-sm text-gray-600 dark:text-gray-400 mb-1">Confirm New Secret Key</label>
          <input type="password" value={confirmSecretKey} onChange={e => setConfirmSecretKey(e.target.value)} required className={inputClass} />
        </div>
        <button type="submit" disabled={saving} className="px-4 py-2 text-sm font-medium text-white bg-indigo-600 hover:bg-indigo-700 rounded-lg disabled:opacity-50 disabled:cursor-not-allowed">
          {saving ? 'Updating...' : 'Update Credentials'}
        </button>
      </form>
    </div>
  )
}

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div className="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-5">
      <h3 className="text-sm font-semibold text-gray-900 dark:text-white mb-3">{title}</h3>
      <div className="space-y-2">
        {children}
      </div>
    </div>
  )
}

function Row({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  return (
    <div className="flex items-center justify-between text-sm">
      <span className="text-gray-500 dark:text-gray-400">{label}</span>
      <span className={`text-gray-900 dark:text-white ${mono ? 'font-mono text-xs' : ''}`}>{value}</span>
    </div>
  )
}
