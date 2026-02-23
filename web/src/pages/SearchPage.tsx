import { useState, useCallback } from 'react'
import { useNavigate } from 'react-router-dom'
import { searchObjects, type SearchResult } from '../api/search'

function formatSize(bytes: number): string {
  if (bytes === 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(1024))
  return `${(bytes / Math.pow(1024, i)).toFixed(i === 0 ? 0 : 1)} ${units[i]}`
}

export default function SearchPage() {
  const navigate = useNavigate()
  const [query, setQuery] = useState('')
  const [bucket, setBucket] = useState('')
  const [results, setResults] = useState<SearchResult[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [searched, setSearched] = useState(false)

  const handleSearch = useCallback(async () => {
    if (!query.trim()) return
    setLoading(true)
    setError('')
    setSearched(true)
    try {
      const data = await searchObjects({ q: query.trim(), bucket: bucket || undefined, limit: 100 })
      setResults(data || [])
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Search failed')
    } finally {
      setLoading(false)
    }
  }, [query, bucket])

  return (
    <div>
      <div className="mb-6">
        <h2 className="text-xl font-semibold text-gray-900 dark:text-white">Search</h2>
        <p className="text-sm text-gray-500 dark:text-gray-400 mt-0.5">Search objects across all buckets</p>
      </div>

      {error && (
        <div className="mb-4 p-3 rounded-lg bg-red-50 dark:bg-red-900/20 text-red-700 dark:text-red-400 text-sm">
          {error}
        </div>
      )}

      {/* Search bar */}
      <div className="flex gap-3 mb-6">
        <input type="text" placeholder="Search by key, content type, or tag..."
          value={query} onChange={e => setQuery(e.target.value)}
          onKeyDown={e => e.key === 'Enter' && handleSearch()}
          className="flex-1 px-4 py-2.5 rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-800 text-gray-900 dark:text-white text-sm" />
        <input type="text" placeholder="Bucket (optional)" value={bucket}
          onChange={e => setBucket(e.target.value)}
          className="w-44 px-3 py-2.5 rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-800 text-gray-900 dark:text-white text-sm" />
        <button onClick={handleSearch} disabled={loading || !query.trim()}
          className="px-5 py-2.5 rounded-lg bg-indigo-600 hover:bg-indigo-700 disabled:bg-indigo-400 text-white text-sm font-medium transition-colors">
          {loading ? 'Searching...' : 'Search'}
        </button>
      </div>

      {/* Results */}
      {searched && (
        <div className="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 overflow-hidden">
          <div className="px-4 py-3 border-b border-gray-200 dark:border-gray-700">
            <span className="text-sm text-gray-500 dark:text-gray-400">{results.length} result{results.length !== 1 ? 's' : ''}</span>
          </div>
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-gray-200 dark:border-gray-700">
                  <th className="text-left px-4 py-3 text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Bucket</th>
                  <th className="text-left px-4 py-3 text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Key</th>
                  <th className="text-left px-4 py-3 text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Size</th>
                  <th className="text-left px-4 py-3 text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Content Type</th>
                  <th className="text-left px-4 py-3 text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Last Modified</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-100 dark:divide-gray-700/50">
                {results.map((r, i) => (
                  <tr key={i}
                    onClick={() => navigate(`/buckets/${r.bucket}/files`)}
                    className="hover:bg-gray-50 dark:hover:bg-gray-700/30 transition-colors cursor-pointer">
                    <td className="px-4 py-3 font-medium text-gray-900 dark:text-white">{r.bucket}</td>
                    <td className="px-4 py-3 text-gray-700 dark:text-gray-300 font-mono text-xs max-w-xs truncate">{r.key}</td>
                    <td className="px-4 py-3 text-gray-500 dark:text-gray-400">{formatSize(r.size)}</td>
                    <td className="px-4 py-3 text-gray-500 dark:text-gray-400">{r.content_type || '-'}</td>
                    <td className="px-4 py-3 text-gray-500 dark:text-gray-400 whitespace-nowrap">
                      {r.last_modified ? new Date(r.last_modified).toLocaleString() : '-'}
                    </td>
                  </tr>
                ))}
                {results.length === 0 && (
                  <tr><td colSpan={5} className="px-4 py-8 text-center text-gray-400">No results found</td></tr>
                )}
              </tbody>
            </table>
          </div>
        </div>
      )}
    </div>
  )
}
