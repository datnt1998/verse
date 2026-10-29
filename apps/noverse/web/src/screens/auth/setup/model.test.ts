// A0 · Setup.dc.html: mã 8 ký tự, khoá 5 lần sai / 10 phút, hết hạn 30 phút, tài khoản quản trị.
import { describe, expect, it } from 'vitest'
import {
  adminFailure, canCreateAdmin, clockOf, codeBoxes, formatCode, lockClock, minutesLeft, normalizeCode,
  setupStepIndex, usernameSuggestion, usernameValid, verifyFailure,
} from './model'

describe('Mã thiết lập 8 ký tự', () => {
  it('gõ hay dán đều chuẩn hoá: bỏ gạch/khoảng trắng, viết hoa, cắt 8 ký tự', () => {
    expect(normalizeCode('k7q2-9xmb')).toBe('K7Q29XMB')
    expect(normalizeCode('  k7q2 9xmb  ')).toBe('K7Q29XMB')
    expect(normalizeCode('k7q2-9xmb-zzz')).toBe('K7Q29XMB')
    expect(formatCode('k7q29xmb')).toBe('K7Q2-9XMB')
    expect(formatCode('k7q2')).toBe('K7Q2')
  })

  it('8 ô: gạch sau ô 4, ô kế tiếp là “cur”, đủ 8 thì không còn ô cur', () => {
    const partial = codeBoxes('K7Q2')
    expect(partial).toHaveLength(8)
    expect(partial[3]).toMatchObject({ ch: '2', dash: true })
    expect(partial[4]).toMatchObject({ ch: '', cls: 'cur' })
    expect(codeBoxes('K7Q29XMB').some((box) => box.cls === 'cur')).toBe(false)
  })

  it('mã sai: cả 8 ô viền đỏ (artboard codeWrong); đang kiểm tra thì không có ô cur', () => {
    expect(codeBoxes('K7Q29XMB', { bad: true }).every((box) => box.cls === 'bad')).toBe(true)
    expect(codeBoxes('K7Q29XMB', { bad: true, checking: true }).every((box) => box.cls === 'bad')).toBe(true)
    expect(codeBoxes('K7Q2', { checking: true }).every((box) => box.cls === '')).toBe(true)
  })
})

describe('Tên đăng nhập gợi ý từ tên hiển thị', () => {
  it('bỏ dấu tiếng Việt, đ → d, ký tự lạ thành dấu chấm, tối đa 32', () => {
    expect(usernameSuggestion('Đạt')).toBe('dat')
    expect(usernameSuggestion('Nguyễn Văn Đạt')).toBe('nguyen.van.dat')
    expect(usernameSuggestion('  Trà Xanh  ')).toBe('tra.xanh')
    expect(usernameSuggestion('a'.repeat(40))).toHaveLength(32)
  })

  it('2–32 ký tự, chữ thường không dấu, số, dấu chấm/gạch', () => {
    expect(usernameValid('dat')).toBe(true)
    expect(usernameValid('nguyen.van_dat-2')).toBe(true)
    expect(usernameValid('d')).toBe(false)
    expect(usernameValid('Đạt')).toBe(false)
    expect(usernameValid('dat ')).toBe(false)
  })
})

describe('Nút “Tạo tài khoản và tiếp tục”', () => {
  const base = { name: 'Đạt', username: 'dat', password: 'matkhau-123', repeat: 'matkhau-123' }
  it('bật khi đủ tên, tên đăng nhập hợp lệ và hai mật khẩu giống nhau (≥ 8 ký tự)', () => {
    expect(canCreateAdmin(base)).toBe(true)
    expect(canCreateAdmin({ ...base, name: '   ' })).toBe(false)
    expect(canCreateAdmin({ ...base, username: 'Đạt' })).toBe(false)
    expect(canCreateAdmin({ ...base, password: 'ngan', repeat: 'ngan' })).toBe(false)
    expect(canCreateAdmin({ ...base, repeat: 'khac-123' })).toBe(false)
  })
})

