// Số liệu đầu kênh (artboard Channel › 4 thẻ): đếm từ danh sách video của
// kênh và tình trạng Tự lấy. Hàm thuần để test; TSX chỉ kết xuất.
import type { ChannelJob } from '../../api/channelVideos'
import { t } from '../../i18n/channels'

const pad = (n: number) => String(n).padStart(2, '0')
const parsed = (iso: string | null | undefined) => {
  const d = iso ? new Date(iso) : null
  return d && !Number.isNaN(d.getTime()) ? d : null
}
/** “26/10” theo giờ máy; tự ghép để dấu phân cách không phụ thuộc ICU của từng môi trường. */
const fmtDay = (iso: string | null | undefined) => { const d = parsed(iso); return d ? `${pad(d.getDate())}/${pad(d.getMonth() + 1)}` : '' }
/** “23:41” theo giờ máy. */
const fmtTime = (iso: string | null | undefined) => { const d = parsed(iso); return d ? `${pad(d.getHours())}:${pad(d.getMinutes())}` : '' }
const stamp = (iso: string | null | undefined): number | null => {
  if (!iso) return null
  const value = new Date(iso).getTime()
  return Number.isNaN(value) ? null : value
}
const startOfDay = (value: number | Date) => { const d = new Date(value); d.setHours(0, 0, 0, 0); return d.getTime() }

const tt = t.channels.stats

export type Stat = { key: string; label: string; value: number; sub: string; tone: 'neutral' | 'warn' }

/** 4 số liệu đầu kênh: Video đã làm · Đã đăng · Chờ bạn · Chờ lượt (artboard). */
export function channelStats(
  jobs: readonly ChannelJob[],
  ctx: { createdAt: string; youtubeUrl: string | null | undefined; autoFetch: boolean; nextEta: string | null; now: number },
): Stat[] {
  const s = tt
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
    : ctx.autoFetch ? s.queuedAutoOn : s.queuedAutoOff
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
