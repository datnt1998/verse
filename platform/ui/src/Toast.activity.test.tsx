// @vitest-environment happy-dom
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { dismissToast, dismissToasts, toast, setToastDuration, ToastHost } from './Toast'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })
let host: HTMLDivElement
let root: Root
beforeEach(async () => {
  vi.useFakeTimers()
  vi.stubGlobal('matchMedia', () => ({ matches: false, addEventListener() {}, removeEventListener() {} }))
  host = document.createElement('div'); document.body.append(host); root = createRoot(host)
  await act(async () => root.render(<ToastHost activity />))
})
afterEach(async () => { setToastDuration(undefined); await act(async () => { dismissToasts(); root.unmount(); vi.runAllTimers() }); host.remove(); vi.useRealTimers(); vi.unstubAllGlobals() })
it('pauses a toast countdown while hovered and resumes it afterwards', async () => {
  await act(async () => { toast('Đã lưu', 'ok', undefined, { duration: 5000 }) })
  await act(async () => { vi.advanceTimersByTime(2000); host.querySelector('.ui-toast-wrap')!.dispatchEvent(new MouseEvent('mouseover', { bubbles: true })); vi.advanceTimersByTime(6000) })
  expect(host.textContent).toContain('Đã lưu')
  await act(async () => { host.querySelector('.ui-toast-wrap')!.dispatchEvent(new MouseEvent('mouseout', { bubbles: true })); vi.advanceTimersByTime(3520) })
  expect(host.textContent).not.toContain('Đã lưu')
})
it('closes the newest toast on Escape', async () => {
  await act(async () => { toast('Một'); toast('Hai'); window.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' })); vi.advanceTimersByTime(520) })
  expect(host.textContent).toContain('Một')
  expect(host.textContent).not.toContain('Hai')
})
it('drops every older pill once the host renders', async () => {
  await act(async () => { toast('Cũ', 'ok', undefined, { duration: 60000 }); toast('Mới nhất', 'ok', undefined, { duration: 60000 }) })
  await act(async () => { vi.advanceTimersByTime(600) })
  await act(async () => {})
  expect(host.textContent).not.toContain('Cũ')
  expect(host.textContent).toContain('Mới nhất')
})
it('retains both job cards within the desktop stack capacity', async () => {
  await act(async () => {
    toast.card({ title: 'Thẻ một', tone: 'warn', action: { label: 'Mở', onClick: () => {} }, duration: 60000 })
    toast.card({ title: 'Thẻ hai', tone: 'ok', action: { label: 'Mở', onClick: () => {} }, duration: 60000 })
    vi.advanceTimersByTime(20)
  })
  expect(host.textContent).toContain('Thẻ một')
  expect(host.textContent).toContain('Thẻ hai')
})
it('drops the oldest non-attention card beyond two', async () => {
  await act(async () => {
    toast.card({ title: 'Thẻ cũ', tone: 'warn', action: { label: 'Mở', onClick: () => {} }, duration: 60000 })
    toast.card({ title: 'Cần xử lý', tone: 'err', action: { label: 'Mở', onClick: () => {} }, duration: 60000 })
    toast.card({ title: 'Thẻ mới', tone: 'ok', action: { label: 'Mở', onClick: () => {} }, duration: 60000 })
  })
  await act(async () => { vi.advanceTimersByTime(600) })
  await act(async () => {})
  expect(host.textContent).not.toContain('Thẻ cũ')
  expect(host.textContent).toContain('Cần xử lý')
  expect(host.textContent).toContain('Thẻ mới')
})
it('uses the configured four-second application duration for action results', async () => {
  setToastDuration(4000)
  await act(async () => { toast('Đã lưu cài đặt', 'ok') })
  await act(async () => { vi.advanceTimersByTime(3999) })
  expect(host.textContent).toContain('Đã lưu cài đặt')
  await act(async () => { vi.advanceTimersByTime(1) })
  expect(host.querySelector('.ui-toast-leaving')).toBeTruthy()
})
it('keeps a failed undo retryable and blocks double activation while it runs', async () => {
  const rejects: Array<(error: unknown) => void> = []
  const onClick = vi.fn(() => new Promise<void>((_, reject) => { rejects.push(reject) }))
  await act(async () => { toast('Đã xoá để hoàn tác', 'ok', { label: 'Hoàn tác', onClick }, { undo: true, duration: 5000 }) })
  await act(async () => { vi.advanceTimersByTime(2000) })
  const findButton = (label: string) => [...host.querySelectorAll('button')].find((button) => button.textContent?.includes(label)) as HTMLButtonElement | undefined
  await act(async () => { findButton('Hoàn tác')!.click(); findButton('Hoàn tác')!.click() })
  expect(onClick).toHaveBeenCalledTimes(1)
  // Đang chờ thao tác: đếm ngược dừng, toast không biến mất sau hạn cũ.
  await act(async () => { vi.advanceTimersByTime(10000) })
  expect(host.textContent).toContain('Đã xoá để hoàn tác')
  await act(async () => { rejects[0]!(new Error('Máy chủ từ chối hoàn tác')) })
  expect(host.textContent).toContain('Máy chủ từ chối hoàn tác')
  const retry = findButton('Retry')!
  expect(retry).toBeTruthy()
  await act(async () => { retry.click(); retry.click() })
  expect(onClick).toHaveBeenCalledTimes(2)
  expect(retry.disabled).toBe(true)
  await act(async () => { rejects[1]!(new Error('Máy chủ vẫn từ chối')) })
})
it('releases a keyboard focus hold when its toast is dismissed', async () => {
  let id = 0
  await act(async () => { id = toast('Đã lưu', 'ok', undefined, { duration: 3000 }) })
  await act(async () => { vi.advanceTimersByTime(2000) })
  const control = host.querySelector('.ui-toast-wrap button') as HTMLButtonElement
  await act(async () => { control.focus() })
  await act(async () => { vi.advanceTimersByTime(2000) })
  expect(host.textContent).toContain('Đã lưu')
  await act(async () => { dismissToast(id) })
  await act(async () => { vi.advanceTimersByTime(600) })
  expect(host.textContent).not.toContain('Đã lưu')
  await act(async () => { toast('Toast sau', 'ok', undefined, { duration: 3000 }) })
  await act(async () => { vi.advanceTimersByTime(2999) })
  expect(host.textContent).toContain('Toast sau')
  await act(async () => { vi.advanceTimersByTime(1) })
  expect(host.querySelector('.ui-toast-leaving')).toBeTruthy()
})
it('drops a previous session immediately and ignores its late action result', async () => {
  const rejects: Array<(error: unknown) => void> = []
  const onClick = vi.fn(() => new Promise<void>((_, reject) => { rejects.push(reject) }))
  await act(async () => { toast('Của phiên cũ', 'ok', { label: 'Hoàn tác', onClick }, { undo: true }) })
  const undo = [...host.querySelectorAll('button')].find((button) => button.textContent?.includes('Hoàn tác')) as HTMLButtonElement
  await act(async () => { undo.click() })
  await act(async () => { dismissToasts() })
  expect(host.textContent).not.toContain('Của phiên cũ')
  expect(host.querySelector('.ui-toast-leaving')).toBeNull()
  await act(async () => { toast('Của phiên mới', 'ok') })
  await act(async () => { rejects[0]!(new Error('Hoàn tác phiên cũ lỗi')) })
  expect(host.textContent).toContain('Của phiên mới')
  expect(host.textContent).not.toContain('Hoàn tác phiên cũ lỗi')
})

it('keeps countdown paused until both pointer and keyboard focus leave', async () => {
  await act(async () => { toast('Kết quả có thao tác', 'ok', undefined, { duration: 4000 }) })
  const wrap = host.querySelector('.ui-toast-wrap')!
  const button = wrap.querySelector('button') as HTMLButtonElement
  await act(async () => {
    vi.advanceTimersByTime(1000)
    wrap.dispatchEvent(new MouseEvent('mouseover', { bubbles: true }))
    button.focus()
    wrap.dispatchEvent(new MouseEvent('mouseout', { bubbles: true }))
    vi.advanceTimersByTime(5000)
  })
  expect(host.querySelector('.ui-toast-leaving')).toBeNull()
  await act(async () => { wrap.dispatchEvent(new MouseEvent('mouseover', { bubbles: true })); button.blur(); vi.advanceTimersByTime(5000) })
  expect(host.querySelector('.ui-toast-leaving')).toBeNull()
  await act(async () => { wrap.dispatchEvent(new MouseEvent('mouseout', { bubbles: true })); vi.advanceTimersByTime(2999) })
  expect(host.querySelector('.ui-toast-leaving')).toBeNull()
  await act(async () => { vi.advanceTimersByTime(1) })
  expect(host.querySelector('.ui-toast-leaving')).not.toBeNull()
})

it('releases pointer holds when a hovered result is removed without a mouseleave', async () => {
  let id = 0
  await act(async () => { id = toast('Kết quả cũ', 'ok') })
  await act(async () => { host.querySelector('.ui-toast-wrap')!.dispatchEvent(new MouseEvent('mouseover', { bubbles: true })); dismissToast(id); vi.advanceTimersByTime(520) })
  await act(async () => { toast('Kết quả mới', 'ok', undefined, { duration: 4000 }) })
  await act(async () => { vi.advanceTimersByTime(4000) })
  expect(host.querySelector('.ui-toast-leaving')).not.toBeNull()
})

it('resurrects a keyed result during its collapse without leaving it visually hidden', async () => {
  let id = 0
  await act(async () => { id = toast('Đang xử lý', 'ok', undefined, { key: 'qa-result' }) })
  await act(async () => { dismissToast(id); vi.advanceTimersByTime(100) })
  await act(async () => { toast('Đã xong', 'ok', undefined, { key: 'qa-result' }) })
  const wrap = host.querySelector('.ui-toast-wrap') as HTMLElement
  expect(wrap.textContent).toContain('Đã xong')
  expect(wrap.style.maxHeight).toBe('')
  expect(wrap.style.overflow).toBe('')
  expect(wrap.classList.contains('ui-toast-leaving')).toBe(false)
})

it('surfaces a failed in-flight undo even when a newer result replaced its visible pill', async () => {
  let reject!: (reason: unknown) => void
  const undo = vi.fn(() => new Promise<void>((_, no) => { reject = no }))
  await act(async () => { toast('Đã gỡ ảnh', 'ok', { label: 'Hoàn tác', onClick: undo }, { undo: true }) })
  await act(async () => { (host.querySelector('.ui-toast-wrap button') as HTMLButtonElement).click() })
  await act(async () => { toast('Đã đổi kênh', 'ok'); vi.advanceTimersByTime(1000) })
  expect(host.textContent).toContain('Đã đổi kênh')
  await act(async () => { reject(new Error('Chưa thể khôi phục ảnh')) })
  expect(host.textContent).toContain('Chưa thể khôi phục ảnh')
  undo.mockResolvedValueOnce(undefined)
  await act(async () => { (host.querySelector('.ui-toast-wrap:not(.ui-toast-leaving) button') as HTMLButtonElement).click() })
  expect(undo).toHaveBeenCalledTimes(2)
  await act(async () => { vi.advanceTimersByTime(520) })
  expect(host.textContent).not.toContain('Chưa thể khôi phục ảnh')
})

it('keeps fresh cards while an older card is submitting and exposes its close control as disabled', async () => {
  let finish!: () => void
  const action = () => new Promise<void>((resolve) => { finish = resolve })
  await act(async () => { toast.card({ title: 'Việc đang gửi', tone: 'warn', action: { label: 'Chạy', onClick: action } }) })
  const pendingWrap = host.querySelector('.ui-toast-wrap') as HTMLElement
  await act(async () => { ([...pendingWrap.querySelectorAll('button')].find((button) => button.textContent === 'Chạy') as HTMLButtonElement).click() })
  expect((pendingWrap.querySelector('.ui-toast-close') as HTMLButtonElement).disabled).toBe(true)
  await act(async () => {
    toast.card({ title: 'Kết quả mới một', tone: 'ok', action: { label: 'Mở một', onClick: () => {} } })
    toast.card({ title: 'Kết quả mới hai', tone: 'ok', action: { label: 'Mở hai', onClick: () => {} } })
    vi.advanceTimersByTime(600)
  })
  expect(host.textContent).toContain('Việc đang gửi')
  expect(host.textContent).toContain('Kết quả mới một')
  expect(host.textContent).toContain('Kết quả mới hai')
  await act(async () => { finish() })
  await act(async () => { vi.advanceTimersByTime(520) })
  expect(host.textContent).not.toContain('Việc đang gửi')
  expect(host.textContent).toContain('Kết quả mới một')
  expect(host.textContent).toContain('Kết quả mới hai')
})

it('does not replay a hidden pending result as an exit animation above a newer result', async () => {
  let finish!: () => void
  await act(async () => { toast('Kết quả cũ', 'ok', { label: 'Hoàn tác', onClick: () => new Promise<void>((resolve) => { finish = resolve }) }, { undo: true }) })
  await act(async () => { (host.querySelector('.ui-toast-wrap button') as HTMLButtonElement).click() })
  await act(async () => { toast('Kết quả mới', 'ok') })
  expect(host.textContent).not.toContain('Kết quả cũ')
  await act(async () => { finish() })
  expect(host.textContent).toContain('Kết quả mới')
  expect(host.textContent).not.toContain('Kết quả cũ')
})
