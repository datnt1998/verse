// Ảnh, phiên bản và prompt đã gửi — 07-data-contract §26.
//
// Mỗi lần vẽ một ảnh là một `ImageVersion`: máy chủ lưu đúng văn bản đã gửi
// nhà cung cấp (`parts` + `sentText`) và ảnh tham chiếu đã gửi kèm (`refs`) để
// giao diện hiện lại và vẽ lại từ đó. Client ở đây chỉ đọc/ghi đúng hợp đồng,
// không tự ghép prompt: `parts` gửi lên là phần người dùng đã sửa.
//
// SSE `image.progress` đi qua luồng sự kiện chung (`/api/events`) như mọi sự
// kiện khác; `useJobImages` ghép tiến độ vào danh sách ảnh đang hiện.
import { useCallback, useEffect, useRef, useState } from 'react'
import { z } from 'zod'
import { ApiError, mutate, request } from './request'
import { subscribeEvents } from '../data/events'

export const promptPartKindSchema = z.enum(['scene', 'style', 'refs', 'learned', 'avoid'])
export type PromptPartKind = z.infer<typeof promptPartKindSchema>

export const promptPartSchema = z.object({
  kind: promptPartKindSchema,
  text: z.string(),
  source: z.object({
    type: z.enum(['artist', 'artist_feedback', 'image_prompt', 'template', 'user']),
    id: z.string().optional(),
    label: z.string(),
  }),
})
export type PromptPart = z.infer<typeof promptPartSchema>

export const refRoleSchema = z.enum(['layout', 'character', 'style', 'object'])
export type RefRole = z.infer<typeof refRoleSchema>

export const imageRefSchema = z.object({
  id: z.string(),
  role: refRoleSchema,
  url: z.string().min(1),
  source: z.object({
    type: z.enum(['rival_frame', 'video_image', 'upload']),
    label: z.string(),
    videoId: z.string().optional(),
    atSec: z.number().optional(),
    imageId: z.string().optional(),
  }),
})
export type ImageRef = z.infer<typeof imageRefSchema>

export const versionStateSchema = z.enum(['drawing', 'ready', 'failed', 'cancelled'])
export type VersionState = z.infer<typeof versionStateSchema>

/**
 * Phần KHÔNG sửa được của prompt đã gửi: hàng rào an toàn (mọi nguồn vẽ nhận)
 * và chỉ dẫn công cụ của đúng nguồn vẽ (Codex nhận, Grok/HTTP không). §26 chưa
 * có chỗ cho nó nên đây là trường thêm; `sentText` vẫn là toàn bộ nguyên văn.
 */
export const promptWrapperSchema = z.object({
  guard: z.array(z.string()).optional(),
  tool: z.array(z.string()).optional(),
  provider: z.string().optional(),
})
export type PromptWrapper = z.infer<typeof promptWrapperSchema>

export const imageVersionSchema = z.object({
  n: z.number(),
  state: versionStateSchema,
  url: z.string().nullable(),
  previewUrl: z.string().nullable(),
  width: z.number(),
  height: z.number(),
  parts: z.array(promptPartSchema),
  wrapper: promptWrapperSchema.optional(),
  sentText: z.string(),
  refs: z.array(imageRefSchema),
  provider: z.string(),
  model: z.string(),
  params: z.object({ size: z.string(), quality: z.string().optional() }),
  feedback: z.string().nullable(),
  diffFromPrev: z.array(z.tuple([z.string(), z.enum(['', 'ins', 'del'])])).nullable(),
  startedAt: z.string(),
  finishedAt: z.string().nullable(),
  durationSec: z.number().nullable(),
  costImages: z.number(),
  error: z.object({
    reason: z.enum(['quota', 'filter', 'network', 'provider']),
    retryAt: z.string().optional(),
    message: z.string(),
  }).nullable(),
  /** Bản cũ chưa tách phần prompt: hiện `sentText` một khối, không sửa từng phần. */
  partsLegacy: z.boolean().optional(),
})
export type ImageVersion = z.infer<typeof imageVersionSchema>

/**
 * Một ảnh của video. §26 chỉ chốt các trường của phiên bản; màn Duyệt còn đọc
 * lời kể, khung giờ, điểm Kiểm định… của cùng ảnh nên schema giữ thêm trường
 * lạ (`.passthrough`) thay vì dựng bản sao thứ hai của cùng payload.
 */
export const jobImageSchema = z.object({
  id: z.string(),
  kind: z.enum(['scene', 'cover']),
  index: z.number(),
  label: z.string(),
  usedVersion: z.number(),
  /** Video đã duyệt/dựng: ảnh mới cần duyệt lại (§26). */
  needsReview: z.boolean().optional(),
  status: z.string().optional(),
  versions: z.array(imageVersionSchema),
  refLimit: z.number(),
}).passthrough()
export type JobImage = z.infer<typeof jobImageSchema>

