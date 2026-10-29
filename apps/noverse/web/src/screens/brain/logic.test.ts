import { describe, expect, it } from 'vitest'
import type { AddressTerm, Brain, BrainEdge, BrainEntity } from '../../api/types'
import { sampleChapterTitle, sampleProse } from '../../../mock/brain'
import {
  addressRowsAt, aliveAt, appearRow, changedAt, changedCountAt, changesAt, chapterNote, countsAt,
  edgeCountsAt, edgeCurve, edgesAt, fitText, initialChapter, isNewAt, labelWidth, listGroups,
  lockSentence, narratorOf, phaseAt, pointOnCurve, railFill, railTicks, relsAt, timelineAt,
} from './model'

const entity = (id: string, patch: Partial<BrainEntity> = {}): BrainEntity => ({
  id, type: 'char', name: id, role: '', desc: '', fromChapter: 1, appears: [1, 2, 3],
  aliases: [], timeline: [], ...patch,
})

const edge = (a: string, b: string, phases: BrainEdge['phases']): BrainEdge => ({ a, b, phases })

const chapters: Brain['chapters'] = [1, 2, 3, 4, 5].map((n) => ({
  n, title: `T${n}`, status: n <= 2 ? 'written' : n === 3 ? 'rewriting' : 'unwritten',
}))

const brain: Brain = {
  chapters,
  entities: [
    entity('th', { name: 'Lâm Tri Hạ' }),
    entity('nv', { name: 'Tống Nhược Vy' }),
    entity('kd', { name: 'Kỳ Dương', fromChapter: 2 }),
    entity('vo', { name: 'Vỏ ổ cứng', type: 'fs', fromChapter: 4 }),
  ],
  edges: [
    edge('th', 'nv', [{ fromChapter: 1, type: 'work', label: 'Bạn diễn cũ' }, { fromChapter: 2, type: 'enemy', label: 'Đối đầu' }]),
    edge('th', 'kd', [{ fromChapter: 2, type: 'ally', label: 'Đồng minh' }]),
    edge('vo', 'th', [{ fromChapter: 4, type: 'secret', label: 'Chứa bản gốc?' }]),
  ],
}

const term = (patch: Partial<AddressTerm> = {}): AddressTerm => ({
  id: 'xh-1', from: 'th', fromName: 'Lâm Tri Hạ', to: 'nv', toName: 'Tống Nhược Vy',
  call: 'chị Vy', self: 'em', fromChapter: 1, untilChapter: 1, reason: 'đồng nghiệp cũ', ...patch,
})

describe('theo chương', () => {
  it('phase gần nhất có fromChapter ≤ N, và chương 1 không gắn “mới”', () => {
    expect(phaseAt(brain.edges[0].phases, 1)?.label).toBe('Bạn diễn cũ')
    expect(phaseAt(brain.edges[0].phases, 2)?.label).toBe('Đối đầu')
    expect(phaseAt(brain.edges[0].phases, 9)?.label).toBe('Đối đầu')
    expect(phaseAt([], 3)).toBeNull()
    expect(isNewAt({ fromChapter: 1 }, 1)).toBe(false)
    expect(isNewAt({ fromChapter: 2 }, 2)).toBe(true)
    expect(aliveAt({ fromChapter: 4 }, 3)).toBe(false)
  })

  it('cạnh hiệu lực bỏ cạnh có đầu chưa xuất hiện', () => {
    expect(edgesAt(brain, 3).map((item) => item.edge.b)).toEqual(['nv', 'kd'])
    expect(edgesAt(brain, 4).map((item) => item.edge.b)).toEqual(['nv', 'kd', 'th'])
    expect(edgesAt(brain, 4)[2].fresh).toBe(true)
  })

  it('quan hệ của một thực thể ghi “kiểu · từ chương N”', () => {
    const rows = relsAt(brain, 'th', 2)
    expect(rows.map((row) => row.since)).toEqual(['Đối đầu · từ chương 2', 'Đồng minh · từ chương 2'])
    expect(rows[0].otherName).toBe('Tống Nhược Vy')
    expect(rows[0].dashed).toBe(false)
  })

  it('đếm thực thể/cạnh theo kiểu và số thay đổi', () => {
    expect(countsAt(brain, 3)).toEqual({ char: 3, place: 0, obj: 0, ev: 0, fs: 0, rule: 0 })
    expect(edgeCountsAt(brain, 2).enemy).toBe(1)
    expect(changedAt(brain, 'th', 2)).toBe(true)
    expect(changedAt(brain, 'vo', 2)).toBe(false)
    expect(changedCountAt(brain, 4)).toBe(2)
    expect(chapterNote(brain, 3)).toEqual({ text: 'Đang viết lại chương này', live: true })
    expect(chapterNote(brain, 2).text).toBe('3 thay đổi so với chương trước')
    expect(chapterNote(brain, 1).text).toBe('3 thay đổi so với chương trước')
  })
})

