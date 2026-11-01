// Luật thuần của Bàn dựng điện thoại (artboard MMain): tiến độ ngày, phạm vi
// kênh, nhãn/trạng thái nhóm video và chọn phiên bản ảnh. Không React để test
// được số liệu thật mà không dựng cả màn.
import type { CloneJobView, JobImage } from '../../../api/workbench'
import type { ChannelSummary } from '../../../api/workbenchChannels'

export type DayProgress = {
  done: number
  cap: number
  /** 0..1 cho MeterBar; cap = 0 thì 0. */
  pct: number
  /** Số kênh bật tự lấy video trong phạm vi. */
  autoOn: number
  /** Số kênh trong phạm vi. */
  autoTotal: number
  label: string
  autoLine: string
}

/** Tiến độ hôm nay của đúng phạm vi đang xem (mọi kênh = cộng các kênh). */
export function dayProgress(channels: readonly ChannelSummary[], scope: string): DayProgress {
  const picked = scope === 'all' ? channels : channels.filter((channel) => channel.channelId === scope)
  const done = picked.reduce((sum, channel) => sum + channel.today.done, 0)
  const cap = picked.reduce((sum, channel) => sum + channel.today.cap, 0)
  const autoOn = picked.filter((channel) => channel.autoFetch).length
  const autoTotal = picked.length
  const single = scope !== 'all' ? picked[0] : undefined
  return {
    done,
    cap,
    pct: cap > 0 ? Math.min(1, done / cap) : 0,
    autoOn,
    autoTotal,
    label: `${done}/${cap} video hôm nay`,
    autoLine: single
      ? single.autoFetch ? 'Tự lấy video bật' : 'Tự lấy video tắt'
      : autoTotal ? `Tự lấy video · ${autoOn}/${autoTotal} kênh` : 'Chưa có kênh',
  }
}

/** Tên phạm vi: "Mọi kênh" hoặc tên kênh thật; kênh không còn trong summary → id. */
export function scopeName(channels: readonly ChannelSummary[], scope: string): string {
  if (scope === 'all') return 'Mọi kênh'
  return channels.find((channel) => channel.channelId === scope)?.name ?? scope
}

export type StageTone = 'queued' | 'paused' | 'working' | 'review' | 'issue' | 'ready' | 'posted'

/** Nhãn nhóm video dùng chung với cột trái máy tính (groupJobs). */
export const groupLabel: Record<string, string> = {
  waiting: 'Chờ bạn',
  working: 'Đang làm',
  ready: 'Sẵn sàng đăng',
  queued: 'Chờ lượt',
  posted: 'Đã đăng',
}

/** Trạng thái từng video → chấm/màu của StatusPill (kit). */
export function stageTone(group: string, stage?: string): StageTone {
  if (group === 'waiting') return stage === 'paused' ? 'paused' : 'review'
  if (group === 'working') return 'working'
  if (group === 'ready') return 'ready'
  if (group === 'queued') return 'queued'
  if (group === 'posted') return 'posted'
  return stage === 'attention' ? 'issue' : 'queued'
}

/** Trạng thái trực tiếp của một video (stageGroup của job) → StatusPill. */
export function stageToneOf(stage: string): StageTone {
  if (stage === 'attention' || stage === 'review') return 'review'
  if (stage === 'paused') return 'paused'
  if (stage === 'working') return 'working'
  if (stage === 'ready') return 'ready'
  if (stage === 'posted') return 'posted'
  return 'queued'
}

/** Dòng trạng thái ngắn của một video trong danh sách. */
export function jobStatusLine(job: CloneJobView): string {
  const running = job.agentView?.find((agent) => agent.state === 'running')
  if (running?.now) return running.now
  if (job.stageGroup === 'attention') return 'Cần bạn xử lý'
  if (job.stageGroup === 'review') return 'Ảnh sẵn sàng để duyệt'
  if (job.stageGroup === 'paused') return 'Đang tạm dừng'
  return job.subline
}

/** Bước đang chạy của job → tiến độ cho MeterBar; không có thì null. */
export function runningProgress(job: CloneJobView): { done: number; total: number; pct: number } | null {
  const step = job.steps?.find((item) => item.status === 'running')
  if (!step) return null
  const total = step.progress.unitsTotal ?? 0
  const done = step.progress.unitsDone
  if (total <= 0) return null
  return { done, total, pct: Math.min(1, done / total) }
}

/** Phiên bản đang hiển thị của một ảnh: ưu tiên bản được chọn, rồi bản duyệt, rồi bản hiện tại. */
export function imageVersion(image: JobImage, chosen?: number): number | undefined {
  if (chosen !== undefined && image.versions?.some((version) => version.version === chosen)) return chosen
  return image.reviewVersion ?? image.currentVersion ?? image.versions?.[0]?.version
}

/** URL ảnh của phiên bản đang xem; server đã trả URL tuyệt đối trong versions[]. */
export function imageUrl(image: JobImage, chosen?: number): string | undefined {
  const version = imageVersion(image, chosen)
  if (version === undefined) return undefined
  return image.versions?.find((item) => item.version === version)?.originalUrl
}

/** Prompt đúng phiên bản; không gán prompt hiện tại cho ảnh lịch sử chưa lưu prompt. */
export function imagePrompt(image: JobImage, chosen?: number): string | undefined {
  const version = imageVersion(image, chosen)
  const saved = image.versions?.find((item) => item.version === version)?.prompt
  return saved || (version === image.currentVersion ? image.prompt || undefined : undefined)
}

/** Nhãn ảnh: không đưa mã cảnh nội bộ (ví dụ sc-f90) ra giao diện. */
export function imageLabel(image: JobImage, index: number): string {
  if (image.kind === 'thumbnail' || image.kind === 'cover') return 'Ảnh bìa'
  if (image.sceneId && /^[1-9]\d*$/.test(image.sceneId)) return `Cảnh ${image.sceneId}`
  return `Cảnh ${index + 1}`
}

/** Khoảng thời gian Bản tin: "từ 21:00 tới 06:00" theo giờ máy. */
export function recapWindow(from: string, to: string): string {
  const time = (value: string) => {
    const date = new Date(value)
    return Number.isFinite(date.getTime()) ? date.toLocaleTimeString('vi-VN', { hour: '2-digit', minute: '2-digit' }) : '—'
  }
  return `từ ${time(from)} tới ${time(to)}`
}

/** mm:ss cho thời lượng nguồn. */
export function durationLabel(seconds: number | null | undefined): string {
  if (seconds == null || !Number.isFinite(seconds) || seconds <= 0) return ''
  return `${Math.floor(seconds / 60)}:${String(Math.floor(seconds % 60)).padStart(2, '0')}`
}
