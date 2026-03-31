interface Props {
  path: string[]
}

export default function PathView({ path }: Props) {
  if (!path || path.length === 0) return null

  return (
    <div className="path" style={{ marginTop: 8 }}>
      <div style={{ fontSize: 11, color: 'var(--text-muted)', marginBottom: 4 }}>Decision Path:</div>
      {path.map((step, i) => (
        <div key={i} className="path-step" style={{ paddingLeft: i * 12 }}>
          <span style={{ color: 'var(--text-muted)', marginRight: 4 }}>{i + 1}.</span>
          <span style={{
            color: step.includes('true') ? 'var(--success)' :
                   step.includes('false') ? 'var(--danger)' :
                   step.includes('value:') ? 'var(--accent)' :
                   'var(--text-secondary)'
          }}>
            {step}
          </span>
        </div>
      ))}
    </div>
  )
}
