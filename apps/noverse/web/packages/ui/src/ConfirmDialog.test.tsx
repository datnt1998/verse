import { renderToStaticMarkup } from 'react-dom/server'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { ConfirmDialog, HOLD_DURATION, matchesConfirmation, scheduleConfirmationHold } from './ConfirmDialog'

afterEach(() => vi.useRealTimers())

describe('gõ để xác nhận', () => {
  it('chỉ chấp nhận đúng tên thư mục, cho phép khoảng trắng ngoài', () => {
    expect(matchesConfirmation('01-tra-xanh', '01-tra-xanh')).toBe(true)
    expect(matchesConfirmation(' 01-tra-xanh ', '01-tra-xanh')).toBe(true)
    expect(matchesConfirmation('01-tra-xan', '01-tra-xanh')).toBe(false)
    expect(matchesConfirmation('01-TRA-XANH', '01-tra-xanh')).toBe(false)
    expect(matchesConfirmation('', '01-tra-xanh')).toBe(false)
  })
  it('nút xoá bị khoá khi chưa gõ, hộp được gắn nhãn và mô tả', () => {
    const html = renderToStaticMarkup(<ConfirmDialog inline title="Xoá truyện?" body="Xoá vĩnh viễn 24 chương." tone="danger" confirmLabel="Xoá vĩnh viễn" phrase="01-tra-xanh" onConfirm={() => {}} onClose={() => {}} />)
    expect(html).toContain('role="alertdialog"')
    expect(html).toContain('aria-labelledby=')
    expect(html).toContain('aria-describedby=')
    expect(html).toContain('disabled=""')
    expect(html).toMatch(/Type <span class="mono"[^>]*>01-tra-xanh<\/span> to confirm/)
  })
})

describe('giữ để xác nhận', () => {
  it('không chạy ở 1199ms, chạy đúng một lần tại 1200ms', () => {
    vi.useFakeTimers()
    const complete = vi.fn()
    scheduleConfirmationHold(complete)
    vi.advanceTimersByTime(1199)
    expect(complete).not.toHaveBeenCalled()
    vi.advanceTimersByTime(1)
    expect(complete).toHaveBeenCalledTimes(1)
    vi.advanceTimersByTime(HOLD_DURATION * 2)
    expect(complete).toHaveBeenCalledTimes(1)
  })
  it('nhả sớm huỷ toàn bộ thời gian giữ, lần giữ sau bắt đầu lại', () => {
    vi.useFakeTimers()
    const complete = vi.fn()
    const release = scheduleConfirmationHold(complete)
    vi.advanceTimersByTime(900)
    release()
    vi.advanceTimersByTime(1000)
    expect(complete).not.toHaveBeenCalled()
    const cancel = scheduleConfirmationHold(complete)
    vi.advanceTimersByTime(1199)
    expect(complete).not.toHaveBeenCalled()
    vi.advanceTimersByTime(1)
    expect(complete).toHaveBeenCalledTimes(1)
    cancel()
  })
})
