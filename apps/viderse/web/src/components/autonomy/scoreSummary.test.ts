import { describe, expect, it } from 'vitest'
import { summarizeScores, type ScoreItem } from './scoreSummary'

const score = (value: number) => ({ value, parts: { match: 90, artifacts: 90, identity: 90, duplicate: 90 }, reason: { agent: 'qa', text: 't', facts: [] } })
const item = (id: string, value: number | null, drawing = false): ScoreItem => ({ id, label: `Cảnh ${id}`, drawing, score: value === null ? null : score(value) })
const ask = { approve: { mode: 'ask' as const, min: 85 } }
const byScore = { approve: { mode: 'score' as const, min: 85 } }

describe('summarizeScores', () => {
  it('không hiện khi chưa ảnh nào có điểm, kể cả kênh duyệt theo điểm', () => {
    expect(summarizeScores([item('1', null), item('2', null)], byScore)).toBeNull()
  })

  it('kênh luôn hỏi và mọi ảnh đạt: không hiện điểm', () => {
    expect(summarizeScores([item('1', 92), item('2', 90)], ask)).toBeNull()
  })

  it('kênh luôn hỏi nhưng có ảnh dưới ngưỡng: hiện, ảnh thấp nhất đứng đầu', () => {
    const summary = summarizeScores([item('1', 92), item('4', 72), item('6', 80)], ask)
    expect(summary).toMatchObject({ scored: 3, passed: 1, min: 85 })
    expect(summary?.below.map((b) => [b.id, b.value])).toEqual([['4', 72], ['6', 80]])
  })

  it('ảnh đang vẽ lại không tính là đang chấm, ảnh chưa có điểm thì có', () => {
    const summary = summarizeScores([item('1', 90), item('2', null, true), item('3', null)], byScore)
    expect(summary).toMatchObject({ scored: 1, passed: 1, pending: 1 })
  })

  it('đúng ngưỡng là đạt', () => {
    expect(summarizeScores([item('1', 85)], byScore)?.below).toEqual([])
  })
})