describe('Mã lỗi của /api/setup/verify → trạng thái artboard', () => {
  it('400 còn lượt → codeWrong + số lần thử', () => {
    expect(verifyFailure(400, { error: 'x', attemptsLeft: 4 })).toEqual({ state: 'codeWrong', attemptsLeft: 4 })
  })
  it('400 hết lượt (0) → codeLocked', () => {
    expect(verifyFailure(400, { attemptsLeft: 0 })).toEqual({ state: 'codeLocked', lockedUntil: '' })
  })
  it('400 mã sai định dạng ({format:true}) → codeFormat, không phải lượt thử nên không khoá', () => {
    expect(verifyFailure(400, { error: 'Mã thiết lập sai định dạng', format: true })).toEqual({ state: 'codeFormat' })
    // Không có `format` thì vẫn là lượt sai bình thường.
    expect(verifyFailure(400, { error: 'x', attemptsLeft: 3, format: false })).toEqual({ state: 'codeWrong', attemptsLeft: 3 })
  })
  it('423 → codeLocked kèm giờ mở lại; 410 → codeExpired; 409 → taken kèm mốc', () => {
    expect(verifyFailure(423, { lockedUntil: '2026-10-25T14:14:00Z' })).toEqual({ state: 'codeLocked', lockedUntil: '2026-10-25T14:14:00Z' })
    expect(verifyFailure(410, {})).toEqual({ state: 'codeExpired' })
    expect(verifyFailure(409, { at: '2026-10-25T13:04:00Z' })).toEqual({ state: 'taken', at: '2026-10-25T13:04:00Z' })
  })
  it('lỗi khác trả null để màn hình hiện thông điệp máy chủ', () => {
    expect(verifyFailure(500, {})).toBeNull()
  })
})

describe('Mã lỗi của /api/setup/admin', () => {
  it('410 → tokenExpired (giữ nguyên dữ liệu đã điền); 409 → taken; 400 → lỗi theo trường', () => {
    expect(adminFailure(410, {})).toEqual({ state: 'tokenExpired' })
    expect(adminFailure(409, { at: '2026-10-25T13:04:00Z' })).toEqual({ state: 'taken', at: '2026-10-25T13:04:00Z' })
    expect(adminFailure(400, { field: 'username' }, 'Tên đăng nhập này đã có người dùng'))
      .toEqual({ state: 'field', field: 'username', message: 'Tên đăng nhập này đã có người dùng' })
    expect(adminFailure(400, { field: 'lạ' }, 'x')).toMatchObject({ state: 'field', field: 'name' })
  })
})

describe('Thời gian bằng lời', () => {
  it('còn bao nhiêu phút để tạo tài khoản (làm tròn lên)', () => {
    const now = new Date('2026-10-25T13:00:00Z').getTime()
    expect(minutesLeft('2026-10-25T13:27:00Z', now)).toBe(27)
    expect(minutesLeft('2026-10-25T13:00:30Z', now)).toBe(1)
    expect(minutesLeft('2026-10-25T12:00:00Z', now)).toBe(0)
    expect(minutesLeft(undefined, now)).toBe(0)
  })

  it('mốc “Thử lại lúc 21:44” là giờ:phút; thiếu mốc thì lấy bây giờ + 10 phút', () => {
    const date = new Date('2026-10-25T14:14:00Z')
    expect(lockClock(date.toISOString())).toBe(`${String(date.getHours()).padStart(2, '0')}:${String(date.getMinutes()).padStart(2, '0')}`)
    const now = new Date('2026-10-25T13:00:00Z').getTime()
    const fallback = new Date(now + 10 * 60_000)
    expect(lockClock('', now)).toBe(`${String(fallback.getHours()).padStart(2, '0')}:${String(fallback.getMinutes()).padStart(2, '0')}`)
    expect(lockClock('không phải ngày', now)).toBe(lockClock('', now))
  })

  it('mốc “lúc 21:04” trong thẻ taken', () => {
    const date = new Date('2026-10-25T21:04:00Z')
    expect(clockOf(date.toISOString())).toBe(`${String(date.getHours()).padStart(2, '0')}:${String(date.getMinutes()).padStart(2, '0')}`)
    expect(clockOf('không phải ngày')).toBe('')
  })
})

describe('Thanh 3 bước', () => {
  it('mọi trạng thái của mã ở bước 1; tài khoản/taken ở bước 2', () => {
    expect(setupStepIndex('code')).toBe(0)
    expect(setupStepIndex('codeWrong')).toBe(0)
    expect(setupStepIndex('codeLocked')).toBe(0)
    expect(setupStepIndex('codeExpired')).toBe(0)
    expect(setupStepIndex('account')).toBe(1)
    expect(setupStepIndex('creating')).toBe(1)
    expect(setupStepIndex('tokenExpired')).toBe(1)
    expect(setupStepIndex('taken')).toBe(1)
  })
})
