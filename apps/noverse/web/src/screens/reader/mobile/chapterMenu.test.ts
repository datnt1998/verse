import { describe, expect, it } from 'vitest'
import { chapterMenuItems, type ChapterMenuItem } from './chapterMenu'

const labels = (items: ChapterMenuItem[]) => items.map((item) => item.label)

describe('M2 · menu ⋯ của từng chương (MReader chapterMenu)', () => {
  it('người có quyền viết có đủ năm mục theo đúng thứ tự và đúng biểu tượng của artboard', () => {
    const items = chapterMenuItems({ writable: true, exporting: false })
    expect(labels(items)).toEqual(['Đọc chương', 'Nghe chương', 'Nhờ viết lại…', 'Xuất riêng chương này…', 'Chép liên kết chương'])
    expect(items.map((item) => item.icon)).toEqual(['book-1', 'headphone', 'magic-star', 'export-square', 'copy'])
    expect(items.filter((item) => item.accent).map((item) => item.id)).toEqual(['rewrite'])
  })

  it('tài khoản chỉ đọc vẫn chép được liên kết chương nhưng không thấy thao tác viết lại / xuất', () => {
    expect(labels(chapterMenuItems({ writable: false, exporting: false }))).toEqual(['Đọc chương', 'Nghe chương', 'Chép liên kết chương'])
  })

  it('đang xuất thì nhãn mục xuất đổi thành trạng thái, không đổi vị trí', () => {
    const items = chapterMenuItems({ writable: true, exporting: true })
    expect(items[3]).toMatchObject({ id: 'export', label: 'Đang xuất chương…' })
  })
})
