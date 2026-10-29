import { describe, expect, it } from 'vitest'
import type { PackLayer, PackPreset } from '../../../api/types'
import { chapterWordsText, findPack, layerCounts, packBadge, packCards, packDesc, packInfoMeta, packLabel, packParamChips, packRuleSummary, PARAM_ICON, sectionCount, sectionRef, viNumber } from './packs'

const PACKS: PackPreset[] = [
  {
    id: 'nu-tan-bao-thu', ten: 'Nữ tần trọng sinh báo thù, vả mặt', so_chuong: 8, nguoi_nghe: 'Nữ 18–35',
    short: 'Nữ tần báo thù', meta: '8 chương · thoại 40–55%',
  },
  { id: 'nien-dai-van', ten: 'Niên đại văn', so_chuong: 10, nguoi_nghe: 'Nam trung niên' },
]

describe('D2 › gói thể loại: thẻ và gói đang chọn', () => {
  it('packCards: tên ngắn + `meta` máy chủ soạn, đúng thứ tự máy chủ trả', () => {
    expect(packCards(PACKS).map(({ id, label, meta }) => ({ id, label, meta }))).toEqual([
      { id: 'nu-tan-bao-thu', label: 'Nữ tần báo thù', meta: '8 chương · thoại 40–55%' },
      // Gói chưa có `short`/`meta`: tên rút gọn, không bịa dòng thông số.
      { id: 'nien-dai-van', label: 'Niên đại văn', meta: '' },
    ])
  })

  it('packCards: máy chủ chưa trả `packs` → không có thẻ nào (không bịa gói)', () => {
    expect(packCards(undefined)).toEqual([])
  })

  it('packLabel (Câu 12): `short` thắng; thiếu thì bỏ “Gói ” và phần sau dấu phẩy', () => {
    expect(packLabel({ ten: 'Nữ tần trọng sinh báo thù, vả mặt' })).toBe('Nữ tần trọng sinh báo thù')
    expect(packLabel({ ten: 'Gói Ngược hôn nhân, ly hôn', short: '  ' })).toBe('Ngược hôn nhân')
    expect(packLabel({ ten: 'Niên đại văn', short: 'Niên đại văn' })).toBe('Niên đại văn')
  })

  it('findPack: khớp id gói, lệch/thiếu → null (style đang là kiểu viết)', () => {
    expect(findPack(PACKS, 'nien-dai-van')?.so_chuong).toBe(10)
    expect(findPack(PACKS, 'default')).toBeNull()
    expect(findPack(PACKS, undefined)).toBeNull()
    expect(findPack(undefined, 'nu-tan-bao-thu')).toBeNull()
  })
})

describe('D2 › chi tiết gói (packOpen)', () => {
  const pack = {
    id: 'a', ten: 'Gói A', so_chuong: 8, nguoi_nghe: 'Nữ 18–35', desc: '', hasRules: true, available: true,
    params: [
      { key: 'chapters', text: '8 chương' },
      { key: 'dialogue', text: 'Thoại 40–55%' },
      { key: 'sentence', text: 'Câu ≤ 40 tiếng · đoạn ≤ 90 tiếng' },
    ],
    overrides: { count: 0, keys: [], rows: [] },
  }
  it('badge: có luật + dùng được = “Có chấm”; thiếu luật hoặc chưa có = “Sắp có”', () => {
    expect(packBadge(pack)).toEqual({ label: 'Có chấm', ok: true })
    expect(packBadge({ ...pack, available: false })).toEqual({ label: 'Sắp có', ok: false })
    expect(packBadge({ ...pack, hasRules: false })).toEqual({ label: 'Sắp có', ok: false })
  })
  it('desc rỗng hoặc trùng tên gói thì ẩn dòng mô tả', () => {
    expect(packDesc(pack)).toBe('')
    expect(packDesc({ ...pack, desc: 'Gói A' })).toBe('')
    expect(packDesc({ ...pack, desc: 'Trọng sinh, vả mặt' })).toBe('Trọng sinh, vả mặt')
  })
  it('đếm mục mỗi tầng luật: bắt sửa + cảnh báo; tầng rỗng ghi “dùng khi chấm tự động”', () => {
    expect(layerCounts({ key: 'master', title: 'Master', errors: 43, warnings: 50 }))
      .toEqual([{ text: '43 bắt sửa', tone: 'err' }, { text: '50 cảnh báo', tone: 'wrn' }])
    expect(layerCounts({ key: 'measure', title: 'Định dạng', errors: 0, warnings: 0 }))
      .toEqual([{ text: 'dùng khi chấm tự động', tone: 'neu' }])
  })
  it('số hiệu mục rút về “§N”; hàng thông số lấy nguyên `params` của máy chủ', () => {
    expect(sectionRef('Master §12')).toBe('§12')
    expect(sectionRef('§3')).toBe('§3')
    expect(pack.params).toEqual([
      { key: 'chapters', text: '8 chương' },
      { key: 'dialogue', text: 'Thoại 40–55%' },
      { key: 'sentence', text: 'Câu ≤ 40 tiếng · đoạn ≤ 90 tiếng' },
    ])
    expect(PARAM_ICON.chapters).toBe('book-1')
    expect(PARAM_ICON.sentence).toBe('edit-2')
  })
  it('khối ghi đè văn phong ẩn khi gói không ghi đè khoá `voice.*` nào (Câu 6)', () => {
    expect(pack.overrides).toEqual({ count: 0, keys: [], rows: [] })
  })
  it('Câu 10: “Trợ lý đọc đủ Master + Bộ A” với số mục cộng từ layers, không số phiên bản', () => {
    const master: PackLayer = { key: 'master', title: 'Luật chung · Master', errors: 43, warnings: 50 }
    const boA: PackLayer = { key: 'pack', title: 'Luật thể loại · Bộ A', errors: 22, warnings: 11 }
    expect(packRuleSummary({ ...pack, layers: [master, boA, { key: 'measure', title: 'Cách đo · Định dạng', errors: 0, warnings: 0 }] }))
      .toEqual({ layers: 'Master + Bộ A', counts: '65 mục bắt sửa, 61 mục cảnh báo' })
    // Gói chưa có mục nào / chưa trả layers: giữ tên tầng, bỏ vế số (không bịa 0).
    expect(packRuleSummary({ ...pack, layers: [master] })).toEqual({ layers: 'Master', counts: '43 mục bắt sửa, 50 mục cảnh báo' })
    expect(packRuleSummary({ ...pack, layers: [{ key: 'measure', title: 'Cách đo · Định dạng', errors: 0, warnings: 0 }] }))
      .toEqual({ layers: '', counts: '' })
    expect(packRuleSummary(pack)).toEqual({ layers: '', counts: '' })
  })
})

