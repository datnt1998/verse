import { describe, expect, it } from 'vitest'
import type { AddressTerm, Brain, RelationChange } from '../../../api/types'
import {
  RELATION_OPTIONS, addFormNote, affectedChapters, appearCells, appliedLabel, applyOptions, applyToast, castMeta,
  chapterRange, confirmBody, confirmTitle, isExpired, lockMessage, otherNames, relationBefore, relationTone,
  relationsFor, termsFor, validateRelation, validateTerm,
} from './logic'

const term = (patch: Partial<AddressTerm> = {}): AddressTerm => ({
  id: 'xh-1', from: 'Tri Hạ', fromName: 'Tri Hạ', to: 'Nhược Vy', toName: 'Nhược Vy',
  call: 'cô', self: 'tôi', fromChapter: 2, untilChapter: null, reason: '', ...patch,
})

describe('bảng xưng hô (D7)', () => {
  it('dòng hết hiệu lực khi until_chapter đã nằm trong phần đã viết', () => {
    expect(isExpired({ untilChapter: 1 }, 3)).toBe(true)
    expect(isExpired({ untilChapter: 3 }, 3)).toBe(true)
    expect(isExpired({ untilChapter: 4 }, 3)).toBe(false)
    expect(isExpired({ untilChapter: null }, 3)).toBe(false)
  })

  it('BE gửi until_chapter = 0 cho dòng còn hiệu lực — không gạch ngang', () => {
    expect(isExpired({ untilChapter: 0 }, 3)).toBe(false)
    expect(appliedLabel({ fromChapter: 2, untilChapter: 0 })).toBe('từ ch.2')
  })

  it('một dòng là một chiều → tính cả hai chiều của nhân vật', () => {
    const terms = [term(), term({ id: 'xh-2', from: 'Nhược Vy', to: 'Tri Hạ' }), term({ id: 'xh-3', from: 'Kỳ Dương', to: 'Tri Hạ' })]
    expect(termsFor(terms, 'Tri Hạ').map((item) => item.id)).toEqual(['xh-1', 'xh-2', 'xh-3'])
  })

  it('meta của thẻ: bỏ vế “Xưng hô” khi nhân vật chưa có dòng nào', () => {
    expect(castMeta(5, 8, 8)).toBe('Xưng hô 5 dòng · xuất hiện 8/8 chương')
    expect(castMeta(0, 2, 8)).toBe('xuất hiện 2/8 chương')
  })

  it('cột Áp dụng: còn hiệu lực “từ ch.2”, đã đóng “ch.1” hoặc “ch.1–3”', () => {
    expect(appliedLabel({ fromChapter: 2, untilChapter: null })).toBe('từ ch.2')
    expect(appliedLabel({ fromChapter: 1, untilChapter: 1 })).toBe('ch.1')
    expect(appliedLabel({ fromChapter: 1, untilChapter: 3 })).toBe('ch.1–3')
  })

  it('chương bị ảnh hưởng = các chương đã viết từ “từ chương”', () => {
    expect(affectedChapters(2, 3)).toEqual([2, 3])
    expect(affectedChapters(1, 3)).toEqual([1, 2, 3])
    expect(affectedChapters(5, 3)).toEqual([])
    expect(affectedChapters(0, 3)).toEqual([])
  })

  it('dải chương: liên tiếp dùng gạch, đứt quãng dùng dấu phẩy', () => {
    expect(chapterRange([2, 3])).toBe('2–3')
    expect(chapterRange([2])).toBe('2')
    expect(chapterRange([2, 4])).toBe('2, 4')
  })
})

describe('banner khoá sửa (D8 · xedit=locked)', () => {
  it('nêu người giữ khoá theo writing.role và số chương', () => {
    expect(lockMessage({ chapter: 4, role: 'writer' }))
      .toBe('Người viết đang viết lại chương 4, nên xưng hô và quan hệ đang khoá để không lệch với chương đang viết.')
    expect(lockMessage({ chapter: 3, role: 'editor' })).toContain('Biên tập viên đang viết lại chương 3')
    expect(lockMessage({ chapter: 0, role: 'architect' })).toBe('Kiến trúc sư đang viết, nên xưng hô và quan hệ đang khoá để không lệch với chương đang viết.')
    // Câu 23: `writing.state` quyết định “đang viết” hay “đang viết lại”.
    expect(lockMessage({ chapter: 5, role: 'writer', state: 'writing' })).toBe('Người viết đang viết chương 5, nên xưng hô và quan hệ đang khoá để không lệch với chương đang viết.')
    expect(lockMessage({ chapter: 4, role: 'writer', state: 'rewriting' })).toContain('đang viết lại chương 4')
  })
})

