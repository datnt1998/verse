import type { JobStep } from '../../api/workbench'
import { progressText } from '../../lib/progress-copy'

type StageJob = { stageGroup: string }
type GroupKey = 'waiting' | 'working' | 'ready' | 'queued' | 'posted'
const definitions: { key: GroupKey; label: string; stages: string[] }[] = [
  { key: 'waiting', label: 'Chờ bạn', stages: ['attention', 'review', 'paused'] },
  { key: 'working', label: 'Đang làm', stages: ['working'] },
  { key: 'ready', label: 'Sẵn sàng đăng', stages: ['ready'] },
  { key: 'queued', label: 'Chờ lượt', stages: ['queued'] },
  { key: 'posted', label: 'Đã đăng', stages: ['posted'] },
]
export function groupJobs<T extends StageJob>(jobs: T[], all: boolean) {
  return definitions.flatMap(({ key, label, stages }) => {
    if (all && key === 'posted') return []
    const matches = jobs.filter(job => stages.includes(job.stageGroup))
    const items = key === 'queued' ? matches.slice(0, all ? 3 : 2) : key === 'posted' ? matches.slice(0, 1) : matches
    return items.length ? [{ key, label, items, count: matches.length }] : []
  })
}
export function readScope(search: URLSearchParams, saved: string | null) {
  return search.get('channel') || (search.get('scope') === 'all' ? 'all' : saved || 'all')
}
export function scheduleScopeRestore(saved: string, readHash: () => string, restore: (scope: string) => void) {
  queueMicrotask(() => {
    if (readHash() === '#/') restore(saved)
  })
}
// Mục hàng chờ đang chọn được nhận thành job mới (id khác): đi theo job cùng nguồn, thế hệ cao nhất.
export function followAdmitted<T extends { id: string; sourceKey: string; generation: number }>(jobs: T[], followed: { id: string; sourceKey: string } | null) {
  if (!followed?.sourceKey || jobs.some(job => job.id === followed.id)) return undefined
  return jobs.filter(job => job.sourceKey === followed.sourceKey).sort((a, b) => b.generation - a.generation)[0]?.id
}

/** Chỉ hiện khoảng cảnh khi máy chủ đã chiếu đủ hai mốc; 0 là mốc hợp lệ. */
export function imageTime(image: { startSec?: number; endSec?: number }): string {
  if (image.startSec == null || image.endSec == null) return ''
  const time = (seconds: number) => `${Math.floor(seconds / 60)}:${String(Math.floor(seconds % 60)).padStart(2, '0')}`
  return `${time(image.startSec)}–${time(image.endSec)}`
}

/** Bảy trạng thái của artboard Main › stage; `queued` là hàng chờ, không phải video đang mở. */
export type JobStage = 'images' | 'review' | 'render' | 'issue' | 'quality' | 'done' | 'paused' | 'queued'

/**
 * Suy trạng thái hiển thị từ dữ liệu thật của job. Lỗi quality_gate của bước
 * score hoặc export còn nằm trong steps sau khi worker đã xoá activeStep; lỗi công cụ hay
 * nguồn AI ở cùng bước vẫn là sự cố, không phải quyền bỏ qua kiểm định.
 */
export function jobStage(job: { stageGroup: string; activeStep: string | null; steps?: Pick<JobStep, 'step' | 'status' | 'error'>[] | null }): JobStage {
  switch (job.stageGroup) {
    case 'paused': return 'paused'
    case 'review': return 'review'
    case 'ready':
    case 'posted': return 'done'
    case 'queued': return 'queued'
    case 'attention': {
      const blocked = job.steps?.find(step => (step.status === 'failed' || step.status === 'waiting') && step.error)
      return blocked
        ? (blocked.step === 'score' || blocked.step === 'export') && blocked.error?.code === 'quality_gate' ? 'quality' : 'issue'
        : job.activeStep === 'score' ? 'quality' : 'issue'
    }
    case 'working': return job.activeStep === 'build' || job.activeStep === 'export' ? 'render' : 'images'
    default: return 'images'
  }
}

/**
 * Câu lý do thật của video Cần xử lý: đúng bước mà máy chủ dùng để soạn dòng
 * phụ bên trái (`attentionSubline` — bước hỏng hoặc đang đỗ có `error.message`).
 * Băng cảnh báo nhờ vậy nói cùng nguyên nhân với dòng đang chọn, không phải
 * câu chung của mọi sự cố, và đổi video là đổi theo video đó. Câu chữ vẫn do
 * `progressText` chọn theo mã lỗi nên không hiện `error.message` thô.
 */
export function issueReason(job: { subline: string; steps: Pick<JobStep, 'step' | 'status' | 'progress' | 'error'>[] | null }): string {
  const step = job.steps?.find(item => (item.status === 'failed' || item.status === 'waiting') && item.error?.message)
  if (step) return progressText(step)
  return job.subline || 'Video cần bạn xử lý.'
}

/** Tên người làm từng bước của máy (01-ia-flows: 9 bước nói thành 4 trợ lý). */
const STEP_AGENT = {
  download: 'scout', backdrop: 'scout', transcript: 'scout',
  metadata: 'director', scenes: 'director',
  images: 'artist',
  build: 'qa', score: 'qa', export: 'qa',
} as const
export type StepAgent = (typeof STEP_AGENT)[keyof typeof STEP_AGENT]

