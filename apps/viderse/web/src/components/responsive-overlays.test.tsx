// Ranh giới hành vi của adapter, không phải bản sao markup: desktop vẫn là
// primitive nổi của kit; mobile là một Sheet; callback chỉ chạy sau khi sheet đã
// gỡ (focus/scroll đã trả), kể cả khi đổi breakpoint trong lúc sheet đang đóng.
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { ResponsiveActionMenu, ResponsiveSelect } from './responsive-overlays'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

let mobile = true
const mediaListeners = new Set<() => void>()

function installMatchMedia() {
  vi.stubGlobal('matchMedia', (query: string) => ({
    get matches() { return query.includes('prefers-reduced-motion') ? false : query.includes('max-width') ? mobile : false },
    media: query,
    addEventListener: (_type: string, listener: () => void) => mediaListeners.add(listener),
    removeEventListener: (_type: string, listener: () => void) => mediaListeners.delete(listener),
    addListener: (listener: () => void) => mediaListeners.add(listener),
    removeListener: (listener: () => void) => mediaListeners.delete(listener),
  }))
}

function setMobile(next: boolean) {
  mobile = next
  for (const listener of [...mediaListeners]) listener()
}

const roots: Root[] = []
async function render(node: React.ReactNode) {
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)
  roots.push(root)
  await act(async () => { root.render(node) })
  return container
}

beforeEach(() => {
  mobile = true
  mediaListeners.clear()
  installMatchMedia()
  vi.useFakeTimers()
})

afterEach(async () => {
  for (const root of roots.splice(0)) await act(() => root.unmount())
  document.body.replaceChildren()
  vi.useRealTimers()
})

const selectOptions = [
  { value: 'a', label: 'Một' },
  { value: 'b', label: 'Hai' },
  { value: 'c', label: 'Ba', disabled: true },
]

describe('ResponsiveSelect', () => {

  it('mobile chọn xong mới gọi onChange, sau khi sheet gỡ và focus về trigger', async () => {
    const observed: { overlay: Element | null; focused: boolean }[] = []
    let trigger: HTMLElement | null = null
    const onChange = vi.fn(() => {
      observed.push({ overlay: document.querySelector('.m-overlay'), focused: document.activeElement === trigger })
    })
    const c = await render(<ResponsiveSelect aria-label="Chọn" value="a" options={selectOptions} onChange={onChange} />)
    trigger = c.querySelector('button[aria-haspopup="dialog"]')
    trigger?.focus()
    await act(async () => { trigger?.click() })
    const rows = [...document.querySelectorAll<HTMLElement>('[role="option"]')]
    expect(rows).toHaveLength(3)
    await act(async () => { rows.find((row) => row.textContent?.includes('Hai'))?.click() })
    expect(onChange).not.toHaveBeenCalled()
    expect(document.querySelector('.m-overlay')).toBeTruthy()
    await act(async () => { vi.advanceTimersByTime(300) })
    expect(onChange).toHaveBeenCalledTimes(1)
    expect(observed[0].overlay).toBeNull()
    expect(observed[0].focused).toBe(true)
  })

  it('option bị khoá không gọi onChange và vẫn để sheet mở', async () => {
    const onChange = vi.fn()
    const c = await render(<ResponsiveSelect aria-label="Chọn" value="a" options={selectOptions} onChange={onChange} />)
    const trigger = c.querySelector<HTMLButtonElement>('button[aria-haspopup="dialog"]')
    await act(async () => { trigger?.click() })
    const locked = [...document.querySelectorAll<HTMLButtonElement>('[role="option"]')].find((row) => row.textContent?.includes('Ba'))
    expect(locked?.disabled).toBe(true)
    await act(async () => { locked?.click() })
    expect(onChange).not.toHaveBeenCalled()
    expect(document.querySelector('.m-overlay')).toBeTruthy()
  })

  it('đổi breakpoint trong lúc mở đóng sheet rồi mới chuyển sang desktop, không lớp trùng', async () => {
    const c = await render(<ResponsiveSelect aria-label="Chọn" value="a" options={selectOptions} onChange={() => {}} />)
    const trigger = c.querySelector<HTMLButtonElement>('button[aria-haspopup="dialog"]')
    await act(async () => { trigger?.click() })
    expect(document.querySelector('.m-overlay')).toBeTruthy()
    await act(async () => { setMobile(false) })
    expect(document.querySelectorAll('.m-overlay')).toHaveLength(1)
    await act(async () => { vi.advanceTimersByTime(300) })
    expect(document.querySelector('.m-overlay')).toBeNull()
    expect(c.querySelector('[role="combobox"]')).toBeTruthy()
  })
})

