import { describe, expect, it } from 'vitest'
import { ApiError } from '../../api/client'
import { recoveryFailure, recoveryReady } from './recovery'

describe('recoveryReady', () => {
  it('cần đủ 16 ký tự chữ-số (không tính dấu gạch) và mật khẩu từ 8 ký tự', () => {
    expect(recoveryReady('TRAX-8K2M-QP7D-41ZN', 'matkhau-moi')).toBe(true)
    expect(recoveryReady('trax8k2mqp7d41zn', 'matkhau1')).toBe(true)
    expect(recoveryReady('TRAX-8K2M-QP7D-41Z', 'matkhau-moi')).toBe(false)
    expect(recoveryReady('TRAX-8K2M-QP7D-41ZN', 'ngan')).toBe(false)
  })

  it('đếm mật khẩu theo ký tự như máy chủ, không theo UTF-16', () => {
    expect(recoveryReady('TRAX-8K2M-QP7D-41ZN', '🔑🔑🔑🔑🔑🔑🔑🔑')).toBe(true)
    expect(recoveryReady('TRAX-8K2M-QP7D-41ZN', '🔑🔑🔑🔑🔑🔑🔑')).toBe(false)
  })
})

describe('recoveryFailure', () => {
  it('401 → mã không đúng hoặc đã dùng', () => {
    expect(recoveryFailure(new ApiError(401, 'x'))).toEqual({ kind: 'code', message: 'Mã khôi phục không đúng hoặc đã dùng' })
  })

  it('400 → mật khẩu quá ngắn', () => {
    expect(recoveryFailure(new ApiError(400, 'x'))).toEqual({ kind: 'password', message: 'Mật khẩu cần ít nhất 8 ký tự' })
  })

  it('429 → giới hạn thử, mang theo retryAfter (mặc định 60 giây)', () => {
    expect(recoveryFailure(new ApiError(429, 'x', 42))).toMatchObject({ kind: 'limited', retryAfter: 42 })
    expect(recoveryFailure(new ApiError(429, 'x'))).toMatchObject({ kind: 'limited', retryAfter: 60 })
  })

  it('lỗi khác giữ nguyên thông báo của máy chủ; lỗi mạng báo mất kết nối', () => {
    expect(recoveryFailure(new ApiError(500, 'Lỗi máy chủ'))).toEqual({ kind: 'message', message: 'Lỗi máy chủ' })
    expect(recoveryFailure(new TypeError('x'))).toEqual({ kind: 'message', message: 'Không kết nối được máy chủ. Vui lòng thử lại.' })
  })
})
