import type { StorageCandidate, StorageMigration, StorageSave, StorageStep, StorageTest, StorageView } from '../../../api/storage-settings'

/** Match bootstrap.privateEndpoint, including Go's literal (not DNS-resolved) IP rule.
 * Do not use URL.hostname for IPv4: WHATWG normalizes shorthand IPs that Go does not parse. */
export function privateEndpoint(endpoint: string): boolean {
  if (/[\u0000-\u001f\u007f]/.test(endpoint) || /%(?![\da-f]{2})/i.test(endpoint)) return false
  const match = /^(https?):\/\/([^/?#]*)/i.exec(endpoint)
  if (!match || !match[2] || /\s/.test(match[2])) return false
  const authority = match[2].split('@').at(-1) ?? ''
  let host: string
  if (authority.startsWith('[')) {
    const end = authority.indexOf(']'); if (end < 0 || !/^(?::\d*)?$/.test(authority.slice(end + 1))) return false
    host = authority.slice(1, end)
  } else {
    if (!/^[^:]+(?::\d*)?$/.test(authority)) return false
    host = authority.split(':')[0]
  }
  if (!host) return false
  if (match[1].toLowerCase() === 'http') return true
  const ipv4Private = (a: number, b: number) => a === 10 || a === 127 || a === 172 && b >= 16 && b <= 31 || a === 192 && b === 168 || a === 169 && b === 254
  if (host.includes(':')) {
    // WHATWG canonicalizes valid IPv6 (including dotted mapped tails) without DNS.
    try {
      const ip = new URL(`https://[${host}]`).hostname.toLowerCase()
      const mapped = /^\[::ffff:([\da-f]+):([\da-f]+)\]$/.exec(ip)
      if (mapped) { const high = parseInt(mapped[1], 16); return ipv4Private(high >>> 8, high & 255) }
      return ip === '[::1]' || /^\[f[cd][\da-f]{2}:/.test(ip) || /^\[fe[89ab][\da-f]:/.test(ip)
    } catch { return !host.includes('.') }
  }
  const parts = host.split('.')
  if (parts.length === 4 && parts.every((part) => /^(0|[1-9]\d{0,2})$/.test(part) && Number(part) <= 255)) return ipv4Private(Number(parts[0]), Number(parts[1]))
  return !host.includes('.')
}

export async function probeBrowser(url: string, signal?: AbortSignal, fetcher: typeof fetch = fetch): Promise<StorageStep> {
  const started = performance.now()
  const result = (ok: boolean, code: string): StorageStep => ({ step: 'cors_probe', ok, code, reason: '', ms: Math.round(performance.now() - started) })
  const parsed = new URL(url)
  if (!['http:', 'https:'].includes(parsed.protocol) || parsed.username || parsed.password) throw new Error('Địa chỉ kiểm tra trình duyệt không hợp lệ')
  try {
    const response = await fetcher(url, { headers: { Range: 'bytes=0-1' }, mode: 'cors', credentials: 'omit', signal, cache: 'no-store' })
    if (response.ok) { await response.arrayBuffer(); return result(true, 'ok') }
    return result(false, 'storage_error')
  } catch (error) { if (signal?.aborted) throw error }
  try {
    // Range is not a no-cors safelisted header; omit it on the reachability-only retry.
    await fetcher(url, { mode: 'no-cors', credentials: 'omit', signal, cache: 'no-store' })
    return result(false, 'browser_cors')
  } catch (error) {
    if (signal?.aborted) throw error
    return result(false, 'browser_unreachable')
  }
}

/** One request at a time; visibility and disposal stop timers immediately. */
export function startVisiblePolling(ms: number, refresh: () => Promise<unknown>, onError: (error: unknown) => void = () => {}): () => void {
  let timer: ReturnType<typeof setTimeout> | undefined
  let disposed = false
  let busy = false
  const clear = () => { clearTimeout(timer); timer = undefined }
  const schedule = () => { if (!disposed && !document.hidden && !busy && timer === undefined) timer = setTimeout(run, ms) }
  const run = async () => {
    timer = undefined
    if (disposed || document.hidden) return
    busy = true
    try { await refresh() } catch (error) { onError(error) }
    finally { busy = false; schedule() }
  }
  const visibility = () => { clear(); schedule() }
  document.addEventListener('visibilitychange', visibility); schedule()
  return () => { disposed = true; clear(); document.removeEventListener('visibilitychange', visibility) }
}

export async function saveAndMigrate(candidate: StorageCandidate, tested: boolean, save: (candidate: StorageCandidate) => Promise<StorageSave>, migrate: (version: number) => Promise<unknown>): Promise<StorageSave> {
  if (!tested) throw new Error('Kiểm tra kết nối đạt rồi mới lưu được')
  const result = await save(candidate)
  if (!result.test.ok) throw new Error('Kho tệp chưa vượt qua kiểm tra kết nối')
  if (result.needsMigration) await migrate(result.version)
  return result
}

export async function completeBrowserTest(test: StorageTest, delivery: string, signal?: AbortSignal): Promise<StorageTest> {
  if (delivery !== 'redirect' || !test.corsProbeUrl) return test
  const probe = await probeBrowser(test.corsProbeUrl, signal)
  const steps = test.steps.filter((step) => step.step !== 'cors_probe')
  const index = steps.findIndex((step) => step.step === 'delete')
  steps.splice(index < 0 ? steps.length : index, 0, probe)
  return { ...test, steps, ok: test.ok && probe.ok }
}

/** The design has six rows. The server's clock check belongs to the first signed request. */
export function displaySteps(test: StorageTest | null, delivery: string, complete = true): StorageStep[] {
  const codes = ['write', 'read', 'range', 'presign', delivery === 'proxy' ? 'app_range' : 'cors_probe', 'delete']
  const steps = codes.map((step) => test?.steps.find((item) => item.step === step) ?? { step, ok: false, ms: 0, code: test && complete ? 'skipped' : 'waiting', reason: '' })
  const clock = test?.steps.find((step) => step.step === 'clock')
  if (clock && !clock.ok && steps[0].ok) steps[0] = { ...clock, step: 'write', ms: steps[0].ms + clock.ms }
  return steps
}

export const migrating = (migration: StorageMigration | null | undefined) => !!migration && ['running', 'paused', 'error'].includes(migration.state)
export const percent = (done: number, total: number) => total > 0 ? Math.min(100, Math.floor(done / total * 100)) : 0
export const num = (n: number) => n.toLocaleString('vi-VN')
export const bytes = (n: number) => n >= 1e9 ? `${(n / 1e9).toFixed(1).replace('.', ',')} GB` : n >= 1e6 ? `${(n / 1e6).toLocaleString('vi-VN', { maximumFractionDigits: 1 })} MB` : `${Math.round(n / 1e3)} KB`
export const time = (at: string | null | undefined) => at ? new Date(at).toLocaleTimeString('vi-VN', { hour: '2-digit', minute: '2-digit' }) : 'chưa kiểm tra'
export const endpointOf = (active: NonNullable<StorageView['active']>) => active.endpointHost ? `${active.endpointScheme || (active.allowHTTP ? 'http' : 'https')}://${active.endpointHost}` : ''
export const corsSnippet = (variant: string, origin: string, endpoint: string, bucket: string) => {
  const rules = [{ AllowedOrigins: [origin], AllowedMethods: ['GET', 'HEAD'], AllowedHeaders: ['range', 'if-none-match', 'if-range'], ExposeHeaders: ['content-length', 'content-range', 'accept-ranges', 'etag'], MaxAgeSeconds: 3600 }]
  if (variant !== 'garage') return JSON.stringify(rules, null, 2)
  const quote = (value: string) => `'${value.replaceAll("'", "'\\''")}'`
  return `${JSON.stringify({ CORSRules: rules }, null, 2)}\n\naws s3api put-bucket-cors --endpoint-url ${quote(endpoint)} \\\n  --bucket ${quote(bucket)} --cors-configuration file://cors.json`
}
