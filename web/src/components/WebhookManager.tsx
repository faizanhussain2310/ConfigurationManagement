import { useState, useEffect, useCallback } from 'react'
import { api, Webhook } from '../api/client'

interface Props {
  showToast: (msg: string, type?: 'success' | 'error') => void
}

export default function WebhookManager({ showToast }: Props) {
  const [webhooks, setWebhooks] = useState<Webhook[]>([])
  const [loading, setLoading] = useState(true)
  const [url, setUrl] = useState('')
  const [events, setEvents] = useState('*')

  const fetchWebhooks = useCallback(async () => {
    try {
      setLoading(true)
      const data = await api.listWebhooks()
      setWebhooks(data.webhooks || [])
    } catch (err: any) {
      showToast(err.message, 'error')
    } finally {
      setLoading(false)
    }
  }, [showToast])

  useEffect(() => {
    fetchWebhooks()
  }, [fetchWebhooks])

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!url.trim()) return
    try {
      await api.createWebhook(url.trim(), events.trim() || '*')
      setUrl('')
      setEvents('*')
      fetchWebhooks()
      showToast('Webhook created')
    } catch (err: any) {
      showToast(err.message, 'error')
    }
  }

  const handleDelete = async (id: number) => {
    if (!confirm('Delete this webhook?')) return
    try {
      await api.deleteWebhook(id)
      fetchWebhooks()
      showToast('Webhook deleted')
    } catch (err: any) {
      showToast(err.message, 'error')
    }
  }

  return (
    <div>
      <h3 style={{ fontSize: 14, marginBottom: 12 }}>Webhook Subscriptions</h3>

      <div className="card">
        <form onSubmit={handleCreate}>
          <div className="form-row">
            <div className="form-group" style={{ flex: 3 }}>
              <label>URL</label>
              <input
                value={url}
                onChange={e => setUrl(e.target.value)}
                placeholder="https://example.com/webhook"
                style={{ width: '100%' }}
              />
            </div>
            <div className="form-group" style={{ flex: 1 }}>
              <label>Events</label>
              <input
                value={events}
                onChange={e => setEvents(e.target.value)}
                placeholder="* or rule.created"
                style={{ width: '100%' }}
              />
            </div>
            <div className="form-group" style={{ flex: 0, alignSelf: 'flex-end' }}>
              <button className="primary" type="submit">Add</button>
            </div>
          </div>
        </form>
        <p style={{ fontSize: 11, color: 'var(--text-muted)', marginTop: 4 }}>
          Events: * (all), rule.created, rule.updated, rule.deleted
        </p>
      </div>

      {loading ? (
        <div className="empty-state">Loading...</div>
      ) : webhooks.length === 0 ? (
        <div className="empty-state">No webhooks configured.</div>
      ) : (
        <div className="card" style={{ marginTop: 12 }}>
          <table style={{ width: '100%', fontSize: 13, borderCollapse: 'collapse' }}>
            <thead>
              <tr style={{ borderBottom: '1px solid var(--border)', textAlign: 'left' }}>
                <th style={{ padding: '6px 8px', color: 'var(--text-secondary)', fontWeight: 500 }}>URL</th>
                <th style={{ padding: '6px 8px', color: 'var(--text-secondary)', fontWeight: 500 }}>Events</th>
                <th style={{ padding: '6px 8px', color: 'var(--text-secondary)', fontWeight: 500 }}>Secret</th>
                <th style={{ padding: '6px 8px', width: 60 }}></th>
              </tr>
            </thead>
            <tbody>
              {webhooks.map(wh => (
                <tr key={wh.id} style={{ borderBottom: '1px solid var(--border)' }}>
                  <td style={{ padding: '6px 8px', fontFamily: 'monospace', fontSize: 12 }}>{wh.url}</td>
                  <td style={{ padding: '6px 8px' }}>{wh.events}</td>
                  <td style={{ padding: '6px 8px', fontFamily: 'monospace', fontSize: 11 }}>
                    {wh.secret ? wh.secret.slice(0, 8) + '...' : '-'}
                  </td>
                  <td style={{ padding: '6px 8px' }}>
                    <button className="danger" onClick={() => handleDelete(wh.id)} style={{ fontSize: 11, padding: '2px 8px' }}>
                      Delete
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}
