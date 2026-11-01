// Client fetch bám sát docs/design/v1/11-api-contract.md.
// Lỗi server: {"error": string} (+ retryAfter cho 429). Mọi request kèm cookie phiên.
import type {
  Asset, ChapterFacts, ChapterInfo, ChapterPlan, ChapterText, Character, CharacterKind, CastEntry, CoCreateState,
  Config, ConfigAccount, ConfigModelInfo, ConfigPresets, ConfigResponse, ConfigScope,
  DiagReport, ExportResult, ExportsResponse, ForeshadowEntry, Idea, ImportRequest, JobEvent,
  FsFolder, FsMkdirResult, FsPlace, LibraryConfig, LibraryInspect, LibraryResponse, ModelsResponse, OAuthStart, OAuthStatus, Outline,
  Pairing, Project, ProjectDetail, ReviewDecision, ReviewEntry, RevisionEntry, Role, RuleFile, RulesResponse,
  Session, Snapshot, StoryFile, StorySummary, SyncChanged, SystemInfo, User, ValidateResult, AppId, AppLink,
  ChaptersResponse, ImportPreview, ProjectCoCreateReply, ProjectCoCreateState, RevealPath, RevealRequest, RevealResult,
  CommandResult, RetryResult, CommandSpec, ExportOnExists, SyncDiff,
  ConfigTestResult, LibraryRootResult, PickFolderResult, ProviderModelList, SwapProviderResult, SwapUndoResult,
  AudioVolume, AudioScript, AudioSegment, AudioCast, AudioVoice, AudioPronunciation,
  AudioSettings, AudioQueue, AudioPeaks, AudioExportOptions, AudioExportPrefs, AudioExportResult, AudioExportState, AudioFlag, AudioCandidate,
  AudioReady, TtsStatus, TtsEngine, TtsInstallProgress,
  RuleApplyResult, RuleUnderstanding, RuleViolationsResponse, RulesEffective,
  SetupAdminResult, SetupState, SetupVerifyResult,
  StyleSample, StyleSamples,
} from './types'
import type { ExportFormat } from './types'
import type { ParagraphComment, ParagraphCommentInput, ParagraphCommentPatch } from './types'
import type { ActivityHistory, ActivityHistoryQuery } from './types'
import type { LiveHistory, ProjectSummary } from './types'
import type { CoCreateReply, CoCreateCommit, ReopenChecks, ImportState, JobDetail, JobsResponse } from './types'
import type { AddressTermInput, AddressTerms, Brain, RelationInput } from './types'
import type { Frame, FrameInput, Retention } from './types'
import type { MetricsScope, StoryMetrics } from './types'
import type { AdminAuditItem, ProjectPermissions, ProjectRole } from './types'
import { engineErrorLabel } from '../lib/labels'

export class ApiError extends Error {
  retryAfter?: number
  /** thân JSON lỗi gốc — vd 409 `/export` mang `result.exists` (GAP-27) */
  payload?: unknown
  constructor(public status: number, message: string, retryAfter?: number, payload?: unknown) {
    message = engineErrorLabel(message) || message
    // 09 §5: lỗi lấy nguyên từ server, viết hoa chữ đầu khi hiển thị (mọi nơi: toast, dòng lỗi).
    super(message.charAt(0).toLocaleUpperCase('vi') + message.slice(1))
    this.name = 'ApiError'
    this.retryAfter = retryAfter
    this.payload = payload
  }
}

/** 409 phiên bản, không nhầm với 409 khoá truyện hoặc tệp xuất đã tồn tại. */
export class ConflictError<T = unknown> extends ApiError {
  constructor(message: string, public current: T, public version?: string) {
    super(409, message, undefined, { conflict: true, current, version })
    this.name = 'ConflictError'
  }
}

// Đăng ký handler 401 toàn cục (App gọi setUnauthorizedHandler lúc mount).
let onUnauthorized: ((error: ApiError) => void | boolean | Promise<boolean>) | null = null
export function setUnauthorizedHandler(fn: ((error: ApiError) => void | boolean | Promise<boolean>) | null) { onUnauthorized = fn }

/**
 * 403 `{code:"app_access_denied"}` — phiên hợp lệ nhưng tài khoản chưa được cấp
 * quyền vào app này: trả view user an toàn đi kèm (grants kèm trong), khác hẳn
 * 401 phiên hết hạn. UI giữ người đang đăng nhập, không quay vòng màn login.
 */
export const accessDeniedUser = (error: unknown): User | null => {
  const payload = error instanceof ApiError && error.status === 403 ? error.payload : null
  return payload && typeof payload === 'object' && 'code' in payload && payload.code === 'app_access_denied' &&
    'user' in payload && payload.user && typeof payload.user === 'object' ? payload.user as User : null
}

const OFFLINE_MSG = 'Không kết nối được máy chủ. Vui lòng thử lại.'