describe('hộp thoại xác nhận dùng chung (D8 · xedit=addConfirm · relConfirm)', () => {
  const edit = {
    mode: 'edit' as const, affected: [2, 3], latestWritten: 3,
    fromName: 'Tri Hạ', toName: 'Nhược Vy', call: 'cô Tống', fromChapter: 2, oldCall: 'cô',
  }

  it('tiêu đề theo thao tác: sửa đếm chương, thêm dòng, đổi quan hệ', () => {
    expect(confirmTitle(edit)).toBe('Sửa này chạm tới 2 chương đã viết')
    expect(confirmTitle({ ...edit, mode: 'add' })).toBe('Dòng mới chạm tới chương đã viết')
    expect(confirmTitle({ mode: 'rel', chapter: 3, latestWritten: 3, other: 'Nhược Vy', before: 'Cắt đứt', after: 'Đối đầu công khai' }))
      .toBe('Thay đổi này nằm ở chương 3 đã viết')
  })

  it('mô tả nêu cách gọi mới, chương bắt đầu và cách gọi cũ', () => {
    expect(confirmBody(edit)).toBe('Tri Hạ gọi Nhược Vy là “cô Tống” từ chương 2. Chương 2 và 3 đã viết theo cách gọi cũ “cô”.')
    expect(confirmBody({ ...edit, affected: [2, 3, 4], oldCall: 'em' })).toContain('Chương 2, 3 và 4 đã viết')
  })

  it('mô tả của dòng mới và của đổi quan hệ đúng câu artboard', () => {
    expect(confirmBody({ ...edit, mode: 'add' }))
      .toBe('Tri Hạ gọi Nhược Vy là “cô Tống” từ chương 2. Chương 2 đã viết khi chưa có dòng này, có thể đang dùng cách gọi khác.')
    expect(confirmBody({ mode: 'rel', chapter: 3, latestWritten: 3, other: 'Nhược Vy', before: 'Cắt đứt', after: 'Đối đầu công khai' }))
      .toBe('Quan hệ với Nhược Vy đổi thành “Đối đầu công khai” từ chương 3. Chương 3 đã viết theo quan hệ cũ “Cắt đứt”.')
    expect(confirmBody({ mode: 'rel', chapter: 3, latestWritten: 3, other: 'Nhược Vy', before: '', after: 'Đồng minh' }))
      .toBe('Quan hệ với Nhược Vy đổi thành “Đồng minh” từ chương 3.')
  })

  it('hai lựa chọn đúng câu của artboard; “đánh dấu cần sửa” đứng sau', () => {
    const [next, mark] = applyOptions(edit)
    expect(next.value).toBe('next')
    expect(next.label).toBe('Chỉ áp dụng cho chương chưa viết')
    expect(next.description).toBe('Chương 2–3 giữ nguyên. Từ chương 4 trở đi, Người viết dùng cách gọi mới.')
    expect(mark.label).toBe('Áp dụng và đánh dấu chương 2–3 cần sửa')
    expect(mark.description).toBe('Hai chương vào hàng chờ sửa. Người viết chỉ sửa câu có cách gọi này, không viết lại cả chương.')
    expect(applyOptions({ ...edit, affected: [2] })[1].description.startsWith('Một chương vào hàng chờ sửa')).toBe(true)
  })

  it('lựa chọn của dòng mới nêu chương mới giữ nguyên và chương áp dụng', () => {
    const [next, mark] = applyOptions({ ...edit, mode: 'add', affected: [2, 3], fromChapter: 2 })
    expect(next.label).toBe('Chỉ áp dụng cho chương chưa viết')
    expect(next.description).toBe('Chương 2 giữ nguyên. Từ chương 4 trở đi, Người viết dùng cách gọi mới.')
    expect(mark.label).toBe('Áp dụng và đánh dấu chương 2–3 cần sửa')
    expect(mark.description).toBe('Chương vào hàng chờ sửa. Người viết chỉ sửa câu có cách gọi này, không viết lại cả chương.')
  })

  it('lựa chọn của đổi quan hệ: “Chỉ ghi nhận…” và “Ghi nhận và đánh dấu…”', () => {
    const [next, mark] = applyOptions({ mode: 'rel', chapter: 3, latestWritten: 3, other: 'Nhược Vy', before: 'Cắt đứt', after: 'Đồng minh' })
    expect(next.label).toBe('Chỉ ghi nhận, chương 3 giữ nguyên')
    expect(next.description).toBe('Não truyện và Biên tập viên dùng quan hệ mới từ chương 3. Chương đã viết không bị đánh dấu.')
    expect(mark.label).toBe('Ghi nhận và đánh dấu chương 3 cần sửa')
    expect(mark.description).toBe('Chương 3 vào hàng chờ sửa. Người viết chỉ sửa đoạn có quan hệ này.')
  })

  it('toast sau khi lưu theo thao tác và lựa chọn', () => {
    expect(applyToast(edit, 'mark')).toBe('Đã lưu · chương 2–3 vào hàng chờ sửa')
    expect(applyToast(edit, 'next')).toBe('Đã lưu · áp dụng từ chương 4')
    expect(applyToast({ ...edit, mode: 'add', affected: [2] }, 'mark')).toBe('Đã thêm · chương 2 vào hàng chờ sửa')
    expect(applyToast({ ...edit, mode: 'add', affected: [2] }, 'next')).toBe('Đã thêm · áp dụng từ chương 4')
    const rel = { mode: 'rel' as const, chapter: 3, latestWritten: 3, other: 'Nhược Vy', before: 'Cắt đứt', after: 'Đồng minh' }
    expect(applyToast(rel, 'mark')).toBe('Đã ghi · chương 3 vào hàng chờ sửa')
    expect(applyToast(rel, 'next')).toBe('Đã ghi thay đổi quan hệ')
  })
})