describe('ResponsiveActionMenu', () => {

  it('mobile chạy action đúng một lần sau khi sheet gỡ và focus đã trả', async () => {
    const observed: { overlay: Element | null; focused: boolean }[] = []
    let trigger: HTMLElement | null = null
    const onSelect = vi.fn(() => {
      observed.push({ overlay: document.querySelector('.m-overlay'), focused: document.activeElement === trigger })
    })
    const c = await render(<ResponsiveActionMenu label="Thao tác" items={[{ id: 'remove', label: 'Xoá', danger: true, onSelect }]} />)
    trigger = c.querySelector('button[aria-haspopup="dialog"]')
    trigger?.focus()
    await act(async () => { trigger?.click() })
    const item = document.querySelector<HTMLElement>('[role="menuitem"]')
    await act(async () => { item?.click() })
    expect(onSelect).not.toHaveBeenCalled()
    expect(document.querySelector('.m-overlay')).toBeTruthy()
    await act(async () => { vi.advanceTimersByTime(300) })
    expect(onSelect).toHaveBeenCalledTimes(1)
    expect(observed[0].overlay).toBeNull()
    expect(observed[0].focused).toBe(true)
  })

  it('mục disabled/locked không chạy action', async () => {
    const disabled = vi.fn()
    const locked = vi.fn()
    const c = await render(<ResponsiveActionMenu label="Thao tác" items={[
      { id: 'off', label: 'Đang chạy', disabled: true, onSelect: disabled },
      { id: 'lock', label: 'Chưa mở', lockedReason: 'Cần quyền', onSelect: locked },
    ]} />)
    const trigger = c.querySelector<HTMLButtonElement>('button[aria-haspopup="dialog"]')
    await act(async () => { trigger?.click() })
    const rows = [...document.querySelectorAll<HTMLElement>('[role="menuitem"]')]
    await act(async () => { rows[0]?.click() })
    expect(disabled).not.toHaveBeenCalled()
    expect(rows[1].getAttribute('aria-disabled')).toBe('true')
    await act(async () => { rows[1]?.click() })
    expect(locked).not.toHaveBeenCalled()
  })

  it('Esc đóng sheet, không chạy action và trả focus về trigger', async () => {
    const onSelect = vi.fn()
    const c = await render(<ResponsiveActionMenu label="Thao tác" items={[{ id: 'one', label: 'Một', onSelect }]} />)
    const trigger = c.querySelector<HTMLButtonElement>('button[aria-haspopup="dialog"]')
    trigger?.focus()
    await act(async () => { trigger?.click() })
    await act(async () => { window.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' })) })
    await act(async () => { vi.advanceTimersByTime(300) })
    expect(onSelect).not.toHaveBeenCalled()
    expect(document.querySelector('.m-overlay')).toBeNull()
    expect(document.activeElement).toBe(trigger)
  })

  it('đổi breakpoint khi action đang chờ vẫn chạy một lần sau khi sheet gỡ', async () => {
    const onSelect = vi.fn()
    const c = await render(<ResponsiveActionMenu label="Thao tác" items={[{ id: 'one', label: 'Một', onSelect }]} />)
    const trigger = c.querySelector<HTMLButtonElement>('button[aria-haspopup="dialog"]')
    await act(async () => { trigger?.click() })
    const item = document.querySelector<HTMLElement>('[role="menuitem"]')
    await act(async () => { item?.click() })
    await act(async () => { setMobile(false) })
    expect(onSelect).not.toHaveBeenCalled()
    await act(async () => { vi.advanceTimersByTime(300) })
    expect(onSelect).toHaveBeenCalledTimes(1)
    expect(document.querySelector('.m-overlay')).toBeNull()
    expect(c.querySelector('button[aria-haspopup="menu"]')).toBeTruthy()
  })
})

describe('cancelled overlay exit', () => {
  it('does not commit a selection clicked while an Esc dismissal is fading out', async () => {
    setMobile(true)
    const onChange = vi.fn()
    const c = await render(<ResponsiveSelect value="a" onChange={onChange} options={selectOptions} aria-label="Kênh" />)
    await act(async () => (c.querySelector('button') as HTMLButtonElement).click())
    await act(async () => { window.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true, cancelable: true })) })
    await act(async () => (document.querySelector('[role="option"][aria-selected="false"]') as HTMLButtonElement).click())
    await act(async () => { vi.advanceTimersByTime(300) })
    expect(document.querySelector('[role="dialog"]')).toBeNull()
    expect(onChange).not.toHaveBeenCalled()
  })

  it('does not run a menu action clicked after its scrim cancelled the menu', async () => {
    setMobile(true)
    const action = vi.fn()
    const c = await render(<ResponsiveActionMenu label="Thao tác" items={[{ id: 'delete', label: 'Xoá', onSelect: action }]} />)
    await act(async () => (c.querySelector('button') as HTMLButtonElement).click())
    await act(async () => (document.querySelector('.m-scrim') as HTMLElement).click())
    await act(async () => (document.querySelector('[role="menuitem"]') as HTMLButtonElement).click())
    await act(async () => { vi.advanceTimersByTime(300) })
    expect(action).not.toHaveBeenCalled()
    expect(document.body.style.overflow).toBe('')
  })
})