async function request<T>(path: string, options?: { method?: 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE'; body?: object; version?: string; signal?: AbortSignal; accept?: string; decode?: (response: Response) => Promise<T> }, retried = false): Promise<T> {
  const method = options?.method ?? (options?.body === undefined ? 'GET' : 'POST')
  let response: Response
  try {
    response = await fetch(path, {
      credentials: 'same-origin', method, signal: options?.signal,
      headers: { ...(options?.body === undefined ? {} : { 'Content-Type': 'application/json' }), ...(options?.version ? { 'If-Match': options.version } : {}), ...(options?.accept ? { Accept: options.accept } : {}) },
      ...(options?.body === undefined ? {} : { body: JSON.stringify(options.body) }),
    })
  } catch (error) { if (options?.signal?.aborted) throw error; throw new ApiError(0, OFFLINE_MSG) }
  if (!response.ok) {
    const payload: unknown = await response.json().catch(() => null)
    const message = payload && typeof payload === 'object' && 'error' in payload && typeof payload.error === 'string' ? payload.error : response.statusText
    const retry = payload && typeof payload === 'object' && 'retryAfter' in payload && typeof payload.retryAfter === 'number' ? payload.retryAfter : undefined
    if (response.status === 409 && payload && typeof payload === 'object' && 'current' in payload) {
      throw new ConflictError(message, payload.current, 'version' in payload && typeof payload.version === 'string' ? payload.version : undefined)
    }
    const error = new ApiError(response.status, message || `Lỗi HTTP ${response.status}`, retry, payload)
    if (response.status === 401 && !retried && path !== '/api/login' && path !== '/api/pair' && path !== '/api/logout' && path !== '/api/recover') {
      if (await onUnauthorized?.(error) === true && !options?.signal?.aborted) return request<T>(path, options, true)
    }
    throw error
  }
  // 204 = xong, không có thân (vd DELETE …/review/items/{id}/keep).
  if (response.status === 204) return undefined as T
  if (options?.decode) return options.decode(response)
  return response.json() as Promise<T>
}

const storyPath = (p: string, s: string) => `/api/projects/${encodeURIComponent(p)}/stories/${encodeURIComponent(s)}`

// ─── Auth & tài khoản ────────────────────────────────────────────────────

export const login = (username: string, password: string) => request<{ user: User }>('/api/login', { body: { username, password } })
export const pair = (code: string) => request<{ user: User }>('/api/pair', { body: { code } })
export const logout = () => request<{ ok: boolean }>('/api/logout', { body: {} })
/** `pendingLessons` (D15) chỉ có khi người gọi là quản trị. */
export const me = () => request<{ user: User; session: Session; apps?: AppLink[]; pendingLessons?: number }>('/api/me')
export const updateMe = (name: string, version?: string) => request<{ user: User }>('/api/me', { body: { name }, version })
export const markOnboarded = () => request<{ user: User }>('/api/me/onboarded', { body: {} })
/**
 * Đổi mật khẩu của chính mình. Thành công thì máy chủ thu hồi MỌI phiên kể cả
 * phiên đang dùng và xoá cookie — `reauthenticate` báo UI phải xoá phiên cục bộ
 * và mời đăng nhập lại, không refresh /api/me (sẽ 401 như thể đổi thất bại).
 */
export const changePassword = (current: string, next: string, version?: string) =>
  request<{ ok: boolean; reauthenticate?: boolean }>('/api/me/password', { body: { current, new: next }, version })
export const mySessions = () => request<{ sessions: Session[] }>('/api/me/sessions')
export const myPairing = () => request<Pairing>('/api/me/pairings', { body: {} })
export const revokeSession = (id: string) => request<{ ok: boolean }>(`/api/sessions/${encodeURIComponent(id)}/revoke`, { body: {} })
export const revokeOtherSessions = () => request<{ ok: boolean }>('/api/me/sessions/revoke-all', { body: {} })
/** Cấp quyền vào một app của kho tài khoản chung (chỉ quản trị; backend file trả 501). */
export const grantAppAccess = (username: string, app: AppId, reason?: string) =>
  request<User>(`/api/users/${encodeURIComponent(username)}/grants/${app}`, { method: 'PUT', body: { role: 'member', ...(reason ? { reason } : {}) } })
/** Thu hồi quyền vào một app — hiệu lực ngay, phiên của tài khoản đó bị thu hồi. */
export const revokeAppAccess = (username: string, app: AppId, reason?: string) =>
  request<User>(`/api/users/${encodeURIComponent(username)}/grants/${app}`, { method: 'DELETE', body: reason ? { reason } : {} })
export const listUsers = () => request<{ users: User[] }>('/api/users')
export const createUser = (username: string, name: string, role: Role, projects: ProjectPermissions, password: string) =>
  request<User>('/api/users', { body: { username, name, role, projects, password } })
export const updateUser = (username: string, patch: { name?: string; role?: Role; projects?: ProjectPermissions; disabled?: boolean; password?: string; reason?: string }, version?: string) =>
  request<User>(`/api/users/${encodeURIComponent(username)}`, { body: patch, version })
export const deleteUser = (username: string, reason?: string) => request<{ ok: boolean }>(`/api/users/${encodeURIComponent(username)}/delete`, { body: { reason } })
export const revokeUserSessions = (username: string) => request<{ ok: boolean }>(`/api/users/${encodeURIComponent(username)}/sessions/revoke-all`, { body: {} })
export const getAdminAudit = (query: { type?: string; actor?: string; before?: string; limit?: number } = {}) => {
  const params = new URLSearchParams()
  for (const [key, value] of Object.entries(query)) if (value !== undefined && value !== '') params.set(key, String(value))
  return request<{ items: AdminAuditItem[]; next: string }>(`/api/admin/audit?${params}`)
}
export const userSessions = (username: string) => request<{ sessions: Session[] }>(`/api/users/${encodeURIComponent(username)}/sessions`)
export const userPairing = (username: string) => request<Pairing>(`/api/users/${encodeURIComponent(username)}/pairings`, { body: {} })

// Mã khôi phục quản trị (GAP-33): hiện một lần; 409 = còn mã chưa dùng → dùng rotate.
export const getRecovery = () => request<{ createdAt: string | null; used: boolean; mask?: string }>('/api/me/recovery')
export const issueRecovery = () => request<{ code: string; createdAt: string }>('/api/me/recovery', { body: {} })
export const rotateRecovery = () => request<{ code: string; createdAt: string }>('/api/me/recovery/rotate', { body: {} })
// Công khai, không cần phiên: đặt mật khẩu mới bằng mã khôi phục rồi đăng nhập luôn.
export const recoverAdmin = (username: string, code: string, newPassword: string) =>
  request<{ user: User }>('/api/recover', { body: { username, code, newPassword } })

// GAP-41 · Lần chạy đầu (A0): mã thiết lập chỉ in ở máy chủ, không bao giờ trả qua API.
export const getSetup = () => request<SetupState>('/api/setup')
export const verifySetup = (code: string) => request<SetupVerifyResult>('/api/setup/verify', { body: { code } })
export const createSetupAdmin = (body: { setupToken: string; name: string; username: string; password: string }) =>
  request<SetupAdminResult>('/api/setup/admin', { body })

/** §AD: system administrators own every project, including future projects. */
export const projectRole = (user: User, project: string): ProjectRole | null =>
  user.disabled ? null : user.role === 'admin' ? 'owner' : user.projects[project] ?? null
const storyProject = (storyId: string) => storyId.includes('/') ? storyId.split('/')[0] : '_'
export const canReadStory = (user: User, storyId: string) => projectRole(user, storyProject(storyId)) !== null
export const canWriteStory = (user: User, storyId: string) => {
  const role = projectRole(user, storyProject(storyId))
  return role === 'owner' || role === 'collab'
}
export const canReadProject = (user: User, p: string) => projectRole(user, p) !== null
export const canCreateStory = (user: User, p: string) => canWriteStory(user, `${p}/`)
export const canManageProject = (user: User, p: string) => projectRole(user, p) === 'owner'

// ─── §3 Thư viện & dự án ────────────────────────────────────────────────

export const getLibrary = () => request<LibraryResponse>('/api/library')
export const listProjects = () => request<{ projects: ProjectSummary[] }>('/api/projects')
export const getLiveHistory = (p: string, s: string, options: { beforeRound?: number; limit?: number } = {}, signal?: AbortSignal) => {
  const query = new URLSearchParams()
  if (options.beforeRound !== undefined) query.set('beforeRound', String(options.beforeRound))
  if (options.limit !== undefined) query.set('limit', String(options.limit))
  const suffix = query.toString()
  return request<LiveHistory>(`${storyPath(p, s)}/live-history${suffix ? `?${suffix}` : ''}`, { signal })
}
export type LibraryWriteResult = { ok: boolean; config: LibraryConfig; version: string }
export const putLibrary = (config: LibraryConfig, version?: string) => request<LibraryWriteResult>('/api/library', { method: 'PUT', body: config, version })
export const createProject = (input: { slug?: string; name: string; description?: string; notes?: string; shareGlobal?: boolean; copyRulesFrom?: string }) =>
  request<Project>('/api/projects', { body: input })
export const adoptProject = (dir: string, name?: string) => request<Project>('/api/projects/adopt', { body: { dir, name } })
export const getProject = (p: string) => request<ProjectDetail>(`/api/projects/${encodeURIComponent(p)}`)
export const updateProject = (p: string, input: { name: string; description: string; notes: string; universeName: string }, version?: string) =>
  request<Project>(`/api/projects/${encodeURIComponent(p)}`, { method: 'PUT', body: input, version })
export const listStories = (p: string) => request<{ stories: StorySummary[] }>(`/api/projects/${encodeURIComponent(p)}/stories`)
export const createStory = (p: string, input: { name?: string; idea?: string }) =>
  request<StorySummary>(`/api/projects/${encodeURIComponent(p)}/stories`, { body: input })
export const getStoryMeta = (p: string, s: string, signal?: AbortSignal) => request<StoryFile>(`${storyPath(p, s)}/meta`, { signal })
export const putStoryMeta = (p: string, s: string, meta: Omit<StoryFile, 'createdAt'>, version?: string) =>
  request<StoryFile>(`${storyPath(p, s)}/meta`, { method: 'PUT', body: { title: meta.title, shortTitle: meta.shortTitle, status: meta.status, tags: meta.tags, idea: meta.idea }, version })
export const renameStory = (p: string, s: string, slug: string) =>
  request<StorySummary>(`${storyPath(p, s)}/rename`, { body: { slug } })
export const closeStory = (p: string, s: string) => request<{ ok: boolean }>(`${storyPath(p, s)}/close`, { body: {} })
export const listIdeas = (p: string) => request<{ ideas: Idea[] }>(`/api/projects/${encodeURIComponent(p)}/ideas`)
export const createIdea = (p: string, text: string, title?: string) =>
  request<Idea>(`/api/projects/${encodeURIComponent(p)}/ideas`, { body: { text, title } })
export const getIdea = (p: string, id: string) => request<Idea & { text: string }>(`/api/projects/${encodeURIComponent(p)}/ideas/${encodeURIComponent(id)}`)
export const listAssets = (p: string) => request<{ assets: Asset[] }>(`/api/projects/${encodeURIComponent(p)}/assets`)
export const validateProject = (p: string) => request<ValidateResult>(`/api/projects/${encodeURIComponent(p)}/validate`)
export const fixProject = (p: string, codes?: string[]) => request<ValidateResult>(`/api/projects/${encodeURIComponent(p)}/validate/fix`, { body: { codes } })

// ─── §4 Truyện — đọc trực tiếp ──────────────────────────────────────────

export const getChapters = (p: string, s: string, signal?: AbortSignal) => request<ChaptersResponse>(`${storyPath(p, s)}/chapters`, { signal })
export const getChapter = (p: string, s: string, n: number, source?: 'committed' | 'draft', signal?: AbortSignal) =>
  request<ChapterText>(`${storyPath(p, s)}/chapters/${n}${source ? `?source=${source}` : ''}`, { signal })
export const getChapterPlan = (p: string, s: string, n: number, signal?: AbortSignal) => request<ChapterPlan>(`${storyPath(p, s)}/chapters/${n}/plan`, { signal })
export const getChapterFacts = (p: string, s: string, n: number, signal?: AbortSignal) => request<ChapterFacts>(`${storyPath(p, s)}/chapters/${n}/facts`, { signal })
export const getChapterRevisions = (p: string, s: string, n: number, signal?: AbortSignal) => request<{ revisions: RevisionEntry[] }>(`${storyPath(p, s)}/chapters/${n}/revisions`, { signal })
/** Gói AG — `volume`: chỉ kèm `chapters` của một tập (0 = chỉ header các tập); mục lục truyện >200 chương tải theo tập. */
export const getOutline = (p: string, s: string, opts?: { signal?: AbortSignal; volume?: number } | AbortSignal) => {
  const signal = opts instanceof AbortSignal ? opts : opts?.signal
  const volume = opts instanceof AbortSignal || !opts ? undefined : opts.volume
  return request<Outline>(`${storyPath(p, s)}/outline${volume !== undefined ? `?volume=${volume}` : ''}`, { signal })
}
export const getCharacters = (p: string, s: string, signal?: AbortSignal) => request<{ characters: Character[]; supporting: CastEntry[] }>(`${storyPath(p, s)}/characters`, { signal })
/** Gói S — sửa “Loại vai”/“Lộ vai ở chương” ở Dự án › Nhân vật; trả lại danh sách đã lưu. */
export type CharacterPatch = { id?: string; name?: string; kind?: CharacterKind; revealAt?: number | null }
export const putCharacters = (p: string, s: string, characters: CharacterPatch[]) =>
  request<{ characters: Character[]; supporting: CastEntry[] }>(`${storyPath(p, s)}/characters`, { method: 'PUT', body: { characters } })
export const getForeshadow = (p: string, s: string, signal?: AbortSignal) => request<{ entries: ForeshadowEntry[] }>(`${storyPath(p, s)}/foreshadow`, { signal })

// GAP-38 — xưng hô & quan hệ (D7/D8). PUT/POST trả cùng dạng GET; 423 khi trợ lý đang viết,
// 409 `{affected}` khi “từ chương” chạm chương đã viết mà thiếu `apply`.
export const getAddressTerms = (p: string, s: string, signal?: AbortSignal) => request<AddressTerms>(`${storyPath(p, s)}/address-terms`, { signal })
export const putAddressTerm = (p: string, s: string, input: AddressTermInput) =>
  request<AddressTerms>(`${storyPath(p, s)}/address-terms`, { method: 'PUT', body: input })
export const postRelation = (p: string, s: string, input: RelationInput) =>
  request<AddressTerms>(`${storyPath(p, s)}/relations`, { body: input })

// GAP-39 — Não truyện (D10). Màn Nhân vật chỉ dùng `chapters` + `entities[].appears`.
export const getBrain = (p: string, s: string, signal?: AbortSignal) => request<Brain>(`${storyPath(p, s)}/brain`, { signal })
/** M8 — xem nhanh trong Trình đọc: máy chủ cắt bỏ mọi thứ sau chương đang đọc (không lộ chương sau). */
export const getBrainUpTo = (p: string, s: string, upTo: number, signal?: AbortSignal) =>
  request<Brain>(`${storyPath(p, s)}/brain?upTo=${Math.max(1, Math.trunc(upTo))}`, { signal })

export type PremiseSections = { sections: { heading: string; text: string }[]; missing: string[]; updatedAt: string | null }
export type WorldRule = { category: string; rule: string; boundary: string }
export const getPremise = (p: string, s: string, signal?: AbortSignal) => request<PremiseSections>(`${storyPath(p, s)}/premise`, { signal })
export const getWorldRules = (p: string, s: string, signal?: AbortSignal) => request<WorldRule[]>(`${storyPath(p, s)}/world-rules`, { signal })

// GAP-37 — khung truyện & bản đồ giữ chân (D5). PUT cần quyền viết (403), 400 khi nhóm ngoài
// 0..4 hoặc ô chữ nhiều dòng/quá 200 ký tự; cả hai trả cùng dạng GET.
export const getFrame = (p: string, s: string, signal?: AbortSignal) => request<Frame>(`${storyPath(p, s)}/frame`, { signal })
export const putFrame = (p: string, s: string, input: FrameInput) =>
  request<Frame>(`${storyPath(p, s)}/frame`, { method: 'PUT', body: input })
export const getRetention = (p: string, s: string, signal?: AbortSignal) => request<Retention>(`${storyPath(p, s)}/retention`, { signal })
export const getLatestReview = (p: string, s: string, signal?: AbortSignal) => request<ReviewEntry>(`${storyPath(p, s)}/reviews/latest`, { signal })
export const getReviews = (p: string, s: string, chapter: number, signal?: AbortSignal) =>
  request<{ chapter: ReviewEntry | null; global: ReviewEntry | null }>(`${storyPath(p, s)}/reviews/${chapter}`, { signal })
/** GAP-47 — giữ nguyên một mục Cảnh báo kèm lý do; Bắt sửa không giữ được (409). */
export const keepReviewItem = (p: string, s: string, itemId: string, reason: string) =>
  request<ReviewDecision>(`${storyPath(p, s)}/review/items/${encodeURIComponent(itemId)}/keep`, { body: { reason } })
/** GAP-47 — bỏ giữ: mục trở lại “Tuỳ bạn” (BE trả 204, không thân). */
export const unkeepReviewItem = (p: string, s: string, itemId: string) =>
  request<void>(`${storyPath(p, s)}/review/items/${encodeURIComponent(itemId)}/keep`, { method: 'DELETE' })
export const getExports = (p: string, s: string, signal?: AbortSignal) => request<ExportsResponse>(`${storyPath(p, s)}/exports`, { signal })
export const createExport = (p: string, s: string, input: {
  format: ExportFormat; from?: number; to?: number; volume?: number
  /** D3 — `doc-so`: viết số thành chữ cho bản đọc TTS */
  readNumbers?: boolean
  /** GAP-27 — tệp trùng: replace = ghi đè · keep = thêm giờ vào tên · fail (mặc định) = 409 kèm `exists` */
  onExists?: ExportOnExists
}) => request<ExportResult>(`${storyPath(p, s)}/exports`, { body: input })

// Tải file qua fetch để bắt lỗi JSON thay vì nhảy sang trang JSON.
async function download(path: string, fallbackName: string, onProgress?: (done: number | null) => void) {
  let response: Response
  try { response = await fetch(path, { credentials: 'same-origin' }) } catch { throw new ApiError(0, OFFLINE_MSG) }
  if (!response.ok) {
    const payload = await response.json().catch(() => null) as { error?: string } | null
    const error = new ApiError(response.status, payload?.error || response.statusText || `Lỗi HTTP ${response.status}`, undefined, payload)
    if (response.status === 401) await onUnauthorized?.(error)
    throw error
  }
  let blob: Blob
  if (onProgress) {
    onProgress(0)
    const reader = response.body?.getReader()
    const total = Number(response.headers.get('content-length')) || 0
    if (!reader) {
      blob = await response.blob(); onProgress(total ? 1 : null)
    } else {
      const chunks: BlobPart[] = []
      let got = 0
      for (;;) {
        const { done, value } = await reader.read()
        if (done) break
        chunks.push(value); got += value.byteLength
        onProgress(total ? Math.min(0.99, got / total) : null)
      }
      onProgress(1)
      blob = new Blob(chunks, { type: response.headers.get('content-type') ?? undefined })
    }
  } else blob = await response.blob()
  const disposition = response.headers.get('content-disposition') || ''
  const encoded = /filename\*=UTF-8''([^;]+)/i.exec(disposition)?.[1]
  // A malformed filename* (e.g. a truncated UTF-8 sequence) must not abort the
  // download: fall back to the ASCII filename, then to the caller's name.
  let file = fallbackName
  if (encoded) {
    try { file = decodeURIComponent(encoded) } catch { /* fall through */ }
  }
  if (file === fallbackName) {
    file = /filename="([^"]*)"/i.exec(disposition)?.[1] || fallbackName
  }
  const link = document.createElement('a')
  link.href = URL.createObjectURL(blob)
  link.download = file
  document.body.append(link)
  link.click()
  link.remove()
  setTimeout(() => URL.revokeObjectURL(link.href), 10000)
  return { file, bytes: blob.size }
}
export const downloadExport = (p: string, s: string, format: ExportFormat, range?: { from?: number; to?: number }, readNumbers?: boolean) => {
  const q = new URLSearchParams({ format })
  if (range?.from) q.set('from', String(range.from))
  if (range?.to) q.set('to', String(range.to))
  if (readNumbers) q.set('readNumbers', '1')
  return download(`${storyPath(p, s)}/export?${q}`, `book.${format === 'narration' ? 'txt' : format}`)
}
/** “Chép liên kết tải” (CmdResults exportDone) — URL tải tệp đã xuất, cùng phiên đăng nhập. */
export const exportDownloadLink = (p: string, s: string, filePath: string) => {
  const path = `${storyPath(p, s)}/exports/file?path=${encodeURIComponent(filePath)}`
  return typeof location === 'undefined' ? path : new URL(path, location.origin).toString()
}
export const downloadExportFile = (p: string, s: string, filePath: string, onProgress?: (done: number | null) => void) => {
  const endpoint = `${storyPath(p, s)}/exports/file?`
  const path = filePath.startsWith(endpoint) ? new URLSearchParams(filePath.slice(endpoint.length)).get('path') ?? '' : filePath
  return download(`${endpoint}path=${encodeURIComponent(path)}`, path.split('/').pop() || 'export', onProgress)
}
/** Tải bản xuất txt/epub bằng tên thân thiện (“Tên truyện · Tập N.epub”) — máy chủ đặt tên trong Content-Disposition. `file` = đường dẫn trong mục Bản xuất. */
export const downloadExportBook = (p: string, s: string, file: string, friendlyName: string) =>
  download(`${storyPath(p, s)}/exports/download?file=${encodeURIComponent(file)}`, friendlyName)

