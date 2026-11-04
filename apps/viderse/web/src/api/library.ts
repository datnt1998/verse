// Client Thư viện — khớp internal/web/library.go.
//   GET    /api/templates                      → { items: TemplateSummary[] }
//   POST   /api/templates { name?, fromId? }   → 201 TemplateDetail
//   GET    /api/templates/:id                  → TemplateDetail
//   GET    /api/templates/:id/preview          → PreviewStatus (trạng thái + nguồn + việc nền)
//   POST   /api/templates/:id/preview { jobId?, minutes? } → 202 PreviewStatus (đang dựng)
//   DELETE /api/templates/:id/preview          → 204 (huỷ lần dựng đang chạy)
//   GET    /api/templates/:id/export           → .ytf-template.zip (Content-Disposition)
//   PATCH  /api/templates/:id { revision, name?, recipe? } → TemplateDetail
//   DELETE /api/templates/:id                  → 204 (409 + channels khi kênh còn dùng)
//   GET    /api/image-prompts?kind&q           → { items: ImagePrompt[] }
//   POST   /api/image-prompts { name, kind, composition?, prompt?, fromId? } → 201
//   GET    /api/image-prompts/:id              → ImagePrompt
//   PATCH  /api/image-prompts/:id { name?, kind?, composition?, prompt? } → ImagePrompt
//   DELETE /api/image-prompts/:id              → 204 (403 có sẵn, 409 in_use + usedBy)
//   GET    /api/image-prompts/:id/export       → tệp .json (Content-Disposition)
//   POST   /api/image-prompts/import           → multipart `file` → 201 ImagePrompt
//   GET    /api/overlays                       → { items: Overlay[] }
//   POST   /api/overlays                       → multipart `file`, `name` → 201 Overlay
//   DELETE /api/overlays/:id                   → 204 (tombstone)
//   GET    /api/overlays/:id/file              → tệp (Range, Cache-Control riêng)
// Recipe là một JSONB: PATCH thay cả khối, trường vắng lấy giá trị hiện tại.
import { ApiError, downloadFile, mutate, request, upload } from './request'

export type ThumbnailStyle = 'clean' | 'hook' | 'circle-arrow' | 'hook-arrow' | 'split' | 'closeup'
export type PromptKind = 'scene' | 'cover'
export type SceneStyle = 'full' | 'cutout'
export type ImagesMode = 'fixed' | 'per-minutes' | 'none'
export type MotionKind = 'ken-burns' | 'wander'
export type PreviewState = 'none' | 'fresh' | 'stale' | 'building'

/** Bố cục ảnh cảnh trong video — suy từ `scenes.style` + `scenes.framing`. */
export type ScenePlacement = 'full' | 'cutout' | 'close'

export type RecipeScenes = {
  style: SceneStyle
  /** '' = bỏ Cận cảnh (PATCH gộp: gửi rỗng để xoá, vắng mặt là giữ nguyên) */
  framing?: 'close' | ''
  images: { mode: ImagesMode; count: number; minutes: number }
  motion: { kind: MotionKind; intensity: number; speed: number }
  scenePromptId: string
  coverPromptId: string
  coverSource: 'upload' | 'ai'
  coverMediaId?: string
  referenceImages: boolean
  referenceStyle: 'composition' | 'master'
  sceneOpacity: number
}

export type RecipeBackdrop = {
  channels: { channelId: string; kinds: string[]; pinned: string[] }[]
  clipCount: number | null
  clipSec: number
  fixedVideoIds: string[]
}

export type RecipeAppearance = {
  opacity: number
  blurRadius: number
  brightness: number
  contrast: number
  saturation: number
  hueShift: number
  speed: number
  zoom: number
  flipX: boolean
  flipY: boolean
}

export type Recipe = {
  scenes: RecipeScenes
  backdrop: RecipeBackdrop | null
  overlays: string[]
  appearance: RecipeAppearance
  output: { compressExport: boolean }
}

export type TemplateImages = {
  mode: ImagesMode
  count: number
  minutes: number
  perVideo: number | null
  known: boolean
}

