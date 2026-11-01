// Logic thuần của Kênh › Video: nhóm bộ lọc, đếm nút hàng loạt, chọn khoảng,
// số liệu đầu kênh và nội dung từng hàng. Không chạm DOM/mạng để test được.
import type { VideoStatus } from '../../components/StatusLabel'
import type { BulkEligibility, ChannelJob } from '../../api/channelVideos'
import { t } from '../../i18n/channels'
import { fmtDuration, fmtScore } from '../explore/fmt'

const pad = (n: number) => String(n).padStart(2, '0')
const parsed = (iso: string | null | undefined) => {
  const d = iso ? new Date(iso) : null
  return d && !Number.isNaN(d.getTime()) ? d : null
}
/** “26/10” theo giờ máy; tự ghép để dấu phân cách không phụ thuộc ICU của từng môi trường. */
const fmtDay = (iso: string | null | undefined) => { const d = parsed(iso); return d ? `${pad(d.getDate())}/${pad(d.getMonth() + 1)}` : '' }
/** “23:41” theo giờ máy. */
const fmtTime = (iso: string | null | undefined) => { const d = parsed(iso); return d ? `${pad(d.getHours())}:${pad(d.getMinutes())}` : '' }

const tt = t.channels.videos

export type FilterId = 'all' | 'working' | 'waiting' | 'queued' | 'ready' | 'posted'
export const FILTER_IDS: readonly FilterId[] = ['all', 'working', 'waiting', 'queued', 'ready', 'posted']

/** Chờ bạn = Chờ duyệt + Cần xử lý + Tạm dừng (ia-flows §3.1). */
const GROUP_OF: Record<string, Exclude<FilterId, 'all'>> = {
  working: 'working',
  review: 'waiting',
  attention: 'waiting',
  paused: 'waiting',
  queued: 'queued',
  ready: 'ready',
  posted: 'posted',
}

export const groupOf = (job: Pick<ChannelJob, 'stageGroup'>): Exclude<FilterId, 'all'> | null => GROUP_OF[job.stageGroup] ?? null

export function countByFilter(jobs: readonly Pick<ChannelJob, 'stageGroup'>[]): Record<FilterId, number> {
  const counts: Record<FilterId, number> = { all: jobs.length, working: 0, waiting: 0, queued: 0, ready: 0, posted: 0 }
  for (const job of jobs) {
    const group = groupOf(job)
    if (group) counts[group]++
  }
  return counts
}

/** Thứ tự bảng (artboard): việc đang chạy trước, rồi chờ bạn, chờ lượt, sẵn sàng, đã đăng cuối. */
const DISPLAY_RANK: Record<string, number> = { working: 0, review: 1, paused: 2, attention: 3, queued: 4, ready: 5, posted: 6 }
const rankOf = (job: Pick<ChannelJob, 'stageGroup'>) => DISPLAY_RANK[job.stageGroup] ?? DISPLAY_RANK.posted + 1

/** Lọc theo nhóm rồi xếp theo thứ tự bảng; trong cùng nhóm giữ nguyên thứ tự máy chủ trả về. */
export function filterJobs<T extends Pick<ChannelJob, 'stageGroup'>>(jobs: readonly T[], filter: FilterId): T[] {
  const kept = filter === 'all' ? [...jobs] : jobs.filter((job) => groupOf(job) === filter)
  return kept.sort((a, b) => rankOf(a) - rankOf(b))
}

/** Trạng thái hiển thị của một job (kit StatusLabel): attention → “Cần xử lý”. */
export function statusOf(job: Pick<ChannelJob, 'stageGroup'>): VideoStatus {
  return job.stageGroup === 'attention' ? 'issue' : (job.stageGroup as VideoStatus)
}

// ─── Chọn nhiều ────────────────────────────────────────────────────────

export type BulkCounts = { retry: number; approve: number; bundle: number; markPosted: number; cancel: number; delete: number }

/** Mỗi nút đếm số video hợp lệ trong số đã chọn; hợp lệ do server tính (`bulkEligibility`). */
export function bulkCounts(selected: ReadonlySet<string>, eligibility: BulkEligibility): BulkCounts {
  const count = (ids: readonly string[]) => ids.reduce((n, id) => n + (selected.has(id) ? 1 : 0), 0)
  return {
    retry: count(eligibility.retry),
    approve: count(eligibility.approve),
    bundle: count(eligibility.bundle),
    markPosted: count(eligibility.markPosted),
    cancel: count(eligibility.cancel),
    delete: count(eligibility.delete),
  }
}

