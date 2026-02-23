import { useAuth } from '../hooks/useAuth'

export default function TopBar() {
  const { user, logout } = useAuth()

  return (
    <header className="h-14 bg-white dark:bg-gray-800 border-b border-gray-200 dark:border-gray-700 flex items-center justify-end px-6">
      <div className="flex items-center gap-4">
        <span className="text-sm text-gray-600 dark:text-gray-400">
          {user?.accessKey}
        </span>
        <button
          onClick={logout}
          className="text-sm text-gray-500 dark:text-gray-400 hover:text-red-600 dark:hover:text-red-400 transition-colors"
        >
          Logout
        </button>
      </div>
    </header>
  )
}