/** Hạn mức ảnh của nhà cung cấp (số thật từ sổ ảnh trả phí). */
export const imageQuotaSchema = z.object({
  providers: z.array(z.object({
    provider: z.string(),
    label: z.string(),
    left: z.number(),
    total: z.number(),
    resetAt: z.string().nullable(),
    paid: z.boolean(),
  })),
  left: z.number(),
  fallback: z.object({ provider: z.string(), label: z.string(), left: z.number() }).nullable(),
})
export type ImageQuota = z.infer<typeof imageQuotaSchema>

/**
 * `images` là mảng §26. Endpoint cũ trả thêm `items` (hình dạng khác, dùng cho
 * danh sách duyệt) — để nguyên, không đọc ở đây, và không làm hỏng cả payload.
 */
export const jobImagesSchema = z.object({
  images: z.array(jobImageSchema).optional(),
  items: z.array(z.unknown()).optional(),
  quota: imageQuotaSchema.nullish(),
  manifest: z.object({ revision: z.number() }).passthrough().nullable().optional(),
}).passthrough().transform((value) => ({
  images: value.images ?? [],
  quota: value.quota ?? undefined,
  /** `expectedRevision` cho thao tác duyệt/bỏ ảnh (endpoint cũ). */
  revision: value.manifest?.revision,
}))
export type JobImages = z.infer<typeof jobImagesSchema>

export const imageProgressSchema = z.object({
  jobId: z.string(),
  imageId: z.string(),
  n: z.number(),
  phase: z.enum(['sending', 'drawing', 'receiving', 'done', 'failed']),
  previewUrl: z.string().optional(),
  elapsedSec: z.number(),
})
export type ImageProgress = z.infer<typeof imageProgressSchema>

const redrawResultSchema = z.object({ versions: z.array(z.object({ imageId: z.string(), n: z.number() })) })
export type RedrawResult = z.infer<typeof redrawResultSchema>

export interface RedrawRef {
  id?: string
  upload?: string
  videoImageId?: string
  rivalFrame?: { videoId: string; atSec: number }
  role: RefRole
}

export interface RedrawBody {
  imageIds: string[]
  /** False leaves the undoable project edit to the requesting Editor. */
  project?: boolean
  feedback?: string
  /** Sửa tay — máy chủ chỉ nhận khi đúng một ảnh. */
  parts?: Partial<Record<PromptPartKind, string>>
  refs?: RedrawRef[]
  provider?: string
}

const base = (jobId: string) => `/api/clone/${encodeURIComponent(jobId)}/images`

export const getJobImages = async (jobId: string, signal?: AbortSignal): Promise<JobImages> =>
  jobImagesSchema.parse(await request<unknown>(base(jobId), { signal }))

export const redrawImages = async (jobId: string, body: RedrawBody): Promise<RedrawResult> =>
  redrawResultSchema.parse(await mutate('POST', `${base(jobId)}/redraw`, { ...body }))

/**
 * Đổi bản đang dùng của ảnh. Editor truyền `{ project: false }` khi tự áp media
 * của clip trong bộ nhớ (máy chủ không ghi lại project.json); bỏ trống thì máy
 * chủ tự đổi binding của project đã dựng. Trả về đủ dữ kiện để UI áp ngay.
 */
export const imageUseResultSchema = z.object({
  imageId: z.string(),
  n: z.number(),
  usedVersion: z.number(),
  needsReview: z.boolean().optional(),
  projectId: z.string().nullable().optional(),
  clipId: z.string().nullable().optional(),
  mediaId: z.string().nullable().optional(),
  src: z.string().nullable().optional(),
  previousMediaId: z.string().nullable().optional(),
  previousVersion: z.number().nullable().optional(),
}).passthrough()
export type ImageUseResult = z.infer<typeof imageUseResultSchema>

export const useImageVersion = async (
  jobId: string, imageId: string, n: number, options?: { project?: boolean },
): Promise<ImageUseResult> =>
  imageUseResultSchema.parse(await mutate('POST', `${base(jobId)}/${encodeURIComponent(imageId)}/use`, {
    n,
    ...(options?.project === false ? { project: false } : {}),
  }))

export const cancelResultSchema = z.object({
  imageId: z.string(),
  n: z.number(),
  cancelled: z.boolean(),
  /** 0 khi nhà cung cấp chưa nhận yêu cầu — huỷ không tốn ảnh. */
  costImages: z.number(),
  usedVersion: z.number(),
})
export type CancelResult = z.infer<typeof cancelResultSchema>

export const cancelImageVersion = async (jobId: string, imageId: string, n: number): Promise<CancelResult> =>
  cancelResultSchema.parse(await mutate('POST', `${base(jobId)}/${encodeURIComponent(imageId)}/versions/${n}/cancel`, {}))

const mediaUploadSchema = z.object({ media: z.object({ id: z.string() }).passthrough() })

/**
 * Tải một ảnh lên để làm ảnh tham chiếu: dùng đúng route nhập media của editor
 * (raw body + `fileName`, trả `{ media }`), rồi gửi `media.id` làm `refs[].upload`.
 */
