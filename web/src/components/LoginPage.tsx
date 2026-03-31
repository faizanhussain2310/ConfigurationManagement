import { useState } from 'react'
import { useAuth } from '../hooks/useAuth'

export default function LoginPage() {
  const { login } = useAuth()
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError(null)
    setLoading(true)
    try {
      await login(username, password)
    } catch (err: any) {
      setError(err.message)
    } finally {
      setLoading(false)
    }
  }

  return (
    <div style={{
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      height: '100vh',
      background: 'var(--bg-primary)',
    }}>
      <div className="card" style={{ width: 360, padding: 24 }}>
        <h2 style={{ fontSize: 18, marginBottom: 16, textAlign: 'center' }}>
          Arbiter Login
        </h2>
        {error && (
          <div style={{
            color: 'var(--danger)',
            fontSize: 12,
            marginBottom: 12,
            padding: '6px 10px',
            background: 'rgba(248,81,73,0.1)',
            borderRadius: 4,
          }}>
            {error}
          </div>
        )}
        <form onSubmit={handleSubmit}>
          <div className="form-group">
            <label>Username</label>
            <input
              value={username}
              onChange={e => setUsername(e.target.value)}
              style={{ width: '100%' }}
              autoFocus
            />
          </div>
          <div className="form-group">
            <label>Password</label>
            <input
              type="password"
              value={password}
              onChange={e => setPassword(e.target.value)}
              style={{ width: '100%' }}
            />
          </div>
          <button className="primary" type="submit" disabled={loading} style={{ width: '100%', marginTop: 8 }}>
            {loading ? 'Logging in...' : 'Login'}
          </button>
        </form>
        <p style={{ fontSize: 11, color: 'var(--text-muted)', textAlign: 'center', marginTop: 12 }}>
          Default: admin / admin
        </p>
      </div>
    </div>
  )
}
