// Tông chấm cảnh báo theo `kind` của /api/status.attention (hỏi đáp #30).
// Bảng kind → tông nằm ở web; kind mới chưa có trong bảng mặc định là cam.
//   tool_failed, login_expired → cam (--color-err = --vv-danger)
//   tool_update                → xanh (--vv-info)
//   quota_low                  → vàng (--color-warn, cùng giá trị --vv-st-review)
export type AttentionTone = 'err' | 'warn' | 'info'

const TONE: Record<string, AttentionTone> = {
  tool_failed: 'err',
  login_expired: 'err',
  tool_update: 'info',
  quota_low: 'warn',
}

/** Mức ưu tiên khi nhiều việc dồn vào một chấm: cam > vàng > xanh. */
const RANK: Record<AttentionTone, number> = { err: 3, warn: 2, info: 1 }

export const attentionTone = (kind: string): AttentionTone => TONE[kind] ?? 'err'

/** Tông của một chấm gộp nhiều kind: kind khẩn nhất thắng; rỗng → không chấm. */
export function strongestTone(kinds: Iterable<string>): AttentionTone | undefined {
  let best: AttentionTone | undefined
  for (const kind of kinds) {
    const tone = attentionTone(kind)
    if (!best || RANK[tone] > RANK[best]) best = tone
  }
  return best
}