describe('xưng hô tại chương N', () => {
  const terms = [
    term(),
    term({ id: 'xh-2', call: 'cô', self: 'tôi', fromChapter: 2, untilChapter: null, reason: 'cắt đứt sau khi biết bị cướp tên' }),
    term({ id: 'xh-3', from: 'nv', fromName: 'Tống Nhược Vy', to: 'th', toName: 'Lâm Tri Hạ', call: 'em', self: 'chị', untilChapter: null }),
  ]

  it('khớp theo id lẫn theo tên (mock cũ gửi tên)', () => {
    expect(addressRowsAt({ terms }, entity('th', { name: 'Lâm Tri Hạ' }), 1)).toHaveLength(2)
    expect(addressRowsAt({ terms }, entity('Lâm Tri Hạ', { name: 'Lâm Tri Hạ' }), 1)).toHaveLength(2)
  })

  it('dòng hết hiệu lực rời bảng, dòng mới kèm câu “Đổi ở chương 2…”', () => {
    const rows = addressRowsAt({ terms }, entity('th', { name: 'Lâm Tri Hạ' }), 2)
    expect(rows.map((row) => row.call)).toEqual(['cô', 'em'])
    expect(rows[0].from).toBe('Lâm Tri Hạ')
    expect(rows[0].was).toBe('Đổi ở chương 2, trước đó gọi “chị Vy” · cắt đứt sau khi biết bị cướp tên')
    expect(rows[1].was).toBeUndefined()
  })
})

describe('đổi ở chương N', () => {
  it('chỉ lấy quan hệ chạm nhân vật, Đổi trước Mới, tối đa 3 dòng', () => {
    const rows = changesAt(brain, 2)
    expect(rows.map((row) => row.text)).toEqual([
      'Đối đầu · Lâm Tri Hạ – Tống Nhược Vy',
      'Đồng minh · Lâm Tri Hạ – Kỳ Dương',
    ])
    expect(rows.map((row) => row.changed)).toEqual([true, false])
    expect(changesAt(brain, 3)).toEqual([])
    const many = { ...brain, edges: [...brain.edges.slice(0, 2), ...brain.edges.slice(0, 2), ...brain.edges.slice(0, 2)] }
    expect(changesAt(many, 2)).toHaveLength(3)
  })
})

