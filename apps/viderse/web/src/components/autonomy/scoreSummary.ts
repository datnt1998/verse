// Tóm tắt điểm Kiểm định của một video chờ duyệt (Review › score=true).
// Tính từ `score` của từng ảnh với ngưỡng cố định85 — không có số bịa:
// ảnh chưa có điểm (đang vẽ, đang chấm, chấm lỗi) không tính vào "đã chấm".
import { APPROVE_DEFAULT, readImageScore, type ImageScore } from '../../api/autonomy'

export type ScoreItem = { id: string; label: string; drawing: boolean; score: unknown }
export type BelowImage = { id: string; label: string; value: number }
export type ScoreSummary = {
  /** số ảnh đã có điểm */
  scored: number
  passed: number
  below: BelowImage[]
  /** ảnh chưa có điểm và không đang vẽ lại */
  pending: number
  min: number
}

export const isLow = (score: ImageScore | null, min: number): boolean => score !== null && score.value < min

/** Điểm hiện khi ít nhất một ảnh có điểm và kênh duyệt theo điểm hoặc có ảnh dưới ngưỡng. */
export function summarizeScores(items: ScoreItem[], autoApprove: boolean): ScoreSummary | null {
  const min = APPROVE_DEFAULT
  const read = items.map((item) => ({ item, score: readImageScore(item.score) }))
  const scored = read.filter((row) => row.score !== null)
  if (scored.length === 0) return null
  const below = scored
    .filter((row) => isLow(row.score, min))
    .map((row) => ({ id: row.item.id, label: row.item.label, value: row.score?.value ?? 0 }))
    .sort((a, b) => a.value - b.value)
  if (!autoApprove && below.length === 0) return null
  return {
    scored: scored.length,
    passed: scored.length - below.length,
    below,
    pending: read.filter((row) => row.score === null && !row.item.drawing).length,
    min,
  }
}
