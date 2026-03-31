import { useState, useEffect } from 'react'

interface Props {
  ruleId: string
  fromVersion: number
  toVersion: number
}

export default function DiffView({ ruleId, fromVersion, toVersion }: Props) {
  const [diff, setDiff] = useState<any>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    loadDiff()
  }, [ruleId, fromVersion, toVersion])

  const loadDiff = async () => {
    setLoading(true)
    try {
      const [fromRes, toRes] = await Promise.all([
        fetch(`/api/rules/${ruleId}/versions`).then(r => r.json()),
        fetch(`/api/rules/${ruleId}/versions`).then(r => r.json()),
      ])

      // Find the version snapshots. We need to compute diff client-side since
      // the API returns summaries. For v1, we show a simple before/after of the
      // tree JSON by fetching the full rule at each version.
      // Simplified: show the version summaries side by side.
      const versions = fromRes.versions || []
      const fromV = versions.find((v: any) => v.version === fromVersion)
      const toV = versions.find((v: any) => v.version === toVersion)
      setDiff({ from: fromV, to: toV })
    } catch {
      setDiff(null)
    } finally {
      setLoading(false)
    }
  }

  if (loading) return <div style={{ color: 'var(--text-secondary)', padding: 8 }}>Loading diff...</div>
  if (!diff) return null

  return (
    <div className="card" style={{ marginTop: 8 }}>
      <div style={{ fontSize: 12, fontWeight: 500, marginBottom: 8, color: 'var(--text-secondary)' }}>
        Changes: v{fromVersion} → v{toVersion}
      </div>
      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 12 }}>
        <div>
          <div style={{ fontSize: 11, color: 'var(--text-muted)', marginBottom: 4 }}>v{fromVersion}</div>
          {diff.from && (
            <div style={{ fontSize: 12 }}>
              <div>Name: {diff.from.name}</div>
              <div>Status: <span className={`badge ${diff.from.status}`}>{diff.from.status}</span></div>
            </div>
          )}
        </div>
        <div>
          <div style={{ fontSize: 11, color: 'var(--text-muted)', marginBottom: 4 }}>v{toVersion}</div>
          {diff.to && (
            <div style={{ fontSize: 12 }}>
              <div>Name: {diff.to.name}{diff.from?.name !== diff.to?.name && <span className="diff-added"> (changed)</span>}</div>
              <div>Status: <span className={`badge ${diff.to.status}`}>{diff.to.status}</span>
                {diff.from?.status !== diff.to?.status && <span className="diff-added"> (changed)</span>}
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