describe('diễn biến', () => {
  const terms = [term({ fromChapter: 2, untilChapter: null })]

  it('dùng timeline máy chủ khi có, mốc sau N đánh dấu future', () => {
    const rows = timelineAt(
      entity('th', { timeline: [{ chapter: 1, text: 'A' }, { chapter: 4, text: 'B' }] }),
      brain, { terms }, 2,
    )
    expect(rows).toEqual([{ chapter: 1, text: 'A', future: false }, { chapter: 4, text: 'B', future: true }])
  })

  it('chưa có timeline thì suy từ đổi quan hệ + xưng hô, tối đa 7 dòng', () => {
    const rows = timelineAt(entity('th', { name: 'Lâm Tri Hạ' }), brain, { terms }, 2)
    expect(rows.map((row) => row.text)).toEqual([
      'Bạn diễn cũ · Tống Nhược Vy',
      'Đổi thành Đối đầu · Tống Nhược Vy',
      'Đồng minh · Kỳ Dương',
      'Lâm Tri Hạ gọi Tống Nhược Vy là “chị Vy”',
      'Chứa bản gốc? · Vỏ ổ cứng',
    ])
    expect(rows.map((row) => row.future)).toEqual([false, false, false, false, true])
  })
})

describe('thanh chương & hàng xuất hiện', () => {
  it('mở ở chương đang viết lại, không thì chương đã viết cuối', () => {
    expect(initialChapter(chapters)).toBe(3)
    expect(initialChapter([{ n: 1, title: 'a', status: 'written' }, { n: 2, title: 'b', status: 'unwritten' }])).toBe(1)
    expect(initialChapter([{ n: 1, title: 'a', status: 'unwritten' }])).toBe(1)
  })

  it('chương `writing` (đang viết lần đầu) mở như `rewriting` và tooltip ghi “đang viết” (Câu 23)', () => {
    const live: Brain['chapters'] = [
      { n: 1, title: 'Một', status: 'written' },
      { n: 2, title: 'Hai', status: 'writing' },
      { n: 3, title: 'Ba', status: 'unwritten' },
    ]
    expect(initialChapter(live)).toBe(2)
    const ticks = railTicks(live, 2)
    expect(ticks.map((tick) => tick.cls)).toEqual(['done', 'live', 'todo'])
    expect(ticks[1].aria).toBe('Chương 2 · Hai · đang viết')
    expect(railFill(live)).toBe('50%')
    expect(chapterNote({ chapters: live, entities: [], edges: [] }, 2)).toEqual({ text: 'Đang viết chương này', live: true })
  })

  it('nấc chương chia đều theo %, đường nối tô tới chương đang viết', () => {
    const ticks = railTicks(chapters, 3)
    expect(ticks.map((tick) => tick.x)).toEqual(['0.000%', '25.000%', '50.000%', '75.000%', '100.000%'])
    expect(ticks.map((tick) => tick.cls)).toEqual(['done', 'done', 'live', 'todo', 'todo'])
    expect(ticks[2].aria).toBe('Chương 3 · T3 · đang viết lại')
    expect(ticks[2].fg).toBe('var(--color-fg)')
    expect(ticks[3].fg).toBe('var(--brain-tick-num)')
    expect(railFill(chapters)).toBe('50%')
  })

  it('ô xuất hiện: có mặt nền tím nhạt, chương đang xem viền tím', () => {
    const cells = appearRow(chapters, [1, 3], 3)
    expect(cells[0]).toEqual({ n: 1, title: 'T1', bg: 'var(--brain-appear-bg)', fg: 'var(--color-fg)', border: 'var(--line-3)' })
    expect(cells[1].bg).toBe('transparent')
    expect(cells[2].border).toBe('var(--line-focus)')
  })
})

describe('danh sách', () => {
  it('nhóm theo loại, lọc theo ô tìm, dòng có “đổi ở ch. N”', () => {
    const groups = listGroups(brain, 2)
    expect(groups.map((group) => group.type)).toEqual(['char', 'fs'])
    expect(groups[0].items[0]).toEqual({
      id: 'th', name: 'Lâm Tri Hạ', since: 'từ chương 1',
      rels: 'Đối đầu · Tống Nhược Vy, Đồng minh · Kỳ Dương', changed: 'đổi ở ch. 2',
    })
    expect(listGroups(brain, 2, 'kỳ').map((group) => group.items.map((item) => item.name))).toEqual([['Kỳ Dương']])
    expect(listGroups(brain, 1)[0].items[2].since).toBe('từ chương 2 · chưa có')
  })
})

