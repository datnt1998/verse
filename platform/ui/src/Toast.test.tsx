import { renderToStaticMarkup } from 'react-dom/server'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { dismissToast, toast, ToastHost } from './Toast'

beforeEach(() => { vi.useFakeTimers() })
afterEach(() => { vi.runOnlyPendingTimers(); vi.runOnlyPendingTimers(); vi.useRealTimers(); vi.unstubAllGlobals() })

describe('keyed toast updates', () => {
  it('reuses the same toast and restarts its expiry timer', () => {
    vi.useFakeTimers()
    const first = toast('Lỗi lần đầu', 'err', undefined, { key: 'command-error', duration: 1000 })
    vi.advanceTimersByTime(800)
    const next = toast('Lỗi mới nhất', 'err', undefined, { key: 'command-error', duration: 1000 })
    expect(next).toBe(first)
    vi.advanceTimersByTime(300)
    expect(toast('Cập nhật', 'err', undefined, { key: 'command-error' })).toBe(first)
    dismissToast(first)
  })

  it('does not merge unkeyed notices or different keys', () => {
    const one = toast('Đã gửi')
    const two = toast('Đã gửi')
    const three = toast('Đã gửi', 'ok', undefined, { key: 'other' })
    expect(new Set([one, two, three]).size).toBe(3)
    ;[one, two, three].forEach(dismissToast)
  })
})

describe('toast lifetime and motion', () => {
  it('starts undo exit at exactly five seconds, retaining its animated shell for 520 ms', () => {
    vi.useFakeTimers()
    vi.stubGlobal('window', { matchMedia: () => ({ matches: false }) })
    const id = toast('Đã xoá để hoàn tác', 'ok', { label: 'Hoàn tác', onClick: () => {} }, { undo: true, duration: 6000 })
    expect(renderToStaticMarkup(<ToastHost />)).toContain('ui-toast-in')
    vi.advanceTimersByTime(4999)
    expect(renderToStaticMarkup(<ToastHost />)).not.toContain('ui-toast-leaving')
    vi.advanceTimersByTime(1)
    expect(renderToStaticMarkup(<ToastHost />)).toContain('ui-toast-leaving')
    expect(renderToStaticMarkup(<ToastHost />)).toContain('Đã xoá để hoàn tác')
    vi.advanceTimersByTime(520)
    expect(renderToStaticMarkup(<ToastHost />)).not.toContain('Đã xoá để hoàn tác')
    dismissToast(id)
  })
})

describe('activity toast host', () => {
  it('shows only the newest three notifications', () => {
    vi.stubGlobal('window', { matchMedia: () => ({ matches: false }) })
    const ids = ['Một', 'Hai', 'Ba', 'Bốn'].map((text) => toast(text))
    const html = renderToStaticMarkup(<ToastHost activity />)
    expect(html).not.toContain('Một')
    expect(html).toContain('Hai')
    expect(html).toContain('Ba')
    expect(html).toContain('Bốn')
    ids.forEach(dismissToast)
  })
  it('shows optional detail and merge metadata without changing the default alert role', () => {
    vi.stubGlobal('window', { matchMedia: () => ({ matches: false }) })
    const id = toast.card({ title: '3 video chờ duyệt', text: 'Kênh 3', meta: 'Gộp 3 thông báo', tone: 'warn', action: { label: 'Duyệt ảnh', onClick: () => {} }, secondaryAction: { label: 'Mở video', onClick: () => {} } })
    const activity = renderToStaticMarkup(<ToastHost activity />)
    expect(activity).toContain('Kênh 3')
    expect(activity).toContain('Gộp 3 thông báo')
    expect(activity).toContain('Mở video')
    expect(activity).toContain('role="status"')
    expect(renderToStaticMarkup(<ToastHost />)).toContain('role="alert"')
    dismissToast(id)
  })
  it('dismisses a timed activity card after its requested duration', () => {
    vi.stubGlobal('window', { matchMedia: () => ({ matches: false }) })
    toast.card({ title: 'Việc đã xong', action: { label: 'Xem', onClick: () => {} }, duration: 5000 })
    vi.advanceTimersByTime(4999)
    expect(renderToStaticMarkup(<ToastHost activity />)).toContain('Việc đã xong')
    vi.advanceTimersByTime(1)
    expect(renderToStaticMarkup(<ToastHost activity />)).toContain('ui-toast-leaving')
  })
})
