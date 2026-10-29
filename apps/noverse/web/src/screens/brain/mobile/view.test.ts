import { describe, expect, it } from 'vitest'
import type { AddressTerms, Brain, BrainEntity, BrainEntityType } from '../../../api/types'
import {
  RAIL_SCROLL_OVER, chapterCells, chapterHead, characterNames, countText, filterChips, personQuickView, phoneGroups,
  scrollLeftToCenter, sheetModel, splitByNames,
} from './view'

const entity = (id: string, type: BrainEntityType, name: string, fromChapter: number, extra: Partial<BrainEntity> = {}): BrainEntity => ({
  id, type, name, role: type === 'char' ? 'main' : '', desc: `${name} — mô tả`, fromChapter,
  appears: [fromChapter], aliases: [], timeline: [], ...extra,
})

const brain: Brain = {
  chapters: [
    { n: 1, title: 'Bản thu đầu', status: 'written' },
    { n: 2, title: 'Buổi đọc thoại', status: 'written' },
    { n: 3, title: 'Bản live', status: 'written' },
    { n: 4, title: 'Số lạ', status: 'rewriting' },
  ],
  entities: [
    entity('th', 'char', 'Lâm Tri Hạ', 1, { aliases: ['Tri Hạ'], role: 'Nữ chính · người kể', kind: 'lead' }),
    entity('nv', 'char', 'Tống Nhược Vy', 2, { aliases: ['Nhược Vy'], role: 'Phản diện chính', kind: 'antagonist' }),
    entity('pc', 'char', 'Phùng Chấn', 3, { role: 'Đồng phạm' }),
    entity('obj', 'obj', 'Vỏ ổ cứng', 2),
    entity('loc', 'place', 'Trường quay số ba', 2),
  ],
  edges: [
    { a: 'th', b: 'nv', phases: [{ fromChapter: 2, type: 'enemy', label: 'Đối đầu' }] },
    { a: 'th', b: 'pc', phases: [{ fromChapter: 3, type: 'ally', label: 'Đồng minh' }] },
    { a: 'nv', b: 'loc', phases: [{ fromChapter: 2, type: 'work', label: 'Công việc' }] },
  ],
}

const address: AddressTerms = {
  terms: [
    { id: 'xh-1', from: 'nv', fromName: 'Tống Nhược Vy', to: 'th', toName: 'Lâm Tri Hạ', call: 'chị Nga', self: 'em', fromChapter: 1, untilChapter: 1, reason: 'Còn giữ phép' },
    { id: 'xh-2', from: 'nv', fromName: 'Tống Nhược Vy', to: 'th', toName: 'Lâm Tri Hạ', call: 'em', self: 'chị', fromChapter: 2, untilChapter: 0, reason: 'Giữ vẻ đàn chị' },
    { id: 'xh-3', from: 'th', fromName: 'Lâm Tri Hạ', to: 'nv', toName: 'Tống Nhược Vy', call: 'cô', self: 'tôi', fromChapter: 2, untilChapter: 0, reason: 'Cắt đứt' },
  ],
  relations: [], latestWritten: 3, writing: { chapter: 4, role: 'writer', state: 'rewriting' },
}