// ─── §5 Truyện — worker live ─────────────────────────────────────────────

export const getSnapshot = (p: string, s: string, signal?: AbortSignal) => request<Snapshot>(`${storyPath(p, s)}/snapshot`, { signal })
export const sendInput = (p: string, s: string, text: string, signal?: AbortSignal) =>
  request<{ ok: boolean; action: 'start' | 'steer' | 'continue' }>(`${storyPath(p, s)}/input`, { body: { text }, signal })
export const resumeStory = (p: string, s: string) => request<{ ok: boolean; label: string }>(`${storyPath(p, s)}/resume`, { body: {} })
export const abortStory = (p: string, s: string, afterStep = false) => request<{ ok: boolean; stopped: boolean }>(`${storyPath(p, s)}/abort`, { body: afterStep ? { afterStep: true } : {} })
export const nextChapter = (p: string, s: string) => request<{ ok: boolean }>(`${storyPath(p, s)}/next`, { body: {} })
export const setReview = (p: string, s: string, on: boolean) => request<{ ok: boolean }>(`${storyPath(p, s)}/review`, { body: { on } })
export const reopenStory = (p: string, s: string, direction: string) => request<{ ok: boolean }>(`${storyPath(p, s)}/reopen`, { body: { direction } })
export const getReopenChecks = (p: string, s: string, signal?: AbortSignal) =>
  request<ReopenChecks>(`${storyPath(p, s)}/reopen`, { signal })

