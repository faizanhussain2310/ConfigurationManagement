import { useState, useEffect } from 'react'
import { api, User } from '../api/client'

interface Props {
  showToast: (msg: string, type?: 'success' | 'error') => void
}

export default function UserManager({ showToast }: Props) {
  const [users, setUsers] = useState<User[]>([])
  const [loading, setLoading] = useState(true)
  const [creating, setCreating] = useState(false)
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [role, setRole] = useState('viewer')

  const loadUsers = async () => {
    try {
      setLoading(true)
      const data = await api.listUsers()
      setUsers(data.users)
    } catch (err: any) {
      showToast(err.message, 'error')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    loadUsers()
  }, [])

  const handleCreate = async () => {
    if (!username.trim() || !password.trim()) return
    try {
      await api.register(username.trim(), password.trim(), role)
      showToast(`User "${username}" created`)
      setUsername('')
      setPassword('')
      setRole('viewer')
      setCreating(false)
      loadUsers()
    } catch (err: any) {
      showToast(err.message, 'error')
    }
  }

  if (loading) return <div style={{ color: 'var(--text-secondary)', padding: 20 }}>Loading users...</div>

  return (
    <div>
      <div className="toolbar">
        <h2 style={{ margin: 0, fontSize: 16 }}>User Management</h2>
        <div className="toolbar-spacer" />
        <button className="primary" onClick={() => setCreating(!creating)}>
          {creating ? 'Cancel' : '+ New User'}
        </button>
      </div>

      {creating && (
        <div className="card">
          <div className="form-row">
            <div className="form-group">
              <label>Username</label>
              <input
                value={username}
                onChange={e => setUsername(e.target.value)}
                placeholder="username"
                style={{ width: '100%' }}
              />
            </div>
            <div className="form-group">
              <label>Password</label>
              <input
                type="password"
                value={password}
                onChange={e => setPassword(e.target.value)}
                placeholder="password"
                style={{ width: '100%' }}
              />
            </div>
            <div className="form-group">
              <label>Role</label>
              <select value={role} onChange={e => setRole(e.target.value)} style={{ width: '100%' }}>
                <option value="viewer">Viewer</option>
                <option value="editor">Editor</option>
                <option value="admin">Admin</option>
              </select>
            </div>
          </div>
          <button className="primary" onClick={handleCreate}>Create User</button>
        </div>
      )}

      <div className="card">
        <table style={{ width: '100%', borderCollapse: 'collapse' }}>
          <thead>
            <tr style={{ borderBottom: '1px solid var(--border)', textAlign: 'left' }}>
              <th style={{ padding: '8px 12px', fontSize: 12, color: 'var(--text-secondary)' }}>Username</th>
              <th style={{ padding: '8px 12px', fontSize: 12, color: 'var(--text-secondary)' }}>Role</th>
              <th style={{ padding: '8px 12px', fontSize: 12, color: 'var(--text-secondary)' }}>Created</th>
            </tr>
          </thead>
          <tbody>
            {users.map(user => (
              <tr key={user.id} style={{ borderBottom: '1px solid var(--border)' }}>
                <td style={{ padding: '8px 12px', fontSize: 13 }}>{user.username}</td>
                <td style={{ padding: '8px 12px' }}>
                  <span className={`badge ${user.role}`} style={{
                    background: user.role === 'admin' ? 'rgba(248,81,73,0.15)' : user.role === 'editor' ? 'rgba(210,153,34,0.15)' : 'rgba(88,166,255,0.15)',
                    color: user.role === 'admin' ? 'var(--danger)' : user.role === 'editor' ? 'var(--warning)' : 'var(--accent)',
                  }}>
                    {user.role}
                  </span>
                </td>
                <td style={{ padding: '8px 12px', fontSize: 11, color: 'var(--text-muted)' }}>
                  {new Date(user.created_at).toLocaleDateString()}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
        {users.length === 0 && (
          <div className="empty-state">No users found.</div>
        )}
      </div>
    </div>
  )
}
