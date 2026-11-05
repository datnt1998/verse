// Điểm ảnh và tóm tắt Kiểm định do máy chủ trả; cấu hình kênh dùng autoApprove.
// Thiếu/sai hình dạng không được biến thành điểm đạt.
import { list, num, oneOf, record, str, ShapeError } from './guard'

const approveModes = ['ask', 'score', 'all'] as const
export const APPROVE_DEFAULT = 85

export type ImageScore = {
  value: number
  parts: { match: number; artifacts: number; identity: number; duplicate: number }
  reason: { agent: 'qa'; text: string; facts: { label: string; value: string; source: string }[] }
}

/** Tóm tắt điểm cả video do server tính (GET …/images › autonomy). */
export type ScoreSummary = {
  mode: (typeof approveModes)[number]
  min: number
  total: number
  passed: number
  pending: number
  below: { id: string; label: string; value: number }[]
} | null

/** Đọc `score` của một ảnh; thiếu/null/sai hình dạng → null (không hiện số bịa). */
export function readImageScore(value: unknown): ImageScore | null {
  try {
    const o = record(value, 'score')
    const parts = record(o.parts, 'score.parts')
    const reason = record(o.reason, 'score.reason')
    if (reason.agent !== 'qa') throw new ShapeError('score.reason.agent')
    const score = num(o.value, 'score.value')
    if (score < 0 || score > 100) throw new ShapeError('score.value')
    return {
      value: score,
      parts: {
        match: num(parts.match, 'score.parts.match'),
        artifacts: num(parts.artifacts, 'score.parts.artifacts'),
        identity: num(parts.identity, 'score.parts.identity'),
        duplicate: num(parts.duplicate, 'score.parts.duplicate'),
      },
      reason: {
        agent: 'qa',
        text: str(reason.text, 'score.reason.text'),
        facts: list(reason.facts, (entry, at) => {
          const fact = record(entry, at)
          return { label: str(fact.label, `${at}.label`), value: str(fact.value, `${at}.value`), source: str(fact.source, `${at}.source`) }
        }, 'score.reason.facts'),
      },
    }
  } catch {
    return null
  }
}

export function readScoreSummary(value: unknown): ScoreSummary {
  if (value == null) return null
  try {
    const o = record(value, 'summary')
    return {
      mode: oneOf(o.mode, approveModes, 'summary.mode'),
      min: num(o.min, 'summary.min'),
      total: num(o.total, 'summary.total'),
      passed: num(o.passed, 'summary.passed'),
      pending: num(o.pending, 'summary.pending'),
      below: list(o.below, (entry, at) => {
        const row = record(entry, at)
        return { id: str(row.id, `${at}.id`), label: str(row.label, `${at}.label`), value: num(row.value, `${at}.value`) }
      }, 'summary.below'),
    }
  } catch {
    return null
  }
}
