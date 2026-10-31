import { describe, expect, it } from 'vitest'
import { isProjectNameTaken, newProjectContents, NEW_PROJECT_NAME_CLASH, canCopyProjectRules, projectCreationInput } from './NewProjectDialog'

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

  it('không cho chọn dự án không có quy tắc', () => {
    expect(canCopyProjectRules({ ruleCount: 0 })).toBe(false)
    expect(canCopyProjectRules({ ruleCount: 5 })).toBe(true)
    expect(canCopyProjectRules({})).toBe(false)
  })

  it('chỉ gửi nguồn chép khi đã chọn dự án có quy tắc', () => {
    const source = { id: 'nguon', ruleCount: 5 }
    expect(projectCreationInput('  Mới ', ' ', true, source)).toEqual({ name: 'Mới', shareGlobal: true, copyRulesFrom: 'nguon' })
    expect(projectCreationInput('Mới', ' mô tả ', false, null)).toEqual({ name: 'Mới', description: 'mô tả', shareGlobal: false })
    expect(projectCreationInput('Mới', '', true, { id: 'rong', ruleCount: 0 })).toEqual({ name: 'Mới', shareGlobal: true })
  })

  it('tóm tắt theo nguồn quy tắc và trợ lý', () => {
    const shared = newProjectContents(true, null)
    expect(shared.map((item) => item.name)).toEqual(['Ý tưởng', 'Tư liệu', 'Truyện', 'Quy tắc viết', 'Văn phong', 'Trợ lý AI'])
    expect(shared[3].note).toBe('trống')
    expect(shared[4].note).toBe('Chung · mặc định')
    expect(shared[5].note).toBe('theo “Mọi truyện”')
    expect(newProjectContents(false, { ruleCount: 5, style: 'Trữ tình' })[3].note).toBe('chép 5 quy tắc')
    expect(newProjectContents(false, { ruleCount: 5, style: 'Trữ tình' })[4].note).toBe('Trữ tình')
    expect(newProjectContents(false, null)[5].note).toBe('bản riêng')
  })
})
