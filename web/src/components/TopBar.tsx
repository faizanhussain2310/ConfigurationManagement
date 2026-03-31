import { useRef } from 'react'
import { api } from '../api/client'

interface Props {
  onImport: () => void
}

export default function TopBar({ onImport }: Props) {
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
      <div style={{ flex: 1 }} />
      <button onClick={() => fileRef.current?.click()}>Import Rule</button>
      <input
        ref={fileRef}
        type="file"
        accept=".json"
        style={{ display: 'none' }}
        onChange={handleImport}
      />
    </div>
  )
}