export type TemplatePrompts = {
  sceneId: string
  sceneName: string
  coverId: string
  coverName: string
  summary: string
}

export type TemplateBackdrop = {
  channelCount: number
  videoCount: number
  clipCount: number | null
  clipSec: number
  enabled: boolean
}

export type TemplatePreview = {
  state: PreviewState
  builtAt: string | null
  /** Project xem thử đã đổ nội dung (bản sao project template). */
  projectId?: string
  /** Video đã xong dùng làm nguồn. */
  jobId?: string
  minutes?: number
}

/** Một video đã dựng xong có thể làm nguồn cho bản xem thử. */
export type PreviewSource = {
  jobId: string
  title: string
  channelName?: string
  durationSec: number
  /** Video không dựng được với recipe hiện tại; `reason` nói vì sao. */
  disabled?: boolean
  reason?: string
}

/** Lỗi có hành động của lần dựng gần nhất (backend giữ nguyên câu phục hồi). */
export type PreviewError = { code?: string; message: string; recovery?: string }

/** Việc nền dựng bản xem thử (kind template_preview). */
export type PreviewTask = { id: string; phase?: string; state: string; done: number; total: number; actionHref?: string }

/** GET/POST /api/templates/:id/preview — trạng thái sống của bản xem thử. */
export type PreviewStatus = {
  state: PreviewState
  builtAt?: string | null
  projectId?: string
  jobId?: string
  minutes?: number
  templateRevision?: number
  task?: PreviewTask
  /** Lần dựng gần nhất hỏng; bản xem thử cũ vẫn còn. */
  error?: PreviewError
  sources: PreviewSource[]
}

export type ChannelRef = { id: string; name: string; position?: number }

export type TemplateSummary = {
  id: string
  name: string
  revision: number
  style: SceneStyle
  framing?: 'close'
  images: TemplateImages
  prompts: TemplatePrompts
  backdrop: TemplateBackdrop
  overlayCount: number
  channels: ChannelRef[]
  preview: TemplatePreview
  createdAt: string
  updatedAt: string
}

export type PriceEstimate = {
  perImage: number
  known: boolean
  imagesPerVideo: number | null
  includesCover: boolean
  mode: ImagesMode
  perMinutes?: number
  totalUSD: number | null
}

export type TemplateDetail = TemplateSummary & { recipe: Recipe; price: PriceEstimate; savedAt: string }

export type PromptTemplateRef = { id: string; name: string }

export type ImagePrompt = {
  id: string
  name: string
  kind: PromptKind
  builtIn: boolean
  composition?: ThumbnailStyle
  prompt: string
  updatedAt: string
  usedBy: PromptTemplateRef[]
  /** id ảnh mẫu đã vẽ xong (mới nhất trước, tối đa 12); rỗng khi chưa vẽ thử */
  samples: string[]
  /** URL ảnh mẫu thật do máy chủ dựng sẵn, cùng thứ tự với `samples` */
  sampleUrls: string[]
}

export type Overlay = {
  id: string
  name: string
  mime: string
  bytes: number
  durationSec: number | null
  width: number | null
  height: number | null
  background: 'black' | 'green' | 'none'
  /** "snow", "rain"… hoặc "" khi là tệp của bạn */
  builtIn: string
  createdAt: string
  usedBy: PromptTemplateRef[]
  url: string
}

/** Kênh của bạn — chỉ để chọn kênh trong Thư viện; màn Kênh sở hữu client đầy đủ. */
export type OwnerChannelRef = { id: string; name: string; avatarUrl: string | null }

export type TemplatePatch = { revision: number; name?: string; recipe?: Recipe }
export type PromptDraft = { name: string; kind: PromptKind; composition?: ThumbnailStyle; prompt?: string }

type FieldError = { field: string; message: string; recovery?: string }

