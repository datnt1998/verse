// Logic thuần của dock việc chạy nền — 14-jobs-dock.md §1.2 (trạng thái), §1.5 (dòng), §1.6 (ưu tiên).
// Không đụng DOM/React để test được bằng vitest.
import type { Job, JobStatus } from '../../api/types'
import { exportLook, exportRunLine, isAudioExportJob, jobError } from './exportFlow'
import { AGENT_PALETTE } from '../../ui/agentPalette'
import { agentLabel } from '../../lib/labels'
import { t } from '../../i18n/vi'

// ─── Thứ tự ưu tiên (§1.6): Cần bạn → Lỗi → Đang chạy → Tạm dừng → Xong ───
const PRIORITY: Record<JobStatus, number> = { waiting: 0, failed: 1, running: 2, paused: 3, done: 4 }

/** Sắp theo ưu tiên; cùng mức thì việc bắt đầu trước đứng trước (ổn định). */
export function byPriority(jobs: readonly Job[]): Job[] {
  return jobs
    .map((job, index) => ({ job, index }))
    .sort((a, b) =>
      PRIORITY[a.job.status] - PRIORITY[b.job.status] ||
      a.job.startedAt.localeCompare(b.job.startedAt) ||
      a.index - b.index)
    .map((item) => item.job)
}

export const isActive = (job: Job) => job.status !== 'done'
export const needsYou = (job: Job) => job.status === 'waiting' || job.status === 'failed'

// ─── Thời gian còn lại ───
/** Approximate remaining time; unknown ETA produces no label. */
export function formatEta(etaSec: number | undefined | null): string {
  if (etaSec === undefined || etaSec === null || !Number.isFinite(etaSec) || etaSec < 0) return ''
  if (etaSec < 60) return t.progress.underMinute
  const minutes = Math.ceil(etaSec / 60)
  if (minutes < 60) return t.progress.remainingApprox(`${minutes} phút`)
  const hours = Math.floor(minutes / 60)
  const rest = minutes % 60
  return t.progress.remainingApprox(rest ? `${hours} giờ ${rest} phút` : `${hours} giờ`)
}

export const pct = (job: Job) => `${Math.round((job.progress ?? 0) * 100)}%`

/** “Chương 5/12” — khi đang chạy hiện mục đang làm (done + 1), xong thì tổng. */
export function unitText(job: Job): string {
  const unit = job.unit
  if (!unit || unit.total <= 0) return ''
  const current = job.status === 'done' ? unit.total : Math.min(unit.total, Math.max(1, unit.done + 1))
  return `${unit.label} ${current}/${unit.total}`
}

/** Dòng phụ việc agent: “Kiến trúc sư · Bước 2/4 · Chọn tuyến chính”. */
export function agentLine(job: Job): string {
  if (!job.agent) return job.step ?? ''
  const parts = [agentLabel(job.agent)]
  if (job.unit && job.unit.total > 0) parts.push(unitText(job))
  if (job.step) parts.push(job.step)
  return parts.join(' · ')
}

// ─── Màu & icon theo loại việc (§1.5) ───
/** Việc của bộ đọc (tạo giọng, cài bộ đọc trên máy) chung một màu violet của audio. */
const AUDIO_COLOR = '#C9A6F5'
const KIND_COLOR: Record<Job['kind'], string> = {
  write: AUDIO_COLOR, audio: AUDIO_COLOR, voice: AUDIO_COLOR, export: '#A6EDC1', sync: '#7C93F3',
  simulate: '#F6CB8C', import: '#F6CB8C', diag: '#B9D0F7',
}
const KIND_ICON: Record<Job['kind'], string> = {
  write: 'book-1', audio: 'voice-square', voice: 'voice-square', export: 'export-square', sync: 'refresh-2',
  simulate: 'magic-star', import: 'document-text', diag: 'activity',
}
export const WAIT_COLOR = '#F6CB8C'
export const FAIL_COLOR = '#FF9FA8'
export const DONE_COLOR = '#A6EDC1'

export const kindColor = (job: Job) => KIND_COLOR[job.kind]
export const kindIcon = (job: Job) => KIND_ICON[job.kind]
/** progress null = không rõ % → vạch chạy qua lại, vòng quay 1,1 s. */
export const isIndeterminate = (job: Job) => job.progress === null
export const isAgentJob = (job: Job) => Boolean(job.agent)

export function iconColor(job: Job): string {
  if (job.status === 'failed') return FAIL_COLOR
  if (job.status === 'waiting') return WAIT_COLOR
  return kindColor(job)
}

