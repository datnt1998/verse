import { z } from 'zod'
import type { Project } from '@editor/types/project'
import type { ServerMediaRecord } from '@editor/storage/types'

export type HeadlessKind = 'render' | 'measure'
export interface HeadlessPageProps { requestId: string; token?: string }
export interface HeadlessProgress {
  phase: string
  framesDone?: number | null
  framesTotal?: number | null
  bytesSent?: number
  bytesTotal?: number
  chunksSent?: number
}

const identifier = z.string().min(1).max(256).regex(/^[\w-]+$/)
const trackSchema = z.object({ id: z.string(), name: z.string(), height: z.number(), locked: z.boolean(), visible: z.boolean(), muted: z.boolean(), solo: z.boolean(), order: z.number() }).passthrough()
const itemSchema = z.object({ id: z.string(), trackId: z.string(), type: z.enum(['video', 'audio', 'image', 'text', 'shape', 'composition', 'adjustment', 'controller', 'lottie']), from: z.number().finite().nonnegative(), durationInFrames: z.number().finite().nonnegative() }).passthrough()
const subCompositionSchema = z.object({
  id: z.string(), name: z.string(), items: z.array(itemSchema), tracks: z.array(trackSchema.extend({ items: z.array(itemSchema) })),
  transitions: z.array(z.object({}).passthrough()), keyframes: z.array(z.object({}).passthrough()),
  fps: z.number().finite().positive(), width: z.number().positive(), height: z.number().positive(), durationInFrames: z.number().finite().nonnegative(),
}).passthrough()
const timelineSchema = z.object({ tracks: z.array(trackSchema), items: z.array(itemSchema), transitions: z.array(z.object({}).passthrough()).optional(), compositions: z.array(subCompositionSchema).optional() }).passthrough()
const projectEnvelopeSchema = z.object({
  id: identifier, name: z.string(), description: z.string(), createdAt: z.number(), updatedAt: z.number(), duration: z.number().finite().nonnegative(),
  metadata: z.object({ width: z.number().int().positive(), height: z.number().int().positive(), fps: z.number().finite().positive(), backgroundColor: z.string().optional() }).passthrough(),
  timeline: timelineSchema.optional(),
}).passthrough()
// Preserve the engine's extensible timeline fields after validating the saved-project envelope.
const projectSchema = z.custom<Project>(value => projectEnvelopeSchema.safeParse(value).success, 'Dữ liệu dự án không hợp lệ')
const mediaEnvelopeSchema = z.object({
  id: identifier, fileName: z.string(), mime: z.string(), size: z.number().nonnegative(), contentHash: z.string(), createdAt: z.number(),
  probe: z.object({ kind: z.enum(['video', 'audio', 'image']), mime: z.string(), durationSec: z.number().nullable(), width: z.number().nullable(), height: z.number().nullable(), fps: z.number().nullable(), videoCodec: z.string().nullable(), audioCodec: z.string().nullable(), sampleRate: z.number().nullable(), channels: z.number().nullable() }),
}).passthrough()
const mediaSchema = z.custom<ServerMediaRecord>(value => mediaEnvelopeSchema.safeParse(value).success, 'Dữ liệu nguồn không hợp lệ')
const requestSchema = z.object({ id: identifier, projectId: identifier, jobId: z.string().nullable().optional(), fence: z.string().nullable().optional(), status: z.string(), binding: z.object({ projectRevision: z.string() }).passthrough().optional(), result: z.object({ lufs: z.number().nullable(), samples: z.number(), durationSec: z.number() }).passthrough().nullable().optional() }).passthrough()
export const runtimeSchema = z.object({
  runtime: z.object({ workspaceId: z.string(), ready: z.boolean(), capabilities: z.object({ audioDecode: z.boolean(), gpuRequired: z.boolean() }) }),
  clone: z.object({ jobId: z.string(), gate: z.object({ allowed: z.boolean(), blockers: z.array(z.object({ id: z.string(), message: z.string() })) }) }).passthrough().nullable(),
})
const snapshotSchema = runtimeSchema.extend({ request: requestSchema.extend({ nonce: z.string().min(1) }), project: projectSchema, revision: z.string().min(1), media: z.array(mediaSchema) })
export type HeadlessSnapshot = z.infer<typeof snapshotSchema>
export const exportRecordSchema = z.object({ id: z.string().min(1), fileName: z.string(), mime: z.string(), size: z.number().nonnegative(), status: z.enum(['staged', 'ready']), createdAt: z.number(), committedAt: z.number().nullable() })

export class HeadlessApiError extends Error {
  constructor(message: string, readonly status: number, readonly body: unknown) { super(message); this.name = 'HeadlessApiError' }
}

export function parseHeadlessCredential(requestId: string, token: string | undefined, hash: string): { requestId: string; token: string } {
  identifier.parse(requestId)
  const query = hash.includes('?') ? hash.slice(hash.indexOf('?') + 1) : ''
  const params = new URLSearchParams(query)
  if (!token && params.getAll('token').length !== 1) throw new Error('Thiếu quyền truy cập yêu cầu')
  const credential = token ?? params.get('token')
  if (!credential || credential.length > 8192 || /\s/.test(credential)) throw new Error('Quyền truy cập yêu cầu không hợp lệ')
  return { requestId, token: credential }
}

export class HeadlessClient {
  readonly baseUrl: string
  fence: string | null = null
  constructor(readonly kind: HeadlessKind, readonly requestId: string, readonly token: string, private readonly fetcher: typeof fetch = fetch) {
    identifier.parse(requestId)
    this.baseUrl = `/api/${kind}/${encodeURIComponent(requestId)}`
  }