// Đồng sáng tác (GAP-6)
export const getCoCreate = (p: string, s: string, signal?: AbortSignal) => request<CoCreateState>(`${storyPath(p, s)}/cocreate`, { signal })
export const sendCoCreate = (p: string, s: string, message: string) =>
  request<CoCreateReply>(`${storyPath(p, s)}/cocreate/message`, { body: { message } })
export const commitCoCreate = (p: string, s: string, draft?: string) =>
  request<CoCreateCommit>(`${storyPath(p, s)}/cocreate/commit`, { body: draft === undefined ? {} : { draft } })
export const cancelCoCreate = (p: string, s: string) => request<{ ok: boolean }>(`${storyPath(p, s)}/cocreate/cancel`, { body: {} })

// Việc dài & lệnh TUI (GAP-8)
export const startImport = (p: string, s: string, input: ImportRequest) => request<{ job: string }>(`${storyPath(p, s)}/import`, { body: input })
export const confirmImport = (p: string, s: string, accept: boolean, storyStatus?: 'open' | 'closed', options?: { continueAfter?: boolean; guide?: string }) =>
  request<{ ok: boolean }>(`${storyPath(p, s)}/import/confirm`, { body: { accept, storyStatus, ...options } })
export const getImport = (p: string, s: string, signal?: AbortSignal) => request<ImportState>(`${storyPath(p, s)}/import`, { signal })
export const startSimulate = (p: string, s: string, dir?: string) => request<{ job: string }>(`${storyPath(p, s)}/simulate`, { body: dir ? { dir } : {} })
export const startImportSim = (p: string, s: string, path: string) => request<{ job: string }>(`${storyPath(p, s)}/importsim`, { body: { path } })
/** GAP-29 — `check: true` chỉ liệt kê chương sửa ngoài app (kèm +chữ/−chữ); `false` chạy việc nền `sync`. */
export const checkSync = (p: string, s: string) =>
  request<{ changed: SyncChanged[] }>(`${storyPath(p, s)}/sync`, { body: { check: true } })
export const runSync = (p: string, s: string) =>
  request<{ job: string }>(`${storyPath(p, s)}/sync`, { body: { check: false } })
export const sendSyncFix = (p: string, s: string, chapter: number, issueIndex = 0, job?: string) =>
  request<{ schemaVersion: 1; ok: boolean; sent: boolean; action: 'steer' | 'continue' }>(`${storyPath(p, s)}/sync/fix`, { body: { chapter, issueIndex, job } })
/** “Xem khác biệt” — 404 khi chương không còn sửa ngoài app. */
export const getSyncDiff = (p: string, s: string, chapter: number, signal?: AbortSignal) =>
  request<SyncDiff>(`${storyPath(p, s)}/sync/diff/${chapter}`, { signal })
/** GAP-28 — chạy chẩn đoán (đồng thời hiện việc `diag` trong dock). */
export const getDiag = (p: string, s: string, signal?: AbortSignal) => request<DiagReport>(`${storyPath(p, s)}/diag`, { signal })
/** Báo cáo gần nhất, không chạy lại (404 khi chưa có) — nút “Xem báo cáo” ở dock. */
export const getLastDiag = (p: string, s: string, signal?: AbortSignal) => request<DiagReport>(`${storyPath(p, s)}/diag/last`, { signal })
/** Gửi `action.message` cho trợ lý (đang chạy → steer, rảnh → continue); 404 = cần phân tích lại. */
export const sendDiagAction = (p: string, s: string, fingerprint: string) =>
  request<{ ok: boolean; action: 'steer' | 'continue' }>(`${storyPath(p, s)}/diag/actions/${encodeURIComponent(fingerprint)}`, { body: {} })
export const downloadDiagExport = (p: string, s: string) =>
  download(`${storyPath(p, s)}/diag/export`, 'chan-doan-an-danh.md')
/** Hồ sơ văn phong (engine SimulationProfile) dạng tệp .json. */
export const downloadSimulationProfile = (p: string, s: string) =>
  download(`${storyPath(p, s)}/simulate/profile?download=1`, 'ho-so-van-phong.json')
export const startFromFile = (p: string, s: string, path: string) => request<{ ok: boolean; action: 'start' }>(`${storyPath(p, s)}/start-file`, { body: { path } })
export const getModels = (p: string, s: string, signal?: AbortSignal) => request<ModelsResponse>(`${storyPath(p, s)}/models`, { signal })

// ─── §6 Cấu hình & quy tắc ──────────────────────────────────────────────

const configQuery = (scope: ConfigScope, project?: string, story?: string) => {
  const q = new URLSearchParams({ scope })
  if (project) q.set('project', project)
  if (story) q.set('story', story)
  return q.toString()
}
export const getConfig = (scope: ConfigScope, project?: string, story?: string, signal?: AbortSignal) =>
  request<ConfigResponse>(`/api/config?${configQuery(scope, project, story)}`, { signal })
export const putConfig = (scope: ConfigScope, config: Config, project?: string, story?: string, version?: string) =>
  request<ConfigResponse>(`/api/config?${configQuery(scope, project, story)}`, { method: 'PUT', body: config, version })
/**
 * Gói AU — đổi hàng loạt trợ lý đang dùng `from` sang `to` (chỉ admin).
 * `version` gửi qua If-Match; lệch → 409 `{conflict,version,current}`.
 */
export const swapProvider = (from: string, to: string, model?: string, version?: string) =>
  request<SwapProviderResult>('/api/config/swap-provider', { body: { from, to, model }, version })
/** Gói AU — hoàn tác đúng các giá trị AI lần chuyển trước đã đổi (409 nếu cấu hình đã đổi). */
export const undoSwapProvider = (token: string) =>
  request<SwapUndoResult>('/api/config/swap-provider/undo', { body: { token } })
export const getConfigPresets = (signal?: AbortSignal) => request<ConfigPresets>('/api/config/presets', { signal })
/** G6 — thiếu `model` thì máy chủ dùng `defaultModel` của preset và trả lại model đã dùng. */
export const testConfig = (provider: string, model?: string, config?: Config) =>
  request<ConfigTestResult>('/api/config/test', { body: { provider, model, config } })
