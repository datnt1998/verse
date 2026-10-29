// 04-components.md:104-109 — bảng luật avatar nhân vật, mỗi ví dụ của spec một dòng.
import { describe, expect, it } from 'vitest'
import { charInitials, roleText, speakerRoleLabel, splitCastLabel } from './cast'

describe('charInitials — luật avatar (04-components.md › Avatar nhân vật)', () => {
  it.each([
    // 1 chữ → chữ cái đầu
    ['Lụa', 'L'],
    // 2 chữ → chữ đầu cả hai
    ['Lâm An', 'LA'],
    ['Phùng Chấn', 'PC'],
    ['Đỗ Mẫn', 'ĐM'],
    ['Hạ Vy', 'HV'],
    ['Trần Duy', 'TD'],
    // 3 chữ trở lên (không danh xưng) → bỏ họ, 2 chữ cuối
    ['Lâm Tri Hạ', 'TH'],
    ['Tống Nhược Vy', 'NV'],
    // Danh xưng → đầu danh xưng + đầu chữ cuối
    ['Luật sư Hàn', 'LH'],
    ['Ông Ba Phi', 'ÔP'],
    ['giám đốc Hùng', 'GH'],
    // Tên là nhãn chức năng vẫn theo luật chung
    ['Dẫn truyện', 'DT'],
    ['Nhân vật phụ', 'VP'],
    ['Giọng đọc chung', 'ĐC'],
  ])('%s → %s', (name, expected) => {
    expect(charInitials(name)).toBe(expected)
  })

  it('ini người dùng tự đặt thắng mọi luật', () => {
    expect(charInitials('Tống Nhược Vy', 'VÂ')).toBe('VÂ')
    expect(charInitials('Lâm Tri Hạ', '  ')).toBe('TH')
    expect(charInitials('Lâm Tri Hạ', null)).toBe('TH')
  })

  it('khớp tên cùng dòng: chữ trong avatar lấy từ tên, không lấy từ vai trong ngoặc', () => {
    // Lỗi “TD nằm ở dòng Hạ Vy”: gốc là tính từ chuỗi “Tên (vai)” — phải tách trước khi tính.
    const row = splitCastLabel('Hạ Vy (vợ Trần Duy)')
    expect(row).toEqual({ name: 'Hạ Vy', role: 'vợ Trần Duy' })
    expect(charInitials(row.name)).toBe('HV')
    expect(charInitials(splitCastLabel('Trần Duy (em của Hạ Vy)').name)).toBe('TD')
  })

  it('cạnh rỗng/lẻ', () => {
    expect(charInitials('')).toBe('?')
    expect(charInitials('   ')).toBe('?')
    expect(charInitials('Bà')).toBe('B')
  })
})

describe('splitCastLabel — tách “Tên (vai; …)” cũ', () => {
  it('ngoặc thường và ngoặc rộng', () => {
    expect(splitCastLabel('Lâm An (Nữ chính; người kể xưng tôi)')).toEqual({ name: 'Lâm An', role: 'Nữ chính · người kể xưng tôi' })
    expect(splitCastLabel('Kiều My（tác giả mới; người ký sai hợp đồng）')).toEqual({ name: 'Kiều My', role: 'tác giả mới · người ký sai hợp đồng' })
  })

  it('không ngoặc / ngoặc rỗng / chỉ có ngoặc', () => {
    expect(splitCastLabel('Trần Duy')).toEqual({ name: 'Trần Duy', role: '' })
    expect(splitCastLabel('Trần Duy ()')).toEqual({ name: 'Trần Duy', role: '' })
    expect(splitCastLabel('(chỉ vai)')).toEqual({ name: '(chỉ vai)', role: '' })
  })

  it('tên có ngoặc, vai ở nhóm cuối', () => {
    expect(splitCastLabel('Hạ Vy (Mẹ Duy) (vợ)')).toEqual({ name: 'Hạ Vy (Mẹ Duy)', role: 'vợ' })
  })
})

describe('roleText', () => {
  it('chuẩn hoá “;” thành “ · ” và cắt ý rỗng', () => {
    expect(roleText('Nữ chính; người kể xưng tôi')).toBe('Nữ chính · người kể xưng tôi')
    expect(roleText('Nữ chính · người kể xưng tôi')).toBe('Nữ chính · người kể xưng tôi')
    expect(roleText('')).toBe('')
  })
})

describe('speakerRoleLabel — vai của người nói trong bản đọc', () => {
  // internal/audio/cast.go gửi slug tiếng Anh; UI chỉ được hiện nhãn tiếng Việt.
  it('dịch ba vai của bản đọc theo bảng thuật ngữ', () => {
    expect(speakerRoleLabel('narrator')).toBe('Dẫn truyện')
    expect(speakerRoleLabel('main')).toBe('Nhân vật chính')
    expect(speakerRoleLabel('support')).toBe('Phụ trợ')
  })

  it('giữ nguyên câu tiếng Việt, kể cả phần sau dấu “ · ”', () => {
    expect(speakerRoleLabel('Nữ chính')).toBe('Nữ chính')
    expect(speakerRoleLabel('Nữ chính · lời thoại')).toBe('Nữ chính · lời thoại')
    expect(speakerRoleLabel('')).toBe('')
  })
})
