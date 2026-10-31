import { describe, expect, it } from 'vitest'
import { premiseRows, groupWorldRules } from './foundation'

describe('story foundation data', () => {
  it('keeps multiline and custom sections while restoring a missing row to template order', () => {
    const sections = [{ heading: 'Thể loại và giọng điệu', text: 'Một giọng kể.\n\nHai dòng.' }, { heading: 'Ghi chú riêng', text: 'Không làm mất mục này.' }, { heading: 'Cam kết cốt lõi', text: 'Lời hứa.' }]
    const rows = premiseRows(sections, ['Vùng cấm sáng tác'])
    expect(rows[0].text).toBe('Một giọng kể.\n\nHai dòng.')
    expect(rows[1]).toEqual({ heading: 'Vùng cấm sáng tác', text: '', missing: true })
    expect(rows.at(-1)?.text).toBe('Không làm mất mục này.')
  })
  it('uses only server missing requirements, never inferring a tier from chapter count', () => {
    expect(premiseRows([], ['Động cơ câu chuyện', 'Chuyển hướng trung kỳ'])).toEqual([
      { heading: 'Động cơ câu chuyện', text: '', missing: true }, { heading: 'Chuyển hướng trung kỳ', text: '', missing: true },
    ])
    expect(premiseRows([{ heading: 'Móc câu khác biệt', text: 'Có nội dung.' }], [])).toEqual([{ heading: 'Móc câu khác biệt', text: 'Có nội dung.', missing: false }])
  })
  it('preserves category ordering, boundaries and an empty world', () => {
    expect(groupWorldRules([])).toEqual([])
    const grouped = groupWorldRules([{ category: 'magic', rule: 'Có phép.', boundary: 'Chỉ ban ngày.' }, { category: 'society', rule: 'Có luật.', boundary: 'Không ngoại lệ.' }, { category: 'unknown', rule: 'Khác.', boundary: '' }])
    expect(grouped.map((group) => group.category)).toEqual(['society', 'magic', 'other'])
    expect(grouped[1].rules[0].boundary).toBe('Chỉ ban ngày.')
  })
})
