import { describe, expect, it } from 'vitest'
import {
  DEFAULT_BAN, DEFAULT_FATIGUE, GROUP_ORDER, changeCount, chipText, degradedChips, emptyEdits, entryFromItem, groupSub, isUncertain,
  learnedMeta, oneLine, parseRuleLines, serializeRuleLines, summarize, visibleRuntime, whenText, type Baseline, type RuleEntry,
} from './logic'

const entry = (text: string, over: Partial<RuleEntry> = {}): RuleEntry => ({ key: text, text, chips: [], uncertain: false, note: '', options: [], status: 'ready', ...over })
const base = (over: Partial<Baseline> = {}): Baseline => ({ story: [], project: [], global: [], runtime: [], ...over })

describe('quy-tac.md — mỗi cấp một tệp, mỗi quy tắc một dòng', () => {
  it('đọc: bỏ tiêu đề, dòng trống và tiền tố gạch đầu dòng', () => {
    expect(parseRuleLines('# Quy tắc\n\n- Kể ngôi thứ nhất.\n* Không dùng “bỗng nhiên”.\nLời dặn trần.\n')).toEqual(['Kể ngôi thứ nhất.', 'Không dùng “bỗng nhiên”.', 'Lời dặn trần.'])
  })
  it('ghi: một dòng `- …`, xuống dòng trong ô nhập gộp thành khoảng trắng, danh sách rỗng ra tệp rỗng', () => {
    expect(serializeRuleLines(['A.', 'B\nvà C.'])).toBe('- A.\n- B và C.\n')
    expect(serializeRuleLines([])).toBe('')
    expect(oneLine('  a \n\n b  ')).toBe('a b')
  })
  it('ghi rồi đọc lại ra đúng danh sách', () => {
    const lines = ['Không dùng từ “thở dài” quá 2 lần mỗi chương.', 'Giọng văn châm biếm nhẹ.']
    expect(parseRuleLines(serializeRuleLines(lines))).toEqual(lines)
  })
})

describe('chip “Noverse hiểu là”', () => {
  it('“Không dùng từ ‘thở dài’ quá 2 lần mỗi chương” → Tối đa “thở dài” · 2 lần/chương', () => {
    expect(chipText({ kind: 'max', text: 'thở dài', limit: 2 })).toBe('Tối đa “thở dài” · 2 lần/chương')
  })
  it('các loại còn lại', () => {
    expect(chipText({ kind: 'ban', text: 'bỗng nhiên' })).toBe('Không dùng “bỗng nhiên”')
    expect(chipText({ kind: 'chr', text: '~' })).toBe('Không dùng ký tự “~”')
    expect(chipText({ kind: 'genre', text: 'showbiz · đô thị' })).toBe('showbiz · đô thị')
    expect(chipText({ kind: 'pref', text: 'Giọng kể ngôi thứ nhất' })).toBe('Giọng kể ngôi thứ nhất')
  })
  it('chuẩn hoá lỗi: thẻ vẫn lưu, chip là “Lời dặn”, không có khung chưa-chắc', () => {
    const chips = degradedChips('Đừng viết dài.')
    expect(chips).toEqual([{ kind: 'pref', text: 'Đừng viết dài.' }])
    expect(chipText(chips[0], true)).toBe('Lời dặn')
    expect(isUncertain(entry('Đừng viết dài.', { chips, status: 'degraded', uncertain: true, note: 'x' }))).toBe(false)
  })
})

describe('khung cam “chưa chắc”', () => {
  const unsure = entry('Đừng để chương quá dài.', { uncertain: true, note: '“quá dài” là bao nhiêu?', options: ['Khoảng 3.000 chữ mỗi chương'] })
  it('hiện khi máy chủ đánh dấu + có lý do; mất khi bấm “Giữ như lời dặn”', () => {
    expect(isUncertain(unsure)).toBe(true)
    expect(isUncertain({ ...unsure, acked: true })).toBe(false)
    expect(isUncertain(entry('Rõ ràng.'))).toBe(false)
    // Câu 26a: `uncertain: true` mà thiếu câu lý do thì không có gì để hỏi → thẻ thường.
    expect(isUncertain({ ...unsure, note: '' })).toBe(false)
  })
  it('entryFromItem đọc `uncertain` boolean + `uncertainNote` của máy chủ, giữ options (0–3 cách hiểu)', () => {
    const made = entryFromItem({
      text: 't', scope: 'story', chips: [], uncertain: true,
      uncertainNote: 'Chưa rõ “quá dài” là bao nhiêu', options: ['a', 'b'], status: 'ready',
    }, 'k')
    expect(made.options).toEqual(['a', 'b'])
    expect(made.uncertain).toBe(true)
    expect(made.note).toBe('Chưa rõ “quá dài” là bao nhiêu')
    expect(entryFromItem({ text: 't', scope: 'story', chips: [], uncertain: false, uncertainNote: '', status: 'ready' }, 'k').note).toBe('')
  })
})

