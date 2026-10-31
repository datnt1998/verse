import { ApiError, mutate, request } from './request'
import { serverStorageCopy as t } from '../i18n/storageServer'

export type StorageKind = 'disk' | 's3'
export type S3Provider = 'garage' | 'r2' | 'aws' | 'gcs' | 'wasabi' | 'b2' | 'do' | 'other'
export type Delivery = 'proxy' | 'redirect'
export type StorageErrorCode = 'wrong_key' | 'no_bucket' | 'no_write_permission' | 'no_delete_permission' | 'unreachable_endpoint' | 'clock_skew' | 'cors_blocked' | 'browser_unreachable' | 'storage_error'
export type UsageGroup = { group: 'images' | 'editor' | 'library' | 'sources' | 'exports'; files: number | null; bytes: number | null }
export type ServerStorage = {
  kind: StorageKind; provider?: S3Provider; endpointHost?: string; presignEndpoint?: string; delivery: Delivery
  region?: string; bucket?: string; forcePathStyle?: boolean; allowInsecure?: boolean; accessKeyLast4?: string
  sources: Partial<Record<'provider' | 'endpoint' | 'region' | 'bucket' | 'keys' | 'delivery' | 'presignEndpoint' | 'forcePathStyle' | 'allowInsecure' | 'cache', 'env' | 'app' | 'default'>>
  encryptionKey: { ok: boolean; path: string; issue?: 'readonly_dir' | 'bad_permissions' | 'corrupt' }
  online: boolean; checkedAt: string; lastError?: StorageErrorCode
  disk?: { path: string; freeBytes: number }
  previous?: { kind: StorageKind; label: string; path?: string; files: number; bytes: number }
}
export type OpStat = { op: 'put' | 'get' | 'presign' | 'delete'; count: number; errors: number; bytes: number; avgMs: number; maxMs: number }
export type MigrationErrorCode = 'content_mismatch' | 'unreachable_endpoint' | 'storage_error' | 'wrong_key'
export type Migration = {
  taskId: string; state: 'running' | 'paused' | 'error' | 'done' | 'cancelled'
  from: { kind: StorageKind; label: string }; to: { kind: StorageKind; label: string }
  files: { done: number; total: number }; bytes: { done: number; total: number }; checked: number; rateBps: number; startedAt: string; finishedAt?: string
  groups: { group: UsageGroup['group']; files: { done: number; total: number }; bytes: { done: number; total: number }; state: 'done' | 'running' | 'waiting' | 'error' }[]
  errors: { group: UsageGroup['group']; jobId?: string; title: string; detail?: string; code: MigrationErrorCode }[]
}
export type ServerStorageResponse = { role: 'admin' | 'member'; storage?: ServerStorage; usage: UsageGroup[]; workCache?: { usedBytes: number; limitBytes: number }; ops?: OpStat[]; migration?: Migration }
export type S3Config = { provider: S3Provider; endpoint?: string; accountId?: string; region?: string; bucket: string; accessKeyId?: string; secretAccessKey?: string; forcePathStyle?: boolean; allowInsecure?: boolean; delivery: Delivery; presignEndpoint?: string }
export type ConnectionResult = {
  testId: string
  steps: { id: 'write' | 'read' | 'range' | 'presign' | 'cors_probe' | 'app_range' | 'delete'; status: 'ok' | 'fail' | 'skip' | 'na' | 'pending'; ms?: number }[]
  probeUrl?: string; error?: { code: StorageErrorCode; step: string }
}
export type MigrationPreview = { files: number; bytes: number; groups: UsageGroup[] }

