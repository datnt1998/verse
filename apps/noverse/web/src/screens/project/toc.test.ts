// @vitest-environment happy-dom
// Gói AG — mục lục cột trái: tìm không dấu + theo số, một tập mở một lúc,
// tập khoá không chọn được, nhớ tập theo truyện (localStorage), lazy >200 chương.
import { beforeEach, describe, expect, it } from 'vitest'
import type { Outline } from '../../api/types'
import {
  buildTocVolumes, defaultTocOpen, getTocOpen, mergeVolumeChapters, savedTocOpen, setTocOpen,
  stripAccents, tocHits, tocLazy, tocVolumeOf, volumeRange,
} from './toc'

type Ch = Outline['volumes'][number]['chapters'][number]
const ch = (n: number, title = `Chương ${n}`, status: Ch['status'] = 'pending'): Ch => ({
  chapter: n, title, coreEvent: '', summary: '', completed: status === 'done', words: 0, status, hasPlan: false, hasReview: false,
})
const span = (from: number, to: number) => Array.from({ length: to - from + 1 }, (_, i) => from + i)
const vol = (index: number, chapters: Ch[], arcRanges?: [number, number][]): Outline['volumes'][number] => ({
  index, title: `Tập ${index}`, summary: '',
  arcs: arcRanges
    ? arcRanges.map(([from, to], i) => ({ index: i + 1, title: '', summary: '', chapters: span(from, to) }))
    : chapters.length ? [{ index: 1, title: '', summary: '', chapters: chapters.map((c) => c.chapter) }] : [],
  chapters,
})

const OUTLINE_3V: Outline['volumes'] = [
  vol(1, Array.from({ length: 40 }, (_, i) => ch(i + 1, i === 5 ? 'Chiếc vỏ ổ cứng' : `Chương ${i + 1}`, i < 3 ? 'done' : i === 3 ? 'writing' : 'pending'))),
  vol(2, Array.from({ length: 40 }, (_, i) => ch(i + 41))),
  vol(3, Array.from({ length: 40 }, (_, i) => ch(i + 81, ''))),
]

describe('tìm chương (nav=search)', () => {
  const volumes = buildTocVolumes(OUTLINE_3V, 'Truyện', 4, '')

  it('bỏ dấu: “o cung” khớp “Chiếc vỏ ổ cứng”', () => {
    const hits = tocHits(volumes, 'o cung')
    expect(hits.map((h) => h.n)).toEqual([6])
    expect(hits[0].volumeLabel).toBe('Tập 1')
  })
  it('khớp chính xác số chương: “37” → chương 37, không khớp 137…', () => {
    const volumes120 = buildTocVolumes(OUTLINE_3V, 'Truyện', 0, '')
    expect(tocHits(volumes120, '37').map((h) => h.n)).toEqual([37])
    expect(tocHits(volumes120, '81').map((h) => h.n)).toEqual([81])
  })
  it('không khớp: trả rỗng để UI hiện “Không có chương nào khớp “…””', () => {
    expect(tocHits(volumes, 'xyz-123-abc')).toEqual([])
  })
  it('stripAccents chuẩn hoá đ/Đ và dấu thanh', () => {
    expect(stripAccents('Đường đội đất')).toBe('Duong doi dat')
  })
})

describe('một tập mở một lúc + tập khoá', () => {
  const volumes = buildTocVolumes(OUTLINE_3V, 'Truyện', 0, '')

  it('tập không có chương nào là khoá: kèm meta “mở khi Tập N-1 xong”', () => {
    const locked = vol(4, [])
    const rows = buildTocVolumes([...OUTLINE_3V, locked], 'Truyện', 0, '')
    const tail = rows[rows.length - 1]
    expect(tail.locked).toBe(true)
    expect(tail.meta).toContain('mở khi Tập')
  })
  it('savedTocOpen không mở tập khoá — rơi về tập đang viết/tập đầu mở được', () => {
    const rows = [...volumes, { index: 9, label: 'Tập 9', meta: '', live: false, locked: true, next: false, chapterCount: 0, from: 0, to: 0, chapters: [] }]
    expect(savedTocOpen(rows, 9)).toBe(1)
    expect(savedTocOpen(rows, 2)).toBe(2)
    expect(savedTocOpen(rows, 0)).toBeNull()
  })
  it('openVolume kiểu số duy nhất — UI chỉ render hàng của một tập', () => {
    // Tập hợp “mở” trong useToc là một số/null, không phải mảng.
    expect(savedTocOpen(volumes, 3)).toBe(3)
  })
})

describe('nhớ tập đang mở theo truyện (localStorage)', () => {
  beforeEach(() => localStorage.clear())
  it('ghi/đọc theo khoá project.story; truyện khác không lẫn', () => {
    setTocOpen('p', 's1', 2)
    setTocOpen('p', 's2', 3)
    setTocOpen('p', 's1', 0)
    expect(getTocOpen('p', 's1')).toBe(0)
    expect(getTocOpen('p', 's2')).toBe(3)
    expect(getTocOpen('p', 'khac')).toBeNull()
  })
  it('chưa lưu: mặc định mở tập đang viết, còn không thì tập đầu mở được', () => {
    const noLive = [vol(1, [ch(1), ch(2)]), vol(2, [ch(41), ch(42)])]
    const rows = buildTocVolumes(noLive, 'Truyện', 42, '')
    expect(defaultTocOpen(rows)).toBe(2)
    expect(defaultTocOpen(buildTocVolumes(noLive, 'Truyện', 0, ''))).toBe(1)
  })
})

describe('lazy trên 200 chương', () => {
  it('tocLazy chỉ bật khi > 200', () => {
    expect(tocLazy(200)).toBe(false)
    expect(tocLazy(201)).toBe(true)
  })
  it('mergeVolumeChapters chỉ điền chương của tập đã tải, giữ header các tập khác', () => {
    const headers: Outline['volumes'] = [
      vol(1, [], [[1, 40]]), // skeleton? không — arcs có số: tập đã bung nhưng chapters chưa tải
      vol(2, [], [[41, 80]]),
      vol(3, []),
    ]
    const patch = { volumes: [vol(2, Array.from({ length: 40 }, (_, i) => ch(i + 41)))] } as Outline
    const merged = mergeVolumeChapters(headers, patch)
    expect(merged[0].chapters).toHaveLength(0)
    expect(merged[1].chapters).toHaveLength(40)
    expect(merged[1].chapters[0].chapter).toBe(41)
    expect(merged[2].chapters).toHaveLength(0)
    // arcs vẫn đếm được để header hiện “40 ch.” và range cho Tới chương đang viết.
    expect(volumeRange(merged[0])).toEqual({ from: 1, to: 40 })
    expect(tocVolumeOf(merged, 55)).toBe(2)
    expect(tocVolumeOf(merged, 200)).toBe(0)
  })
})
