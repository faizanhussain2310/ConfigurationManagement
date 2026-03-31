import { createContext, useContext, useState, useEffect, useCallback, ReactNode } from 'react'
import { api, setToken, getToken } from '../api/client'

interface AuthState {
  username: string | null
  role: string | null
  loading: boolean
  login: (username: string, password: string) => Promise<void>
  logout: () => void
}

const AuthContext = createContext<AuthState>({
  username: null,
  role: null,
  loading: true,
  login: async () => {},
  logout: () => {},
})

export function AuthProvider({ children }: { children: ReactNode }) {
  const [username, setUsername] = useState<string | null>(null)
  const [role, setRole] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    if (getToken()) {
      api.me()
        .then(data => {
          setUsername(data.username)
          setRole(data.role)
        })
        .catch(() => {
          setToken(null)
        })
        .finally(() => setLoading(false))
    } else {
      setLoading(false)
    }
  }, [])

  const login = useCallback(async (user: string, pass: string) => {
    const data = await api.login(user, pass)
    setToken(data.token)
    setUsername(data.username)
    setRole(data.role)
  }, [])

  const logout = useCallback(() => {
    setToken(null)
    setUsername(null)
    setRole(null)
  }, [])

  return (
    <AuthContext.Provider value={{ username, role, loading, login, logout }}>
      {children}
    </AuthContext.Provider>
  )
}

export function useAuth() {
  return useContext(AuthContext)
}