describe('M8 Não truyện trên điện thoại', () => {
  it('ẩn chip loại chưa có thực thể ở chương đang xem và đếm theo chương + từ khoá', () => {
    expect(filterChips(brain, 1, '', 'all').map((chip) => [chip.key, chip.count])).toEqual([['all', 1], ['char', 1]])
    expect(filterChips(brain, 2, '', 'all').map((chip) => [chip.key, chip.count])).toEqual([
      ['all', 4], ['char', 2], ['obj', 1], ['place', 1],
    ])
    expect(filterChips(brain, 2, 'nhược', 'all').map((chip) => [chip.key, chip.count])).toEqual([['all', 1], ['char', 1]])
    expect(filterChips(brain, 1, '', 'place').map((chip) => [chip.key, chip.on])).toEqual([['all', false], ['char', false], ['place', true]])
  })

  it('xếp nhóm theo thứ tự artboard, gắn “mới” cho thực thể vừa xuất hiện và “đổi ở ch. N” cho đổi quan hệ', () => {
    const groups = phoneGroups(brain, 2, 'all', '')
    expect(groups.map((group) => group.type)).toEqual(['char', 'obj', 'place'])
    const [narrator, rival] = groups[0].items
    expect(narrator).toMatchObject({ name: 'Lâm Tri Hạ', role: 'Nữ chính · người kể', isNew: false, changed: true, rel: 'Đối đầu · Tống Nhược Vy', more: 0 })
    expect(rival).toMatchObject({ name: 'Tống Nhược Vy', isNew: true, changed: false, more: 1 })
    expect(phoneGroups(brain, 3, 'char', '').flatMap((group) => group.items.map((item) => item.name))).toEqual(['Lâm Tri Hạ', 'Tống Nhược Vy', 'Phùng Chấn'])
    expect(phoneGroups(brain, 2, 'char', 'phùng')).toEqual([])
  })

  it('thanh trên đếm thực thể đang có và quan hệ đang hiệu lực tại chương', () => {
    expect(countText(brain, 2, '')).toBe('4 thực thể · 2 quan hệ')
    expect(countText(brain, 3, '')).toBe('5 thực thể · 3 quan hệ')
    expect(countText(brain, 2, 'nhược')).toBe('1 thực thể · 2 quan hệ')
  })

  it('hàng đầu thẻ chương: dòng phụ ghi số thay đổi, chip trạng thái theo chương đang xem', () => {
    expect(chapterHead(brain, 4)).toEqual({ note: 'Người viết đang viết lại · chưa đổi gì', chip: { label: 'Đang viết lại', tone: 'live' } })
    expect(chapterHead(brain, 2)).toEqual({ note: '5 thay đổi so với chương trước', chip: { label: 'Đã viết', tone: 'done' } })
    expect(chapterHead(brain, 1)).toEqual({ note: '1 thay đổi so với chương trước', chip: { label: 'Đã viết', tone: 'done' } })
  })

  it('chương chưa viết: dòng phụ “Theo dàn ý · chưa có trong bản viết”, chip xám', () => {
    const outline: Brain = { ...brain, chapters: [...brain.chapters, { n: 5, title: 'Đêm họp báo', status: 'unwritten' }] }
    expect(chapterHead(outline, 5)).toEqual({ note: 'Theo dàn ý · chưa có trong bản viết', chip: { label: 'Chưa viết', tone: 'todo' } })
  })

  it('chương `writing` (Câu 23) cũng là chương đang viết: chip “Đang viết”, ô sọc tím', () => {
    const live: Brain = { ...brain, chapters: brain.chapters.map((item) => (item.n === 4 ? { ...item, status: 'writing' as const } : item)) }
    expect(chapterHead(live, 4)).toEqual({ note: 'Người viết đang viết · chưa đổi gì', chip: { label: 'Đang viết', tone: 'live' } })
    expect(chapterCells(live.chapters, 4)[3]).toEqual({ n: 4, tone: 'live', on: true, aria: 'Chương 4 · Số lạ · đang viết' })
  })

  it('ô số: sọc theo trạng thái từng chương, chỉ ô đang xem được chọn', () => {
    const outline: Brain['chapters'] = [...brain.chapters, { n: 5, title: 'Đêm họp báo', status: 'unwritten' }]
    const cells = chapterCells(outline, 2)
    expect(cells.map((cell) => [cell.n, cell.tone, cell.on])).toEqual([
      [1, 'done', false], [2, 'done', true], [3, 'done', false], [4, 'live', false], [5, 'todo', false],
    ])
    expect(cells[4].aria).toBe('Chương 5 · Đêm họp báo · chưa viết')
  })

  it('Câu 17: quá 10 chương mới cuộn ngang, ô đang xem luôn được đưa vào giữa', () => {
    // Ngưỡng của artboard: `segCls = len > 10 ? 'scroll' : ''`.
    expect([8, 10, 11, 30].map((count) => count > RAIL_SCROLL_OVER)).toEqual([false, false, true, true])
    // Khung 324 px, ô 38 px: canh giữa ô 20 (ở 966 px, đã gồm đệm trái) → 966 − (324 − 38) / 2.
    expect(scrollLeftToCenter(966, 38, 324)).toBe(823)
    // Đầu truyện không đủ chỗ canh giữa: kẹp ở 0 (trình duyệt cũng kẹp như vậy).
    expect(scrollLeftToCenter(6, 38, 324)).toBe(0)
  })

  it('sheet chi tiết chỉ kể tới chương đang xem: quan hệ, xưng hô, diễn biến', () => {
    const rival = sheetModel(brain, address, 'nv', 2)!
    expect(rival).toMatchObject({ name: 'Tống Nhược Vy', isChar: true, sub: 'Phản diện chính · có từ chương 2', notYet: false })
    expect(rival.relations).toEqual([
      { label: 'Đối đầu', other: 'Lâm Tri Hạ', color: 'var(--brain-e-enemy)', dashed: false, since: 'Đối đầu · từ chương 2' },
      { label: 'Công việc', other: 'Trường quay số ba', color: 'var(--brain-e-work)', dashed: false, since: 'Công việc · từ chương 2' },
    ])
    expect(rival.address.map((row) => [row.from, row.call, row.self])).toEqual([
      ['Tống Nhược Vy', 'em', 'chị'], ['Lâm Tri Hạ', 'cô', 'tôi'],
    ])
    expect(sheetModel(brain, address, 'pc', 2)).toMatchObject({ notYet: true, fromChapter: 3, relations: [] })
    expect(sheetModel(brain, address, 'missing', 2)).toBeNull()
  })

  it('xem nhanh trong Trình đọc: quan hệ với nhân vật chính + xưng hô hai chiều tại chương đang đọc', () => {
    const rival = personQuickView(brain, address, 'nv', 2)!
    expect(rival).toMatchObject({ name: 'Tống Nhược Vy', role: 'Phản diện chính', fromChapter: 2 })
    expect(rival.rows).toEqual([
      { label: 'Với Lâm Tri Hạ', value: 'Đối đầu', note: 'từ chương 2' },
      { label: 'Gọi Lâm Tri Hạ', value: '“em”', note: 'tự xưng “chị”' },
      { label: 'Lâm Tri Hạ gọi', value: '“cô”', note: 'tự xưng “tôi”' },
    ])
    // Chương 1: dòng “chị Nga” đã hết hiệu lực, quan hệ chưa hình thành → chỉ còn xưng hô cũ.
    expect(personQuickView(brain, address, 'nv', 1)!.rows).toEqual([
      { label: 'Gọi Lâm Tri Hạ', value: '“chị Nga”', note: 'tự xưng “em”' },
    ])
    expect(personQuickView(brain, address, 'obj', 2)).toBeNull()
  })

  it('nhân vật chính xem nhanh chính mình thì lấy quan hệ đầu tiên làm đối chiếu', () => {
    const main = personQuickView(brain, address, 'th', 2)!
    expect(main.rows.map((row) => [row.label, row.value])).toEqual([
      ['Với Tống Nhược Vy', 'Đối đầu'], ['Gọi Tống Nhược Vy', '“cô”'], ['Tống Nhược Vy gọi', '“em”'],
    ])
  })

  it('gạch chấm mọi tên và bí danh của nhân vật đã xuất hiện, tên dài thắng tên ngắn', () => {
    expect(characterNames(brain, 2).map((item) => item.name)).toEqual(['Tống Nhược Vy', 'Lâm Tri Hạ', 'Nhược Vy', 'Tri Hạ'])
    expect(characterNames(brain, 1).map((item) => item.name)).toEqual(['Lâm Tri Hạ', 'Tri Hạ'])
    expect(splitByNames('Tri Hạ nhìn Nhược Vy.', characterNames(brain, 2))).toEqual([
      { text: 'Tri Hạ', id: 'th', color: 'var(--kind-lead-fg)' },
      { text: ' nhìn ' },
      { text: 'Nhược Vy', id: 'nv', color: 'var(--kind-anta-fg)' },
      { text: '.' },
    ])
    expect(splitByNames('Tống Nhược Vy gọi.', characterNames(brain, 2))[0]).toMatchObject({ text: 'Tống Nhược Vy', id: 'nv' })
    expect(splitByNames('Không tên nào.', [])).toEqual([{ text: 'Không tên nào.' }])
  })

  // Dàn nhân vật “Thiên Kim Thật Đòi Lại Giọng Nói” (bug 2710): tên gọi trần nằm trong `aliases`.
  const prose: Brain = {
    chapters: [{ n: 1, title: 'Giọng thu thử', status: 'written' }],
    entities: [
      entity('la', 'char', 'Lâm An', 1, { aliases: ['An'] }),
      entity('ln', 'char', 'Lâm Nhã', 1, { aliases: ['Nhã'] }),
      entity('td', 'char', 'Trần Duy', 1, { aliases: ['Duy'] }),
      entity('hv', 'char', 'Hạ Vy', 1, { aliases: ['Vy'] }),
    ],
    edges: [],
  }

  it('bug 2710: chỉ gạch cả từ, đúng dấu, đúng hoa/thường — bảng nhãn/tên gọi trần', () => {
    const names = characterNames(prose, 1)
    const marked = (value: string) => splitByNames(value, names).flatMap((segment) => (segment.id ? [segment.text] : []))
    const rows: [string, string[]][] = [
      // Từ thường dính tên: "Nhãn" chứa "Nhã", "Anh" chứa "An" — không gạch.
      ['Nhãn phân loại chỉ cho biết người lưu kho chia chúng ra sao;', []],
      ['Anh bắt máy sau hồi chuông thứ hai.', []],
      ['Tôi gọi cho Anh.', []],
      // Đúng dấu và đúng hoa/thường: "Ân"/"an" không phải "An".
      ['Tôi đưa Ân tập kịch bản.', []],
      ['an mở cửa.', []],
      // Tên đầy đủ và tên gọi trần giữa câu: gạch đúng một lần, tên dài thắng.
      ['Tôi gọi cho Trần Duy.', ['Trần Duy']],
      ['Duy nhấc máy.', ['Duy']],
      ['Lâm An ngồi xuống.', ['Lâm An']],
      ['Tôi đưa An tập kịch bản.', ['An']],
      ['Tôi gọi cho Duy. Anh bắt máy.', ['Duy']],
      // Đầu câu: tên gọi trần ≤ 2 chữ cái không gạch (đại từ cũng viết hoa như tên).
      ['An mở cửa.', []],
      ['“An mở cửa.”', []],
      ['Tôi gọi cho Duy. An nghe máy.', ['Duy']],
      // Tên gọi trần dài hơn 2 chữ cái vẫn gạch ở đầu câu, có tên đầy đủ thì luôn gạch.
      ['Lâm Nhã đứng tên bản ghi âm.', ['Lâm Nhã']],
      ['Tôi gặp Nhã ở hành lang.', ['Nhã']],
      ['Nhã đứng tên bản ghi âm.', ['Nhã']],
      // NFD (dấu rời) vẫn khớp sau khi chuẩn hoá NFC.
      ['Lâm An ngồi xuống.'.normalize('NFD'), ['Lâm An']],
      ['Nhãn phân loại.'.normalize('NFD'), []],
    ]
    expect(rows.map(([value]) => marked(value))).toEqual(rows.map(([, expected]) => expected))
    // Mảnh ghép lại đúng chữ NFC của câu, kể cả khi đầu vào là NFD.
    const nfd = 'Lâm An và Anh ấy.'.normalize('NFD')
    expect(splitByNames(nfd, names).map((segment) => segment.text).join('')).toBe('Lâm An và Anh ấy.'.normalize('NFC'))
  })
})