describe('nhóm theo nguồn', () => {
  it('chỉ có quy tắc truyện, học từ góp ý và dự án', () => {
    expect(GROUP_ORDER.story).toEqual(['story', 'learned', 'project'])
    expect(GROUP_ORDER.project).toEqual(['project'])
    expect(GROUP_ORDER.global).toEqual([])
  })
  it('nhóm kế thừa ghi “· kế thừa”; nhóm học từ góp ý có chú thích Trọng tài', () => {
    expect(groupSub(2, 'project', 'story')).toBe('2 quy tắc · kế thừa')
    expect(groupSub(4, 'story', 'story')).toBe('4 quy tắc')
    expect(groupSub(2, 'learned', 'story')).toBe('2 quy tắc · Trọng tài ghi lại khi bạn dặn trong chat')
  })
  it('dòng meta của luật Trọng tài', () => {
    expect(learnedMeta({ chapter: 12, createdAt: '2020-01-05T00:00:00Z' }, true)).toBe('Từ góp ý ở chương 12')
    expect(learnedMeta({ chapter: 12, createdAt: '2020-01-05T00:00:00Z' })).toMatch(/^Từ góp ý của bạn ở chương 12 · \d+\/\d+$/)
  })
})

describe('bản sửa chưa Áp dụng', () => {
  it('thêm một quy tắc = 1 thay đổi; xoá lại đúng như cũ = 0', () => {
    const b = base({ story: [entry('A.')] })
    const added = { ...emptyEdits(), lists: { story: [entry('B.'), entry('A.')] }, ops: 1 }
    expect(changeCount(b, added)).toBe(1)
    const reverted = { ...emptyEdits(), lists: { story: [entry('A.')] }, ops: 2 }
    expect(changeCount(b, reverted)).toBe(0)
  })
  it('Giữ / Bỏ luật Trọng tài tính là thay đổi và ẩn luật khỏi nhóm', () => {
    const rt = entry('Tri Hạ không khóc.', { fromRuntime: 'rt-1', key: 'rt:rt-1' })
    const b = base({ runtime: [rt] })
    const kept = { ...emptyEdits(), kept: ['rt-1'], ops: 1 }
    expect(changeCount(b, kept)).toBe(1)
    expect(visibleRuntime(b, kept)).toEqual([])
    expect(visibleRuntime(b, emptyEdits())).toEqual([rt])
  })
})

describe('mặc định của Noverse', () => {
  it('4 cụm không dùng và đủ 16 từ dễ lặp mà engine đang dùng', () => {
    expect(DEFAULT_BAN).toHaveLength(4)
    expect(DEFAULT_FATIGUE).toHaveLength(16)
    expect(new Set(DEFAULT_FATIGUE.map(([word]) => word)).size).toBe(16)
  })
  it('cấm ở cấp truyện vẫn giữ nguyên 4 mặc định trong bảng tổng hợp', () => {
    const rows = summarize([entry('Không dùng “bỗng nhiên”.', { chips: [{ kind: 'ban', text: 'bỗng nhiên' }] })])
    const ban = rows.find((row) => row.kind === 'ban')!
    expect(ban.items).toEqual(['“bỗng nhiên”', '+ 4 cụm mặc định'])
  })
  it('nới/đè một từ mặc định thì bớt khỏi “+ N từ mặc định”', () => {
    const rows = summarize([entry('“dường như” tối đa 1 lần.', { chips: [{ kind: 'max', text: 'dường như', limit: 1 }] })])
    expect(rows.find((row) => row.kind === 'max')!.items).toEqual(['“dường như” · 1 lần/chương', '+ 15 từ mặc định'])
  })
  it('thẻ degraded không sinh chip máy, chỉ vào “Lời dặn”', () => {
    const rows = summarize([entry('Đừng viết dài.', { status: 'degraded', chips: degradedChips('Đừng viết dài.') })])
    expect(rows.find((row) => row.kind === 'pref')!.items).toEqual(['Đừng viết dài.'])
    expect(rows.find((row) => row.kind === 'ban')!.items).toEqual(['+ 4 cụm mặc định'])
  })
})

describe('cột phải cấp truyện', () => {
  it('khi nào có hiệu lực', () => {
    expect(whenText('story', { effectiveFrom: 26, inProgress: 25, completed: 24 })).toBe('Quy tắc mới có hiệu lực từ chương 26. Chương 25 đang viết dở giữ quy tắc cũ; 24 chương đã viết không bị sửa.')
    expect(whenText('story', { effectiveFrom: 7, inProgress: 0, completed: 6 })).toBe('Quy tắc mới có hiệu lực từ chương 7. 6 chương đã viết không bị sửa.')
    expect(whenText('project', undefined)).toMatch(/^Có hiệu lực cho truyện mới/)
  })
})
