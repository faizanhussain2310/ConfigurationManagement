import { useState, useEffect } from 'react'
import CodeMirror from '@uiw/react-codemirror'
import { json } from '@codemirror/lang-json'
import { Rule, api } from '../api/client'

interface Props {
  rule: Rule
  onSave: (rule: Rule) => void
  onDelete: () => void
  showToast: (msg: string, type?: 'success' | 'error') => void
}

export default function Editor({ rule, onSave, onDelete, showToast }: Props) {
  const [name, setName] = useState(rule.name)
  const [description, setDescription] = useState(rule.description)
  const [type, setType] = useState(rule.type)
  const [status, setStatus] = useState(rule.status)
  const [treeJson, setTreeJson] = useState(JSON.stringify(rule.tree, null, 2))
  const [defaultJson, setDefaultJson] = useState(
    rule.default_value != null ? JSON.stringify(rule.default_value, null, 2) : ''
  )
  const [jsonError, setJsonError] = useState<string | null>(null)
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    setName(rule.name)
    setDescription(rule.description)
    setType(rule.type)
    setStatus(rule.status)
    setTreeJson(JSON.stringify(rule.tree, null, 2))
    setDefaultJson(rule.default_value != null ? JSON.stringify(rule.default_value, null, 2) : '')
    setJsonError(null)
  }, [rule.id, rule.version])

  const handleSave = async () => {
    let tree: any
    try {
      tree = JSON.parse(treeJson)
      setJsonError(null)
    } catch {
      setJsonError('Invalid JSON in tree')
      return
    }

    let defaultValue = undefined
    if (defaultJson.trim()) {
      try {
        defaultValue = JSON.parse(defaultJson)
      } catch {
        setJsonError('Invalid JSON in default value')
        return
      }
    }

    setSaving(true)
    try {
      const updated = await api.updateRule(rule.id, {
        name,
        description,
        type: type as any,
        status: status as any,
        tree,
        default_value: defaultValue,
      })
      onSave(updated)
    } catch (err: any) {
      showToast(err.message, 'error')
    } finally {
      setSaving(false)
    }
  }

  const handleDelete = async () => {
    if (!confirm(`Delete rule "${rule.name}"? This cannot be undone.`)) return
    try {
      await api.deleteRule(rule.id)
      onDelete()
    } catch (err: any) {
      showToast(err.message, 'error')
    }
  }

  const handleDuplicate = async () => {
    try {
      const dup = await api.duplicate(rule.id)
      showToast(`Duplicated as ${dup.id}`)
    } catch (err: any) {
      showToast(err.message, 'error')
    }
  }

  const handleExport = () => {
    window.open(api.exportRule(rule.id), '_blank')
  }

  return (
    <div>
      <div className="toolbar">
        <span style={{ fontSize: 13, color: 'var(--text-secondary)' }}>
          {rule.id} &middot; v{rule.version}
        </span>
        <div className="toolbar-spacer" />
        <button onClick={handleExport}>Export</button>
        <button onClick={handleDuplicate}>Duplicate</button>
        <button className="danger" onClick={handleDelete}>Delete</button>
      </div>

      <div className="card">
        <div className="form-row">
          <div className="form-group">
            <label>Name</label>
            <input value={name} onChange={e => setName(e.target.value)} style={{ width: '100%' }} />
          </div>
          <div className="form-group">
            <label>Type</label>
            <select value={type} onChange={e => setType(e.target.value as any)} style={{ width: '100%' }}>
              <option value="feature_flag">Feature Flag</option>
              <option value="decision_tree">Decision Tree</option>
              <option value="kill_switch">Kill Switch</option>
            </select>
          </div>
          <div className="form-group">
            <label>Status</label>
            <select value={status} onChange={e => setStatus(e.target.value as any)} style={{ width: '100%' }}>
              <option value="active">Active</option>
              <option value="draft">Draft</option>
              <option value="disabled">Disabled</option>
            </select>
          </div>
        </div>
        <div className="form-group">
          <label>Description</label>
          <input value={description} onChange={e => setDescription(e.target.value)} style={{ width: '100%' }} />
        </div>
      </div>

      <div className="card">
        <label style={{ fontSize: 12, fontWeight: 500, color: 'var(--text-secondary)', marginBottom: 4, display: 'block' }}>
          Decision Tree
        </label>
        {jsonError && (
          <div style={{ color: 'var(--danger)', fontSize: 12, marginBottom: 8, padding: '4px 8px', background: 'rgba(248,81,73,0.1)', borderRadius: 4 }}>
            {jsonError}
          </div>
        )}
        <CodeMirror
          value={treeJson}
          onChange={setTreeJson}
          extensions={[json()]}
          theme="dark"
          height="300px"
          basicSetup={{ lineNumbers: true, foldGutter: true }}
        />
      </div>

      <div className="card">
        <label style={{ fontSize: 12, fontWeight: 500, color: 'var(--text-secondary)', marginBottom: 4, display: 'block' }}>
          Default Value (optional)
        </label>
        <CodeMirror
          value={defaultJson}
          onChange={setDefaultJson}
          extensions={[json()]}
          theme="dark"
          height="60px"
          basicSetup={{ lineNumbers: false }}
        />
      </div>

      <div className="actions">
        <button className="primary" onClick={handleSave} disabled={saving}>
          {saving ? 'Saving...' : 'Save'}
        </button>
      </div>
    </div>
  )
}