/** Id hợp lệ trong số đã chọn, giữ thứ tự của danh sách hợp lệ. */
export const eligibleSelected = (selected: ReadonlySet<string>, ids: readonly string[]) => ids.filter((id) => selected.has(id))

/** Shift-click: mọi id từ mốc tới đích theo thứ tự đang hiện (cả hai đầu). Không mốc → chỉ đích. */
export function rangeIds(order: readonly string[], anchor: string | null, target: string): string[] {
  const to = order.indexOf(target)
  if (to < 0) return []
  const from = anchor === null ? -1 : order.indexOf(anchor)
  if (from < 0) return [target]
  const [low, high] = from <= to ? [from, to] : [to, from]
  return order.slice(low, high + 1)
}

export type Selection = { active: boolean; picked: ReadonlySet<string>; anchor: string | null }
export const NO_SELECTION: Selection = { active: false, picked: new Set(), anchor: null }

/**
 * Bấm ô chọn. Shift + có mốc: chọn/bỏ cả khoảng theo trạng thái mới của ô vừa bấm
 * (kiểu Gmail); không Shift: đảo một ô. Mốc luôn dời tới ô vừa bấm.
 */
export function toggleSelection(state: Selection, id: string, shift: boolean, order: readonly string[]): Selection {
  const picked = new Set(state.picked)
  const turnOn = !picked.has(id)
  const ids = shift && state.anchor !== null ? rangeIds(order, state.anchor, id) : [id]
  for (const item of ids) {
    if (turnOn) picked.add(item)
    else picked.delete(item)
  }
  return { active: true, picked, anchor: id }
}

/** Bỏ khỏi lựa chọn các id không còn trong danh sách (video bị xoá khi đang chọn). */
export function pruneSelection(state: Selection, existing: readonly string[]): Selection {
  if (state.picked.size === 0) return state
  const alive = new Set(existing)
  const picked = new Set([...state.picked].filter((id) => alive.has(id)))
  if (picked.size === state.picked.size) return state
  return { ...state, picked, anchor: state.anchor !== null && alive.has(state.anchor) ? state.anchor : null }
}

// ─── Số liệu đầu kênh ──────────────────────────────────────────────────

export type Stat = { key: 'made' | 'posted' | 'waiting' | 'queued'; label: string; value: number; sub: string; tone: 'neutral' | 'warn' }

const startOfDay = (value: number | Date) => { const d = new Date(value); d.setHours(0, 0, 0, 0); return d.getTime() }

export function channelStats(
  jobs: readonly ChannelJob[],
  ctx: { createdAt: string; youtubeUrl: string | null | undefined; autoPick: boolean; nextEta: string | null; now: number },
): Stat[] {
  const s = tt.stats
  const postedAt = jobs.filter((job) => job.stageGroup === 'posted').map((job) => stamp(job.postedAt) ?? stamp(job.updatedAt)).filter((v): v is number => v !== null)
  const last = postedAt.length ? Math.max(...postedAt) : null
  const counts = { review: 0, attention: 0, paused: 0, posted: postedAt.length, queued: 0 }
  for (const job of jobs) {
    if (job.stageGroup === 'review') counts.review++
    else if (job.stageGroup === 'attention') counts.attention++
    else if (job.stageGroup === 'paused') counts.paused++
    else if (job.stageGroup === 'queued') counts.queued++
  }
  const waiting = counts.review + counts.attention + counts.paused
  const waitingSub = [
    counts.review ? s.waitingReview(counts.review) : '',
    counts.attention ? s.waitingIssue(counts.attention) : '',
    counts.paused ? s.waitingPaused(counts.paused) : '',
  ].filter(Boolean).join(' · ')
  const createdAt = stamp(ctx.createdAt)
  const madeSub = createdAt !== null && startOfDay(createdAt) === startOfDay(ctx.now) ? s.madeToday : s.madeSince(fmtDay(ctx.createdAt))
  const queuedSub = counts.queued > 0
    ? (ctx.nextEta ? s.queuedNext(fmtTime(ctx.nextEta)) : s.queuedNoEta)
    : ctx.autoPick ? s.queuedAutoOn : s.queuedAutoOff
  return [
    { key: 'made', label: s.made, value: jobs.length, sub: madeSub, tone: 'neutral' },
    {
      key: 'posted', label: s.posted, value: counts.posted, tone: 'neutral',
      sub: last !== null ? s.postedLast(fmtDay(new Date(last).toISOString())) : ctx.youtubeUrl ? s.postedNone : s.postedNoYoutube,
    },
    { key: 'waiting', label: s.waiting, value: waiting, sub: waiting > 0 ? waitingSub : s.waitingNone, tone: waiting > 0 ? 'warn' : 'neutral' },
    { key: 'queued', label: s.queued, value: counts.queued, sub: queuedSub, tone: 'neutral' },
  ]
}

