import { useState, useEffect, useCallback } from 'react'
import { useParams, useSearchParams, Link } from 'react-router-dom'
import { listObjects, deleteObject, getDownloadUrl, type ObjectItem } from '../api/objects'
import UploadDropzone from '../components/UploadDropzone'

export default function FileBrowserPage() {
  const { name: bucket } = useParams<{ name: string }>()
  const [searchParams, setSearchParams] = useSearchParams()
  const prefix = searchParams.get('prefix') || ''

  const [objects, setObjects] = useState<ObjectItem[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [deleteTarget, setDeleteTarget] = useState<string | null>(null)

  const fetchObjects = useCallback(async () => {
    if (!bucket) return
    setLoading(true)
    setError('')
    try {
      const data = await listObjects(bucket, prefix)
      setObjects(data.objects || [])
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to list objects')
    } finally {
      setLoading(false)
    }
  }, [bucket, prefix])

  useEffect(() => { fetchObjects() }, [fetchObjects])

  const handleDelete = async (key: string) => {
    if (!bucket) return
    setError('')
    try {
      await deleteObject(bucket, key)
      setDeleteTarget(null)
      fetchObjects()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to delete object')
    }
  }

  const navigatePrefix = (p: string) => {
    if (p) {
      setSearchParams({ prefix: p })
    } else {
      setSearchParams({})
    }
  }

  // Breadcrumb segments
  const breadcrumbs = prefix
    ? prefix.split('/').filter(Boolean).map((seg, i, arr) => ({
        label: seg,
        prefix: arr.slice(0, i + 1).join('/') + '/',
      }))
    : []

  if (!bucket) return null

  return (
    <div>
      <div className="flex items-center justify-between mb-4">
        <div>
          <div className="flex items-center gap-1 text-sm text-gray-500 dark:text-gray-400 mb-1">
            <Link to={`/buckets/${bucket}`} className="hover:text-indigo-600 dark:hover:text-indigo-400">
              {bucket}
            </Link>
            <span>/</span>
            <button onClick={() => navigatePrefix('')} className="hover:text-indigo-600 dark:hover:text-indigo-400">
              files
            </button>
            {breadcrumbs.map((bc) => (
              <span key={bc.prefix} className="flex items-center gap-1">
                <span>/</span>
                <button
                  onClick={() => navigatePrefix(bc.prefix)}
                  className="hover:text-indigo-600 dark:hover:text-indigo-400"
                >
                  {bc.label}
                </button>
              </span>
            ))}
          </div>
          <h2 className="text-xl font-semibold text-gray-900 dark:text-white">Files</h2>
        </div>
      </div>

      <div className="mb-4">
        <UploadDropzone bucket={bucket} prefix={prefix} onUploaded={() => fetchObjects()} />
      </div>

      {error && (
        <div className="mb-4 p-3 rounded-lg bg-red-50 dark:bg-red-900/20 text-red-700 dark:text-red-400 text-sm">
          {error}
        </div>
      )}

      {/* Delete confirmation modal */}
      {deleteTarget && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40">
          <div className="bg-white dark:bg-gray-800 rounded-xl shadow-xl border border-gray-200 dark:border-gray-700 p-6 w-full max-w-sm mx-4">
            <h3 className="text-lg font-semibold text-gray-900 dark:text-white mb-2">Delete Object</h3>
            <p className="text-sm text-gray-600 dark:text-gray-400 mb-4">
              Are you sure you want to delete <strong className="break-all">{deleteTarget}</strong>?
            </p>
            <div className="flex gap-2 justify-end">
              <button
                onClick={() => setDeleteTarget(null)}
                className="px-4 py-2 rounded-lg text-sm text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors"
              >
                Cancel
              </button>
              <button
                onClick={() => handleDelete(deleteTarget)}
                className="px-4 py-2 rounded-lg bg-red-600 hover:bg-red-700 text-white text-sm font-medium transition-colors"
              >
                Delete
              </button>
            </div>
          </div>
        </div>
      )}

      {loading ? (
        <div className="flex items-center justify-center h-64">
          <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-indigo-600" />
        </div>
      ) : objects.length === 0 ? (
        <div className="text-center py-16 bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700">
          <svg className="w-12 h-12 mx-auto text-gray-400 dark:text-gray-500 mb-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}>
            <path strokeLinecap="round" strokeLinejoin="round" d="M2.25 12.75V12A2.25 2.25 0 014.5 9.75h15A2.25 2.25 0 0121.75 12v.75m-8.69-6.44l-2.12-2.12a1.5 1.5 0 00-1.061-.44H4.5A2.25 2.25 0 002.25 6v12a2.25 2.25 0 002.25 2.25h15A2.25 2.25 0 0021.75 18V9a2.25 2.25 0 00-2.25-2.25h-5.379a1.5 1.5 0 01-1.06-.44z" />
          </svg>
          <p className="text-gray-500 dark:text-gray-400 text-sm">No files here yet</p>
          <p className="text-gray-400 dark:text-gray-500 text-xs mt-1">Upload files using the dropzone above</p>
        </div>
      ) : (
        <div className="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 overflow-hidden">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-gray-200 dark:border-gray-700">
                <th className="text-left px-4 py-3 text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Name</th>
                <th className="text-left px-4 py-3 text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Size</th>
                <th className="text-left px-4 py-3 text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Type</th>
                <th className="text-left px-4 py-3 text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Modified</th>
                <th className="text-right px-4 py-3 text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-100 dark:divide-gray-700/50">
              {objects.map((obj) => (
                <tr key={obj.key} className="hover:bg-gray-50 dark:hover:bg-gray-700/30 transition-colors">
                  <td className="px-4 py-3">
                    {obj.isPrefix ? (
                      <button
                        onClick={() => navigatePrefix(obj.key)}
                        className="flex items-center gap-2 text-indigo-600 dark:text-indigo-400 font-medium hover:underline"
                      >
                        <FolderIcon />
                        {displayName(obj.key, prefix)}
                      </button>
                    ) : (
                      <span className="flex items-center gap-2 text-gray-900 dark:text-white">
                        <FileIcon />
                        {displayName(obj.key, prefix)}
                      </span>
                    )}
                  </td>
                  <td className="px-4 py-3 text-gray-700 dark:text-gray-300">
                    {obj.isPrefix ? '-' : formatSize(obj.size)}
                  </td>
                  <td className="px-4 py-3 text-gray-500 dark:text-gray-400">
                    {obj.isPrefix ? 'Folder' : (obj.contentType || '-')}
                  </td>
                  <td className="px-4 py-3 text-gray-500 dark:text-gray-400">
                    {obj.isPrefix ? '-' : formatDate(obj.lastModified)}
                  </td>
                  <td className="px-4 py-3 text-right">
                    {!obj.isPrefix && (
                      <div className="flex items-center justify-end gap-2">
                        <a
                          href={getDownloadUrl(bucket, obj.key)}
                          className="text-gray-400 hover:text-indigo-600 dark:hover:text-indigo-400 transition-colors"
                          title="Download"
                        >
                          <DownloadIcon />
                        </a>
                        <button
                          onClick={() => setDeleteTarget(obj.key)}
                          className="text-gray-400 hover:text-red-600 dark:hover:text-red-400 transition-colors"
                          title="Delete"
                        >
                          <TrashIcon />
                        </button>
                      </div>
                    )}
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

function displayName(key: string, prefix: string): string {
  const rel = key.slice(prefix.length)
  return rel.endsWith('/') ? rel.slice(0, -1) : rel
}

function formatSize(bytes: number): string {
  if (bytes === 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(1024))
  return `${(bytes / Math.pow(1024, i)).toFixed(i > 0 ? 1 : 0)} ${units[i]}`
}

function formatDate(iso: string): string {
  if (!iso) return '-'
  return new Date(iso).toLocaleDateString(undefined, { year: 'numeric', month: 'short', day: 'numeric' })
}

function FolderIcon() {
  return (
    <svg className="w-4 h-4 text-yellow-500" fill="currentColor" viewBox="0 0 20 20">
      <path d="M2 6a2 2 0 012-2h5l2 2h5a2 2 0 012 2v6a2 2 0 01-2 2H4a2 2 0 01-2-2V6z" />
    </svg>
  )
}

function FileIcon() {
  return (
    <svg className="w-4 h-4 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}>
      <path strokeLinecap="round" strokeLinejoin="round" d="M19.5 14.25v-2.625a3.375 3.375 0 00-3.375-3.375h-1.5A1.125 1.125 0 0113.5 7.125v-1.5a3.375 3.375 0 00-3.375-3.375H8.25m2.25 0H5.625c-.621 0-1.125.504-1.125 1.125v17.25c0 .621.504 1.125 1.125 1.125h12.75c.621 0 1.125-.504 1.125-1.125V11.25a9 9 0 00-9-9z" />
    </svg>
  )
}

function DownloadIcon() {
  return (
    <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
      <path strokeLinecap="round" strokeLinejoin="round" d="M3 16.5v2.25A2.25 2.25 0 005.25 21h13.5A2.25 2.25 0 0021 18.75V16.5M16.5 12L12 16.5m0 0L7.5 12m4.5 4.5V3" />
    </svg>
  )
}

function TrashIcon() {
  return (
    <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
      <path strokeLinecap="round" strokeLinejoin="round" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
    </svg>
  )
}
