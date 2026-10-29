// @vitest-environment happy-dom
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { ChapterTimeline } from './ChapterTimeline'
import type { AudioTimelineChapter } from '../../api/types'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })
let root: Root
let container: HTMLDivElement
let resize: (width: number) => void
beforeEach(() => {
 container = document.createElement('div'); document.body.append(container); root = createRoot(container)
 vi.stubGlobal('ResizeObserver', class {
  constructor(callback: ResizeObserverCallback) { resize = width => callback([{ contentRect: { width } } as ResizeObserverEntry], this as unknown as ResizeObserver) }
  observe() {} disconnect() {}
 })
})
afterEach(async () => { await act(() => root.unmount()); container.remove(); vi.unstubAllGlobals() })
const chapters: AudioTimelineChapter[] = Array.from({ length: 16 }, (_, i) => ({ chapter: i + 1, title: `Lên sóng ${i + 1}`, state: i === 0 ? 'done' : i === 1 ? 'rendering' : 'queued', sec: i === 0 ? 1000 : null, estSec: 1000, pct: i === 1 ? 45 : 0 }))

it('observes available width and changes full → number → bar without losing its one Tab stop', async () => {
 await act(() => root.render(<ChapterTimeline volume={1} chapters={chapters} madeSec={1000} onListen={() => {}} />))
 const group = container.querySelector('[role="group"]')!
 for (const [width, mode] of [[1212, 'full'], [508, 'number'], [507, 'bar']] as const) {
  await act(() => resize(width))
  expect(group.getAttribute('data-mode')).toBe(mode)
  expect(group.querySelectorAll('button[tabindex="0"]')).toHaveLength(1)
  expect(group.querySelectorAll('.audio-run')).toHaveLength(1)
 }
 expect(container.textContent).toContain('Ch. 2 · 45%')
 expect(container.textContent).toContain('Chưa chốt')
})

it('opens cards on hover/focus and supports roving keys, Enter listening only to done chapters', async () => {
 const listen = vi.fn()
 await act(() => root.render(<ChapterTimeline volume={1} chapters={chapters} madeSec={1000} onListen={listen} />))
 const buttons = [...container.querySelectorAll('button')]
 await act(() => buttons[1].focus())
 expect(container.querySelector('[role="tooltip"]')?.textContent).toContain('đang tạo 45% · khoảng 16:40')
 for (const [key, index] of [['ArrowRight', 2], ['End', 15], ['ArrowLeft', 14], ['Home', 0]] as const) {
  await act(() => document.activeElement?.dispatchEvent(new KeyboardEvent('keydown', { key, bubbles: true })))
  expect(document.activeElement).toBe(buttons[index])
  expect(buttons.filter(button => button.tabIndex === 0)).toHaveLength(1)
 }
 // Native buttons map Enter to click; assert the same activation path for assistive input.
 await act(() => buttons[0].click())
 expect(listen).toHaveBeenCalledWith(1)
 await act(() => buttons[1].click())
 expect(listen).toHaveBeenCalledTimes(1)
 await act(() => { buttons[0].blur(); buttons[3].dispatchEvent(new MouseEvent('mouseover', { bubbles: true })) })
 expect(container.querySelector('[role="tooltip"]')?.textContent).toContain('Chương 4')
})