export const getConfigModels = (provider: string, models: readonly string[] = []) => request<{ models: ConfigModelInfo[] }>(`/api/config/models?provider=${encodeURIComponent(provider)}${models.map((model) => `&model=${encodeURIComponent(model)}`).join('')}`)
/**
 * Danh sách model của một nhà cung cấp (API key hoặc OAuth). `config` là bản nháp
 * cùng nghĩa với /api/config/test (api_key trống → máy chủ thừa kế khoá đã lưu); `refresh` bỏ cache 10 phút.
 * Luôn 200: lỗi nằm trong `error`/`message`/`unsupported`.
 */
export const listProviderModels = (provider: string, config?: Config, refresh?: boolean) =>
  request<ProviderModelList>('/api/config/models/list', { body: { provider, config, refresh } })
export const getConfigAccount = (provider: string) => request<ConfigAccount>(`/api/config/account?provider=${encodeURIComponent(provider)}`)
export const startOAuth = (provider: string, device: boolean) => request<OAuthStart>('/api/config/oauth/start', { body: { provider, device } })
export const getOAuthStatus = (id: string) => request<OAuthStatus>(`/api/config/oauth/status?id=${encodeURIComponent(id)}`)
export const cancelOAuth = (id: string) => request<{ ok: boolean }>('/api/config/oauth/cancel', { body: { id } })
export const completeOAuth = (id: string, code: string) => request<{ ok: boolean }>('/api/config/oauth/complete', { body: { id, code } })
export const logoutOAuth = (provider: string) => request<{ ok: boolean }>('/api/config/oauth/logout', { body: { provider } })

export const getRules = (scope: ConfigScope, project?: string, story?: string, signal?: AbortSignal) =>
  request<RulesResponse>(`/api/rules?${configQuery(scope, project, story)}`, { signal })
export const getRuleFile = (scope: ConfigScope, name: string, project?: string, story?: string, signal?: AbortSignal) =>
  request<RuleFile>(`/api/rules/file?${configQuery(scope, project, story)}&name=${encodeURIComponent(name)}`, { signal })
export const putRuleFile = (scope: ConfigScope, name: string, text: string, project?: string, story?: string, version?: string) =>
  request<{ name: string; size: number; updatedAt: string | null; version?: string }>(`/api/rules/file?${configQuery(scope, project, story)}&name=${encodeURIComponent(name)}`, { method: 'PUT', body: { scope, project, story, name, text }, version })
export const deleteRuleFile = (scope: ConfigScope, name: string, project?: string, story?: string) =>
  request<{ ok: boolean }>(`/api/rules/file/delete?${configQuery(scope, project, story)}`, { body: { scope, project, story, name } })
export const getSystem = (signal?: AbortSignal) => request<SystemInfo>('/api/system', { signal })

// ─── §8 Bổ sung 14-gap-closure.md ───────────────────────────────────────

/** G1 — mở thư mục trên máy chủ; chỉ loopback, khác → 403. */
export const revealFolder = (input: RevealRequest) =>
  request<RevealResult>('/api/reveal', { body: input })

/** G9 — guide được Kiến trúc sư chia lại và lưu để xác nhận. */
export const previewImport = (p: string, s: string, path: string, guide?: string) =>
  request<ImportPreview>(`${storyPath(p, s)}/import/preview`, { body: { path, guide } })

// G8 — đồng sáng tác cấp dự án: `draft` là id phiên nháp, chưa tạo truyện cho tới /commit.
const projectCocreatePath = (p: string, draft?: string) =>
  `/api/projects/${encodeURIComponent(p)}/cocreate${draft ? `?draft=${encodeURIComponent(draft)}` : ''}`
export const getProjectCoCreate = (p: string, draft?: string) =>
  request<ProjectCoCreateState>(projectCocreatePath(p, draft))
export const sendProjectCoCreate = (p: string, message: string, draft?: string) =>
  request<ProjectCoCreateReply>(`/api/projects/${encodeURIComponent(p)}/cocreate/message`,
    { body: draft ? { message, draft } : { message } })
export const commitProjectCoCreate = (p: string, draft: string, name?: string, brief?: string) =>
  request<{ story: StorySummary }>(`/api/projects/${encodeURIComponent(p)}/cocreate/commit`,
    { body: { draft, ...(name ? { name } : {}), ...(brief === undefined ? {} : { brief }) } })
export const cancelProjectCoCreate = (p: string, draft: string) =>
  request<{ ok: boolean }>(`/api/projects/${encodeURIComponent(p)}/cocreate/cancel?draft=${encodeURIComponent(draft)}`,
    { body: {} })

/** G1 — chỉ hỏi đường dẫn (không mở gì); dùng cho nút “Chép đường dẫn” khi canReveal=false. */
export const getRevealPath = (input: RevealRequest) => {
  const q = new URLSearchParams({ target: input.target })
  if (input.project) q.set('project', input.project)
  if (input.story) q.set('story', input.story)
  if (input.scope) q.set('scope', input.scope)
  return request<RevealPath>(`/api/reveal?${q.toString()}`)
}

/** G4 — “Thử lại ngay”: đánh thức vòng chờ backoff (409 khi không có gì đang chờ). */
export const retryStory = (p: string, s: string) => request<RetryResult>(`${storyPath(p, s)}/retry`, { body: {} })

/** G13 — lệnh gõ trong ô chat web; server phân tích bằng registry G11/G12 rồi chạy. */
export const postCommand = (p: string, s: string, line: string) =>
  request<CommandResult>(`${storyPath(p, s)}/command`, { body: { line } })

/** G11 — registry lệnh dùng chung cho popover `/` và trang Hướng dẫn (mảng trần, không bọc). */
export const getCommands = (signal?: AbortSignal) => request<CommandSpec[]>('/api/commands', { signal })

/** G2 — đổi thư mục thư viện lúc chạy (admin; 409 khi đang có truyện chạy, 400 khi đường dẫn sai). */
export const putLibraryRoot = (root: string) =>
  request<LibraryRootResult>('/api/library', { method: 'PUT', body: { root } })

/** G2 — hộp chọn thư mục của hệ điều hành trên máy chủ; 501 khi không hỗ trợ. */
export const pickFolder = (start?: string) =>
  request<PickFolderResult>('/api/system/pick-folder', { body: start ? { start } : {} })

// ─── §9 Audio — bản đọc, phân vai, tạo giọng, ghép & xuất (GAP-17…20) ───

const audioVol = (p: string, s: string, volume: number) => `${storyPath(p, s)}/audio/volumes/${volume}`
const audioChapter = (p: string, s: string, volume: number, chapter: number) =>
  `${audioVol(p, s, volume)}/chapters/${chapter}`

/** GAP-17 — danh sách chương của một tập + trạng thái bản đọc/audio từng chương. */
export const getAudioVolume = (p: string, s: string, volume: number, signal?: AbortSignal) =>
  request<AudioVolume>(audioVol(p, s, volume), { signal })

/** GAP-17 — AI soạn bản đọc cho các chương đã nghiệm thu; tiến độ qua SSE `audio.compose`. */
export const composeAudioScripts = (p: string, s: string, volume: number, chapters: number[]) =>
  request<{ ok: boolean }>(`${audioVol(p, s, volume)}/scripts`, { body: { chapters } })

export const getAudioScript = (p: string, s: string, volume: number, chapter: number, signal?: AbortSignal) =>
  request<AudioScript>(`${audioVol(p, s, volume)}/scripts/${chapter}`, { signal })

export const putAudioScript = (p: string, s: string, volume: number, chapter: number, segments: AudioSegment[], version?: string) =>
  request<AudioScript>(`${audioVol(p, s, volume)}/scripts/${chapter}`, { method: 'PUT', body: { segments }, version })

export const lockAudioChapter = (p: string, s: string, volume: number, chapter: number) =>
  request<{ ok: boolean }>(`${audioVol(p, s, volume)}/scripts/${chapter}/lock`, { body: {} })

export const unlockAudioChapter = (p: string, s: string, volume: number, chapter: number) =>
  request<{ ok: boolean }>(`${audioVol(p, s, volume)}/scripts/${chapter}/unlock`, { body: {} })

/** Chương đã chốt bị sửa: soạn lại đúng các đoạn đổi. */
export const refreshAudioChapter = (p: string, s: string, volume: number, chapter: number) =>
  request<{ ok: boolean }>(`${audioVol(p, s, volume)}/scripts/${chapter}/refresh`, { body: {} })

/** GAP-18 — phân vai cả truyện + chế độ một giọng. */
export const getAudioCast = (p: string, s: string, signal?: AbortSignal) => request<AudioCast>(`${storyPath(p, s)}/audio/cast`, { signal })
export const putAudioCast = (p: string, s: string, cast: AudioCast, version = cast.version) =>
  request<AudioCast>(`${storyPath(p, s)}/audio/cast`, { method: 'PUT', body: cast, version })

