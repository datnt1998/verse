import type { StorageStep, StorageTest } from './storage-settings'

const interrupted = 'Kiểm tra kết nối bị gián đoạn. Vui lòng kiểm tra lại.'
const invalid = 'Máy chủ trả kết quả kiểm tra không hợp lệ. Vui lòng kiểm tra lại.'
type TestFrame = { done?: unknown; error?: unknown; ok?: unknown; steps?: unknown; step?: unknown; corsProbeUrl?: unknown; corsProbeExpiresAt?: unknown }
const isStep = (value: unknown): value is StorageStep => value !== null && typeof value === 'object' && 'step' in value && typeof value.step === 'string' && 'ok' in value && typeof value.ok === 'boolean' && 'ms' in value && typeof value.ms === 'number' && Number.isFinite(value.ms) && value.ms >= 0 && 'code' in value && typeof value.code === 'string' && 'reason' in value && typeof value.reason === 'string'

/** Read real progress; only a validated final frame can unlock Save. */
export async function readStorageTest(body: ReadableStream<Uint8Array>, onStep?: (step: StorageStep) => void, signal?: AbortSignal): Promise<StorageTest> {
  signal?.throwIfAborted()
  const decoder = new TextDecoder()
  const text = new TransformStream<Uint8Array, string>({
    transform(chunk, controller) { controller.enqueue(decoder.decode(chunk, { stream: true })) },
    flush(controller) { controller.enqueue(decoder.decode()) },
  })
  const reader = body.pipeThrough(text, { signal }).getReader()
  let pending = ''
  let result: StorageTest | undefined
  const parse = (line: string) => {
    if (!line.trim()) return
    let raw: unknown
    try { raw = JSON.parse(line) } catch { throw new Error(invalid) }
    if (raw === null || typeof raw !== 'object' || Array.isArray(raw)) throw new Error(invalid)
    // Fields remain unknown until their individual boundary checks below.
    const value = raw as TestFrame
    if (typeof value.error === 'string') throw new Error(value.error)
    if (value.done === true) {
      if (typeof value.ok !== 'boolean' || !Array.isArray(value.steps) || !value.steps.every(isStep) || value.corsProbeUrl !== undefined && typeof value.corsProbeUrl !== 'string' || value.corsProbeExpiresAt !== undefined && typeof value.corsProbeExpiresAt !== 'string') throw new Error(invalid)
      result = { ok: value.ok, steps: value.steps, ...(typeof value.corsProbeUrl === 'string' ? { corsProbeUrl: value.corsProbeUrl } : {}), ...(typeof value.corsProbeExpiresAt === 'string' ? { corsProbeExpiresAt: value.corsProbeExpiresAt } : {}) }
    } else {
      if (!isStep(raw)) throw new Error(invalid)
      onStep?.(raw)
    }
  }
  try {
    while (!result) {
      signal?.throwIfAborted()
      const chunk = await reader.read()
      signal?.throwIfAborted()
      if (chunk.done) {
        if (pending) parse(pending)
        if (!result) throw new Error(interrupted)
        break
      }
      pending += chunk.value
      let end: number
      while (!result && (end = pending.indexOf('\n')) !== -1) {
        const line = pending.slice(0, end); pending = pending.slice(end + 1)
        parse(line)
        signal?.throwIfAborted()
      }
      if (pending.length > 1024 * 1024) throw new Error(invalid)
    }
    return result
  } finally {
    try { await reader.cancel() }
    catch (error) { if (!signal?.aborted) throw error }
    finally { reader.releaseLock() }
  }
}
