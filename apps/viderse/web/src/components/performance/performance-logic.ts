import type { Insight, Performance } from '../../api/performance'
import { performanceCopy, ratioText, viewsShort } from '../../i18n/performance'

const copy = performanceCopy

/** Thanh dài nhất vẽ 90% (artboard: --w: 90%), thanh kia tỉ lệ theo trung vị 7 ngày. */
export function barWidths(values: readonly number[]): number[] {
  const max = Math.max(...values, 0)
  if (max <= 0) return values.map(() => 0)
  return values.map((value) => Math.round((value / max) * 90))
}

/** Hệ số nhóm đầu so với nhóm sau (×1,4); undefined khi thiếu nhóm hoặc mẫu số 0. */
export function insightRatio(insight: Pick<Insight, 'groups'>): number | undefined {
  const [first, second] = insight.groups
  if (!first || !second || second.median7d <= 0) return undefined
  return first.median7d / second.median7d
}

const lowerFirst = (value: string) => value.charAt(0).toLowerCase() + value.slice(1)

/** Một mệnh đề ngắn cho dòng bằng chứng của thẻ gợi ý (“Mẫu chuẩn ×1,4 so với Cận cảnh”). */
export function insightEvidence(insight: Insight): string | undefined {
  const ratio = insightRatio(insight)
  const [first, second] = insight.groups
  if (ratio === undefined || !first || !second) return undefined
  const x = ratioText(ratio)
  if (insight.dimension === 'title_pattern') return `Tiêu đề dạng ${lowerFirst(first.label)} ${x}`
  if (insight.dimension === 'template') return `${first.label} ${x} so với ${second.label}`
  if (insight.dimension === 'length') return `Độ dài ${lowerFirst(first.label)} ${x} so với ${lowerFirst(second.label)}`
  return `Giờ đăng ${first.label} ${x} so với ${second.label}`
}

/** Dòng bằng chứng của thẻ mời xem: các mệnh đề cách nhau “ · ”, thêm “Sơ bộ” khi còn sơ bộ. */
export function teaserEvidence(insights: readonly Insight[]): string {
  const parts = insights.map(insightEvidence).filter((part): part is string => !!part)
  if (insights.some((item) => item.preliminary)) parts.push('Sơ bộ')
  return parts.join(' · ')
}

/** Dòng phụ của ô Trạng thái ở hàng Đã đăng: số 7 ngày nếu đã đọc, không thì mốc mới nhất; null khi chưa đọc được gì. */
export function postedDetail(perf: Performance | undefined, jobId: string): string | null {
  const row = perf?.posted.find((item) => item.jobId === jobId)
  if (!row) return null
  const pick = row.at7d
    ? { stat: row.at7d, window: '7 ngày' as const }
    : row.at24h ? { stat: row.at24h, window: '24 giờ' as const }
    : null
  if (!pick) return null
  const vs = pick.window === '7 ngày' && row.vsMedian7d != null ? ratioText(row.vsMedian7d) : undefined
  return copy.posted.detail(viewsShort(pick.stat.views), pick.window, vs)
}

/** Link YouTube hợp lệ: youtube.com/watch?v=…, /shorts/…, /live/…, youtu.be/…; rỗng = không có link. */
export function isYoutubeLink(raw: string): boolean {
  const value = raw.trim()
  if (!value) return true
  let url: URL
  try { url = new URL(/^[a-z]+:\/\//i.test(value) ? value : `https://${value}`) } catch { return false }
  const host = url.hostname.replace(/^(www|m|music)\./, '')
  if (host === 'youtu.be') return url.pathname.length > 1
  if (host !== 'youtube.com') return false
  if (url.pathname === '/watch') return !!url.searchParams.get('v')
  return /^\/(shorts|live|embed)\/[^/]+/.test(url.pathname)
}