export const putAudioRoleVoice = (p: string, s: string, role: string, input: { voiceId: string; rerenderOld: boolean; sameAs?: string }, version?: string) =>
  request<AudioCast & { queuedSegments: number }>(`${storyPath(p, s)}/audio/cast/${encodeURIComponent(role)}`, { method: 'PUT', body: input, version })

export const getProjectVoices = (p: string, signal?: AbortSignal) =>
  request<AudioVoice[]>(`/api/projects/${encodeURIComponent(p)}/voices`, { signal })

/** Thêm mẫu giọng 10–30 giây (multipart) → Tài sản › Giọng của dự án. */
async function uploadVoice(url: string, name: string, file: File, gender = ''): Promise<AudioVoice> {
  const form = new FormData()
  form.append('name', name)
  form.append('file', file)
  if (gender) form.append('gender', gender)
  let response: Response
  try {
    response = await fetch(url, { method: 'POST', credentials: 'same-origin', body: form })
  } catch { throw new ApiError(0, OFFLINE_MSG) }
  if (!response.ok) {
    const payload: unknown = await response.json().catch(() => null)
    const message = payload && typeof payload === 'object' && 'error' in payload && typeof payload.error === 'string' ? payload.error : 'Không tải được mẫu giọng. Hãy thử lại.'
    const error = new ApiError(response.status, message || 'Không tải được mẫu giọng. Hãy thử lại.', undefined, payload)
    if (response.status === 401) await onUnauthorized?.(error)
    throw error
  }
  return response.json() as Promise<AudioVoice>
}

export const uploadProjectVoice = (p: string, name: string, file: File, gender = '') =>
  uploadVoice(`/api/projects/${encodeURIComponent(p)}/voices`, name, file, gender)
export const uploadAudioVoice = (p: string, s: string, name: string, file: File, gender = '') =>
  uploadVoice(`${storyPath(p, s)}/audio/voices`, name, file, gender)

/** Tư liệu của dự án (multipart): `cover` → ảnh bìa, `music` → nhạc nền. Giọng mẫu đi qua `uploadProjectVoice`. */
export async function uploadProjectAsset(p: string, kind: 'cover' | 'music', file: File): Promise<Asset> {
  const form = new FormData()
  form.append('kind', kind)
  form.append('file', file)
  let response: Response
  try {
    response = await fetch(`/api/projects/${encodeURIComponent(p)}/assets`, { method: 'POST', credentials: 'same-origin', body: form })
  } catch { throw new ApiError(0, OFFLINE_MSG) }
  if (!response.ok) {
    const payload: unknown = await response.json().catch(() => null)
    const message = payload && typeof payload === 'object' && 'error' in payload && typeof payload.error === 'string' ? payload.error : response.statusText
    const error = new ApiError(response.status, message || `Lỗi HTTP ${response.status}`, undefined, payload)
    if (response.status === 401) await onUnauthorized?.(error)
    throw error
  }
  return response.json() as Promise<Asset>
}

export type PronunciationResource = { entries: AudioPronunciation[]; version: string }
/** Từ điển giữ mảng JSON hiện có; phiên bản được gửi qua ETag. */
export async function getPronunciationResource(p: string, signal?: AbortSignal): Promise<PronunciationResource> {
  let response: Response
  try { response = await fetch(`/api/projects/${encodeURIComponent(p)}/pronunciations`, { credentials: 'same-origin', signal }) }
  catch (error) { if (signal?.aborted) throw error; throw new ApiError(0, OFFLINE_MSG) }
  if (!response.ok) {
    const payload = await response.json().catch(() => null) as { error?: string } | null
    const error = new ApiError(response.status, payload?.error ?? response.statusText, undefined, payload)
    if (response.status === 401) await onUnauthorized?.(error)
    throw error
  }
  return { entries: await response.json() as AudioPronunciation[], version: response.headers.get('ETag') ?? '' }
}
export const savePronunciationResource = (p: string, entries: AudioPronunciation[], version?: string) =>
  request<{ ok: boolean; version?: string }>(`/api/projects/${encodeURIComponent(p)}/pronunciations`, {
    method: 'PUT', body: entries, version,
  })

/** GAP-19 — chất lượng giọng + tự tạo khi chốt + công tắc tự chuẩn hoá. */
export const getAudioSettings = (p: string, s: string, signal?: AbortSignal) => request<AudioSettings>(`${storyPath(p, s)}/audio/settings`, { signal })
export const putAudioSettings = (p: string, s: string, settings: AudioSettings, version = settings.version) =>
  request<AudioSettings>(`${storyPath(p, s)}/audio/settings`, { method: 'PUT', body: settings, version })

export const getAudioQueue = (p: string, s: string, signal?: AbortSignal, volume = 1) => request<AudioQueue>(`${storyPath(p, s)}/audio/queue?volume=${volume}`, { signal })

export const enqueueAudioChapters = (p: string, s: string, volume: number, input: { chapters: number[]; quality: AudioSettings['quality']; voiceMode: 'cast' | 'single' | 'pack'; lock: boolean; redo?: number[] }) =>
  request<{ chapters: number[]; etaSec: number; warnings: { chapter: number; reason: string }[] }>(`${storyPath(p, s)}/audio/queue?volume=${volume}`, { body: input })
export const controlAudioQueue = (p: string, s: string, volume: number, action: 'pause' | 'resume' | 'retry', quality?: AudioSettings['quality']) =>
  request<{ ok: boolean }>(`${storyPath(p, s)}/audio/queue/${action}?volume=${volume}`, { body: { quality } })
export const prioritizeAudioChapter = (p: string, s: string, volume: number, chapter: number) =>
  request<{ ok: boolean }>(`${audioVol(p, s, volume)}/chapters/${chapter}/prioritize`, { body: {} })

/** Xếp một chương đã chốt vào hàng đợi tạo giọng (khi tắt tự tạo khi chốt). */
export const renderAudioChapter = (p: string, s: string, volume: number, chapter: number) =>
  request<{ ok: boolean }>(`${audioVol(p, s, volume)}/chapters/${chapter}/render`, { body: {} })

export const pauseAudioRender = (p: string, s: string, jobId: string) =>
  request<{ ok: boolean }>(`${storyPath(p, s)}/audio/renders/${encodeURIComponent(jobId)}/pause`, { body: {} })
export const resumeAudioRender = (p: string, s: string, jobId: string) =>
  request<{ ok: boolean }>(`${storyPath(p, s)}/audio/renders/${encodeURIComponent(jobId)}/resume`, { body: {} })
export const cancelAudioRender = (p: string, s: string, jobId: string) =>
  request<{ ok: boolean }>(`${storyPath(p, s)}/audio/renders/${encodeURIComponent(jobId)}/cancel`, { body: {} })

/** Nghe thử một đoạn = tạo & GIỮ đoạn đó (nút nghe ở 06). */
export const previewAudioSegment = (p: string, s: string, volume: number, chapter: number, segmentId: string) =>
  request<{ url: string; durationSec: number; cached: boolean }>(
    `${storyPath(p, s)}/audio/segments/${chapter}-${encodeURIComponent(segmentId)}/preview?volume=${volume}`,
    { body: {} },
  )

export const rerenderAudioSegment = (p: string, s: string, volume: number, chapter: number, segmentId: string,
  input: { voiceId?: string; speed?: number; tone?: string }) =>
  request<AudioCandidate>(
    `${storyPath(p, s)}/audio/segments/${chapter}-${encodeURIComponent(segmentId)}/rerender?volume=${volume}`,
    { body: input },
  )

/** A5 — “Dùng bản mới” / “Giữ bản cũ”. */
export const acceptAudioCandidate = (p: string, s: string, volume: number, chapter: number, segmentId: string, candidateId: string) =>
  request<{ ok: boolean }>(
    `${storyPath(p, s)}/audio/segments/${chapter}-${encodeURIComponent(segmentId)}/accept?volume=${volume}`,
    { body: { candidateId } },
  )
export const rejectAudioCandidate = (p: string, s: string, volume: number, chapter: number, segmentId: string, candidateId: string) =>
  request<{ ok: boolean }>(
    `${storyPath(p, s)}/audio/segments/${chapter}-${encodeURIComponent(segmentId)}/reject?volume=${volume}`,
    { body: { candidateId } },
  )

/**
 * Audio một chương — HTMLAudioElement tự gửi Range tới địa chỉ này. `version`
 * (AudioChapterAudio.version) đổi khi chương được dựng lại: URL khác thì thẻ
 * audio đang mở mới tải tệp mới thay vì phát tiếp bản cũ đã nạp.
 */
