import { useState } from 'react'
import CodeMirror from '@uiw/react-codemirror'
import { json } from '@codemirror/lang-json'
import { Rule, EvalResult, api } from '../api/client'
import PathView from './PathView'

interface Props {
  rule: Rule
}

export default function TestPanel({ rule }: Props) {
  const [contextJson, setContextJson] = useState('{\n  \n}')
  const [result, setResult] = useState<EvalResult | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)

  const handleEvaluate = async () => {
    let ctx: Record<string, any>
    try {
      ctx = JSON.parse(contextJson)
      setError(null)
    } catch {
      setError('Invalid JSON context')
      return
    }

    setLoading(true)
    try {
      const res = await api.evaluate(rule.id, ctx)
      setResult(res)
      if (res.error) {
        setError(res.error)
      }
    } catch (err: any) {
      setError(err.message)
    } finally {
      setLoading(false)
    }
  }

  return (
    <div>
      <div className="card">
        <label style={{ fontSize: 12, fontWeight: 500, color: 'var(--text-secondary)', marginBottom: 8, display: 'block' }}>
          Evaluation Context
        </label>
        <CodeMirror
          value={contextJson}
          onChange={setContextJson}
          extensions={[json()]}
          theme="dark"
          height="200px"
          basicSetup={{ lineNumbers: true }}
        />
        <div className="actions">
          <button className="primary" onClick={handleEvaluate} disabled={loading}>
            {loading ? 'Evaluating...' : 'Evaluate'}
          </button>
        </div>
      </div>

      {error && (
        <div className="card" style={{ borderColor: 'var(--danger)' }}>
          <div style={{ color: 'var(--danger)', fontSize: 13 }}>{error}</div>
        </div>
      )}

      {result && !result.error && (
        <div className="result-card">
          <div className="value" style={{
            color: result.value === true ? 'var(--success)' :
                   result.value === false ? 'var(--danger)' :
                   'var(--accent)'
          }}>
            {JSON.stringify(result.value)}
            {result.default && <span style={{ fontSize: 12, color: 'var(--text-muted)', marginLeft: 8 }}>(default)</span>}
          </div>
          <PathView path={result.path} />
          <div className="elapsed">Evaluated in {result.elapsed}</div>
        </div>
      )}
    </div>
  )
}
