import { useState, useCallback } from 'react'
import { Rule } from './api/client'
import { useRules } from './hooks/useRules'
import { AuthProvider, useAuth } from './hooks/useAuth'
import LoginPage from './components/LoginPage'
import TopBar from './components/TopBar'
import RuleList from './components/RuleList'
import Editor from './components/Editor'
import TestPanel from './components/TestPanel'
import TreeEditor from './components/TreeEditor'
import HistoryView from './components/HistoryView'
import WebhookManager from './components/WebhookManager'
import UserManager from './components/UserManager'

type Tab = 'editor' | 'test' | 'visual' | 'history'
type AdminView = 'webhooks' | 'users' | null

function Dashboard() {
  const { role } = useAuth()
  const [environment, setEnvironment] = useState('')
  const { rules, loading, refresh } = useRules(environment)
  const [selectedRule, setSelectedRule] = useState<Rule | null>(null)
  const [tab, setTab] = useState<Tab>('editor')
  const [adminView, setAdminView] = useState<AdminView>(null)
  const [toast, setToast] = useState<{ msg: string; type: 'success' | 'error' } | null>(null)

  const showToast = useCallback((msg: string, type: 'success' | 'error' = 'success') => {
    setToast({ msg, type })
    setTimeout(() => setToast(null), 3000)
  }, [])

  const handleSelect = useCallback((rule: Rule) => {
    setSelectedRule(rule)
    setTab('editor')
    setAdminView(null)
  }, [])

  const handleRuleChange = useCallback(() => {
    refresh()
  }, [refresh])

  return (
    <div className="layout">
      <TopBar
        onImport={() => { refresh(); showToast('Rule imported') }}
        onWebhooks={role === 'admin' ? () => { setAdminView('webhooks'); setSelectedRule(null) } : undefined}
        onUsers={role === 'admin' ? () => { setAdminView('users'); setSelectedRule(null) } : undefined}
        environment={environment}
        onEnvironmentChange={setEnvironment}
      />
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
        {adminView === 'webhooks' ? (
          <WebhookManager showToast={showToast} />
        ) : adminView === 'users' ? (
          <UserManager showToast={showToast} />
        ) : selectedRule ? (
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
            {tab === 'visual' && <TreeEditor tree={selectedRule.tree} />}
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

export default function App() {
  const { username, loading } = useAuth()

  if (loading) {
    return (
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', height: '100vh', background: 'var(--bg-primary)' }}>
        <span style={{ color: 'var(--text-secondary)' }}>Loading...</span>
      </div>
    )
  }

  if (!username) {
    return <LoginPage />
  }

  return <Dashboard />
}