export const audioChapterURL = (p: string, s: string, volume: number, chapter: number, version?: string) =>
  `${audioChapter(p, s, volume, chapter)}.mp3${version ? `?v=${encodeURIComponent(version)}` : ''}`

export const getAudioPeaks = (p: string, s: string, volume: number, chapter: number, signal?: AbortSignal) =>
  request<AudioPeaks>(`${audioChapter(p, s, volume, chapter)}/peaks`, { signal })

/** Gói AE — ghép chạy nền: trả {jobId} ngay (202); tiến độ qua SSE job.* · 409 khi đang ghép cùng tập. */
export const createAudioExport = (p: string, s: string, volume: number, input: AudioExportOptions) =>
  request<{ jobId: string }>(`${audioVol(p, s, volume)}/exports`, { body: input })

export const getAudioExportPrefs = (p: string, s: string, signal?: AbortSignal) =>
  request<AudioExportPrefs>(`${storyPath(p, s)}/audio/export-prefs`, { signal })
export const saveAudioExportPrefs = (p: string, s: string, prefs: AudioExportPrefs) =>
  request<AudioExportPrefs>(`${storyPath(p, s)}/audio/export-prefs`, { method: 'PUT', body: prefs })

/** Tệp audio đã ghép — cùng đường tải của màn Xuất, HTMLAudioElement tự gửi Range. */
export const audioExportFileURL = (p: string, s: string, path: string) =>
  `${storyPath(p, s)}/exports/file?path=${encodeURIComponent(path)}`

export const getAudioExportState = (p: string, s: string, volume: number, signal?: AbortSignal) =>
  request<AudioExportState>(`${audioVol(p, s, volume)}/exports`, { signal })

export const getAudioFlags = (p: string, s: string, volume: number, signal?: AbortSignal) =>
  request<{ flags: AudioFlag[] }>(`${storyPath(p, s)}/audio/flags?volume=${volume}`, { signal })
export const addAudioFlag = (p: string, s: string, input: { chapter: number; segmentId: string; atSec: number; reason: string }) =>
  request<AudioFlag>(`${storyPath(p, s)}/audio/flags`, { body: input })
export const deleteAudioFlag = (p: string, s: string, id: string) =>
  request<{ ok: boolean }>(`${storyPath(p, s)}/audio/flags/${encodeURIComponent(id)}`, { method: 'DELETE' })

/** Thiếu ffmpeg / bộ đọc chưa cài → màn Audio hiện thẻ hướng dẫn cài. */
export const getAudioReady = (p: string, s: string, signal?: AbortSignal) => request<AudioReady>(`${storyPath(p, s)}/audio/ready`, { signal })

/** SSE của audio: `audio.render` · `audio.queue` · `audio.compose` · `audio.export` · `audio.install`. */

// ─── §9b Bộ đọc trên máy (GAP-21) ───────────────────────────────────────

export const getTtsStatus = (signal?: AbortSignal) => request<TtsStatus>('/api/tts/status', { signal })
export const getTtsEngines = (signal?: AbortSignal) =>
  request<{ engines: TtsEngine[]; active?: string }>('/api/tts/engines', { signal }).then((r) => r.engines ?? [])
/** Cài bộ đọc trên máy; tiến độ qua `status.install` (và SSE `audio.install` khi có). */
export const installTts = () => request<{ job: string }>('/api/tts/install', { body: {} })
/** Mốc tiến độ của lượt cài (dự phòng khi không dùng SSE `tts.install`). */
export const getTtsInstall = (signal?: AbortSignal) => request<TtsInstallProgress>('/api/tts/install', { signal })
export const getTtsVoices = (signal?: AbortSignal) =>
  request<{ voices: AudioVoice[] }>('/api/tts/voices', { signal }).then((r) => r.voices ?? [])

/** §AT — Thử lại hàng Chuẩn bị bộ đọc trong dock (jobs API, row id cố định). */
export const retryTtsInstall = () => request<{ ok: true }>('/api/jobs/voice/install/retry', { body: {} })
/** §AT — Huỷ hàng Chuẩn bị bộ đọc đang lỗi (bỏ khỏi danh sách). */
export const cancelTtsInstall = () => request<{ ok: true }>('/api/jobs/voice/install/cancel', { body: {} })
/** §AT §8.5 — Thu hồi quyền máy đọc đang phục vụ (Nâng cao, một bước). */
export const revokeTtsMachine = () => request<{ ok: true; machine: string }>('/api/tts/machine/revoke', { body: {} })

/** Nghe thử giọng — trả blob URL để phát bằng HTMLAudioElement. */
export async function previewTtsVoice(voiceId: string, text: string, tone?: string): Promise<string> {
  let response: Response
  try {
    response = await fetch('/api/tts/preview', {
      method: 'POST', credentials: 'same-origin',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(tone ? { voiceId, text, tone } : { voiceId, text }),
    })
  } catch { throw new ApiError(0, OFFLINE_MSG) }
  if (!response.ok) {
    const payload: unknown = await response.json().catch(() => null)
    const message = payload && typeof payload === 'object' && 'error' in payload && typeof payload.error === 'string' ? payload.error : response.statusText
    const error = new ApiError(response.status, message || `Lỗi HTTP ${response.status}`, undefined, payload)
    if (response.status === 401) await onUnauthorized?.(error)
    throw error
  }
  const blob = await response.blob()
  return URL.createObjectURL(blob)
}

// ─── §10 Việc chạy nền (14-jobs-dock.md §1.7 · GAP-26) ──────────────────

/** Id việc chứa `/` (vd `p/s/audio/1/export`): giữ dấu `/` để cổng tách đúng `<id>/<op>`. */
const jobPath = (id: string) => id.split('/').map(encodeURIComponent).join('/')
export const getJobs = () => request<JobsResponse>('/api/jobs')
export const getJob = (id: string, signal?: AbortSignal) =>
  request<JobDetail>(`/api/jobs/${jobPath(id)}`, { signal })
/** Các sự kiện job.* đi chung /api/events; dùng data/events.subscribeEvents, không mở luồng riêng. */
export const jobAction = (id: string, action: 'pause' | 'resume' | 'cancel' | 'retry', body?: Record<string, unknown>) =>
  request<{ ok: boolean }>(`/api/jobs/${jobPath(id)}/${action}`, { body: body ?? {} })
/** Nút “Tải xuống” của việc xuất: `href` là URL API đầy đủ do máy chủ dựng. */
export const downloadJobFile = (href: string, fallbackName: string, onProgress?: (done: number | null) => void) => download(href, fallbackName, onProgress)

/** M2 — góp ý lưu tại gateway, vẫn dùng khi TUI đang giữ khoá truyện. */
export const paragraphComments = (p: string, s: string, chapter: number, signal?: AbortSignal) =>
  request<{ comments: ParagraphComment[] }>(`${storyPath(p, s)}/chapters/${chapter}/comments`, { signal })
export const addParagraphComment = (p: string, s: string, chapter: number, input: ParagraphCommentInput, signal?: AbortSignal) =>
  request<ParagraphComment>(`${storyPath(p, s)}/chapters/${chapter}/comments`, { body: input, signal })
export const deleteParagraphComment = (p: string, s: string, chapter: number, id: string) =>
  request<{ ok: boolean }>(`${storyPath(p, s)}/chapters/${chapter}/comments?id=${encodeURIComponent(id)}`, { method: 'DELETE' })
export const patchParagraphComment = (p: string, s: string, chapter: number, id: string, patch: ParagraphCommentPatch) =>
  request<ParagraphComment>(`${storyPath(p, s)}/chapters/${chapter}/comments/${encodeURIComponent(id)}`, { method: 'PATCH', body: patch })
export const approveWithComments = (p: string, s: string, chapter: number) =>
  request<{ ok: boolean; appliedComments: number }>(`${storyPath(p, s)}/next`, { body: { applyComments: true, chapter } })

/** Xoá vĩnh viễn truyện; confirm là tên thư mục truyện (tham số s). */
export const deleteStory = (p: string, s: string, confirm: string) =>
  request<{ ok: boolean }>(storyPath(p, s), { method: 'DELETE', body: { confirm } })

/** GAP-30 — dữ liệu dùng chung cho M7 và Tuỳ chỉnh trên máy tính. */
import type { NotifyPrefs, PushSubscriptionInfo, NotificationsResponse } from './types'
export const getNotifyPrefs = (signal?: AbortSignal) => request<NotifyPrefs>('/api/notify/prefs', { signal })
export const putNotifyPrefs = (prefs: NotifyPrefs, version: string) =>
  request<NotifyPrefs>('/api/notify/prefs', { method: 'PUT', body: prefs, version })