function stamp(iso: string | null | undefined): number | null {
  if (!iso) return null
  const value = new Date(iso).getTime()
  return Number.isNaN(value) ? null : value
}

// ─── Hàng của bảng ─────────────────────────────────────────────────────

export type RowAction = 'open' | 'review' | 'fix' | 'pack'

/** Nút “Việc tiếp”: theo trạng thái (ia-flows §3.1). Chờ lượt/Đã đăng không có nút trên dòng. */
export function rowAction(job: Pick<ChannelJob, 'stageGroup' | 'id'>, bundleIds: ReadonlySet<string>): RowAction | null {
  switch (job.stageGroup) {
    case 'working': case 'paused': return 'open'
    case 'review': return 'review'
    case 'attention': return 'fix'
    case 'ready': return bundleIds.has(job.id) ? 'pack' : 'open'
    default: return null
  }
}

export function agoText(ms: number): string {
  const minutes = Math.floor(ms / 60_000)
  if (minutes < 1) return tt.list.agoNow
  if (minutes < 60) return tt.list.agoMinutes(minutes)
  const hours = Math.floor(minutes / 60)
  if (hours < 24) return tt.list.agoHours(hours)
  return tt.list.agoDays(Math.floor(hours / 24))
}

const sizeFormat = new Intl.NumberFormat('vi-VN')
const mbText = (bytes: number | null | undefined) => (bytes && bytes > 0 ? `${sizeFormat.format(Math.max(1, Math.round(bytes / (1024 * 1024))))} MB` : '')

function stepData(job: ChannelJob, step: string, key: string): number | null {
  const value = job.steps?.find((item) => item.step === step)?.output?.data?.[key]
  return typeof value === 'number' ? value : null
}

function lastFinished(job: ChannelJob): number | null {
  let latest: number | null = null
  for (const step of job.steps ?? []) {
    const at = stamp(step.finishedAt)
    if (at !== null && (latest === null || at > latest)) latest = at
  }
  return latest
}

/** Lý do ngắn của video Cần xử lý: câu lỗi của bước hỏng/đang chờ, không có thì dòng phụ của server. */
function issueReason(job: ChannelJob): string {
  const failed = job.steps?.find((step) => (step.status === 'failed' || step.status === 'waiting') && step.error?.message)
  return failed?.error?.message || job.subline
}

/** Dòng phụ dưới nhãn trạng thái (ia-flows §3.1). `posted` = chuỗi 7 ngày của `postedDetail` (đã tính ngoài). */
export function statusDetail(job: ChannelJob, ctx: { now: number; etaStart?: string | null; posted?: string | null }): string {
  switch (job.stageGroup) {
    case 'review': {
      const images = stepData(job, 'images', 'imageCount')
      const at = lastFinished(job)
      return images && at !== null ? tt.list.imagesAgo(images, agoText(ctx.now - at)) : job.subline
    }
    case 'attention': return issueReason(job)
    case 'ready': {
      const dur = job.source.durationSec != null ? fmtDuration(job.source.durationSec) : ''
      return dur ? tt.list.mp4(dur, mbText(job.exportBytes)) : job.subline
    }
    case 'queued': return ctx.etaStart ? tt.list.queueDetail(fmtEtaText(ctx.etaStart, ctx.now)) : job.subline
    case 'posted': return ctx.posted ?? tt.list.postedOn(fmtDay(job.postedAt ?? job.updatedAt))
    default: return job.subline
  }
}

function fmtEtaText(iso: string, now: number): string {
  const at = stamp(iso)
  if (at === null) return ''
  return startOfDay(at) === startOfDay(now) ? fmtTime(iso) : `${fmtDay(iso)} ${fmtTime(iso)}`
}

/** “Nocturne Stories · ×4,2 · 23:41”: kênh nguồn · điểm nổi · giờ (hôm nay) hoặc ngày; chờ lượt chưa có mốc. */
export function sourceLine(job: ChannelJob, now: number): string {
  const at = job.stageGroup === 'queued' ? null
    : job.stageGroup === 'posted' ? (job.postedAt ?? job.updatedAt)
      : job.stageGroup === 'ready' ? (job.updatedAt)
        : job.createdAt
  const when = stamp(at) === null ? '' : startOfDay(stamp(at)!) === startOfDay(now) ? fmtTime(at) : fmtDay(at)
  return [job.source.channelTitle ?? '', job.source.outlier != null ? fmtScore(job.source.outlier) : '', when].filter(Boolean).join(' · ')
}
