import { describe, expect, it } from 'vitest'
import type { Quality, QualityChapterStatus, QualityIssue } from '../../api/types'
import { t } from '../../i18n/vi'
import { autoChapters, colTotal, filterIssues, kpis, matrixRows, rowTotal, tally, todoCount, todoGroups } from './model'

// Bộ mẫu IS8 của Quality.dc.html: [chương, mục, mức, trạng thái].
const IS8: [number, string, 'e' | 'w', QualityIssue['state']][] = [
  [1, 'Master §2', 'w', 'open'], [1, 'Master §4', 'e', 'fixed'], [2, 'Master §13', 'w', 'kept'], [2, 'Quy tắc của bạn', 'w', 'fixed'],
  [3, 'Master §7', 'e', 'fixed'], [3, 'Xưng hô', 'e', 'fixed'], [3, 'Xưng hô', 'w', 'open'], [3, 'Master §13', 'w', 'open'],
  [3, 'Bộ A §4.3', 'w', 'open'], [3, 'Quy tắc của bạn', 'w', 'open'], [4, 'Master §6', 'w', 'open'],
  [5, 'Master §15', 'w', 'open'], [5, 'Bộ A §3', 'w', 'open'], [6, 'Master §7', 'w', 'open'], [6, 'Master §5', 'e', 'fixed'],
  [7, 'Master §7', 'w', 'open'], [7, 'Master §12', 'w', 'open'], [8, 'Master §16', 'e', 'open'], [8, 'Master §13', 'w', 'kept'],
]
const STATUS: QualityChapterStatus[] = ['rewritten', 'rewritten', 'rewritten', 'rewriting', 'previous', 'previous', 'previous', 'previous']
/** Câu 9: “Đã viết lại” chỉ khi có lần viết lại đã ghi; chương đang/đã chờ viết lại vẫn có thể có. */
const REWRITES = [{ at: '2026-10-25T20:00:00+07:00', cause: 'review' as const }]
const refs = [...new Set(IS8.map((row) => row[1]))]
const data: Quality = {
  pack: { id: 'nu-tan-bao-thu', ten: 'Nữ tần báo thù' },
  sections: refs.map((ref) => ({ ref, refName: ref })),
  chapters: STATUS.map((status, i) => ({ n: i + 1, title: `Chương ${i + 1}`, status, rewrites: status === 'rewritten' ? REWRITES : [] })),
  cells: [],
  issues: IS8.map(([ch, ref, level, state], i) => ({
    id: `i${i}`, level: level === 'e' ? 'error' : 'warning', ref, refName: ref, text: '', quote: '', fix: '', ch, state,
  })),
}
const row = (ref: string, issues = data.issues) => rowTotal(tally(issues.filter((issue) => issue.ref === ref)), 'all')
const col = (ch: number, issues = data.issues) => {
  const here = issues.filter((issue) => issue.ch === ch)
  return colTotal(tally(here), here.length > 0)
}

describe('Chất lượng truyện — số ma trận tính ở client', () => {
  it('tổng hàng: còn mở, không thì ✓đã sửa, không thì —', () => {
    expect(row('Master §7')).toEqual({ text: '2', tone: 'warn' })
    expect(row('Master §4')).toEqual({ text: '✓1', tone: 'ok' })
    expect(row('Master §13')).toEqual({ text: '1', tone: 'warn' })
    expect(row('Master §16')).toEqual({ text: '1', tone: 'err' })
  })
  it('tổng cột và tổng chung = số mục còn mở', () => {
    expect(col(3)).toEqual({ text: '4', tone: 'warn' })
    expect(col(8)).toEqual({ text: '1', tone: 'err' })
    expect(data.issues.filter((issue) => issue.state === 'open')).toHaveLength(12)
    expect(matrixRows(data)).toHaveLength(refs.length)
  })
  it('bộ lọc mức + phạm vi chạy ở client', () => {
    const errorsOpen = filterIssues(data.issues, 'error', 'open')
    expect(errorsOpen.map((issue) => issue.ch)).toEqual([8])
    expect(col(3, errorsOpen)).toEqual({ text: '0', tone: 'muted' })
  })
  it('ba nhóm Việc cần làm và 5 ô số', () => {
    const groups = todoGroups(data, 'all', new Set())
    expect([groups.block.length, groups.look.length, groups.auto.length]).toEqual([1, 5, 6])
    expect(autoChapters(groups.auto)).toBe('4–7')
    expect(todoCount(data)).toBe(6)
    const k = kpis(data)
    expect([k.all.e, k.all.w, k.all.f, k.all.k, `${k.passed}/${k.total}`, k.errorChapters]).toEqual([1, 11, 5, 2, '3/8', '8'])
  })
  it('Câu 12: “N việc cần xem” ở Trình đọc tính từ dữ liệu, khớp chữ artboard “6 việc”', () => {
    expect(t.quality.readerLink(todoCount(data))).toBe('Xem chất lượng cả truyện · 6 việc cần xem')
  })
})
