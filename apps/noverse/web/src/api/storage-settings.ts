/** Gói AN — storage settings wire contract; secrets are write-only. */
export type StorageUsage = { count: number; bytes: number }
export type StorageCandidate = {
  version: number; driver: 's3' | 'local'; provider: string; endpoint?: string; presignEndpoint?: string
  region?: string; bucket?: string; pathStyle: boolean; allowHTTP: boolean; delivery: 'redirect' | 'proxy'
  accessKeyId?: string; secretAccessKey?: string
}
export type StorageActive = Omit<StorageCandidate, 'version' | 'endpoint' | 'accessKeyId' | 'secretAccessKey'> & {
  endpointHost: string; endpointScheme?: string; accessKeyLast4: string; env: string; source: Record<string, string>
}
export type StorageStep = { step: string; ok: boolean; ms: number; code: string; reason: string }
export type StorageTest = { ok: boolean; steps: StorageStep[]; corsProbeUrl?: string; corsProbeExpiresAt?: string; conditionalPut?: string }
export type StorageMigration = {
  id: string; state: 'running' | 'paused' | 'error' | 'completed' | 'cancelled'; errorCode?: string
  copiedCount: number; copiedBytes: number; totalCount: number; totalBytes: number; bytesPerSecond: number
  errorCount: number; startedAt: string; completedAt?: string | null; purgedAt?: string | null
  errors: { kind: string; story: string; storyTitle?: string; code: string; reason: string }[]
}
export type StorageView = {
  mode: 'file' | 'postgres'; version?: number; locked?: boolean; active?: StorageActive
  pending?: { provider: string; bucket: string; endpointHost: string } | null
  encryption: { available: boolean }; usage?: Record<string, StorageUsage>; total?: StorageUsage
  cache?: { usedBytes: number; maxBytes: number }
  operations: { op: string; outcome: string; count: number; bytes: number; durationSeconds: number; maxDurationSeconds: number }[]
  migration: StorageMigration | null; health?: { lastCheck: string | null; error: string }
}
export type StorageSave = { version: number; needsMigration: boolean; count: number; bytes: number; test: StorageTest }
