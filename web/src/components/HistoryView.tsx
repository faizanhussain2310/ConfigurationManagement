import { useState, useEffect } from 'react'
import { Rule, VersionSummary, EvalHistoryEntry, api } from '../api/client'
import DiffView from './DiffView'

interface Props {
  rule: Rule
  onRollback: (rule: Rule) => void
}

type HistoryTab = 'versions' | 'evaluations'

export default function HistoryView({ rule, onRollback }: Props) {
  const [historyTab, setHistoryTab] = useState<HistoryTab>('versions')
  const [versions, setVersions] = useState<VersionSummary[]>([])
  const [evalHistory, setEvalHistory] = useState<EvalHistoryEntry[]>([])
  const [selectedVersion, setSelectedVersion] = useState<number | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    loadData()
  }, [rule.id, rule.version])

  const loadData = async () => {
    setLoading(true)
    try {
      const [vData, hData] = await Promise.all([
        api.getVersions(rule.id),
        api.getHistory(rule.id),
      ])
      setVersions(vData.versions)
      setEvalHistory(hData.entries)
    } catch {
      // handle error
    } finally {
      setLoading(false)
    }
  }

  const handleRollback = async (version: number) => {
    if (!confirm(`Rollback to version ${version}? This will create a new version.`)) return
    try {
      const updated = await api.rollback(rule.id, version)
      onRollback(updated)
    } catch (err: any) {
      alert('Rollback failed: ' + err.message)
    }
  }

  if (loading) return <div style={{ color: 'var(--text-secondary)' }}>Loading...</div>

  return (
    <div>
      <div className="tabs">
        <button className={`tab ${historyTab === 'versions' ? 'active' : ''}`} onClick={() => setHistoryTab('versions')}>
          Versions ({versions.length})
        </button>
        <button className={`tab ${historyTab === 'evaluations' ? 'active' : ''}`} onClick={() => setHistoryTab('evaluations')}>
          Evaluations ({evalHistory.length})
        </button>
      </div>

      {historyTab === 'versions' && (
        <div className="card">
          <div className="version-list">
            {versions.map(v => (
              <div key={v.version} className="version-item">
                <div>
                  <strong>v{v.version}</strong>
                  <span style={{ marginLeft: 8, color: 'var(--text-secondary)', fontSize: 12 }}>
                    {v.name}
                  </span>
                  <span className={`badge ${v.status}`} style={{ marginLeft: 8 }}>{v.status}</span>
                </div>
                <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
                  <span style={{ fontSize: 11, color: 'var(--text-muted)' }}>
                    {new Date(v.created_at).toLocaleString()}
                  </span>
                  {v.version < rule.version && (
                    <>
                      <button onClick={() => setSelectedVersion(selectedVersion === v.version ? null : v.version)} style={{ fontSize: 11 }}>
                        {selectedVersion === v.version ? 'Hide Diff' : 'Show Diff'}
                      </button>
                      <button onClick={() => handleRollback(v.version)} style={{ fontSize: 11 }}>
                        Rollback
                      </button>
                    </>
                  )}
                  {v.version === rule.version && (
                    <span style={{ fontSize: 11, color: 'var(--success)' }}>current</span>
                  )}
                </div>
              </div>
            ))}
          </div>
        </div>
      )}

      {historyTab === 'versions' && selectedVersion !== null && (
        <DiffView ruleId={rule.id} fromVersion={selectedVersion} toVersion={selectedVersion + 1} />
      )}

      {historyTab === 'evaluations' && (
        <div className="card">
          {evalHistory.length === 0 ? (
            <div className="empty-state">No evaluations yet. Use the Test tab to evaluate.</div>
          ) : (
            evalHistory.map(entry => (
              <div key={entry.id} className="history-entry">
                <div style={{ display: 'flex', justifyContent: 'space-between' }}>
                  <span style={{ fontWeight: 500 }}>
                    Result: {JSON.stringify(typeof entry.result === 'string' ? JSON.parse(entry.result).value : entry.result?.value)}
                  </span>
                  <span style={{ fontSize: 11, color: 'var(--text-muted)' }}>
                    {new Date(entry.created_at).toLocaleString()}
                  </span>
                </div>
                <div style={{ fontSize: 11, color: 'var(--text-secondary)', marginTop: 2, fontFamily: 'monospace' }}>
                  ctx: {typeof entry.context === 'string' ? entry.context : JSON.stringify(entry.context)}
                </div>
              </div>
            ))
          )}
        </div>
      )}
    </div>
  )
}