describe('D2 › số mục từng § trong packOpen', () => {
  const sec = { ref: 'Master §7', title: 'Đầu chương và cuối chương', level: 'error', errors: 3, warnings: 1 }
  it('“bắt sửa / cảnh báo” như artboard, màu theo mức nặng nhất', () => {
    expect(sectionCount(sec)).toEqual({ text: '3 / 1', tone: 'err' })
    expect(sectionCount({ ...sec, level: 'warning', errors: 0, warnings: 3 })).toEqual({ text: '0 / 3', tone: 'wrn' })
  })
  it('mục không có lỗi nào → “—” màu trung tính', () => {
    expect(sectionCount({ ...sec, level: '', errors: 0, warnings: 0 })).toEqual({ text: '—', tone: 'neu' })
  })
})

describe('Câu 1 › độ dài chương của gói', () => {
  const p = { id: 'a', ten: 'Gói A', so_chuong: 8, nguoi_nghe: '', meta: '8 chương · thoại 40–55%', params: [
    { key: 'chapters', text: '8 chương' },
    { key: 'dialogue', text: 'Thoại 40–55%' },
    { key: 'sentence', text: 'Câu ≤ 40 tiếng · đoạn ≤ 90 tiếng' },
  ] }
  it('Settings: “1.400–1.600 tiếng mỗi chương”, số kiểu vi (dấu chấm ngăn nghìn)', () => {
    expect(viNumber(1400)).toBe('1.400')
    expect(chapterWordsText({ min: 1400, max: 1600 })).toBe('1.400–1.600 tiếng mỗi chương')
    expect(chapterWordsText({ min: 900, max: 1200 }, { perChapter: false })).toBe('900–1.200 tiếng')
  })
  it('gói không khai `chapterWords` (null / thiếu / thiếu vế) → chuỗi rỗng, không bịa số', () => {
    expect(chapterWordsText(null)).toBe('')
    expect(chapterWordsText(undefined)).toBe('')
    expect(chapterWordsText({ min: 1400 } as unknown as PackPreset['chapterWords'])).toBe('')
  })
  it('chip độ dài chương đứng ngay sau “N chương”; null thì đúng `params`, không chip rỗng', () => {
    expect(packParamChips({ ...p, chapterWords: { min: 1400, max: 1600 } }).map((chip) => chip.text))
      .toEqual(['8 chương', '1.400–1.600 tiếng mỗi chương', 'Thoại 40–55%', 'Câu ≤ 40 tiếng · đoạn ≤ 90 tiếng'])
    expect(packParamChips({ ...p, chapterWords: null })).toEqual(p.params)
    expect(packParamChips({ chapterWords: { min: 1400, max: 1600 } }))
      .toEqual([{ key: 'chapterWords', text: '1.400–1.600 tiếng mỗi chương' }])
    expect(PARAM_ICON.chapterWords).toBe('document-text')
  })
  it('MMore dòng 2: “8 chương · 1.400–1.600 tiếng · thoại 40–55%”; null thì giữ nguyên `meta`', () => {
    expect(packInfoMeta({ ...p, chapterWords: { min: 1400, max: 1600 } })).toBe('8 chương · 1.400–1.600 tiếng · thoại 40–55%')
    expect(packInfoMeta({ ...p, chapterWords: null })).toBe('8 chương · thoại 40–55%')
    expect(packInfoMeta({ id: 'b', ten: 'Gói B', so_chuong: 10, nguoi_nghe: '', meta: '10 chương · thoại 30–45%' }))
      .toBe('10 chương · thoại 30–45%')
  })
})