describe('đồ thị', () => {
  it('người kể là nhân vật đầu tiên của dữ liệu', () => {
    expect(narratorOf(brain.entities)).toBe('th')
  })

  it('cung nối cắt ở mép thẻ và cong ra ngoài tâm', () => {
    const curve = edgeCurve({ x: 430, y: 345 }, { x: 604, y: 226 }, { w: 120, h: 46 }, { w: 120, h: 46 }, 0)
    expect(curve.ax).toBeGreaterThan(430)
    expect(curve.ay).toBeLessThan(345)
    expect(curve.d.startsWith('M')).toBe(true)
    const mid = pointOnCurve(curve, 0.5)
    expect(Math.hypot(mid.x - 517, mid.y - 285.5)).toBeGreaterThan(5)
  })

  it('nhãn pill rộng theo chữ và “MỚI”', () => {
    expect(labelWidth(40, false)).toBe(73)
    expect(labelWidth(40, true, 22)).toBe(102)
  })

  it('chính văn truyện mẫu đổi sang dàn nhân vật của Não truyện, không đổi truyện khác', () => {
    expect(sampleProse('01-tra-xanh-cuop-kich-ban', 'Kiều My gặp Trần Nga; Quách và Nhân Phát đứng cạnh.'))
      .toBe('Lâm Tri Hạ gặp Tống Nhược Vy; Phùng Chấn và Hứa Hoài Châu đứng cạnh.')
    expect(sampleProse('khac', 'Kiều My')).toBe('Kiều My')
  })

  it('tiêu đề chương của truyện mẫu khớp /brain (không lệch một chương)', () => {
    expect(sampleChapterTitle('01-tra-xanh-cuop-kich-ban', 1)).toBe('Vai của tôi là đồ canh')
    expect(sampleChapterTitle('01-tra-xanh-cuop-kich-ban', 2)).toBe('Chiếc cặp ở giữa bàn')
    expect(sampleChapterTitle('01-tra-xanh-cuop-kich-ban', 3)).toBe('Hai chiếc bát trên bàn')
    expect(sampleChapterTitle('01-tra-xanh-cuop-kich-ban', 4)).toBe('Bảy mươi ba giây')
    expect(sampleChapterTitle('khac', 2)).toBeNull()
  })
})

describe('cắt chữ', () => {
  const measure = (text: string) => text.length * 10

  it('vừa thì giữ nguyên, dài thì cắt “…” và bỏ dấu cách/·/, thừa', () => {
    expect(fitText('Ngắn', 50, measure)).toBe('Ngắn')
    expect(fitText('Một cái tên rất dài', 80, measure)).toBe('Một cái…')
    expect(fitText('Đối đầu · abcdef', 60, measure)).toBe('Đối đ…')
  })

  it('tên dài chỉ cần số lần đo theo logarit, không ép đo từng ký tự', () => {
    let calls = 0
    const fitted = fitText('Tên quan hệ rất dài '.repeat(30), 128, (text) => { calls++; return text.length * 6 })
    expect(fitted.endsWith('…')).toBe(true)
    expect(fitted.length * 6).toBeLessThanOrEqual(128)
    expect(calls).toBeLessThanOrEqual(12)
  })
})

describe('khoá sửa (D8)', () => {
  it('câu banner đúng artboard', () => {
    expect(lockSentence({ chapter: 4, role: 'writer', state: 'rewriting' })).toBe('Người viết đang viết lại chương 4, nên đang khoá sửa.')
    expect(lockSentence({ chapter: 0, role: 'editor', state: 'writing' })).toBe('Biên tập viên đang viết, nên đang khoá sửa.')
    expect(lockSentence({ chapter: 5, role: 'writer', state: 'writing' })).toBe('Người viết đang viết chương 5, nên đang khoá sửa.')
  })
})