export const getPushKey = () => request<{ publicKey: string }>('/api/push/vapid')
export const getPushSubscriptions = (signal?: AbortSignal) =>
  request<{ subscriptions: PushSubscriptionInfo[] }>('/api/push/subscriptions', { signal })
export const subscribePush = (body: Pick<PushSubscriptionInfo, 'endpoint' | 'keys' | 'device'>) =>
  request<{ ok: boolean }>('/api/push/subscriptions', { body })
export const unsubscribePush = (endpoint: string) => request<{ ok: boolean }>('/api/push/subscriptions', { method: 'DELETE', body: { endpoint } })
export const getNotifications = (before?: string, signal?: AbortSignal) =>
  request<NotificationsResponse>(`/api/notifications${before ? `?before=${encodeURIComponent(before)}` : ''}`, { signal })
export const readNotifications = (body: { ids?: string[]; all?: boolean }) =>
  request<{ ok: boolean; unread: number }>('/api/notifications/read', { body })
export const testPush = () => request<{ ok: boolean }>('/api/notify/test', { body: {} })
export const notificationAction = (id: string, action: string) =>
  request<{ ok: boolean; url?: string }>(`/api/notifications/${encodeURIComponent(id)}/action`, { body: { action } })

/** GAP-31 — không khởi động worker; đọc được cả khi TUI đang giữ khoá. */
export const getStoryEvents = (p: string, s: string, query: ActivityHistoryQuery = {}, signal?: AbortSignal) => {
  const params = new URLSearchParams()
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined && value !== '' && value !== false) params.set(key, value === true ? '1' : String(value))
  }
  return request<ActivityHistory>(`${storyPath(p, s)}/events?${params}`, { signal })
}

// ─── GAP-34 — duyệt & kiểm tra thư mục (admin) ───
export const getFsPlaces = () => request<FsPlace[]>('/api/fs/places')
export const listFsFolders = (path: string, signal?: AbortSignal) =>
  request<FsFolder[]>(`/api/fs/list?path=${encodeURIComponent(path)}`, { signal })
export const mkFsFolder = (parent: string, name: string) =>
  request<FsMkdirResult>('/api/fs/mkdir', { body: { parent, name } })
export const inspectLibraryPath = (path: string, signal?: AbortSignal) =>
  request<LibraryInspect>(`/api/library/inspect?path=${encodeURIComponent(path)}`, { signal })

// ─── Quy tắc viết (GAP-32) ───
export const understandRule = (text: string, scope?: ConfigScope, project?: string, story?: string, signal?: AbortSignal) =>
  request<RuleUnderstanding>('/api/rules/understand', { body: { text, scope, project, story }, signal })
export const getRulesEffective = (scope: ConfigScope, project?: string, story?: string, signal?: AbortSignal) =>
  scope === 'story' && project && story
    ? request<RulesEffective>(`${storyPath(project, story)}/rules/effective`, { signal })
    : request<RulesEffective>(`/api/rules/effective?${configQuery(scope, project, undefined)}`, { signal })
export const applyRules = (p: string, s: string, recheck: boolean) =>
  request<RuleApplyResult>(`${storyPath(p, s)}/rules/apply`, { body: recheck ? { recheck: true } : {} })
export const keepRuntimeRule = (p: string, s: string, id: string) =>
  request<{ ok: boolean; text: string; file: string }>(`${storyPath(p, s)}/rules/runtime/${encodeURIComponent(id)}/keep`, { body: {} })
export const dropRuntimeRule = (p: string, s: string, id: string) =>
  request<{ ok: boolean }>(`${storyPath(p, s)}/rules/runtime/${encodeURIComponent(id)}`, { method: 'DELETE' })
export const getRuleViolations = (p: string, s: string, last = 3, signal?: AbortSignal) =>
  request<RuleViolationsResponse>(`${storyPath(p, s)}/rules/violations?last=${last}`, { signal })

// ─── GAP-35 — văn mẫu cho /simulate (Tuỳ chỉnh › Cách viết › Văn phong của bạn) ───
export const getStyleSamples = (p: string, s: string, signal?: AbortSignal) =>
  request<StyleSamples>(`${storyPath(p, s)}/style-samples`, { signal })
export const createStyleSample = (p: string, s: string, input: { title?: string; text: string }) =>
  request<StyleSample>(`${storyPath(p, s)}/style-samples`, { body: input })
export const deleteStyleSample = (p: string, s: string, id: string) =>
  request<null>(`${storyPath(p, s)}/style-samples/${encodeURIComponent(id)}`, { method: 'DELETE' })

// ─── D6 — Đo bằng máy: chín số đo của gói cho từng chương đã viết ───
export const getStoryMetrics = (p: string, s: string, chapters: MetricsScope = 'last3', signal?: AbortSignal) =>
  request<StoryMetrics>(`${storyPath(p, s)}/metrics?chapters=${chapters}`, { signal })

// ─── D12 — Não truyện: mở kho ghi chú bằng Obsidian trên máy chủ (không trả đường dẫn) ───
export const openNaoVault = (input: { target: 'nao-story' | 'nao-project'; project: string; story?: string }) =>
  request<{ ok: true }>('/api/reveal', { body: input })

// ─── D14 — sổ cái, vũ trụ chung và tìm trong dự án ───
import type { ProjectLedger, LedgerCheckResult, LedgerSuggestion, ProjectUniverse, ProjectSearch } from './types'
const projectBrainPath = (p: string) => `/api/projects/${encodeURIComponent(p)}`
export const getProjectLedger = (p: string, signal?: AbortSignal) =>
  request<ProjectLedger>(`${projectBrainPath(p)}/ledger`, { signal })
export const checkProjectLedger = (p: string, brief: string, signal?: AbortSignal) =>
  request<LedgerCheckResult>(`${projectBrainPath(p)}/ledger/check`, { body: { brief }, signal })
export const suggestProjectLedger = (p: string, brief: string) =>
  request<LedgerSuggestion>(`${projectBrainPath(p)}/ledger/suggest`, { body: { brief } })
export const getProjectUniverse = (p: string, signal?: AbortSignal) =>
  request<ProjectUniverse>(`${projectBrainPath(p)}/universe`, { signal })
export const searchProjectBrain = (p: string, q: string, signal?: AbortSignal) =>
  request<ProjectSearch>(`${projectBrainPath(p)}/search?q=${encodeURIComponent(q)}`, { signal })

// ─── D15 · Não hệ thống (GAP-46, chỉ quản trị — người khác nhận 403) ───
import type { Lesson, LessonAction, LessonState, LessonsResponse, ViolationsResponse } from './types'
export const getLessons = (state: LessonState, signal?: AbortSignal) =>
  request<LessonsResponse>(`/api/system/lessons?state=${state}`, { signal })
export const actOnLesson = (id: string, input: LessonAction) =>
  request<Lesson>(`/api/system/lessons/${encodeURIComponent(id)}`, { body: input })
export const getViolations = (pack: string, signal?: AbortSignal) =>
  request<ViolationsResponse>(`/api/system/violations?pack=${encodeURIComponent(pack)}`, { signal })

// ─── D13 · Chất lượng truyện (GAP-42) ──────────────────────────────────
import type { Quality } from './types'
export const getQuality = (p: string, s: string, signal?: AbortSignal) =>
  request<Quality>(`${storyPath(p, s)}/quality`, { signal })

// Gói AN · Quản trị kho tệp (secrets never returned by GET).
import type { StorageCandidate, StorageSave, StorageStep, StorageTest, StorageView } from './storage-settings'
import { readStorageTest } from './storage-stream'
export const getStorage = (signal?: AbortSignal) => request<StorageView>('/api/system/storage', { signal })
export const testStorage = (candidate: StorageCandidate | { current: true }, signal?: AbortSignal, onStep?: (step: StorageStep) => void) =>
  request<StorageTest>('/api/system/storage/test', { body: candidate, signal, accept: 'application/x-ndjson', decode: (response) => {
    if (!response.body || !response.headers.get('Content-Type')?.includes('application/x-ndjson')) throw new Error('Máy chủ chưa trả được tiến độ kiểm tra. Vui lòng kiểm tra lại.')
    return readStorageTest(response.body, onStep, signal)
  } })
export const putStorage = (candidate: StorageCandidate) =>
  request<StorageSave>('/api/system/storage', { method: 'PUT', body: candidate })
export const controlStorageMigration = (action: 'start' | 'pause' | 'resume' | 'cancel', version: number) =>
  request<StorageView>(`/api/system/storage/migration${action === 'start' ? '' : `/${action}`}`, { body: { version } })