  async response(path: string, init: RequestInit = {}): Promise<Response> {
    const headers = new Headers(init.headers)
    headers.set('Authorization', `Bearer ${this.token}`)
    const response = await this.fetcher(`${this.baseUrl}${path}`, { ...init, cache: 'no-store', credentials: 'omit', headers })
    if (!response.ok) {
      const body: unknown = await response.json().catch(() => null)
      const failure = z.object({ error: z.string() }).safeParse(body)
      throw new HeadlessApiError(failure.success ? failure.data.error : `Yêu cầu không thành công (${response.status})`, response.status, body)
    }
    return response
  }

  async json<T>(path: string, schema: z.ZodType<T>, init?: RequestInit): Promise<T> {
    return schema.parse(await (await this.response(path, init)).json())
  }

  async post(path: string, data: unknown, signal?: AbortSignal): Promise<Response> {
    return this.response(path, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(data), signal })
  }

  async snapshot(signal?: AbortSignal): Promise<HeadlessSnapshot> {
    const snapshot = await this.json('', snapshotSchema, { signal })
    if (snapshot.request.id !== this.requestId || snapshot.request.projectId !== snapshot.project.id) throw new Error('Yêu cầu không thuộc dự án đã tải')
    if (snapshot.request.binding && snapshot.request.binding.projectRevision !== snapshot.revision) throw new Error('Dự án đã đổi sau khi tạo yêu cầu')
    return snapshot
  }

  async claim(nonce: string, signal?: AbortSignal): Promise<string> {
    const body: unknown = await (await this.post('/claim', { nonce, ownerId: `headless-${crypto.randomUUID()}` }, signal)).json()
    const claimed = z.object({ request: requestSchema.extend({ fence: z.string().min(1) }) }).parse(body).request
    if (claimed.id !== this.requestId) throw new Error('Máy chủ trả về yêu cầu khác')
    this.fence = claimed.fence
    return claimed.fence
  }

  requireFence(): string {
    if (!this.fence) throw new Error('Yêu cầu chưa được nhận')
    return this.fence
  }

  mediaUrl(mediaId: string, contentHash = ''): string {
    return `${this.baseUrl}/media/${encodeURIComponent(mediaId)}?${new URLSearchParams({ token: this.token, v: contentHash })}`
  }

  async heartbeat(progress: HeadlessProgress | undefined, signal?: AbortSignal): Promise<void> {
    const fence = this.requireFence()
    await this.post('/heartbeat', { fence }, signal)
    if (progress) await this.post('/progress', { fence, progress }, signal)
  }

  async bind(exportId: string, signal?: AbortSignal): Promise<void> { await this.post('/bind', { fence: this.requireFence(), exportId }, signal) }
  async complete(exportId?: string, signal?: AbortSignal): Promise<z.infer<typeof requestSchema>> {
    return z.object({ request: requestSchema }).parse(await (await this.post('/complete', { fence: this.requireFence(), ...(exportId ? { exportId } : {}) }, signal)).json()).request
  }
  async fail(error: unknown): Promise<void> {
    await this.post('/fail', { fence: this.requireFence(), error: { code: error instanceof DOMException && error.name === 'AbortError' ? 'cancelled' : 'internal_error', message: error instanceof Error ? error.message : String(error) } })
  }
  async uploadPcm(seq: number, bytes: Uint8Array, signal?: AbortSignal): Promise<void> {
    if (bytes.byteLength % 8 !== 0) throw new Error('Dữ liệu âm thanh không đúng khung stereo')
    if (!(bytes.buffer instanceof ArrayBuffer)) throw new Error('Dữ liệu âm thanh không có bộ đệm riêng')
    await this.response(`/chunks?${new URLSearchParams({ seq: String(seq), sampleRate: '48000', channels: '2' })}`, {
      method: 'POST', headers: { 'Content-Type': 'application/octet-stream', 'X-Render-Fence': this.requireFence() }, body: new Blob([bytes as Uint8Array<ArrayBuffer>]), signal,
    })
  }
}

/** A failed lease update cancels CPU/GPU work instead of allowing an orphaned render. */
export async function withHeadlessLease<T>(client: HeadlessClient, controller: AbortController, progress: () => HeadlessProgress | undefined, run: () => Promise<T>, finish?: (result: T) => Promise<void>, intervalMs = 2000): Promise<T> {
  let leaseError: unknown
  let pending: Promise<void> | undefined
  const beat = () => {
    if (pending || controller.signal.aborted) return
    pending = client.heartbeat(progress(), controller.signal).catch(error => { leaseError = error; controller.abort(error) }).finally(() => { pending = undefined })
  }
  let timer: ReturnType<typeof setInterval> | undefined
  try {
    await client.heartbeat(progress(), controller.signal)
    timer = setInterval(beat, intervalMs)
    const result = await run()
    clearInterval(timer)
    await pending
    if (leaseError) throw leaseError
    controller.signal.throwIfAborted()
    await client.heartbeat(progress(), controller.signal)
    await finish?.(result)
    return result
  } catch (error) {
    clearInterval(timer)
    await pending
    const cause = leaseError && error !== leaseError
      ? new AggregateError([leaseError, error], 'Yêu cầu mất quyền xử lý', { cause: leaseError })
      : leaseError ?? error
    try { await client.fail(cause) } catch (reportError) { throw new AggregateError([cause, reportError], 'Không thể báo lỗi yêu cầu', { cause }) }
    throw cause
  } finally {
    clearInterval(timer)
    await pending
  }
}