describe('kiểm tra tại chỗ', () => {
  it('dòng xưng hô: thiếu cách gọi / tự xưng / chương / trùng nhân vật', () => {
    const ok = { from: 'A', to: 'B', call: 'cô', self: 'tôi', fromChapter: 2, reason: 'Gặp lần đầu' }
    expect(validateTerm(ok)).toBeNull()
    expect(validateTerm({ ...ok, from: '' })).toBe('Chọn đủ hai nhân vật của dòng xưng hô.')
    expect(validateTerm({ ...ok, to: 'A' })).toBe('Một dòng xưng hô cần hai nhân vật khác nhau.')
    expect(validateTerm({ ...ok, call: '  ' })).toBe('Chưa nhập cách gọi.')
    expect(validateTerm({ ...ok, self: '' })).toBe('Chưa nhập cách tự xưng.')
    expect(validateTerm({ ...ok, fromChapter: 0 })).toBe('Chương bắt đầu phải từ 1 trở lên.')
  })

  it('sửa trên web bắt buộc lý do — dòng xưng hô và quan hệ (Câu 20)', () => {
    expect(validateTerm({ from: 'A', to: 'B', call: 'cô', self: 'tôi', fromChapter: 2, reason: ' ' })).toBe('Chưa nhập lý do cho dòng xưng hô.')
    expect(validateRelation({ a: 'A', b: 'B', after: 'Đồng minh', fromChapter: 4, reason: '' })).toBe('Chưa nhập lý do cho thay đổi quan hệ.')
  })

  it('quan hệ: cần hai nhân vật khác nhau và quan hệ sau khi đổi', () => {
    const ok = { a: 'A', b: 'B', after: 'Đồng minh', fromChapter: 4, reason: 'Cùng nhóm' }
    expect(validateRelation(ok)).toBeNull()
    expect(validateRelation({ ...ok, b: 'A' })).toBe('Quan hệ cần hai nhân vật khác nhau.')
    expect(validateRelation({ ...ok, after: ' ' })).toBe('Chưa nhập quan hệ sau khi đổi.')
  })
})

