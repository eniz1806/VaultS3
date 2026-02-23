const BASE = '/api/v1'

export async function apiFetch<T>(path: string, opts: RequestInit = {}): Promise<T> {
  const token = localStorage.getItem('vaults3_token')
  const res = await fetch(`${BASE}${path}`, {
    ...opts,
    headers: {
      'Content-Type': 'application/json',
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
      ...(opts.headers as Record<string, string>),
    },
  })

  if (res.status === 401) {
    localStorage.removeItem('vaults3_token')
    window.location.href = '/dashboard/login'
    throw new Error('Unauthorized')
  }

  if (!res.ok) {
    const body = await res.json().catch(() => ({ error: res.statusText }))
    throw new Error(body.error || res.statusText)
  }

  if (res.status === 204) return undefined as T
  return res.json()
}
