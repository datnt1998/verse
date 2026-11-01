// @vitest-environment happy-dom
// Hành vi thanh tab + runtime điện thoại: chế độ, vạch chỉ báo lạc quan, neo toast, bàn phím.
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { MobileTabBar, type MobileTabItem } from './tabs'
import { MobileRuntimeProvider, useMobileRuntime, useTabMode, MobilePaneScope } from './runtime'
import { useToastBottom, useVisualViewportKeyboard } from './safeArea'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })
let host: HTMLDivElement
let root: Root

beforeEach(() => {
  host = document.createElement('div')
  document.body.append(host)
  root = createRoot(host)
})
afterEach(() => {
  act(() => root.unmount())
  host.remove()
  document.documentElement.removeAttribute('data-tab-mode')
  document.documentElement.style.removeProperty('--m-toast-bottom')
  document.documentElement.style.removeProperty('--kb')
})

const items: MobileTabItem[] = [
  { key: 'work', label: 'Viết', icon: 'pen', href: '/work' },
  { key: 'read', label: 'Đọc', icon: 'book', href: '/read' },
  { key: 'new', center: true, ariaLabel: 'Video mới' },
  { key: 'audio', label: 'Nghe', icon: 'headphone', href: '/audio' },
  { key: 'more', label: 'Thêm', icon: 'grid', href: '/more' },
]

const tabButtons = () => [...host.querySelectorAll<HTMLButtonElement>('nav button')]

describe('MobileTabBar', () => {
  it('vẽ đủ ô, ô giữa là nút hành động, tab đang chọn có aria-current', () => {
    act(() => root.render(<MobileTabBar items={items} activeKey="read" centerAction={<span className="fab">+</span>} />))
    const buttons = tabButtons()
    expect(buttons).toHaveLength(5)
    expect(host.querySelector('.fab')?.textContent).toBe('+')
    expect(buttons[1].getAttribute('aria-current')).toBe('page')
    expect(buttons[0].getAttribute('aria-current')).toBeNull()
    expect(host.querySelector('nav')?.getAttribute('style')).toContain('--tab-index: 1')
  })

  it('cú chạm đặt vạch chỉ báo lạc quan rồi nhả về tab thật sau 550ms', () => {
    vi.useFakeTimers()
    const onSelect = vi.fn()
    act(() => root.render(<MobileTabBar items={items} activeKey="read" onSelect={onSelect} />))
    const nav = host.querySelector('nav')!
    act(() => tabButtons()[3].click())
    expect(onSelect).toHaveBeenCalledWith(items[3], 3)
    expect(nav.style.getPropertyValue('--tab-index')).toBe('3')
    act(() => vi.advanceTimersByTime(549))
    expect(nav.style.getPropertyValue('--tab-index')).toBe('3')
    act(() => vi.advanceTimersByTime(1))
    expect(nav.style.getPropertyValue('--tab-index')).toBe('1')
    vi.useRealTimers()
  })

  it('chế độ hide ẩn khỏi a11y và không còn trong thứ tự tab', () => {
    act(() => root.render(<MobileTabBar items={items} activeKey="work" mode="hide" />))
    expect(host.querySelector('nav')?.getAttribute('aria-hidden')).toBe('true')
    expect(tabButtons().every((button) => button.getAttribute('tabindex') === '-1')).toBe(true)
    act(() => tabButtons()[2].click())
    expect(host.querySelector('nav')?.getAttribute('aria-hidden')).toBe('true')
  })

  it('chấm "có việc" không hiện ở tab đang đứng', () => {
    const dotted = items.map((item) => item.key === 'read' ? { ...item, dot: true } : item)
    act(() => root.render(<MobileTabBar items={dotted} activeKey="read" />))
    expect(host.querySelector('.m-dot')).toBeNull()
    act(() => root.render(<MobileTabBar items={dotted} activeKey="work" />))
    expect(host.querySelector('.m-dot')?.getAttribute('aria-label')).toBe('Có việc cần bạn')
  })
})

