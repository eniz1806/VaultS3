import { createContext, useContext, useState, useEffect, useCallback, type ReactNode } from 'react'
import { createElement } from 'react'
import { login as apiLogin, getMe, type MeResponse } from '../api/auth'

interface AuthContextType {
  token: string | null
  user: MeResponse | null
  isAuthenticated: boolean
  isLoading: boolean
  login: (accessKey: string, secretKey: string) => Promise<void>
  logout: () => void
}

const AuthContext = createContext<AuthContextType | null>(null)

export function AuthProvider({ children }: { children: ReactNode }) {
  const [token, setToken] = useState<string | null>(() => localStorage.getItem('vaults3_token'))
  const [user, setUser] = useState<MeResponse | null>(null)
  const [isLoading, setIsLoading] = useState(!!token)

  useEffect(() => {
    if (token) {
      getMe()
        .then(setUser)
        .catch(() => {
          localStorage.removeItem('vaults3_token')
          setToken(null)
        })
        .finally(() => setIsLoading(false))
    }
  }, [token])

  const login = useCallback(async (accessKey: string, secretKey: string) => {
    const res = await apiLogin(accessKey, secretKey)
    localStorage.setItem('vaults3_token', res.token)
    setToken(res.token)
    const me = await getMe()
    setUser(me)
  }, [])

  const logout = useCallback(() => {
    localStorage.removeItem('vaults3_token')
    setToken(null)
    setUser(null)
  }, [])

  return createElement(AuthContext.Provider, {
    value: { token, user, isAuthenticated: !!token && !!user, isLoading, login, logout },
    children,
  })
}

export function useAuth(): AuthContextType {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error('useAuth must be used within AuthProvider')
  return ctx
}
