// Kiểu wire khớp JSON của apps/viderse/internal/web.

export interface Brief {
  topic: string
  audience?: string
  target_seconds?: number
  target_words?: number
  words_per_minute?: number
}

export type RunStatus = 'queued' | 'running' | 'succeeded' | 'failed'

export interface Run {
  id: string
  status: RunStatus
  provider: string
  model: string
  brief?: Brief
  input_tokens: number
  output_tokens: number
  cost_usd?: number
  error?: string
  created_at: string
  started_at?: string
  finished_at?: string
}

export interface ScriptScene {
  narration: string
  visual_note: string
}

export interface ScriptSection {
  kind: string
  title: string
  narration: string
  rehook: boolean
  cta: boolean
  scenes: ScriptScene[]
}

export interface ScriptSummary {
  id: string
  run_id: string
  title: string
  full_text: string
  word_count: number
  est_duration_sec: number
  created_at: string
  sections?: ScriptSection[]
}

export interface User {
  username: string
  name: string
  role: string
}

export interface SetupStatus {
  needed: boolean
  expiresAt?: string
  expired?: boolean
  attemptsLeft: number
  lockedUntil?: string
}

export class ApiError extends Error {
  status: number
  constructor(status: number, message: string) {
    super(message)
    this.status = status
  }
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(path, {
    headers: init?.body ? { 'Content-Type': 'application/json' } : undefined,
    ...init,
  })
  const body = (await res.json().catch(() => ({}))) as { error?: string }
  if (!res.ok) {
    throw new ApiError(res.status, body.error ?? `HTTP ${res.status}`)
  }
  return body as T
}

export const api = {
  setupStatus: () => request<SetupStatus>('/api/setup'),
  setupVerify: (code: string) =>
    request<{ setupToken: string; expiresAt: string }>('/api/setup/verify', {
      method: 'POST',
      body: JSON.stringify({ code }),
    }),
  setupAdmin: (setupToken: string, name: string, username: string, password: string) =>
    request<{ user: User; recoveryCode: string }>('/api/setup/admin', {
      method: 'POST',
      body: JSON.stringify({ setupToken, name, username, password }),
    }),
  login: (username: string, password: string) =>
    request<{ user: User }>('/api/login', {
      method: 'POST',
      body: JSON.stringify({ username, password }),
    }),
  logout: () => request<{ ok: boolean }>('/api/logout', { method: 'POST', body: '{}' }),
  me: () => request<{ user: User }>('/api/me'),
  listRuns: () => request<{ runs: Run[] }>('/api/runs'),
  getRun: (id: string) => request<{ run: Run }>(`/api/runs/${id}`),
  createRun: (brief: Brief) =>
    request<{ run: Run }>('/api/runs', { method: 'POST', body: JSON.stringify(brief) }),
  listScripts: () => request<{ scripts: ScriptSummary[] }>('/api/scripts'),
  getScript: (id: string) => request<{ script: ScriptSummary }>(`/api/scripts/${id}`),
}
