// Logic thuần của màn Video mới: tách URL từ ô nhập, gộp dòng không đọc
// được vào "lỗi", chọn ưu tiên pick|batch cho plan/enqueue, và soạn dòng
// kế hoạch (§4) cho phần dưới ô link.
import type { ClonePlan } from '../../api/clone'
import { t } from '../../i18n/newScreen'

/** Delay gọi POST /api/clone/plan sau lần đổi ô link cuối (V2 nghiệm thu). */
export const PLAN_DEBOUNCE_MS = 300

/** Tách ô nhập thành danh sách URL sạch (mỗi dòng/mỗi phần cách bởi
 *  whitespace là một URL; dán nhanh từ sheet vẫn đúng). */
export function parseUrls(text: string): string[] {
  return text
    .split(/\s+/)
    .map((s) => s.trim())
    .filter(Boolean)
}

/** Ưu tiên job: 1 link → pick (lên đầu); nhiều link → batch (theo nhịp). */
export const pickPriority = (urls: readonly string[]): 'pick' | 'batch' =>
  urls.length <= 1 ? 'pick' : 'batch'

/** Dòng kế hoạch dưới ô link — null khi chưa nên hiện gì (ô trống). */
export function planLine(plan: ClonePlan | null, opts: {
  urls: string[]
  running: number
  channelName: string | null
}): { text: string; detail: string | null } | null {
  const tp = t.screens.new.plan
  if (opts.urls.length === 0) return null
  if (!plan) return { text: '', detail: null }
  if (!plan.template.id) return { text: tp.blockedNoTemplate, detail: null }
  if (plan.accepted.length === 0) return null // toàn trùng/lỗi — banner đã nói
  // Phần plan báo có link bỏ qua: "N đã làm cho kênh này · sẽ bỏ qua" (+ "N
  // link lỗi" khi có link không đọc được) — nối sau dòng chính.
  const skips = [
    plan.duplicates.length > 0 ? tp.skipDup(plan.duplicates.length) : null,
    plan.invalid.length > 0 ? tp.skipInvalid(plan.invalid.length) : null,
  ].filter(Boolean).join(' · ') || null
  const single = plan.position === 'front'
  if (single) {
    // Máy rảnh: không có video nào đang chạy để chờ, nên nói thẳng là bấm được ngay.
    if (opts.running === 0) return { text: tp.startNow, detail: skips }
    const eta = plan.etaStart
    if (eta) {
      const time = new Date(eta).toLocaleTimeString('vi-VN', { hour: '2-digit', minute: '2-digit' })
      return {
        text: `${tp.queueHead} ${time} ${tp.queueHeadMid} ${opts.running || 1} ${tp.queueHeadEnd}`,
        detail: [tp.batchLeft(plan.paceLeftToday), skips].filter(Boolean).join(' · ') || null,
      }
    }
    return { text: tp.queueHead, detail: [tp.pickFallback, skips].filter(Boolean).join(' · ') || null }
  }
  // nhiều video → batch theo nhịp kênh
  const pace = plan.paceLeftToday
  const ch = opts.channelName ?? ''
  const text = ch ? tp.batchTail(pace, ch) : tp.channelTail(pace)
  const day = plan.overflowDays > 0 || pace <= 0 ? tp.batchOverflow : tp.batchLeft(pace)
  return { text, detail: [day, skips].filter(Boolean).join(' · ') || null }
}

