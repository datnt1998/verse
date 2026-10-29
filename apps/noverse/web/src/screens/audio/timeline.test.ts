import { describe, expect, it } from 'vitest'
import type { AudioTimelineChapter } from '../../api/types'
import { timelineSeconds, timelineMode, timelineIndex, timelineTotal, timelineTicks } from './timeline'

const chapter = (patch: Partial<AudioTimelineChapter> = {}): AudioTimelineChapter => ({ chapter: 1, title: 'Lên sóng', state: 'queued', sec: null, estSec: 1000, pct: 0, ...patch })

describe('Gói V duration strip', () => {
 it('gives missing and zero-second chapters their estimated width', () => {
  expect(timelineSeconds(chapter())).toBe(1000)
  expect(timelineSeconds(chapter({ sec: 0 }))).toBe(1000)
  expect(timelineSeconds(chapter({ sec: 980, state: 'done' }))).toBe(980)
 })
 it.each([[72, 'full'], [71.99, 'number'], [28, 'number'], [27.99, 'bar']])('switches at measured %s pixels', (width, mode) => {
  expect(timelineMode(Number(width))).toBe(mode)
 })
 it('moves with arrows and Home/End, clamped to the chapter strip', () => {
  expect(timelineIndex('ArrowRight', 1, 16)).toBe(2)
  expect(timelineIndex('ArrowLeft', 1, 16)).toBe(0)
  expect(timelineIndex('Home', 8, 16)).toBe(0)
  expect(timelineIndex('End', 8, 16)).toBe(15)
  expect(timelineIndex('ArrowRight', 15, 16)).toBe(15)
  expect(timelineIndex('ArrowLeft', 0, 16)).toBe(0)
  expect(timelineIndex('Tab', 2, 16)).toBeNull()
 })
 it('totals actual plus estimates and counts only finished chapters', () => {
  const chapters = [chapter({ state: 'done', sec: 600 }), chapter({ chapter: 2, state: 'rendering', pct: 50 }), chapter({ chapter: 3 })]
  expect(timelineTotal(chapters, 650)).toBe('đã tạo 0 giờ 10 phút / khoảng 0 giờ 43 phút · 1/3 chương')
  expect(timelineTotal([chapter({ state: 'done', sec: 600 })], 600)).toBe('đã tạo 0 giờ 10 phút · 1/1 chương')
 })
 it('uses 5/10/25 ruler steps, keeps endpoints, and replaces nearby ticks with the live label', () => {
  for (const [count, step] of [[40, 5], [120, 10], [200, 25]]) {
   const chapters = Array.from({ length: count }, (_, i) => chapter({ chapter: i + 1 }))
   expect(timelineTicks(chapters).map(tick => tick.index)).toEqual([0, ...Array.from({ length: Math.floor(count / step) }, (_, i) => (i + 1) * step - 1)])
  }
  const chapters = Array.from({ length: 40 }, (_, i) => chapter({ chapter: i + 1, state: i === 5 ? 'rendering' : 'queued', pct: 45 }))
  const ticks = timelineTicks(chapters)
  expect(ticks.find(tick => tick.index === 5)?.active).toBe(true)
  expect(ticks.some(tick => tick.index === 4)).toBe(false)
 })
})