/** Mã + phần bổ sung của envelope lỗi ({error,code,recovery,fields,channels,usedBy,reason}). */
export type LibraryErrorInfo = {
  code: string | null
  recovery: string | null
  fields: FieldError[]
  /** tên kênh đang dùng (409 khi xoá template) */
  channels: string[]
  /** id template đang dùng prompt (409/403 in_use) */
  usedBy: string[]
  reason: string | null
  retryAt: string | null
}

const EMPTY_ERROR: LibraryErrorInfo = {
  code: null, recovery: null, fields: [], channels: [], usedBy: [], reason: null, retryAt: null,
}

const list = <T>(value: unknown): T[] => (Array.isArray(value) ? (value as T[]) : [])

export function libraryErrorInfo(err: unknown): LibraryErrorInfo {
  if (!(err instanceof ApiError) || !err.payload || typeof err.payload !== 'object') return EMPTY_ERROR
  const p = err.payload as Record<string, unknown>
  return {
    code: typeof p.code === 'string' ? p.code : null,
    recovery: typeof p.recovery === 'string' ? p.recovery : null,
    fields: list<FieldError>(p.fields).filter((f) => f && typeof f.field === 'string' && typeof f.message === 'string'),
    channels: list<unknown>(p.channels).filter((c): c is string => typeof c === 'string'),
    usedBy: list<unknown>(p.usedBy).filter((c): c is string => typeof c === 'string'),
    reason: typeof p.reason === 'string' ? p.reason : null,
    retryAt: typeof p.retryAt === 'string' ? p.retryAt : null,
  }
}

/** Lỗi theo từng trường (`fields[].field` → câu) — hiện ngay dưới ô. */
export function fieldErrors(err: unknown): Record<string, string> {
  const out: Record<string, string> = {}
  for (const field of libraryErrorInfo(err).fields) out[field.field] = field.message
  return out
}

// ─── Template ────────────────────────────────────────────────────────────────

export const listTemplates = (signal?: AbortSignal) =>
  request<{ items: TemplateSummary[] }>('/api/templates', { signal })

export const getTemplate = (id: string, signal?: AbortSignal) =>
  request<TemplateDetail>(`/api/templates/${encodeURIComponent(id)}`, { signal })

export function createTemplate(input: { name?: string; fromId?: string }): Promise<TemplateDetail> {
  return request<TemplateDetail>('/api/templates', { body: input })
}

export const duplicateTemplate = (fromId: string) => createTemplate({ fromId })

export function patchTemplate(id: string, patch: TemplatePatch): Promise<TemplateDetail> {
  return mutate<TemplateDetail>('PATCH', `/api/templates/${encodeURIComponent(id)}`, patch)
}

export const removeTemplate = (id: string): Promise<void> =>
  mutate<void>('DELETE', `/api/templates/${encodeURIComponent(id)}`)

export const getTemplatePreview = (id: string, signal?: AbortSignal) =>
  request<PreviewStatus>(`/api/templates/${encodeURIComponent(id)}/preview`, { signal })

export function startTemplatePreview(id: string, input: { jobId?: string; minutes?: number } = {}): Promise<PreviewStatus> {
  return mutate<PreviewStatus>('POST', `/api/templates/${encodeURIComponent(id)}/preview`, input)
}

export const cancelTemplatePreview = (id: string): Promise<void> =>
  mutate<void>('DELETE', `/api/templates/${encodeURIComponent(id)}/preview`)

/** Đường xuất tệp template — tải qua fetch để biết thành công/thất bại thật. */
export const templateExportHref = (id: string) => `/api/templates/${encodeURIComponent(id)}/export`

/** Tải tệp template thật rồi mới trả về — lỗi mạng/403/404 ném để caller báo. */
export async function downloadTemplate(id: string): Promise<void> {
  const { blob, filename } = await downloadFile(templateExportHref(id))
  const url = URL.createObjectURL(blob)
  try {
    const anchor = document.createElement('a')
    anchor.href = url
    anchor.download = filename ?? `template-${id}.ytf-template.zip`
    if (blob.type) anchor.type = blob.type
    anchor.rel = 'noopener'
    document.body.appendChild(anchor)
    anchor.click()
    anchor.remove()
  } finally {
    setTimeout(() => URL.revokeObjectURL(url), 0)
  }
}

