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