/** Ring v2 uses a single colour, preserving agent, paused and attention states. */
export function ringColor(job: Job): string {
  if (job.status === 'waiting') return WAIT_COLOR
  if (job.status === 'failed') return FAIL_COLOR
  if (job.status === 'done') return DONE_COLOR
  if (job.status === 'paused') return 'var(--ring-paused)'
  if (isIndeterminate(job)) return WAIT_COLOR
  return job.agent ? AGENT_PALETTE[job.agent].c2 : kindColor(job)
}

/** Chấm trắng ở đầu cung — chỉ khi đang chạy/tạm dừng, có %, đã quá 2%. */
export const hasRingHead = (job: Job) =>
  (job.status === 'running' || job.status === 'paused') && !isIndeterminate(job) && (job.progress ?? 0) > 0.02

export const ringHeadColor = (job: Job) => (job.agent ? AGENT_PALETTE[job.agent].c3 : kindColor(job))

// ─── Dòng chữ tự chạy trên pill ───
export type PillLine = { a: string; b: string; fg: string }

export function pillLine(job: Job): PillLine {
  if (job.status === 'waiting') return { a: `Cần bạn · ${job.title}`, b: job.step ?? job.result?.sub ?? '', fg: WAIT_COLOR }
  if (job.status === 'failed') return { a: isAudioExportJob(job) ? exportLook(job).title : `Lỗi · ${job.title}`, b: jobError(job), fg: '#FFC9CE' }
  if (job.status === 'paused') return { a: `${job.title} · đã tạm dừng`, b: unitText(job) || pct(job), fg: '#C9C9D1' }
  if (job.agent) return { a: isIndeterminate(job) ? job.title : `${job.title} · ${pct(job)}`, b: agentLine(job), fg: '#EDEDEF' }
  if (isAudioExportJob(job)) return { a: exportLook(job).title, b: exportRunLine(job) || job.step || '', fg: '#EDEDEF' }
  return { a: `${job.title} · ${pct(job)}`, b: [unitText(job), formatEta(job.etaSec)].filter(Boolean).join(' · '), fg: '#EDEDEF' }
}

/** Dòng chữ pill: việc đang hoạt động theo ưu tiên; hết việc → “✓ …” của việc vừa xong. */
export function pillLines(active: readonly Job[], lastDone: Job | undefined): PillLine[] {
  const lines = byPriority(active).map(pillLine)
  if (!lines.length && lastDone) lines.push({ a: `✓ ${lastDone.result?.title ?? lastDone.title}`, b: 'Mọi việc đã xong', fg: DONE_COLOR })
  if (!lines.length) lines.push({ a: 'Không có việc nào', b: '', fg: '#8E8E99' })
  return lines
}

/** Vạch gộp 2 px: trung bình % của việc đang chạy có %; chỉ còn việc không rõ % → null (chạy hoa văn). */
export function aggregateProgress(active: readonly Job[]): number | null {
  const runs = active.filter((job) => job.status === 'running')
  const known = runs.filter((job) => job.progress !== null)
  if (known.length) return known.reduce((sum, job) => sum + (job.progress ?? 0), 0) / known.length
  if (runs.length) return null
  return active.length ? 0 : 1
}

/** Đầu bảng: “1 cần bạn · 3 đang chạy”. */
export function panelSubtitle(active: readonly Job[]): string {
  const need = active.filter(needsYou).length
  const run = active.filter((job) => job.status === 'running').length
  const parts: string[] = []
  if (need) parts.push(`${need} cần bạn`)
  if (run) parts.push(`${run} đang chạy`)
  return parts.join(' · ') || 'Không còn việc đang chạy'
}

// ─── Nhóm trong bảng ───
export type JobGroup = { id: 'need' | 'run' | 'done'; label: string; color: string; jobs: Job[] }

export function groupJobs(jobs: readonly Job[]): JobGroup[] {
  const sorted = byPriority(jobs)
  const groups: JobGroup[] = [
    { id: 'need', label: 'Cần bạn', color: WAIT_COLOR, jobs: sorted.filter(needsYou) },
    { id: 'run', label: 'Đang chạy', color: '#7E7E8A', jobs: sorted.filter((job) => job.status === 'running' || job.status === 'paused') },
    { id: 'done', label: 'Vừa xong', color: '#7E7E8A', jobs: sorted.filter((job) => job.status === 'done') },
  ]
  return groups.filter((group) => group.jobs.length > 0)
}

// ─── Lọc theo màn ───
export type Where = { workspace: boolean; project?: string; story?: string }

