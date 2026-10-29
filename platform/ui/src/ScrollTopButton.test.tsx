// @vitest-environment happy-dom
import { act, useRef } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { ScrollTopButton, useScrollTop } from './ScrollTopButton'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })
let root: Root
let box: HTMLDivElement
let height = 500
let contentHeight = 5000
let reduced = false
let frames: FrameRequestCallback[] = []
let resizes: (() => void)[] = []
const flush = () => act(() => { const pending = frames; frames = []; pending.forEach((fn) => fn(0)) })
const scroll = async (top: number) => { await act(() => { box.scrollTop = top; box.dispatchEvent(new Event('scroll')) }); await flush() }
function Fixture({ resetKey = 1, blocked = false, focusHeading = true }: { resetKey?: number; blocked?: boolean; focusHeading?: boolean }) {
  const target = useRef<HTMLDivElement>(null)
  const heading = useRef<HTMLHeadingElement>(null)
  const state = useScrollTop(target)
  return <><div ref={(node) => {
    target.current = node
    if (node) { box = node; Object.defineProperties(node, { clientHeight: { configurable: true, get: () => height }, scrollHeight: { configurable: true, get: () => contentHeight } }) }
  }}><h1 ref={heading} tabIndex={-1}>Chương một</h1></div><output>{JSON.stringify(state)}</output><ScrollTopButton key={resetKey} target={target} focusTarget={focusHeading ? heading : undefined} label="Lên đầu chương" progress size="md" hidden={blocked} /></>
}
beforeEach(() => {
  height = 500; contentHeight = 5000; reduced = false; frames = []; resizes = []
  vi.stubGlobal('requestAnimationFrame', (fn: FrameRequestCallback) => { frames.push(fn); return frames.length })
  vi.stubGlobal('cancelAnimationFrame', vi.fn())
  vi.stubGlobal('ResizeObserver', class { constructor(callback: () => void) { resizes.push(callback) } observe() {} disconnect() {} })
  vi.stubGlobal('matchMedia', () => ({ matches: reduced }))
  root = createRoot(document.body.appendChild(document.createElement('div')))
})
afterEach(async () => { await act(() => root.unmount()); document.body.replaceChildren(); vi.restoreAllMocks(); vi.unstubAllGlobals() })
const mount = async () => { await act(() => root.render(<Fixture />)); await flush() }
const state = () => JSON.parse(document.querySelector('output')!.textContent!)
const button = () => document.querySelector<HTMLButtonElement>('[data-scroll-top]')!

describe('nút lên đầu', () => {
  it('chỉ gắn khi đủ ba màn hình và có hysteresis một/nửa màn hình', async () => {
    contentHeight = 1499; await mount(); await scroll(800)
    expect(button()).toBeNull()
    contentHeight = 1500; box.scrollTop = 500; resizes.forEach((fn) => fn()); await flush(); expect(state().visible).toBe(false)
    await scroll(501); expect(state().visible).toBe(true)
    await scroll(250); expect(state().visible).toBe(true)
    await scroll(249); expect(state().visible).toBe(false)
  })
  it('gộp scroll trong một rAF và đăng ký passive', async () => {
    const listener = vi.spyOn(HTMLElement.prototype, 'addEventListener')
    await mount()
    expect(listener.mock.calls.some(([name, , options]) => name === 'scroll' && (options as AddEventListenerOptions)?.passive)).toBe(true)
    await act(() => { box.scrollTop = 600; box.dispatchEvent(new Event('scroll')); box.scrollTop = 900; box.dispatchEvent(new Event('scroll')) })
    expect(state().progress).toBe(0)
    await flush(); expect(state().progress).toBe(900 / 4500)
    listener.mockRestore()
  })
  it('xa hơn sáu màn hình thì nhảy trước một màn rồi cuộn mượt và focus preventScroll', async () => {
    await mount(); await scroll(3500)
    const focus = vi.spyOn(document.querySelector('h1')!, 'focus')
    const go = vi.fn(() => { expect(box.scrollTop).toBe(500) })
    box.scrollTo = go
    await act(() => button().click())
    expect(go).toHaveBeenCalledWith({ top: 0, behavior: 'smooth' })
    expect(focus).toHaveBeenCalledWith({ preventScroll: true })
    expect(document.activeElement).toBe(document.querySelector('h1'))
    expect(button().tabIndex).toBe(-1)
  })
  it('giảm chuyển động nhảy thẳng bằng auto', async () => {
    reduced = true; await mount(); await scroll(3500)
    box.scrollTo = vi.fn()
    await act(() => button().click())
    expect(box.scrollTo).toHaveBeenCalledWith({ top: 0, behavior: 'auto' })
  })
  it('ẩn thì ra khỏi Tab ngay, kể cả khi lớp phủ mở', async () => {
    await mount(); expect(button().tabIndex).toBe(-1)
    await scroll(600); expect(button().tabIndex).toBe(0)
    await act(() => root.render(<Fixture blocked />)); expect(button().tabIndex).toBe(-1)
    expect(button().getAttribute('aria-hidden')).toBe('true')
    await act(() => root.render(<Fixture />)); await scroll(200); expect(button().tabIndex).toBe(-1)
  })
  it('không có tiêu đề thì focus chính vùng cuộn', async () => {
    await act(() => root.render(<Fixture focusHeading={false} />)); await flush(); await scroll(600); box.scrollTo = vi.fn()
    await act(() => button().click())
    expect(document.activeElement).toBe(box)
  })
  it('đổi chiều cao nội dung đo lại và gỡ listener khi không đủ ba màn hình', async () => {
    await mount(); await scroll(600); expect(button().tabIndex).toBe(0)
    const remove = vi.spyOn(box, 'removeEventListener')
    await act(() => { contentHeight = 1200; resizes.forEach((fn) => fn()) }); await flush()
    expect(button()).toBeNull()
    expect(remove.mock.calls.some(([name]) => name === 'scroll')).toBe(true)
  })
  it('bàn phím ảo mở thì ẩn nút ngay', async () => {
    const viewport = new EventTarget()
    Object.assign(viewport, { height: window.innerHeight, offsetTop: 0 })
    vi.stubGlobal('visualViewport', viewport)
    await mount(); await scroll(600); expect(button().tabIndex).toBe(0)
    await act(() => { Object.assign(viewport, { height: window.innerHeight - 300 }); viewport.dispatchEvent(new Event('resize')) })
    expect(button().tabIndex).toBe(-1)
    await act(() => { Object.assign(viewport, { height: window.innerHeight }); viewport.dispatchEvent(new Event('resize')) })
    expect(button().tabIndex).toBe(0)
  })
  it('cuộn mượt còn xa thì chốt ở đầu sau 400 ms', async () => {
    await mount(); await scroll(2000); box.scrollTo = vi.fn()
    vi.useFakeTimers()
    try {
      await act(() => button().click())
      await act(() => vi.advanceTimersByTime(399))
      expect(box.scrollTo).toHaveBeenCalledTimes(1)
      await act(() => vi.advanceTimersByTime(1))
      expect(box.scrollTo).toHaveBeenLastCalledWith({ top: 0, behavior: 'instant' })
    } finally { vi.useRealTimers() }
  })
})
