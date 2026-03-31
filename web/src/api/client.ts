const BASE = '/api'

export interface Rule {
  id: string
  name: string
  description: string
  type: 'feature_flag' | 'decision_tree' | 'kill_switch'
  version: number
  tree: any
  default_value?: any
  status: 'active' | 'draft' | 'disabled'
  created_at: string
  updated_at: string
}

export interface EvalResult {
  value: any
  path: string[]
  default: boolean
  error?: string
  elapsed: string
}

export interface VersionSummary {
  version: number
  name: string
  status: string
  created_at: string
}

export interface EvalHistoryEntry {
  id: number
  rule_id: string
  context: any
  result: any
  created_at: string
}

async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const res = await fetch(BASE + path, {
    headers: { 'Content-Type': 'application/json' },
    ...options,
  })
  const data = await res.json()
  if (!res.ok) {
    throw new Error(data.error || `HTTP ${res.status}`)
  }
  return data as T
}

export const api = {
  health: () => request<{ status: string }>('/health'),

  listRules: (limit = 50, offset = 0) =>
    request<{ rules: Rule[]; total: number }>(`/rules?limit=${limit}&offset=${offset}`),

  getRule: (id: string) => request<Rule>(`/rules/${id}`),

  createRule: (rule: Partial<Rule>) =>
    request<Rule>('/rules', { method: 'POST', body: JSON.stringify(rule) }),

  updateRule: (id: string, rule: Partial<Rule>) =>
    request<Rule>(`/rules/${id}`, { method: 'PUT', body: JSON.stringify(rule) }),

  deleteRule: (id: string) =>
    request<{ status: string }>(`/rules/${id}`, { method: 'DELETE' }),

  evaluate: (id: string, context: Record<string, any>) =>
    request<EvalResult>(`/rules/${id}/evaluate`, {
      method: 'POST',
      body: JSON.stringify({ context }),
    }),

  getHistory: (id: string, limit = 50, offset = 0) =>
    request<{ entries: EvalHistoryEntry[]; total: number }>(
      `/rules/${id}/history?limit=${limit}&offset=${offset}`
    ),

  getVersions: (id: string) =>
    request<{ versions: VersionSummary[] }>(`/rules/${id}/versions`),

  rollback: (id: string, version: number) =>
    request<Rule>(`/rules/${id}/rollback/${version}`, { method: 'POST' }),

  duplicate: (id: string) =>
    request<Rule>(`/rules/${id}/duplicate`, { method: 'POST' }),

  exportRule: (id: string) => `${BASE}/rules/${id}/export`,

  importRule: (rule: any, force = false) =>
    request<Rule>(`/rules/import${force ? '?force=true' : ''}`, {
      method: 'POST',
      body: JSON.stringify(rule),
    }),
}