/** Ở Workspace, việc agent (chưa xong) của chính truyện đang mở đã hiện ở cột agent → không lặp trong dock. */
export function hiddenOnWorkspace(job: Job, where: Where): boolean {
  return where.workspace && Boolean(job.agent) && job.status !== 'done' &&
    job.project === where.project && job.story === where.story
}

export const visibleJobs = (jobs: readonly Job[], where: Where) => jobs.filter((job) => !hiddenOnWorkspace(job, where))

// ─── Máy trạng thái dock (§1.2) ───
export type DockMode = 'hidden' | 'pill' | 'mini' | 'panel'
export type DockState = {
  mode: DockMode
  /** morph clip-path của bảng: 'in' khi mở, 'out' khi đang thu (đích = closingTo) */
  panelAnim: '' | 'in' | 'out'
  closingTo: 'pill' | 'mini' | null
  /** đang chạy hiệu ứng biến mất (xong hết + 4,2 s) */
  leaving: boolean
  /** tăng mỗi lần dock xuất hiện lại → chạy lại animation vào */
  enterN: number
}
export type DockEvent =
  | { type: 'restore'; mode: Exclude<DockMode, 'hidden'> }
  | { type: 'jobAdded' }
  | { type: 'clickPill'; workspace: boolean }
  | { type: 'toMini' }
  | { type: 'close' }
  | { type: 'open' }
  | { type: 'toggle' }
  | { type: 'settle' }
  | { type: 'idleOut' }
  | { type: 'hide' }

export const initialDock: DockState = { mode: 'hidden', panelAnim: '', closingTo: null, leaving: false, enterN: 0 }

const openPanel = (state: DockState): DockState =>
  ({ ...state, mode: 'panel', panelAnim: 'in', closingTo: null, leaving: false, enterN: state.mode === 'hidden' ? state.enterN + 1 : state.enterN })
const closePanel = (state: DockState, to: 'pill' | 'mini'): DockState =>
  (state.mode === 'panel' ? { ...state, panelAnim: 'out', closingTo: to } : state)

export function dockReducer(state: DockState, event: DockEvent): DockState {
  switch (event.type) {
    case 'restore':
      if (state.mode !== 'hidden') return state
      return event.mode === 'panel' ? openPanel(state) : { ...state, mode: event.mode, leaving: false, enterN: state.enterN + 1 }
    case 'jobAdded':
      if (state.mode === 'hidden') return { ...state, mode: 'pill', leaving: false, enterN: state.enterN + 1 }
      return state.leaving ? { ...state, leaving: false } : state
    case 'clickPill':
      // Chấm ngoài Workspace → pill; ở Workspace (luôn là chấm) hoặc từ pill → bảng.
      if (state.mode === 'mini' && !event.workspace) return { ...state, mode: 'pill' }
      return openPanel(state)
    case 'toMini':
      if (state.mode === 'panel') return closePanel(state, 'mini')
      return state.mode === 'pill' ? { ...state, mode: 'mini' } : state
    case 'close':
      return closePanel(state, 'pill')
    case 'open':
      return state.mode === 'panel' && state.panelAnim !== 'out' ? state : openPanel(state)
    case 'toggle':
      if (state.mode === 'panel' && state.panelAnim !== 'out') return closePanel(state, 'pill')
      return openPanel(state)
    case 'settle':
      if (state.panelAnim === 'out' && state.closingTo) return { ...state, mode: state.closingTo, panelAnim: '', closingTo: null }
      return state.panelAnim === 'in' ? { ...state, panelAnim: '' } : state
    case 'idleOut':
      return state.mode === 'hidden' || state.mode === 'panel' ? state : { ...state, leaving: true }
    case 'hide':
      return state.leaving ? { ...state, mode: 'hidden', leaving: false, panelAnim: '', closingTo: null } : state
  }
}

/** Ở Workspace, pill luôn thu thành chấm (pill đè ô chat). */
export const showsAsDot = (mode: DockMode, workspace: boolean) => mode === 'mini' || (workspace && mode === 'pill')

// ─── Toast (§1.4) ───
export type ToastKind = 'done' | 'wait' | 'err'
export const TOAST_MS = 6000
export const MAX_TOASTS = 2

/** Đổi trạng thái nào thì bật toast: xong · chờ bạn · lỗi. `prev` undefined = việc mới thấy qua SSE. */
export function toastKindFor(prev: JobStatus | undefined, next: JobStatus): ToastKind | null {
  if (prev === next) return null
  if (next === 'done') return 'done'
  if (next === 'waiting') return 'wait'
  if (next === 'failed') return 'err'
  return null
}
