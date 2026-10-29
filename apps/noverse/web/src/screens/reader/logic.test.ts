import { describe, expect, it } from 'vitest'
import type { ChapterInfo, ForeshadowEntry, Outline } from '../../api/types'
import {
  exportChapterCount, groupChapters, latestExport, normalizeRange, relevantForeshadow,
  shouldReloadChapter, totalWords,
} from './logic'

const chapter = (n: number, volume: number, arc: number, words = 100): ChapterInfo => ({
  chapter: n, title: `Chương ${n}`, completed: true, words, hasDraft: false, volume, arc,
})

const outline: Outline = {
  layered: true, path: 'output/novel/outline.json',
  volumes: [{
    index: 1, title: 'Tập 1 · Khởi đầu', summary: '',
    arcs: [{ index: 1, title: 'Hồi 1 · Ký tên', summary: '', chapters: [1, 2] }, { index: 2, title: 'Hồi 2 · Lớn lên', summary: '', chapters: [3] }],
    chapters: [],
  }],
  nextVolumeTitle: '', compassDirection: '', compassScale: '',
}

describe('groupChapters', () => {
  it('nhóm theo volume/arc và lấy tiêu đề từ đại cương', () => {
    const groups = groupChapters([chapter(1, 1, 1), chapter(2, 1, 1), chapter(3, 1, 2), chapter(4, 2, 1)], outline)
    expect(groups.map((group) => group.volume)).toEqual([1, 2])
    expect(groups[0].title).toBe('Tập 1 · Khởi đầu')
    expect(groups[0].arcs.map((arc) => arc.title)).toEqual(['Hồi 1 · Ký tên', 'Hồi 2 · Lớn lên'])
    expect(groups[0].arcs[1].chapters.map((item) => item.chapter)).toEqual([3])
    expect(groups[1].title).toBe('Tập 2')
    expect(groups[1].arcs[0].title).toBe('Hồi 1')
  })

  it('giữ thứ tự chương trong hồi và cộng số chữ', () => {
    const list = [chapter(3, 1, 1, 50), chapter(1, 1, 1, 70), chapter(2, 1, 1, 30)]
    const groups = groupChapters(list, null)
    expect(groups[0].arcs[0].chapters.map((item) => item.chapter)).toEqual([3, 1, 2])
    expect(totalWords(list)).toBe(150)
  })
})

describe('relevantForeshadow', () => {
  const entries: ForeshadowEntry[] = [
    { id: 'a', description: 'A', planted_at: 1, status: 'planted' },
    { id: 'b', description: 'B', planted_at: 6, status: 'planted' },
    { id: 'c', description: 'C', planted_at: 2, status: 'resolved', resolved_at: 5 },
    { id: 'd', description: 'D', planted_at: 3, status: 'advanced' },
  ]

  it('giữ phục bút đã gieo trước chương, còn mở, và trả đúng chương', () => {
    expect(relevantForeshadow(entries, null, 5).map((entry) => entry.id)).toEqual(['a', 'c', 'd'])
  })

  it('giữ phục bút được facts nhắc tới kể cả chưa gieo, và đã trả đúng chương', () => {
    const facts = {
      title: '', summary: '', characters: [], key_events: [], timeline_events: [],
      foreshadow_updates: [{ id: 'b', action: 'plant' }, { id: 'c', action: 'resolve' }],
      relationship_changes: [], state_changes: [], cast_intros: [],
    }
    expect(relevantForeshadow(entries, facts, 5).map((entry) => entry.id)).toEqual(['a', 'b', 'c', 'd'])
  })

  it('bỏ phục bút đã trả ở chương khác, giữ phục bút gieo đúng chương', () => {
    expect(relevantForeshadow(entries, null, 6).map((entry) => entry.id)).toEqual(['a', 'b', 'd'])
  })

  it('phục bút đã bỏ chỉ hiện ở chương bỏ tuyến', () => {
    const dropped: ForeshadowEntry[] = [{ id: 'e', description: 'E', planted_at: 1, status: 'abandoned', abandoned_at: 4, abandon_reason: 'tác giả đổi hướng' }]
    expect(relevantForeshadow(dropped, null, 4).map((entry) => entry.id)).toEqual(['e'])
    expect(relevantForeshadow(dropped, null, 6)).toEqual([])
  })
})

describe('normalizeRange', () => {
  it('nhận khoảng hợp lệ trong tổng số chương', () => {
    expect(normalizeRange(1, 8, 8)).toEqual({ from: 1, to: 8 })
  })

  it('từ chối khoảng ngược, vượt tổng, không phải số nguyên và tổng rỗng', () => {
    expect(normalizeRange(5, 3, 8)).toBeNull()
    expect(normalizeRange(1, 9, 8)).toBeNull()
    expect(normalizeRange(1, Number.NaN, 8)).toBeNull()
    expect(normalizeRange(1, 1, 0)).toBeNull()
  })
})

describe('shouldReloadChapter (G10)', () => {
  it('tải lại khi revision đổi', () => {
    expect(shouldReloadChapter('mtime-1', 'mtime-2')).toBe(true)
    expect(shouldReloadChapter(null, 'mtime-1')).toBe(true)
  })

  it('không tải lại khi revision y hệt hoặc máy chủ cũ trả rỗng', () => {
    expect(shouldReloadChapter('mtime-1', 'mtime-1')).toBe(false)
    expect(shouldReloadChapter('mtime-1', '')).toBe(false)
    expect(shouldReloadChapter(null, '')).toBe(false)
    expect(shouldReloadChapter('mtime-1', undefined)).toBe(false)
  })
})

describe('exports', () => {
  it('chọn bản xuất mới nhất, bỏ audio', () => {
    const files = [
      { path: 'a.txt', name: 'a.txt', format: 'txt' as const, size: 1, updatedAt: '2026-01-01T00:00:00Z', volume: 1 },
      { path: 'b.epub', name: 'b.epub', format: 'epub' as const, size: 2, updatedAt: '2026-02-01T00:00:00Z', volume: 1 },
      { path: 'c.audio', name: 'c.audio', format: 'audio' as const, size: 3, updatedAt: '2026-03-01T00:00:00Z', volume: 1 },
    ]
    expect(latestExport(files)?.path).toBe('b.epub')
    expect(latestExport([files[2]])).toBeNull()
  })

  it('đếm số chương theo matrix của tập', () => {
    const file = { path: 'a.txt', name: 'a.txt', format: 'txt' as const, size: 1, updatedAt: null, volume: 1 }
    const matrix = [{ volume: 1, title: 'Tập 1', from: 1, to: 8, complete: true, txt: file, epub: null, audio: [] }]
    expect(exportChapterCount(file, matrix)).toBe(8)
    expect(exportChapterCount({ ...file, volume: 2 }, matrix)).toBeNull()
  })
})
