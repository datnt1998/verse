import { describe, expect, it } from 'vitest'
import { isProjectNameTaken, newProjectContents, NEW_PROJECT_NAME_CLASH } from './NewProjectDialog'

describe('Dự án mới · trùng tên và danh sách có sẵn', () => {
  const projects = [{ name: 'Kênh truyện audio' }, { name: '_', legacy: true }]

  it('trùng tên không phân biệt hoa thường và khoảng trắng; dự án ẩn danh không tính', () => {
    expect(isProjectNameTaken('  KÊNH truyện AUDIO ', projects)).toBe(true)
    expect(isProjectNameTaken('Kênh truyện', projects)).toBe(false)
    expect(isProjectNameTaken('_', projects)).toBe(false)
    expect(isProjectNameTaken('   ', projects)).toBe(false)
  })

  it('báo lỗi bằng lời thường, không nhắc thư mục', () => {
    expect(NEW_PROJECT_NAME_CLASH).toBe('Đã có dự án cùng tên — đặt tên khác nhé')
  })

  it('danh sách có sẵn: năm mục theo artboard, ghi chú “Quy tắc viết” và “Trợ lý AI” đổi theo ô dùng chung', () => {
    const shared = newProjectContents(true)
    expect(shared.map((item) => item.name)).toEqual(['Ý tưởng', 'Tư liệu', 'Truyện', 'Quy tắc viết', 'Trợ lý AI'])
    expect(shared[3].note).toBe('chép từ “Mọi truyện”')
    expect(shared[4].note).toBe('theo “Mọi truyện”')
    expect(newProjectContents(false)[3].note).toBe('trống')
    expect(newProjectContents(false)[4].note).toBe('riêng dự án')
    expect(JSON.stringify(shared)).not.toMatch(/y-tuong|tai-san|kich_ban|\.json|\//)
  })
})
