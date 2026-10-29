import { describe, expect, it } from 'vitest'
import type { JobEvent } from '../../../api/types'
import type { DiffLine } from '../logic'
import { blockedNote, changedLines, chapterRange, defaultContinue, importCommand, importPath, importSteps, impactKind, longChapterWarning, longChapters, mobileDiagTiles, otherImportWarnings, previewRows, qualifyJobId, replyDelta, resultKind, splitSummary, stepCount } from './logic'

describe('kết quả lệnh điện thoại', () => {
  it('mở đúng loại từ dòng lệnh hoặc việc nền trong liên kết sâu', () => {
    expect(resultKind('/sync --check')).toBe('sync')
    expect(resultKind('', 'import')).toBe('import')
    expect(resultKind('reopen')).toBe('reopen')
    expect(resultKind('/export')).toBeNull()
  })
  it('đặt guide cuối, giữ đường dẫn có dấu cách và loại continue cho truyện đã hoàn', () => {
    expect(importCommand('ban thao.txt', 'open', true, '  Tách ở dòng ***  ')).toBe('/import "ban thao.txt" --story=open --continue --yes --guide=Tách ở dòng ***')
    expect(importCommand('truyen.txt', 'closed', true, '')).toBe('/import truyen.txt --story=closed --yes')
  })
  it('đọc đường dẫn có dấu cách mà không ăn cờ hoặc gợi ý', () => {
    expect(importPath('/import "ban thao.txt" --story=open --guide=Tách ở dòng ***')).toBe('ban thao.txt')
    expect(importPath('/import --story=open')).toBe('')
  })
  it('giữ đúng sáu hàng gồm chương bất thường và chương mới tách; xem hết không làm mất hàng', () => {
    const rows = Array.from({ length: 24 }, (_, index) => ({ n: index + 1, title: `Chương ${index + 1}`, words: index === 12 ? 15900 : 7000, firstLine: '' }))
    expect(previewRows(rows, false).map((row) => row.n)).toEqual([1, 2, 3, 12, 13, 14])
    expect(previewRows(rows, true)).toBe(rows)
    const split = rows.map((row) => ({ ...row, words: 7000, newlySplit: row.n === 13 || row.n === 14 }))
    expect(previewRows(split, false).filter((row) => row.newlySplit).map((row) => row.n)).toEqual([13, 14])
  })
  it('hiện năm bước từ tiến trình thật và không biến việc lỗi thành hoàn tất', () => {
    const job: JobEvent = { job: 'import/1', kind: 'import', stage: 'analyzing', current: 13, total: 24, message: '', level: 'info', retryAt: null, done: false, error: '', awaiting: '' }
    expect(importSteps(job).map((step) => step.status)).toEqual(['done', 'done', 'running', 'pending', 'pending'])
    expect(importSteps(job)[2]).toMatchObject({ current: 13, total: 24 })
    expect(importSteps({ ...job, done: true, stage: 'done' }).every((step) => step.status === 'done')).toBe(true)
    expect(importSteps({ ...job, done: true, error: 'Hết hạn mức' })[2].status).toBe('running')
    expect(importSteps({ ...job, stage: 'synthesizing' })[3].status).toBe('running')
    expect(importSteps({ ...job, stage: 'publishing' })[4].status).toBe('running')
    expect(importSteps({ ...job, stage: 'awaiting_story_status' })[4].status).toBe('running')
  })
  it('tách trả lời mới khỏi lượt cũ và giữ chữ đầu khi worker reset luồng', () => {
    expect(replyDelta('Lượt cũĐang trả lời', 'Lượt cũ')).toBe('Đang trả lời')
    expect(replyDelta('Mới', 'Lượt trả lời cũ dài hơn')).toBe('Mới')
  })

  it('chương dài: lấy chương có cảnh báo của backend hoặc dài từ gấp đôi trung bình, không bắt chương ngay dưới ngưỡng', () => {
    const rows = [1000, 1000, 1000, 1000, 1500, 4000].map((words, index) => ({ n: index + 1, title: '', words, firstLine: '' }))
    expect(longChapters(rows).map((row) => row.n)).toEqual([6])
    expect(longChapters([...rows.slice(0, 5), { ...rows[0], n: 9, warning: 'Dài' }]).map((row) => row.n)).toEqual([9])
    expect(longChapters([])).toEqual([])
  })
  it('câu cảnh báo chương dài đúng chữ artboard', () => {
    expect(longChapterWarning({ n: 13, title: '', words: 15900, firstLine: '' })).toBe('Chương 13 dài 15.900 chữ — gấp đôi trung bình, có thể đang dính hai chương.')
  })
  it('cảnh báo khác bỏ câu chương dài đã dựng riêng', () => {
    const chapters = [{ n: 1, title: '', words: 1, firstLine: '', warning: 'Chương 1 dài' }]
    expect(otherImportWarnings({ chapters, warnings: ['Chương 1 dài', 'Chương 2 gấp đôi trung bình', 'Tệp mã hoá GBK'] })).toEqual(['Tệp mã hoá GBK'])
  })
  it('dòng phụ số chương: theo cách chia của backend, theo gợi ý khi có guide', () => {
    const chapters = [{ n: 1, title: '', words: 7000, firstLine: '' }, { n: 2, title: '', words: 8520, firstLine: '' }]
    expect(splitSummary({ chapters, splitHow: 'theo dòng «Chương N»', words: 15520 }, false)).toBe('Chia theo dòng «Chương N» · trung bình 7.760 chữ/chương')
    expect(splitSummary({ chapters, splitHow: '', words: undefined }, false)).toBe('trung bình 7.760 chữ/chương')
    expect(splitSummary({ chapters, splitHow: 'theo dòng «Chương N»', words: 15520 }, true)).toBe('Chia theo gợi ý của bạn · 15.520 chữ')
  })
  it('mặc định viết tiếp: bật khi lệnh không nêu trạng thái, theo cờ khi có --story', () => {
    expect(defaultContinue('/import a.txt')).toBe(true)
    expect(defaultContinue('/import a.txt --story=open')).toBe(false)
    expect(defaultContinue('/import a.txt --story=open --continue --yes')).toBe(true)
    expect(defaultContinue('/import a.txt --story=closed')).toBe(false)
  })
  it('chip tác động: phục bút > giữ nguyên > sửa dàn ý', () => {
    expect(impactKind('Thêm phục bút mới').icon).toBe('flag')
    expect(impactKind('Giữ nguyên chương 3').icon).toBe('lock')
    expect(impactKind('Không ảnh hưởng chương cũ').icon).toBe('lock')
    expect(impactKind('Đổi tên quán').icon).toBe('edit-2')
  })
  it('khoảng chương đề xuất: một chương thì chỉ số đó, nhiều chương lấy nhỏ–lớn dù thứ tự lộn xộn', () => {
    expect(chapterRange([])).toBe('')
    expect(chapterRange([{ chapter: 26 }])).toBe('26')
    expect(chapterRange([{ chapter: 27 }, { chapter: 25 }, { chapter: 26 }])).toBe('25–27')
  })
  it('dòng chặn mở lại: đếm chương của điều kiện hỏng đầu tiên, rồi ghi chú, rồi câu chung', () => {
    expect(blockedNote([{ id: 'a', ok: true, note: '', chapters: [] }, { id: 'clean', ok: false, note: 'x', chapters: [28, 29] }])).toBe('Cần duyệt 2 chương trước khi mở lại')
    expect(blockedNote([{ id: 'idle', ok: false, note: 'Đang có phiên', chapters: [] }])).toBe('Đang có phiên')
    expect(blockedNote([{ id: 'idle', ok: false, note: '', chapters: [] }])).toBe('Cần hoàn tất các điều kiện trước khi mở lại')
  })
  it('cột số của bước nhập: số chương ở bước 2 khi xong, tiến độ ở bước 3 khi đã bắt đầu', () => {
    const step = (status: 'done' | 'running' | 'pending', current = 0, total = 0) => ({ id: '', title: '', status, current, total })
    expect(stepCount(step('done'), 1, 24)).toBe('24')
    expect(stepCount(step('running'), 1, 24)).toBe('')
    expect(stepCount(step('running', 11, 24), 2, 24)).toBe('11/24')
    expect(stepCount(step('pending', 0, 24), 2, 24)).toBe('')
    expect(stepCount(step('done', 24, 24), 0, 24)).toBe('')
  })
  it('chỉ vẽ phần sửa: bỏ đoạn không đổi, đoạn cũ đứng trước đoạn mới trong cùng cụm', () => {
    const lines: DiffLine[] = [{ kind: 'same', text: 'a' }, { kind: 'add', text: 'new1' }, { kind: 'del', text: 'old1' }, { kind: 'same', text: 'b' }, { kind: 'add', text: 'new2' }]
    expect(changedLines(lines).map((line) => line.text)).toEqual(['old1', 'new1', 'new2'])
  })
  it('ô tổng quan: nhãn và chữ phụ theo MCmd (Giai đoạn viết thường chữ đầu, Phục bút mở, Đã viết)', () => {
    const stats = { completedChapters: 3, totalChapters: 8, phase: 'writing', totalWords: 9720, avgWordsPerChapter: 3240, avgReviewScore: 8.2, reviewCount: 5, rewriteCount: 2, foreshadowOpen: 4, foreshadowStale: 1, totalVolumes: 0, startedAt: '' } as unknown as Parameters<typeof mobileDiagTiles>[0]
    const tiles = mobileDiagTiles(stats, [], () => 'Sáng tác')
    expect(tiles[0].sub).toBe('Giai đoạn sáng tác')
    expect(tiles.some((tile) => tile.key === 'Phục bút mở')).toBe(true)
  })
  it('id việc nền giữ dạng `<loại>/<số>` của worker khi ghép tiền tố dự án/truyện, không ghép hai lần', () => {
    expect(qualifyJobId('proj', 'story', 'import/10')).toBe('proj/story/import/10')
    expect(qualifyJobId('proj', 'story', 'proj/story/import/10')).toBe('proj/story/import/10')
    expect(qualifyJobId('proj', 'story', 'simulate/3')).toBe('proj/story/simulate/3')
  })
})
