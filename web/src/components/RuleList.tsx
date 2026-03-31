import { useState } from 'react'
import { Rule, api } from '../api/client'

interface Props {
  rules: Rule[]
  loading: boolean
  selectedId: string | null
  onSelect: (rule: Rule) => void
  onRefresh: () => void
  showToast: (msg: string, type?: 'success' | 'error') => void
}

export default function RuleList({ rules, loading, selectedId, onSelect, onRefresh, showToast }: Props) {
  const [creating, setCreating] = useState(false)
  const [newId, setNewId] = useState('')
  const [newName, setNewName] = useState('')
  const [newType, setNewType] = useState<string>('feature_flag')

  const handleCreate = async () => {
    if (!newId.trim() || !newName.trim()) return
    try {
      const rule = await api.createRule({
        id: newId.trim(),
        name: newName.trim(),
        type: newType as any,
        tree: { value: newType === 'feature_flag' || newType === 'kill_switch' ? false : '' },
        status: 'draft',
      })
      onRefresh()
      onSelect(rule)
      setCreating(false)
      setNewId('')
      setNewName('')
      showToast('Rule created')
    } catch (err: any) {
      showToast(err.message, 'error')
    }
  }

  if (loading) {
    return <div style={{ padding: 16, color: 'var(--text-secondary)' }}>Loading...</div>
  }

  return (
    <div>
      <div style={{ padding: '4px 8px', marginBottom: 8 }}>
        <button className="primary" style={{ width: '100%' }} onClick={() => setCreating(!creating)}>
          {creating ? 'Cancel' : '+ New Rule'}
        </button>
      </div>

      {creating && (
        <div className="card" style={{ margin: '0 4px 8px' }}>
          <div className="form-group">
            <label>ID</label>
            <input value={newId} onChange={e => setNewId(e.target.value)} placeholder="my_feature_flag" style={{ width: '100%' }} />
          </div>
          <div className="form-group">
            <label>Name</label>
            <input value={newName} onChange={e => setNewName(e.target.value)} placeholder="My Feature Flag" style={{ width: '100%' }} />
          </div>
          <div className="form-group">
            <label>Type</label>
            <select value={newType} onChange={e => setNewType(e.target.value)} style={{ width: '100%' }}>
              <option value="feature_flag">Feature Flag</option>
              <option value="decision_tree">Decision Tree</option>
              <option value="kill_switch">Kill Switch</option>
            </select>
          </div>
          <button className="primary" onClick={handleCreate} style={{ width: '100%' }}>Create</button>
        </div>
      )}

      {rules.map(rule => (
        <div
          key={rule.id}
          className={`rule-item ${rule.id === selectedId ? 'active' : ''}`}
          onClick={() => onSelect(rule)}
        >
          <div className="name">{rule.name}</div>
          <div className="meta">
            <span className={`badge ${rule.type}`}>{rule.type.replace('_', ' ')}</span>
            <span className={`badge ${rule.status}`}>{rule.status}</span>
            <span>v{rule.version}</span>
          </div>
        </div>
      ))}

      {rules.length === 0 && !loading && (
        <div className="empty-state" style={{ padding: 20 }}>
          <p>No rules yet.</p>
        </div>
      )}
    </div>
  )
}
