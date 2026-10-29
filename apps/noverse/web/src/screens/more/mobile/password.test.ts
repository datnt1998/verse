import { describe, expect, it } from 'vitest'
import { passwordChecklist, passwordStrength } from './passwordLogic'

describe('danh sách điều kiện mật khẩu điện thoại', () => {
  it('cập nhật đủ bốn điều kiện, không chấp nhận ô trống hay mật khẩu cũ', () => {
    expect(passwordChecklist('', '', '').map((rule) => rule.ok)).toEqual([false, false, false, false])
    expect(passwordChecklist('MatKhau1', 'Abc1234', 'Abc1234')[0].ok).toBe(false)
    expect(passwordChecklist('MatKhau1', 'MatKhau1', 'MatKhau1').map((rule) => rule.ok)).toEqual([true, true, false, true])
    expect(passwordChecklist('MatKhau1', 'abcdefghi', 'khac').map((rule) => rule.ok)).toEqual([true, false, true, false])
    expect(passwordChecklist('MatKhau1', 'Moi-2026', 'Moi-2026').every((rule) => rule.ok)).toBe(true)
    expect(passwordChecklist('Tam-4821', 'Moi-2026', 'Moi-2026', true)[2].label).toBe('Khác mật khẩu tạm')
  })
})

describe('độ mạnh cùng công thức máy tính', () => {
  it('giữ ngưỡng tám ký tự rồi tính độ dài, ký tự đặc biệt, chữ và số', () => {
    expect(passwordStrength('').score).toBe(0)
    expect(passwordStrength('A1!').score).toBe(0)
    expect(passwordStrength('abcdefgh').score).toBe(1)
    expect(passwordStrength('abcdefg1').score).toBe(2)
    expect(passwordStrength('abcdefg1!').score).toBe(3)
    expect(passwordStrength('TraXanh-2026!')).toMatchObject({ score: 4, label: 'Rất mạnh' })
  })
})
