// @vitest-environment happy-dom
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { toast, ToastHost } from './Toast'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })
let host: HTMLDivElement
let root: Root
beforeEach(async () => {
  vi.useFakeTimers()
  vi.stubGlobal('matchMedia', () => ({ matches: false, addEventListener() {}, removeEventListener() {} }))
  host = document.createElement('div'); document.body.append(host); root = createRoot(host)
  await act(async () => root.render(<ToastHost activity />))
})
afterEach(async () => { await act(async () => { root.unmount(); vi.runAllTimers() }); host.remove(); vi.useRealTimers(); vi.unstubAllGlobals() })
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
it('shows two job cards and recedes the older one until hovered', async () => {
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
