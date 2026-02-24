import { useState, useEffect, useCallback } from 'react'
import {
  listUsers, createUser, deleteUser, attachUserPolicy, detachUserPolicy,
  addUserToGroup, removeUserFromGroup, setIPRestrictions,
  listGroups, createGroup, deleteGroup, attachGroupPolicy, detachGroupPolicy,
  listPolicies, createPolicy, deletePolicy,
  type IAMUser, type IAMGroup, type IAMPolicy,
} from '../api/iam'

type Tab = 'users' | 'groups' | 'policies'

export default function IAMPage() {
  const [tab, setTab] = useState<Tab>('users')
  const [users, setUsers] = useState<IAMUser[]>([])
  const [groups, setGroups] = useState<IAMGroup[]>([])
  const [policies, setPolicies] = useState<IAMPolicy[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  // modal state
  const [showCreate, setShowCreate] = useState(false)
  const [createName, setCreateName] = useState('')
  const [policyDoc, setPolicyDoc] = useState('{\n  "Version": "2012-10-17",\n  "Statement": []\n}')
  const [deleteTarget, setDeleteTarget] = useState<string | null>(null)

  // detail/expand
  const [expandedUser, setExpandedUser] = useState<string | null>(null)
  const [expandedGroup, setExpandedGroup] = useState<string | null>(null)
  const [attachInput, setAttachInput] = useState('')
  const [cidrInput, setCidrInput] = useState('')

  const fetchAll = useCallback(async () => {
    try {
      const [u, g, p] = await Promise.all([listUsers(), listGroups(), listPolicies()])
      setUsers(u || [])
      setGroups(g || [])
      setPolicies(p || [])
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load IAM data')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { fetchAll() }, [fetchAll])

  const handleCreate = async () => {
    setError('')
    try {
      if (tab === 'users') await createUser(createName)
      else if (tab === 'groups') await createGroup(createName)
      else await createPolicy(createName, policyDoc)
      setShowCreate(false)
      setCreateName('')
      setPolicyDoc('{\n  "Version": "2012-10-17",\n  "Statement": []\n}')
      fetchAll()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Create failed')
    }
  }

  const handleDelete = async () => {
    if (!deleteTarget) return
    setError('')
    try {
      if (tab === 'users') await deleteUser(deleteTarget)
      else if (tab === 'groups') await deleteGroup(deleteTarget)
      else await deletePolicy(deleteTarget)
      setDeleteTarget(null)
      fetchAll()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Delete failed')
    }
  }

  const handleAttachPolicy = async (target: string, policy: string, isGroup: boolean) => {
    setError('')
    try {
      if (isGroup) await attachGroupPolicy(target, policy)
      else await attachUserPolicy(target, policy)
      setAttachInput('')
      fetchAll()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Attach failed')
    }
  }

  const handleDetachPolicy = async (target: string, policy: string, isGroup: boolean) => {
    setError('')
    try {
      if (isGroup) await detachGroupPolicy(target, policy)
      else await detachUserPolicy(target, policy)
      fetchAll()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Detach failed')
    }
  }

  const handleAddToGroup = async (userName: string, groupName: string) => {
    setError('')
    try {
      await addUserToGroup(userName, groupName)
      setAttachInput('')
      fetchAll()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Add to group failed')
    }
  }

  const handleRemoveFromGroup = async (userName: string, groupName: string) => {
    setError('')
    try {
      await removeUserFromGroup(userName, groupName)
      fetchAll()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Remove from group failed')
    }
  }

  const handleAddCIDR = async (userName: string, cidr: string) => {
    setError('')
    try {
      const user = users.find(u => u.name === userName)
      const current = user?.allowedCidrs || []
      if (current.includes(cidr)) return
      await setIPRestrictions(userName, [...current, cidr])
      setCidrInput('')
      fetchAll()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to add CIDR')
    }
  }

  const handleRemoveCIDR = async (userName: string, cidr: string) => {
    setError('')
    try {
      const user = users.find(u => u.name === userName)
      const updated = (user?.allowedCidrs || []).filter(c => c !== cidr)
      await setIPRestrictions(userName, updated)
      fetchAll()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to remove CIDR')
    }
  }

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-indigo-600" />
      </div>
    )
  }

  const tabs: { key: Tab; label: string; count: number }[] = [
    { key: 'users', label: 'Users', count: users.length },
    { key: 'groups', label: 'Groups', count: groups.length },
    { key: 'policies', label: 'Policies', count: policies.length },
  ]

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <div>
          <h2 className="text-xl font-semibold text-gray-900 dark:text-white">IAM</h2>
          <p className="text-sm text-gray-500 dark:text-gray-400 mt-0.5">Users, groups, and policies</p>
        </div>
        <button onClick={() => { setShowCreate(true); setCreateName('') }}
          className="px-4 py-2 rounded-lg bg-indigo-600 hover:bg-indigo-700 text-white text-sm font-medium transition-colors">
          Create {tab === 'users' ? 'User' : tab === 'groups' ? 'Group' : 'Policy'}
        </button>
      </div>

      {error && (
        <div className="mb-4 p-3 rounded-lg bg-red-50 dark:bg-red-900/20 text-red-700 dark:text-red-400 text-sm">
          {error}
        </div>
      )}

      {/* Tabs */}
      <div className="flex gap-1 mb-4 bg-gray-100 dark:bg-gray-800 rounded-lg p-1">
        {tabs.map(t => (
          <button key={t.key} onClick={() => setTab(t.key)}
            className={`flex-1 px-3 py-2 rounded-md text-sm font-medium transition-colors ${
              tab === t.key
                ? 'bg-white dark:bg-gray-700 text-gray-900 dark:text-white shadow-sm'
                : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-white'
            }`}>
            {t.label} <span className="ml-1 text-xs text-gray-400">({t.count})</span>
          </button>
        ))}
      </div>

      {/* Users tab */}
      {tab === 'users' && (
        <div className="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 overflow-hidden">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-gray-200 dark:border-gray-700">
                <th className="text-left px-4 py-3 text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Name</th>
                <th className="text-left px-4 py-3 text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Groups</th>
                <th className="text-left px-4 py-3 text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Policies</th>
                <th className="text-left px-4 py-3 text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Created</th>
                <th className="text-right px-4 py-3 text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-100 dark:divide-gray-700/50">
              {users.map(u => (
                <tr key={u.name} className="hover:bg-gray-50 dark:hover:bg-gray-700/30 transition-colors cursor-pointer"
                  onClick={() => setExpandedUser(expandedUser === u.name ? null : u.name)}>
                  <td className="px-4 py-3 font-medium text-gray-900 dark:text-white">{u.name}</td>
                  <td className="px-4 py-3 text-gray-500 dark:text-gray-400">{(u.groups || []).length}</td>
                  <td className="px-4 py-3 text-gray-500 dark:text-gray-400">{(u.policyArns || []).length}</td>
                  <td className="px-4 py-3 text-gray-500 dark:text-gray-400">
                    {u.createdAt ? new Date(u.createdAt).toLocaleDateString() : '-'}
                  </td>
                  <td className="px-4 py-3 text-right" onClick={e => e.stopPropagation()}>
                    <button onClick={() => setDeleteTarget(u.name)}
                      className="text-gray-400 hover:text-red-600 dark:hover:text-red-400 transition-colors" title="Delete">
                      <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                        <path strokeLinecap="round" strokeLinejoin="round" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                      </svg>
                    </button>
                  </td>
                </tr>
              ))}
              {users.length === 0 && (
                <tr><td colSpan={5} className="px-4 py-8 text-center text-gray-400">No users</td></tr>
              )}
            </tbody>
          </table>
        </div>
      )}

      {/* Expanded user detail */}
      {tab === 'users' && expandedUser && (() => {
        const u = users.find(x => x.name === expandedUser)
        if (!u) return null
        return (
          <div className="mt-3 bg-gray-50 dark:bg-gray-900 rounded-xl border border-gray-200 dark:border-gray-700 p-4">
            <h4 className="text-sm font-semibold text-gray-900 dark:text-white mb-3">Details: {u.name}</h4>
            <div className="grid grid-cols-3 gap-4">
              <div>
                <p className="text-xs font-medium text-gray-500 dark:text-gray-400 mb-2">Policies</p>
                <div className="space-y-1">
                  {(u.policyArns || []).map(p => (
                    <div key={p} className="flex items-center justify-between bg-white dark:bg-gray-800 rounded px-2 py-1">
                      <span className="text-sm text-gray-700 dark:text-gray-300">{p}</span>
                      <button onClick={() => handleDetachPolicy(u.name, p, false)}
                        className="text-xs text-red-500 hover:text-red-700">Detach</button>
                    </div>
                  ))}
                </div>
                <div className="flex gap-1 mt-2">
                  <select value={attachInput} onChange={e => setAttachInput(e.target.value)}
                    className="flex-1 text-xs px-2 py-1 rounded border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-800 text-gray-900 dark:text-white">
                    <option value="">Attach policy...</option>
                    {policies.filter(p => !(u.policyArns || []).includes(p.name)).map(p => (
                      <option key={p.name} value={p.name}>{p.name}</option>
                    ))}
                  </select>
                  <button onClick={() => attachInput && handleAttachPolicy(u.name, attachInput, false)}
                    disabled={!attachInput}
                    className="text-xs px-2 py-1 rounded bg-indigo-600 text-white disabled:opacity-50">Add</button>
                </div>
              </div>
              <div>
                <p className="text-xs font-medium text-gray-500 dark:text-gray-400 mb-2">Groups</p>
                <div className="space-y-1">
                  {(u.groups || []).map(g => (
                    <div key={g} className="flex items-center justify-between bg-white dark:bg-gray-800 rounded px-2 py-1">
                      <span className="text-sm text-gray-700 dark:text-gray-300">{g}</span>
                      <button onClick={() => handleRemoveFromGroup(u.name, g)}
                        className="text-xs text-red-500 hover:text-red-700">Remove</button>
                    </div>
                  ))}
                </div>
                <div className="flex gap-1 mt-2">
                  <select value={attachInput} onChange={e => setAttachInput(e.target.value)}
                    className="flex-1 text-xs px-2 py-1 rounded border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-800 text-gray-900 dark:text-white">
                    <option value="">Add to group...</option>
                    {groups.filter(g => !(u.groups || []).includes(g.name)).map(g => (
                      <option key={g.name} value={g.name}>{g.name}</option>
                    ))}
                  </select>
                  <button onClick={() => attachInput && handleAddToGroup(u.name, attachInput)}
                    disabled={!attachInput}
                    className="text-xs px-2 py-1 rounded bg-indigo-600 text-white disabled:opacity-50">Add</button>
                </div>
              </div>
              <div>
                <p className="text-xs font-medium text-gray-500 dark:text-gray-400 mb-2">IP Restrictions (CIDR)</p>
                <div className="space-y-1">
                  {(u.allowedCidrs || []).map(c => (
                    <div key={c} className="flex items-center justify-between bg-white dark:bg-gray-800 rounded px-2 py-1">
                      <span className="text-sm text-gray-700 dark:text-gray-300 font-mono">{c}</span>
                      <button onClick={() => handleRemoveCIDR(u.name, c)}
                        className="text-xs text-red-500 hover:text-red-700">Remove</button>
                    </div>
                  ))}
                  {(u.allowedCidrs || []).length === 0 && (
                    <p className="text-xs text-gray-400 italic">No restrictions (all IPs allowed)</p>
                  )}
                </div>
                <div className="flex gap-1 mt-2">
                  <input type="text" value={cidrInput} onChange={e => setCidrInput(e.target.value)}
                    placeholder="e.g. 10.0.0.0/8"
                    className="flex-1 text-xs px-2 py-1 rounded border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-800 text-gray-900 dark:text-white font-mono" />
                  <button onClick={() => cidrInput && handleAddCIDR(u.name, cidrInput)}
                    disabled={!cidrInput}
                    className="text-xs px-2 py-1 rounded bg-indigo-600 text-white disabled:opacity-50">Add</button>
                </div>
              </div>
            </div>
          </div>
        )
      })()}

      {/* Groups tab */}
      {tab === 'groups' && (
        <div className="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 overflow-hidden">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-gray-200 dark:border-gray-700">
                <th className="text-left px-4 py-3 text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Name</th>
                <th className="text-left px-4 py-3 text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Policies</th>
                <th className="text-left px-4 py-3 text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Created</th>
                <th className="text-right px-4 py-3 text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-100 dark:divide-gray-700/50">
              {groups.map(g => (
                <tr key={g.name} className="hover:bg-gray-50 dark:hover:bg-gray-700/30 transition-colors cursor-pointer"
                  onClick={() => setExpandedGroup(expandedGroup === g.name ? null : g.name)}>
                  <td className="px-4 py-3 font-medium text-gray-900 dark:text-white">{g.name}</td>
                  <td className="px-4 py-3 text-gray-500 dark:text-gray-400">{(g.policyArns || []).length}</td>
                  <td className="px-4 py-3 text-gray-500 dark:text-gray-400">
                    {g.createdAt ? new Date(g.createdAt).toLocaleDateString() : '-'}
                  </td>
                  <td className="px-4 py-3 text-right" onClick={e => e.stopPropagation()}>
                    <button onClick={() => setDeleteTarget(g.name)}
                      className="text-gray-400 hover:text-red-600 dark:hover:text-red-400 transition-colors" title="Delete">
                      <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                        <path strokeLinecap="round" strokeLinejoin="round" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                      </svg>
                    </button>
                  </td>
                </tr>
              ))}
              {groups.length === 0 && (
                <tr><td colSpan={4} className="px-4 py-8 text-center text-gray-400">No groups</td></tr>
              )}
            </tbody>
          </table>
        </div>
      )}

      {/* Expanded group detail */}
      {tab === 'groups' && expandedGroup && (() => {
        const g = groups.find(x => x.name === expandedGroup)
        if (!g) return null
        return (
          <div className="mt-3 bg-gray-50 dark:bg-gray-900 rounded-xl border border-gray-200 dark:border-gray-700 p-4">
            <h4 className="text-sm font-semibold text-gray-900 dark:text-white mb-3">Policies: {g.name}</h4>
            <div className="space-y-1">
              {(g.policyArns || []).map(p => (
                <div key={p} className="flex items-center justify-between bg-white dark:bg-gray-800 rounded px-2 py-1">
                  <span className="text-sm text-gray-700 dark:text-gray-300">{p}</span>
                  <button onClick={() => handleDetachPolicy(g.name, p, true)}
                    className="text-xs text-red-500 hover:text-red-700">Detach</button>
                </div>
              ))}
            </div>
            <div className="flex gap-1 mt-2">
              <select value={attachInput} onChange={e => setAttachInput(e.target.value)}
                className="flex-1 text-xs px-2 py-1 rounded border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-800 text-gray-900 dark:text-white">
                <option value="">Attach policy...</option>
                {policies.filter(p => !(g.policyArns || []).includes(p.name)).map(p => (
                  <option key={p.name} value={p.name}>{p.name}</option>
                ))}
              </select>
              <button onClick={() => attachInput && handleAttachPolicy(g.name, attachInput, true)}
                disabled={!attachInput}
                className="text-xs px-2 py-1 rounded bg-indigo-600 text-white disabled:opacity-50">Add</button>
            </div>
          </div>
        )
      })()}

      {/* Policies tab */}
      {tab === 'policies' && (
        <div className="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 overflow-hidden">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-gray-200 dark:border-gray-700">
                <th className="text-left px-4 py-3 text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Name</th>
                <th className="text-left px-4 py-3 text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Type</th>
                <th className="text-left px-4 py-3 text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Created</th>
                <th className="text-right px-4 py-3 text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-100 dark:divide-gray-700/50">
              {policies.map(p => (
                <tr key={p.name} className="hover:bg-gray-50 dark:hover:bg-gray-700/30 transition-colors">
                  <td className="px-4 py-3 font-medium text-gray-900 dark:text-white">{p.name}</td>
                  <td className="px-4 py-3">
                    {p.name === 'ReadOnlyAccess' || p.name === 'ReadWriteAccess' || p.name === 'FullAccess' ? (
                      <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-amber-100 dark:bg-amber-900/30 text-amber-700 dark:text-amber-400">Built-in</span>
                    ) : (
                      <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-400">Custom</span>
                    )}
                  </td>
                  <td className="px-4 py-3 text-gray-500 dark:text-gray-400">
                    {p.createdAt ? new Date(p.createdAt).toLocaleDateString() : '-'}
                  </td>
                  <td className="px-4 py-3 text-right">
                    {!(p.name === 'ReadOnlyAccess' || p.name === 'ReadWriteAccess' || p.name === 'FullAccess') && (
                      <button onClick={() => setDeleteTarget(p.name)}
                        className="text-gray-400 hover:text-red-600 dark:hover:text-red-400 transition-colors" title="Delete">
                        <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                          <path strokeLinecap="round" strokeLinejoin="round" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                        </svg>
                      </button>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {/* Create modal */}
      {showCreate && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40">
          <div className="bg-white dark:bg-gray-800 rounded-xl shadow-xl border border-gray-200 dark:border-gray-700 p-6 w-full max-w-md mx-4">
            <h3 className="text-lg font-semibold text-gray-900 dark:text-white mb-4">
              Create {tab === 'users' ? 'User' : tab === 'groups' ? 'Group' : 'Policy'}
            </h3>
            <div className="space-y-3">
              <div>
                <label className="text-xs text-gray-500 dark:text-gray-400 font-medium">Name</label>
                <input type="text" value={createName} onChange={e => setCreateName(e.target.value)}
                  className="w-full mt-1 px-3 py-2 rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 text-gray-900 dark:text-white text-sm"
                  placeholder={tab === 'users' ? 'Username' : tab === 'groups' ? 'Group name' : 'Policy name'} />
              </div>
              {tab === 'policies' && (
                <div>
                  <label className="text-xs text-gray-500 dark:text-gray-400 font-medium">Policy Document (JSON)</label>
                  <textarea value={policyDoc} onChange={e => setPolicyDoc(e.target.value)} rows={8}
                    className="w-full mt-1 px-3 py-2 rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 text-gray-900 dark:text-white text-sm font-mono" />
                </div>
              )}
            </div>
            <div className="flex gap-2 justify-end mt-4">
              <button onClick={() => setShowCreate(false)}
                className="px-4 py-2 rounded-lg text-sm text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors">Cancel</button>
              <button onClick={handleCreate} disabled={!createName}
                className="px-4 py-2 rounded-lg bg-indigo-600 hover:bg-indigo-700 disabled:bg-indigo-400 text-white text-sm font-medium transition-colors">Create</button>
            </div>
          </div>
        </div>
      )}

      {/* Delete confirmation modal */}
      {deleteTarget && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40">
          <div className="bg-white dark:bg-gray-800 rounded-xl shadow-xl border border-gray-200 dark:border-gray-700 p-6 w-full max-w-sm mx-4">
            <h3 className="text-lg font-semibold text-gray-900 dark:text-white mb-2">Delete {tab.slice(0, -1)}</h3>
            <p className="text-sm text-gray-600 dark:text-gray-400 mb-4">
              Are you sure you want to delete <strong>{deleteTarget}</strong>?
            </p>
            <div className="flex gap-2 justify-end">
              <button onClick={() => setDeleteTarget(null)}
                className="px-4 py-2 rounded-lg text-sm text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors">Cancel</button>
              <button onClick={handleDelete}
                className="px-4 py-2 rounded-lg bg-red-600 hover:bg-red-700 text-white text-sm font-medium transition-colors">Delete</button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