describe('MobileRuntimeProvider', () => {
  function Probe({ mode }: { mode?: 'default' | 'mini' | 'hide' }) {
    useTabMode(mode)
    const runtime = useMobileRuntime()
    return <output data-mode={runtime.tabMode}>{runtime.tabMode}</output>
  }

  it('màn xin hide thì cả provider ẩn thanh tab, gỡ ra thì trả lại', () => {
    act(() => root.render(<MobileRuntimeProvider><Probe mode="hide" /></MobileRuntimeProvider>))
    expect(host.querySelector('output')?.getAttribute('data-mode')).toBe('hide')
    expect(document.documentElement.dataset.tabMode).toBe('hide')
    act(() => root.render(<MobileRuntimeProvider><Probe /></MobileRuntimeProvider>))
    expect(host.querySelector('output')?.getAttribute('data-mode')).toBe('default')
    expect(document.documentElement.dataset.tabMode).toBe('default')
  })

  it('pane ẩn không đẩy chế độ thanh tab', () => {
    act(() => root.render(<MobileRuntimeProvider><MobilePaneScope active={false}><Probe mode="hide" /></MobilePaneScope></MobileRuntimeProvider>))
    expect(host.querySelector('output')?.getAttribute('data-mode')).toBe('default')
  })

  it('hidden (hết phiên) ẩn thanh tab bất kể màn', () => {
    act(() => root.render(<MobileRuntimeProvider hidden><Probe /></MobileRuntimeProvider>))
    expect(host.querySelector('output')?.getAttribute('data-mode')).toBe('hide')
  })

  it('bàn phím ảo ghi --kb và ẩn thanh tab', () => {
    const listeners: Record<string, () => void> = {}
    const viewport = { height: window.innerHeight, offsetTop: 0, addEventListener: (type: string, fn: () => void) => { listeners[type] = fn }, removeEventListener: () => {} }
    vi.stubGlobal('visualViewport', viewport)
    act(() => root.render(<MobileRuntimeProvider><Probe /></MobileRuntimeProvider>))
    expect(document.documentElement.style.getPropertyValue('--kb')).toBe('0px')
    viewport.height = window.innerHeight - 300
    act(() => listeners.resize())
    expect(document.documentElement.style.getPropertyValue('--kb')).toBe('300px')
    expect(host.querySelector('output')?.getAttribute('data-mode')).toBe('hide')
    vi.unstubAllGlobals()
  })
})

describe('useToastBottom', () => {
  function Anchor({ px, priority }: { px: number | null; priority?: number }) {
    useToastBottom(px, priority)
    return null
  }

  it('đặt và gỡ --m-toast-bottom theo ưu tiên', () => {
    act(() => root.render(<><Anchor px={108} /><Anchor px={148} priority={1} /></>))
    expect(document.documentElement.style.getPropertyValue('--m-toast-bottom')).toBe('148px')
    act(() => root.render(<><Anchor px={108} /><Anchor px={null} priority={1} /></>))
    expect(document.documentElement.style.getPropertyValue('--m-toast-bottom')).toBe('108px')
    act(() => root.render(<><Anchor px={null} /></>))
    expect(document.documentElement.style.getPropertyValue('--m-toast-bottom')).toBe('')
  })
})

describe('useVisualViewportKeyboard', () => {
  function Probe() {
    const px = useVisualViewportKeyboard()
    return <output data-kb={px} />
  }

  it('ngưỡng 80px: thanh địa chỉ co giãn không bị coi là bàn phím', () => {
    const listeners: Record<string, () => void> = {}
    const viewport = { height: window.innerHeight, offsetTop: 0, addEventListener: (type: string, fn: () => void) => { listeners[type] = fn }, removeEventListener: () => {} }
    vi.stubGlobal('visualViewport', viewport)
    act(() => root.render(<Probe />))
    viewport.height = window.innerHeight - 60
    act(() => listeners.resize())
    expect(host.querySelector('output')?.getAttribute('data-kb')).toBe('0')
    viewport.height = window.innerHeight - 200
    act(() => listeners.resize())
    expect(host.querySelector('output')?.getAttribute('data-kb')).toBe('200')
    expect(document.documentElement.style.getPropertyValue('--kb')).toBe('200px')
    vi.unstubAllGlobals()
  })
})
