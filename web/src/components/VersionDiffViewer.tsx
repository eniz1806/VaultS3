import { useState, useEffect } from 'react'
import { getVersionDiff, type DiffResult } from '../api/versions'

interface Props {
  bucket: string
  objectKey: string
  v1: string
  v2: string
  onClose: () => void
}

export default function VersionDiffViewer({ bucket, objectKey, v1, v2, onClose }: Props) {
  const [diff, setDiff] = useState<DiffResult | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  useEffect(() => {
    setLoading(true)
    setError('')
    getVersionDiff(bucket, objectKey, v1, v2)
      .then(setDiff)
      .catch(err => setError(err instanceof Error ? err.message : 'Failed to load diff'))
      .finally(() => setLoading(false))
  }, [bucket, objectKey, v1, v2])

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40">
      <div className="bg-white dark:bg-gray-800 rounded-xl shadow-xl border border-gray-200 dark:border-gray-700 w-full max-w-3xl mx-4 max-h-[80vh] flex flex-col">
        {/* Header */}
        <div className="flex items-center justify-between px-5 py-4 border-b border-gray-200 dark:border-gray-700">
          <div>
            <h3 className="text-lg font-semibold text-gray-900 dark:text-white">Version Diff</h3>
            <p className="text-xs text-gray-500 dark:text-gray-400 mt-0.5 font-mono">
              {v1.slice(0, 12)}... vs {v2.slice(0, 12)}...
            </p>
          </div>
          <button
            onClick={onClose}
            className="text-gray-400 hover:text-gray-600 dark:hover:text-gray-300"
          >
            <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
              <path strokeLinecap="round" strokeLinejoin="round" d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>

        {/* Content */}
        <div className="flex-1 overflow-auto p-5">
          {loading && (
            <div className="flex justify-center py-12">
              <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-indigo-600" />
            </div>
          )}

          {error && (
            <div className="p-3 rounded-lg bg-red-50 dark:bg-red-900/20 text-red-700 dark:text-red-400 text-sm">
              {error}
            </div>
          )}

          {diff && diff.isText && diff.lines && (
            <div className="border border-gray-200 dark:border-gray-700 rounded-lg overflow-hidden">
              <div className="px-3 py-1.5 bg-gray-50 dark:bg-gray-900 border-b border-gray-200 dark:border-gray-700 text-xs text-gray-500 dark:text-gray-400">
                Text Diff &middot; {diff.lines.length} lines
              </div>
              <pre className="text-xs font-mono overflow-auto max-h-[50vh]">
                {diff.lines.map((line, i) => (
                  <div
                    key={i}
                    className={`px-3 py-0.5 ${
                      line.type === 'add'
                        ? 'bg-green-50 dark:bg-green-900/20 text-green-700 dark:text-green-400'
                        : line.type === 'remove'
                        ? 'bg-red-50 dark:bg-red-900/20 text-red-700 dark:text-red-400'
                        : 'text-gray-700 dark:text-gray-300'
                    }`}
                  >
                    <span className="select-none text-gray-400 dark:text-gray-500 w-5 inline-block">
                      {line.type === 'add' ? '+' : line.type === 'remove' ? '-' : ' '}
                    </span>
                    {line.content}
                  </div>
                ))}
              </pre>
            </div>
          )}

          {diff && !diff.isText && (
            <div className="border border-gray-200 dark:border-gray-700 rounded-lg overflow-hidden">
              <div className="px-3 py-1.5 bg-gray-50 dark:bg-gray-900 border-b border-gray-200 dark:border-gray-700 text-xs text-gray-500 dark:text-gray-400">
                Binary Comparison (metadata only)
              </div>
              <div className="p-4">
                <table className="w-full text-sm">
                  <thead>
                    <tr className="text-left text-xs text-gray-500 dark:text-gray-400">
                      <th className="pb-2">Property</th>
                      <th className="pb-2">Version A</th>
                      <th className="pb-2">Version B</th>
                    </tr>
                  </thead>
                  <tbody className="text-gray-700 dark:text-gray-300">
                    {diff.metaA && diff.metaB && Object.keys(diff.metaA).map(k => (
                      <tr key={k} className="border-t border-gray-100 dark:border-gray-700/50">
                        <td className="py-1.5 font-mono text-xs text-gray-500 dark:text-gray-400">{k}</td>
                        <td className="py-1.5 font-mono text-xs">{String(diff.metaA![k])}</td>
                        <td className={`py-1.5 font-mono text-xs ${
                          String(diff.metaA![k]) !== String(diff.metaB![k]) ? 'text-yellow-600 dark:text-yellow-400 font-medium' : ''
                        }`}>{String(diff.metaB![k])}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