describe('màu chip quan hệ và quan hệ đang áp dụng (Câu 20)', () => {
  it('gom ba nhóm: enemy đối đầu · ally thân · còn lại và rel', () => {
    expect(relationTone('enemy')).toBe('hostile')
    expect(relationTone('ally')).toBe('close')
    for (const type of ['work', 'own', 'secret', 'rel', undefined]) expect(relationTone(type)).toBe('other')
  })

  it('quan hệ đang áp dụng ở chương liền trước của đúng cặp, hai chiều', () => {
    const log: RelationChange[] = [
      { chapter: 1, a: 'A', aName: 'A', b: 'B', bName: 'B', before: '', after: 'Bạn diễn cũ', reason: '', type: 'work' },
      { chapter: 3, a: 'B', aName: 'B', b: 'A', bName: 'A', before: 'Bạn diễn cũ', after: 'Cắt đứt', reason: '', type: 'enemy' },
      { chapter: 2, a: 'A', aName: 'A', b: 'C', bName: 'C', before: '', after: 'Đồng minh', reason: '', type: 'ally' },
    ]
    expect(relationBefore(log, 'A', 'B', 3)?.after).toBe('Bạn diễn cũ')
    expect(relationBefore(log, 'B', 'A', 4)?.after).toBe('Cắt đứt')
    expect(relationBefore(log, 'A', 'B', 1)).toBeNull()
    expect(relationBefore(log, 'B', 'C', 5)).toBeNull()
  })

  it('form thêm dòng: danh sách quan hệ mới và ghi chú theo số chương đã viết', () => {
    expect(RELATION_OPTIONS).toEqual(['Đối đầu công khai', 'Đồng minh', 'Tin tưởng một phần', 'Người thân'])
    expect(addFormNote(3)).toBe('Mỗi dòng một chiều. Nếu chiều này đã có dòng đang áp dụng, dòng cũ tự kết thúc ở chương liền trước. Từ chương đã viết (1–3) sẽ hỏi cách áp dụng.')
    expect(addFormNote(0)).toBe('Mỗi dòng một chiều. Nếu chiều này đã có dòng đang áp dụng, dòng cũ tự kết thúc ở chương liền trước.')
  })
})

describe('hàng “Xuất hiện” (D7)', () => {
  const brain = {
    chapters: [
      { n: 1, title: 'Một', status: 'written' as const },
      { n: 2, title: 'Hai', status: 'written' as const },
      { n: 3, title: 'Ba', status: 'rewriting' as const },
      { n: 4, title: 'Bốn', status: 'unwritten' as const },
    ],
    entities: [], edges: [],
  } satisfies Pick<Brain, 'chapters' | 'entities' | 'edges'>

  it('ô có mặt sáng, chương đang viết viền tím, ô vắng để trống', () => {
    const cells = appearCells(brain, [1, 3], 3)
    expect(cells.map((cell) => cell.n)).toEqual([1, 2, 3, 4])
    expect(cells[0]).toMatchObject({ present: true, bg: 'var(--kit-ok-8)', fg: 'var(--color-fg)', border: 'var(--line-3)' })
    expect(cells[1]).toMatchObject({ present: false, bg: 'transparent', fg: 'var(--kit-fg-icon)' })
    expect(cells[2]).toMatchObject({ present: true, bg: 'var(--tint-accent)', fg: 'var(--color-fg)', border: 'var(--kit-acc-45)' })
    expect(cells[3]).toMatchObject({ present: false, border: 'var(--line-3)' })
  })

  it('có mặt ở chương chưa viết thì chữ vẫn xám (artboard: ô 5–8)', () => {
    expect(appearCells(brain, [4], null)[3].fg).toBe('var(--kit-fg-icon)')
  })
})

describe('lịch sử quan hệ', () => {
  const relations: RelationChange[] = [
    { chapter: 1, a: 'Tri Hạ', aName: 'Tri Hạ', b: 'Nhược Vy', bName: 'Nhược Vy', before: 'Bạn', after: 'Đối thủ', reason: '', type: 'enemy' },
    { chapter: 2, a: 'Kỳ Dương', aName: 'Kỳ Dương', b: 'Tri Hạ', bName: 'Tri Hạ', before: 'Người lạ', after: 'Đồng minh', reason: '', type: 'ally' },
    { chapter: 3, a: 'Khác', aName: 'Khác', b: 'Người', bName: 'Người', before: '', after: '', reason: '', type: 'rel' },
  ]

  it('chỉ lấy dòng của nhân vật và nêu tên người còn lại', () => {
    const mine = relationsFor(relations, 'Tri Hạ')
    expect(mine.map((item) => [item.chapter, item.other])).toEqual([[1, 'Nhược Vy'], [2, 'Kỳ Dương']])
  })

  it('ô chọn nhân vật của dòng mới bỏ chính nhân vật đang xem', () => {
    expect(otherNames(['A', 'B', 'C'], 'B')).toEqual(['A', 'C'])
  })
})