export const uploadRefImage = async (file: File): Promise<string> => {
  let response: Response
  try {
    response = await fetch(`/api/editor/media?fileName=${encodeURIComponent(file.name)}`, {
      method: 'POST',
      credentials: 'same-origin',
      headers: { 'Content-Type': file.type || 'application/octet-stream' },
      body: file,
    })
  } catch {
    throw new ApiError(0, 'Không kết nối được máy chủ. Vui lòng thử lại.')
  }
  const payload: unknown = await response.json().catch(() => null)
  if (!response.ok) {
    const message = payload && typeof payload === 'object' && 'error' in payload && typeof payload.error === 'string' ? payload.error : 'Chưa tải được ảnh lên.'
    throw new ApiError(response.status, message, undefined, payload)
  }
  return mediaUploadSchema.parse(payload).media.id
}


/** 422 `{ code: "too_many_refs", limit }` — ảnh tham chiếu vượt giới hạn nhà cung cấp. */
export function tooManyRefs(error: unknown): number | null {
  if (!(error instanceof ApiError) || error.status !== 422) return null
  const payload = error.payload
  if (!payload || typeof payload !== 'object' || !('code' in payload) || payload.code !== 'too_many_refs') return null
  return 'limit' in payload && typeof payload.limit === 'number' ? payload.limit : 0
}

/** Sự kiện tiến độ vẽ của mọi ảnh trong mọi video (lọc theo `jobId` khi dùng). */
export function subscribeImageProgress(handler: (progress: ImageProgress) => void): () => void {
  return subscribeEvents('image.progress', (data) => {
    const parsed = imageProgressSchema.safeParse(data)
    if (parsed.success) handler(parsed.data)
  })
}

/** Bản đang dùng của ảnh (`usedVersion`), rơi về bản mới nhất nếu server chưa kịp đổi. */
export const usedVersionOf = (image: JobImage): ImageVersion | undefined =>
  image.versions.find((version) => version.n === image.usedVersion) ?? image.versions.at(-1)

/**
 * Ghép tiến độ SSE vào ảnh: bản đang vẽ hiện `drawing` + `previewUrl` (ảnh xem
 * dần) ngay cả khi server chưa kịp trả phiên bản mới trong GET.
 */
export function withProgress(image: JobImage, progress?: ImageProgress): JobImage {
  if (!progress || progress.imageId !== image.id) return image
  const existing = image.versions.find((version) => version.n === progress.n)
  const drawing = progress.phase !== 'done' && progress.phase !== 'failed'
  const next: ImageVersion = existing
    ? { ...existing, state: drawing ? 'drawing' : existing.state, previewUrl: progress.previewUrl ?? existing.previewUrl }
    : {
        n: progress.n,
        state: 'drawing',
        url: null,
        previewUrl: progress.previewUrl ?? null,
        width: 0,
        height: 0,
        parts: [],
        sentText: '',
        refs: [],
        provider: '',
        model: '',
        params: { size: '' },
        feedback: null,
        diffFromPrev: null,
        startedAt: new Date(Date.now() - progress.elapsedSec * 1000).toISOString(),
        finishedAt: null,
        durationSec: null,
        costImages: 0,
        error: null,
      }
  const versions = existing ? image.versions.map((version) => (version.n === progress.n ? next : version)) : [...image.versions, next]
  return { ...image, versions }
}

export interface UseJobImages {
  data?: JobImages
  error: boolean
  loading: boolean
  reload: () => Promise<void>
  /** Tiến độ mới nhất theo `imageId` — dùng cho hiệu ứng đang vẽ và nhãn MỚI. */
  progress: Record<string, ImageProgress>
}

/**
 * Danh sách ảnh + phiên bản của một video, tự ghép tiến độ vẽ.
 * Xong hoặc lỗi thì đọc lại danh sách: `usedVersion` và `error` chỉ có ở GET.
 */
export function useJobImages(jobId: string | null): UseJobImages {
  const [data, setData] = useState<JobImages>()
  const [error, setError] = useState(false)
  const [loading, setLoading] = useState(false)
  const [progress, setProgress] = useState<Record<string, ImageProgress>>({})
  const alive = useRef(true)
  const requested = useRef<string | null>(null)
  const reload = useCallback(async () => {
    if (!jobId) return
    setLoading(true)
    try {
      const result = await getJobImages(jobId)
      if (!alive.current || requested.current !== jobId) return
      setData(result)
      setError(false)
    } catch {
      if (alive.current && requested.current === jobId) setError(true)
    } finally {
      if (alive.current && requested.current === jobId) setLoading(false)
    }
  }, [jobId])
  useEffect(() => {
    alive.current = true
    requested.current = jobId
    setData(undefined)
    setProgress({})
    if (jobId) void reload()
    return () => { alive.current = false }
  }, [jobId, reload])
  useEffect(() => {
    if (!jobId) return
    return subscribeImageProgress((event) => {
      if (event.jobId !== jobId) return
      setProgress((old) => ({ ...old, [event.imageId]: event }))
      if (event.phase === 'done' || event.phase === 'failed') void reload()
    })
  }, [jobId, reload])
  return { data, error, loading, reload, progress }
}
