import { useState, useCallback } from 'react'
import { Rule } from './api/client'
import { useRules } from './hooks/useRules'
import TopBar from './components/TopBar'
import RuleList from './components/RuleList'
import Editor from './components/Editor'
import TestPanel from './components/TestPanel'
import TreeView from './components/TreeView'
import HistoryView from './components/HistoryView'

type Tab = 'editor' | 'test' | 'visual' | 'history'

export default function App() {
  const { rules, loading, refresh } = useRules()
  const [selectedRule, setSelectedRule] = useState<Rule | null>(null)
  const [tab, setTab] = useState<Tab>('editor')
  const [toast, setToast] = useState<{ msg: string; type: 'success' | 'error' } | null>(null)

  const showToast = useCallback((msg: string, type: 'success' | 'error' = 'success') => {
    setToast({ msg, type })
    setTimeout(() => setToast(null), 3000)
  }, [])

  const handleSelect = useCallback((rule: Rule) => {
    setSelectedRule(rule)
    setTab('editor')
  }, [])

  const handleRuleChange = useCallback(() => {
    refresh()
  }, [refresh])

  return (
    <div className="layout">
      <TopBar onImport={() => { refresh(); showToast('Rule imported') }} />
      <div className="sidebar">
        <RuleList
          rules={rules}
          loading={loading}
          selectedId={selectedRule?.id ?? null}
          onSelect={handleSelect}
          onRefresh={refresh}
          showToast={showToast}
        />
      </div>
      <div className="main">
        {selectedRule ? (
          <>
            <div className="tabs">
              <button className={`tab ${tab === 'editor' ? 'active' : ''}`} onClick={() => setTab('editor')}>Editor</button>
              <button className={`tab ${tab === 'test' ? 'active' : ''}`} onClick={() => setTab('test')}>Test</button>
              <button className={`tab ${tab === 'visual' ? 'active' : ''}`} onClick={() => setTab('visual')}>Visual</button>
              <button className={`tab ${tab === 'history' ? 'active' : ''}`} onClick={() => setTab('history')}>History</button>
            </div>
            {tab === 'editor' && (
              <Editor
                rule={selectedRule}
                onSave={(updated) => { setSelectedRule(updated); handleRuleChange(); showToast('Rule saved') }}
                onDelete={() => { setSelectedRule(null); handleRuleChange(); showToast('Rule deleted') }}
                showToast={showToast}
              />
            )}
            {tab === 'test' && <TestPanel rule={selectedRule} />}
            {tab === 'visual' && <TreeView tree={selectedRule.tree} />}
            {tab === 'history' && <HistoryView rule={selectedRule} onRollback={(r) => { setSelectedRule(r); handleRuleChange(); showToast(`Rolled back to v${r.version - 1}`) }} />}
          </>
        ) : (
          <div className="empty-state">
            <p>Select a rule from the sidebar, or create a new one.</p>
          </div>
        )}
      </div>
      {toast && <div className={`toast ${toast.type}`}>{toast.msg}</div>}
    </div>
  )
}
