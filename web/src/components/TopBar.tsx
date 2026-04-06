import { useRef } from 'react'
import { api } from '../api/client'
import { useAuth } from '../hooks/useAuth'

interface Props {
  onImport: () => void
  onWebhooks?: () => void
  onUsers?: () => void
  environment: string
  onEnvironmentChange: (env: string) => void
}

export default function TopBar({ onImport, onWebhooks, onUsers, environment, onEnvironmentChange }: Props) {
  const { username, role, logout } = useAuth()
  const fileRef = useRef<HTMLInputElement>(null)

  const handleImport = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (!file) return
    try {
      const text = await file.text()
      const rule = JSON.parse(text)
      await api.importRule(rule)
      onImport()
    } catch (err: any) {
      if (err.message?.includes('conflict')) {
        if (confirm('Rule already exists. Force overwrite?')) {
          const text = await file.text()
          const rule = JSON.parse(text)
          await api.importRule(rule, true)
          onImport()
        }
      } else {
        alert('Import failed: ' + err.message)
      }
    }
    if (fileRef.current) fileRef.current.value = ''
  }

  return (
    <div className="topbar">
      <h1>Arbiter</h1>
      <select
        value={environment}
        onChange={e => onEnvironmentChange(e.target.value)}
        style={{ marginLeft: 12, fontSize: 12, padding: '4px 8px', background: 'var(--bg-secondary)', border: '1px solid var(--border)', borderRadius: 4, color: 'var(--text-primary)' }}
      >
        <option value="">All Environments</option>
        <option value="production">Production</option>
        <option value="staging">Staging</option>
        <option value="development">Development</option>
      </select>
      <div style={{ flex: 1 }} />
      {onUsers && (
        <button onClick={onUsers}>Users</button>
      )}
      {onWebhooks && (
        <button onClick={onWebhooks}>Webhooks</button>
      )}
      <button onClick={() => fileRef.current?.click()}>Import Rule</button>
      <input
        ref={fileRef}
        type="file"
        accept=".json"
        style={{ display: 'none' }}
        onChange={handleImport}
      />
      <span style={{ fontSize: 12, color: 'var(--text-secondary)' }}>
        {username} <span className="badge" style={{ background: 'rgba(88,166,255,0.15)', color: 'var(--accent)' }}>{role}</span>
      </span>
      <button onClick={logout} style={{ fontSize: 12 }}>Logout</button>
    </div>
  )
}