/** Only the missing capability route selects V22; outages must not expose local controls. */
export async function getServerStorage(signal?: AbortSignal): Promise<ServerStorageResponse | null> {
  try {
    const data = await request<ServerStorageResponse>('/api/storage/server', { signal })
    const validUsage = data && Array.isArray(data.usage) && data.usage.every(group =>
      group && Object.hasOwn(t.groups, group.group)
      && (group.files === null || Number.isSafeInteger(group.files) && group.files >= 0)
      && (group.bytes === null || Number.isFinite(group.bytes) && group.bytes >= 0))
    if (!validUsage || data.role !== 'admin' && data.role !== 'member') throw new Error(t.invalidResponse)
    // Defensive role filtering also prevents leaked administrator fields entering the cache.
    if (data.role === 'member') return { role: data.role, usage: data.usage }
    const storage = data.storage
    if (!storage || !['disk', 's3'].includes(storage.kind) || !['proxy', 'redirect'].includes(storage.delivery)
      || typeof storage.online !== 'boolean' || typeof storage.checkedAt !== 'string'
      || !storage.sources || !storage.encryptionKey || typeof storage.encryptionKey.ok !== 'boolean'
      || storage.kind === 's3' && (!storage.provider || !Object.hasOwn(t.providers, storage.provider))) throw new Error(t.invalidResponse)
    return data
  } catch (error) {
    if (error instanceof ApiError && error.status === 404) return null
    throw error
  }
}
export const saveServerStorage = (config: S3Config, testId: string) => mutate<{ needsMigration: MigrationPreview | null }>('PUT', '/api/storage/server/config', { config, testId })
export const startStorageMigration = () => request<{ taskId: string }>('/api/storage/server/migrate', { body: {} })
export const controlStorageMigration = (id: string, action: 'pause' | 'resume' | 'cancel' | 'retry') => request<void>(`/api/storage/server/migrate/${encodeURIComponent(id)}/${action}`, { body: {} })

/** A browser Range read, not a server request, is the evidence for redirect delivery. */
export async function testServerStorage(config: S3Config, signal?: AbortSignal, onProgress?: (result: ConnectionResult) => void): Promise<ConnectionResult> {
  let result = await request<ConnectionResult>('/api/storage/server/test', { body: { config }, signal })
  if (!result.probeUrl) return result
  onProgress?.(result)
  let probe: { ok: boolean; reason?: 'cors' | 'network' }
  try {
    const url = new URL(result.probeUrl)
    if (url.protocol !== 'https:' && url.protocol !== 'http:') throw new Error('Invalid probe protocol')
    const response = await fetch(url.href, { headers: { Range: 'bytes=0-1023' }, credentials: 'omit', signal })
    const bytes = await response.arrayBuffer()
    probe = response.status === 206 && bytes.byteLength === 1024 ? { ok: true } : { ok: false, reason: 'network' }
  } catch (error) {
    if (signal?.aborted) throw error
    // Fetch cannot distinguish a blocked CORS response from an unreachable host.
    // The server uses its own transport/config facts to classify this failure.
    probe = { ok: false, reason: 'network' }
  }
  result = await request<ConnectionResult>(`/api/storage/server/test/${encodeURIComponent(result.testId)}/probe`, { body: probe, signal })
  return result
}

export function configFromStorage(storage: ServerStorage): S3Config {
  const accountId = storage.provider === 'r2' ? storage.endpointHost?.match(/^([^.]+)\.r2\.cloudflarestorage\.com$/)?.[1] : undefined
  return { provider: storage.provider ?? 'garage', endpoint: storage.endpointHost ? `${storage.allowInsecure ? 'http' : 'https'}://${storage.endpointHost}` : undefined, accountId, region: storage.region, bucket: storage.bucket ?? '', delivery: storage.delivery, presignEndpoint: storage.presignEndpoint, forcePathStyle: storage.forcePathStyle, allowInsecure: storage.allowInsecure }
}

/** A partial or pending response can never authorize Save. */
export function connectionPassed(result: ConnectionResult | null): boolean {
  if (!result || result.error || result.probeUrl || !result.testId || result.steps.length !== 6) return false
  const ids = new Set(result.steps.map(step => step.id))
  return ids.size === 6 && ['write', 'read', 'range', 'presign', 'delete'].every(id => ids.has(id as ConnectionResult['steps'][number]['id']))
    && (ids.has('cors_probe') !== ids.has('app_range'))
    && result.steps.every(step => step.status === 'ok' || step.id === 'presign' && step.status === 'na')
}