/** Ngưỡng cổng xuất của Kiểm định: `internal/domain/quality.MinScore` (90). */
export const QUALITY_MIN_SCORE = 90

export function stepAgent(step: string): StepAgent | undefined {
  return STEP_AGENT[step as keyof typeof STEP_AGENT]
}

/** Giây → “0:41”, “42:10” cho badge thời lượng. */
export function clockLabel(seconds: number): string {
  return `${Math.floor(seconds / 60)}:${String(Math.floor(seconds % 60)).padStart(2, '0')}`
}

/** Giây → “41 giây”, “1 phút”, “10 phút 31 giây”; bỏ phần 0. */
export function durationLabel(seconds: number): string {
  if (!Number.isFinite(seconds) || seconds < 0) return ''
  const whole = Math.round(seconds)
  if (whole < 60) return `${whole} giây`
  const minutes = Math.floor(whole / 60)
  const rest = whole % 60
  return rest ? `${minutes} phút ${rest} giây` : `${minutes} phút`
}

/** Thời lượng thật của một bước: đã xong thì finishedAt, đang chạy thì tới hiện tại. */
export function stepDuration(step: { startedAt: string | null; finishedAt: string | null; status: string }, now = Date.now()): string {
  if (!step.startedAt) return ''
  const start = Date.parse(step.startedAt)
  if (Number.isNaN(start)) return ''
  const end = step.finishedAt ? Date.parse(step.finishedAt) : step.status === 'running' || step.status === 'waiting' ? now : null
  if (end === null || Number.isNaN(end)) return ''
  return durationLabel((end - start) / 1000)
}

/** “0:03:12” như cột giờ của Nhật ký; rỗng khi server chưa có mốc. */
export function stepClock(iso: string | null): string {
  if (!iso) return ''
  const date = new Date(iso)
  return Number.isNaN(date.getTime()) ? '' : date.toLocaleTimeString('vi-VN', { hour12: false })
}

/** Tỉ lệ phần trăm thật của bước dựng/xuất; null khi server chưa có tổng. */
export function stepPercent(step: { status: string; progress: { unitsDone: number; unitsTotal: number | null } }): number | null {
  if (!step.progress.unitsTotal) return step.status === 'succeeded' ? 100 : null
  return Math.min(100, Math.round(step.progress.unitsDone / step.progress.unitsTotal * 100))
}

/** Bản MP4 của video đã xong: đường dẫn phát được + số đo thật của file. */
export type FinishedVideo = {
  url: string
  /** Thời lượng thật của bản xuất (máy chủ ghi `durationMs`); null khi chỉ còn artifact cũ. */
  durationSec: number | null
  /** Dung lượng file máy chủ ghi; null khi chưa có. */
  bytes: number | null
}

const nonEmpty = (value: unknown) => typeof value === 'string' && value !== '' ? value : ''
const positive = (value: unknown) => typeof value === 'number' && Number.isFinite(value) && value > 0 ? value : null

type FinishedJob = { id: string; steps?: Pick<JobStep, 'step' | 'status' | 'output'>[] | null }

/**
 * File cuối của video đã xuất xong theo đúng dữ liệu thật của job.
 *
 * Pipeline hiện tại công bố MP4 thành bản xuất của editor và chỉ để lại
 * `output.data { projectId, exportId, fileName, size, durationMs, removedAt }`,
 * không có artifact `video/mp4` — nên chỉ tìm artifact sẽ báo "file không còn"
 * dù bản xuất vẫn nằm trên máy chủ. Job cũ có artifact thì nhánh đó giữ nguyên.
 * Bản xuất đã xoá (`removedAt`) hoặc bước chưa xong không được hiện file.
 */
export function finishedVideo(job: FinishedJob): FinishedVideo | null {
  const steps = job.steps ?? []
  const step = steps.find(item => item.step === 'export')
  const data = step?.output?.data
  if (!step?.output) return null
  // `removedAt` chỉ được ghi sau khi máy chủ xoá file; `accepted` là dấu "Vẫn xuất"
  // của chủ (07-data-contract), không phải dấu file tồn tại.
  if (nonEmpty(data?.removedAt)) return null
  const durationMs = positive(data?.durationMs)
  const size = positive(data?.size)
  const measured = { durationSec: durationMs === null ? null : durationMs / 1000, bytes: size }
  const artifact = step.output.artifacts?.find(item => item.contentType === 'video/mp4')
  if (artifact) return { url: `/api/clone/${encodeURIComponent(job.id)}/artifacts/${encodeURIComponent(artifact.id)}`, ...measured }
  if (step.status !== 'succeeded') return null
  // Bản xuất thật do editor công bố: `projectId` của bước xuất, thiếu thì lấy của bước dựng.
  const projectId = nonEmpty(data?.projectId) || nonEmpty(steps.find(item => item.step === 'build')?.output?.data?.projectId)
  const exportId = nonEmpty(data?.exportId)
  if (!projectId || !exportId) return null
  // Cùng đường dẫn editor dùng để phát/tải bản xuất (storage-backend `exportUrl`).
  return { url: `/api/editor/projects/${encodeURIComponent(projectId)}/exports/${encodeURIComponent(exportId)}`, ...measured }
}
