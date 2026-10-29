// @vitest-environment happy-dom
import { act, StrictMode } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, expect, it, vi } from 'vitest'
import { Dialog, DIALOG_EXIT_MS } from './Dialog'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })
const roots: Root[] = []
afterEach(async () => {
  for (const root of roots.splice(0)) await act(() => root.unmount())
  vi.useRealTimers(); vi.restoreAllMocks(); document.body.replaceChildren()
})

const reducedMotion = (matches: boolean) => vi.spyOn(window, 'matchMedia').mockImplementation((query: string) => ({
  matches: matches && query.includes('reduce'), media: query, onchange: null,
  addListener: () => {}, removeListener: () => {}, addEventListener: () => {}, removeEventListener: () => {}, dispatchEvent: () => false,
}) as MediaQueryList)

async function mount() {
  const container = document.createElement('div')
  container.id = 'root'
  document.body.append(container)
  const root = createRoot(container)
  await act(async () => { root.render(<StrictMode><Dialog title="Thiết bị" onClose={() => {}}><button type="button">Đăng xuất</button></Dialog></StrictMode>) })
  return root
}

it('mở trong StrictMode chỉ có một hộp, không có lớp đóng', async () => {
  reducedMotion(false)
  roots.push(await mount())
  expect(document.querySelectorAll('[role="dialog"]')).toHaveLength(1)
  expect(document.querySelector('.kit-dlg-closing')).toBeNull()
  expect(document.querySelector('.kit-dlg-scrim')).not.toBeNull()
  expect(document.getElementById('root')!.hasAttribute('inert')).toBe(true)
})

it('đóng giữ lớp phủ trơ chạy chuyển động đóng rồi gỡ sau 200 ms', async () => {
  reducedMotion(false)
  vi.useFakeTimers()
  const root = await mount()
  await act(async () => root.unmount())
  const ghost = document.querySelector<HTMLElement>('.kit-dlg-closing')
  expect(ghost).not.toBeNull()
  expect(ghost!.querySelector('.kit-dlg')).not.toBeNull()
  expect(ghost!.getAttribute('aria-hidden')).toBe('true')
  expect(ghost!.hasAttribute('inert')).toBe(true)
  expect(ghost!.style.pointerEvents).toBe('none')
  expect(document.querySelector('[role="dialog"], [aria-modal="true"], [id^="«"], [id^=":"]')).toBeNull()
  expect(document.getElementById('root')!.hasAttribute('inert')).toBe(false)
  vi.advanceTimersByTime(DIALOG_EXIT_MS - 1)
  expect(ghost!.isConnected).toBe(true)
  vi.advanceTimersByTime(1)
  expect(ghost!.isConnected).toBe(false)
})

it('giảm chuyển động: đóng gỡ ngay, không có lớp đóng', async () => {
  reducedMotion(true)
  const root = await mount()
  await act(async () => root.unmount())
  expect(document.querySelector('.kit-dlg-closing')).toBeNull()
  expect(document.querySelector('.kit-dlg')).toBeNull()
})