// ─── Prompt ảnh ──────────────────────────────────────────────────────────────

export function listPrompts(query: { kind?: PromptKind; q?: string } = {}, signal?: AbortSignal): Promise<{ items: ImagePrompt[] }> {
  const params = new URLSearchParams()
  if (query.kind) params.set('kind', query.kind)
  if (query.q) params.set('q', query.q)
  const s = params.toString()
  return request<{ items: ImagePrompt[] }>(`/api/image-prompts${s ? `?${s}` : ''}`, { signal })
}

export const getPrompt = (id: string, signal?: AbortSignal) =>
  request<ImagePrompt>(`/api/image-prompts/${encodeURIComponent(id)}`, { signal })

export function createPrompt(input: PromptDraft & { fromId?: string }): Promise<ImagePrompt> {
  return request<ImagePrompt>('/api/image-prompts', { body: input })
}

export function patchPrompt(id: string, input: Partial<PromptDraft>): Promise<ImagePrompt> {
  return mutate<ImagePrompt>('PATCH', `/api/image-prompts/${encodeURIComponent(id)}`, input)
}

export const removePrompt = (id: string): Promise<void> =>
  mutate<void>('DELETE', `/api/image-prompts/${encodeURIComponent(id)}`)

/** Tệp xuất là JSON có `Content-Disposition: attachment` — tải thẳng bằng thẻ <a>. */
export const promptExportHref = (id: string) => `/api/image-prompts/${encodeURIComponent(id)}/export`

export function importPrompt(file: File): Promise<ImagePrompt> {
  const form = new FormData()
  form.append('file', file)
  return upload<ImagePrompt>('/api/image-prompts/import', form)
}

// ─── Nhập template cũ ───────────────────────────────────────────────────────

/** Một dòng kết quả nhập template (khớp TemplateImportItemWire của máy chủ). */
export type TemplateImportItem = {
  file: string
  name: string
  templateId?: string
  ok: boolean
  warnings: string[]
  error?: string
  media: number
  mediaReused: number
}

export type TemplateImportResult = { items: TemplateImportItem[]; imported: number; failed: number }

/** Nhập nhiều tệp `.ytf-template.zip` một lượt — cùng route với `viderse template import`. */
export function importTemplates(files: File[]): Promise<TemplateImportResult> {
  const form = new FormData()
  for (const file of files) form.append('file', file)
  return upload<TemplateImportResult>('/api/templates/import', form)
}

// ─── Lớp phủ ─────────────────────────────────────────────────────────────────

export const listOverlays = (signal?: AbortSignal) =>
  request<{ items: Overlay[] }>('/api/overlays', { signal })

export function addOverlay(file: File, name?: string): Promise<Overlay> {
  const form = new FormData()
  form.append('file', file)
  if (name) form.append('name', name)
  return upload<Overlay>('/api/overlays', form)
}

export const removeOverlay = (id: string): Promise<void> =>
  mutate<void>('DELETE', `/api/overlays/${encodeURIComponent(id)}`)

/** Bỏ template khỏi danh sách luân phiên của một kênh (POST action=remove). */
export function removeTemplateFromChannel(channelId: string, templateId: string): Promise<{ templates: { id: string; name: string; position: number }[] }> {
  return mutate('POST', `/api/owner-channels/${encodeURIComponent(channelId)}/templates`, { action: 'remove', templateId })
}

// ─── Kênh của bạn (chọn kênh để gắn template) ────────────────────────────────

export function listOwnerChannels(signal?: AbortSignal): Promise<{ channels: OwnerChannelRef[] }> {
  return request<{ channels: { id: string; name: string; avatarUrl?: string | null }[] }>('/api/owner-channels', { signal })
    .then((body) => ({
      channels: list<{ id: string; name: string; avatarUrl?: string | null }>(body?.channels)
        .map(({ id, name, avatarUrl }) => ({ id, name, avatarUrl: avatarUrl ?? null })),
    }))
}
