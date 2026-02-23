import { useEffect } from 'react'

export default function OIDCCallbackPage() {
  useEffect(() => {
    const hash = window.location.hash.substring(1)
    const params = new URLSearchParams(hash)
    const idToken = params.get('id_token')

    if (idToken && window.opener) {
      window.opener.postMessage({ type: 'oidc-callback', idToken }, window.location.origin)
      window.close()
    }
  }, [])

  return (
    <div className="min-h-screen flex items-center justify-center bg-gray-50 dark:bg-gray-900">
      <p className="text-gray-500 dark:text-gray-400 text-sm">Completing sign in...</p>
    </div>
  )
}
