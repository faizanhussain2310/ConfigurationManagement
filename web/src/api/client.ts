const BASE = '/api'

let authToken: string | null = localStorage.getItem('arbiter_token')

export function setToken(token: string | null) {
  authToken = token
  if (token) {
    localStorage.setItem('arbiter_token', token)
  } else {
    localStorage.removeItem('arbiter_token')
  }
}

export function getToken(): string | null {
  return authToken
}

export interface Rule {
  id: string
  name: string
  description: string
  type: 'feature_flag' | 'decision_tree' | 'kill_switch' | 'composite'
  version: number
  tree: any
  default_value?: any
  status: 'active' | 'draft' | 'disabled'
  environment: string
  active_from?: string | null
  active_until?: string | null
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
  modified_by: string
  created_at: string
}

export interface EvalHistoryEntry {
  id: number
  rule_id: string
  context: any
  result: any
  created_at: string
}

export interface User {
  id: number
  username: string
  role: string
  created_at: string
}

export interface Webhook {
  id: number
  url: string
  events: string
  secret: string
  active: boolean
  created_at: string
}

async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const headers: Record<string, string> = { 'Content-Type': 'application/json' }
  if (authToken) {
    headers['Authorization'] = `Bearer ${authToken}`
  }
  const res = await fetch(BASE + path, {
    headers,
    ...options,
  })
  const data = await res.json()
  if (!res.ok) {
    if (res.status === 401) {
      setToken(null)
    }
    throw new Error(data.error || `HTTP ${res.status}`)
  }
  return data as T
}

export const api = {
  // Auth
  login: (username: string, password: string) =>
    request<{ token: string; username: string; role: string }>('/auth/login', {
      method: 'POST',
      body: JSON.stringify({ username, password }),
    }),

  me: () => request<{ username: string; role: string }>('/auth/me'),

  register: (username: string, password: string, role: string) =>
    request<User>('/auth/register', {
      method: 'POST',
      body: JSON.stringify({ username, password, role }),
    }),

  listUsers: () => request<{ users: User[] }>('/auth/users'),

  // Rules
  health: () => request<{ status: string }>('/health'),

  listRules: (limit = 50, offset = 0, environment = '') => {
    let url = `/rules?limit=${limit}&offset=${offset}`
    if (environment) url += `&environment=${environment}`
    return request<{ rules: Rule[]; total: number }>(url)
  },

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

  // Webhooks
  listWebhooks: () => request<{ webhooks: Webhook[] }>('/webhooks'),

  createWebhook: (url: string, events: string) =>
    request<Webhook>('/webhooks', {
      method: 'POST',
      body: JSON.stringify({ url, events }),
    }),

  deleteWebhook: (id: number) =>
    request<{ status: string }>(`/webhooks/${id}`, { method: 'DELETE' }),
}
