import { describe, expect, it } from 'vitest'
import { budgetUsage, fmtUsd, parseUsd } from './budget'
import { changedFields, projectFormOf } from './info'
import { styleCards } from './style'

describe('Cách viết › thẻ văn phong', () => {
  it('5 thẻ có sẵn đúng thứ tự artboard + thẻ “Văn phong của bạn” khoá khi chưa có tệp tự tạo', () => {
    const cards = styleCards(['default', 'romance', 'suspense', 'fantasy', 'wuxia'], 'default')
    expect(cards.map((card) => card.label)).toEqual(['Chung', 'Ngôn tình', 'Hồi hộp', 'Huyền huyễn', 'Kiếm hiệp', 'Văn phong của bạn'])
    expect(cards[5]).toMatchObject({ id: '', disabled: true })
  })
  it('tệp tự tạo từ presets.styles trở thành thẻ chọn được; tên đang chọn nhưng tệp đã mất vẫn hiện', () => {
    const cards = styleCards(['default', 'giong-tre'], 'default')
    expect(cards.at(-1)).toMatchObject({ id: 'giong-tre', label: 'Văn phong của bạn' })
    expect(cards.at(-1)!.disabled).toBeUndefined()
    expect(styleCards([], 'da-xoa').at(-1)).toMatchObject({ id: 'da-xoa' })
  })
  it('D2: id gói thể loại không bị coi là văn phong tự tạo; gói đang chọn không sinh thẻ thứ 6', () => {
    const cards = styleCards(['default', 'giong-tre'], 'nu-tan-bao-thu', { packs: ['nu-tan-bao-thu'] })
    expect(cards.map((card) => card.id)).toEqual(['default', 'romance', 'suspense', 'fantasy', 'wuxia', 'giong-tre'])
    expect(cards.at(-1)).toMatchObject({ label: 'Văn phong của bạn' })
  })
  it('GAP-35: mô tả thẻ “Văn phong của bạn” lấy số văn mẫu thật, chưa có mẫu thì giữ lời cũ', () => {
    expect(styleCards([], 'default', { samples: { count: 6, chars: 18240 } }).at(-1)!.desc)
      .toBe('Học từ 6 văn mẫu bạn đưa vào · 18.240 chữ')
    expect(styleCards([], 'default', { samples: { count: 0, chars: 0 } }).at(-1)!.desc)
      .toBe('Học từ văn mẫu của bạn (lệnh /simulate)')
  })
  it('Gói AL › custom=blocked (dự án / Mọi truyện): thẻ tự tạo mờ kèm câu dẫn về truyện', () => {
    const cards = styleCards(['default', 'giong-tre'], 'default', { custom: 'blocked' })
    expect(cards.at(-1)).toMatchObject({ id: 'giong-tre', label: 'Văn phong của bạn', disabled: true })
    expect(cards.at(-1)!.desc).toBe('Văn mẫu thêm theo từng truyện ở Dự án & truyện')
    // Thẻ đang chọn giữ khả dụng (không khoá chính lựa chọn hiện tại).
    expect(styleCards(['default'], 'giong-tre', { custom: 'blocked' }).at(-1)!.disabled).toBeUndefined()
  })
  it('Gói AL › custom=story: chưa có văn mẫu thì mờ kèm “Thêm văn mẫu ở bên dưới”, có mẫu thì chọn được', () => {
    const none = styleCards(['default', 'giong-tre'], 'default', { custom: 'story', samples: { count: 0, chars: 0 } }).at(-1)!
    expect(none.disabled).toBe(true)
    expect(none.desc).toBe('Thêm văn mẫu ở bên dưới để dùng')
    const ready = styleCards(['default', 'giong-tre'], 'default', { custom: 'story', samples: { count: 3, chars: 9380 } }).at(-1)!
    expect(ready.disabled).toBeUndefined()
    expect(ready.desc).toBe('Học từ 3 văn mẫu của truyện này · 9.380 chữ')
    // Thẻ đang chọn giữ khả dụng kể cả khi văn mẫu đã bị xoá.
    expect(styleCards(['default', 'giong-tre'], 'giong-tre', { custom: 'story', samples: { count: 0, chars: 0 } }).at(-1)!.disabled).toBeUndefined()
  })
})

describe('Ngân sách', () => {
  it('đọc số tiền: dấu phẩy, rỗng = 0, rác = null', () => {
    expect(parseUsd('10,5')).toBe(10.5)
    expect(parseUsd('')).toBe(0)
    expect(parseUsd('abc')).toBeNull()
    expect(parseUsd('-3')).toBeNull()
    expect(fmtUsd(3.2)).toBe('$3,20')
  })
  it('thẻ đã dùng: ước số chương còn lại = (giới hạn − đã dùng) / trung bình mỗi chương; vàng khi chạm mức báo trước', () => {
    const usage = budgetUsage({ totalCostUSD: 3.2, completedCount: 25 }, 10, 0.8)
    expect(usage.perChapter).toBeCloseTo(0.128)
    expect(usage.chaptersLeft).toBe(53)
    expect(usage.pct).toBe(32)
    expect(usage.warn).toBe(false)
    expect(budgetUsage({ totalCostUSD: 8.5, completedCount: 25 }, 10, 0.8).warn).toBe(true)
  })
  it('không giới hạn: thanh 0%, không ước số chương', () => {
    expect(budgetUsage({ totalCostUSD: 3.2, completedCount: 25 }, 0, 0.8)).toMatchObject({ pct: 0, warn: false, chaptersLeft: null })
    expect(budgetUsage(null, 10, 0.8).perChapter).toBeNull()
  })
})

describe('Thông tin', () => {
  it('đếm ô đã đổi', () => {
    const base = projectFormOf({ name: 'A', description: '', notes: '' })
    expect(changedFields(base, base)).toBe(0)
    expect(changedFields({ ...base, name: 'B', notes: 'Mới' }, base)).toBe(2)
  })
  it('dự án: tên vũ trụ tuỳ chọn, thiếu thì rỗng (= dùng tên dự án)', () => {
    const project = { name: 'Kênh truyện audio', description: '', notes: '' }
    expect(projectFormOf({ ...project, universeName: 'Thành phố Vân Châu' }).universeName).toBe('Thành phố Vân Châu')
    expect(projectFormOf(project).universeName).toBe('')
    const base = projectFormOf(project)
    expect(changedFields({ ...base, universeName: 'Thành phố Vân Châu' }, base)).toBe(1)
  })
})
