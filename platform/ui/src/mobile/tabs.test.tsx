// @vitest-environment happy-dom
// Hành vi thanh tab + runtime điện thoại: chế độ, vạch chỉ báo lạc quan, neo toast, bàn phím.
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { MobileTabBar, type MobileTabItem } from './tabs'
import { MobileRuntimeProvider, useMobileRuntime, useTabMode, MobilePaneScope } from './runtime'
import { useToastBottom } from './safeArea'

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
    return <MobileTabBar items={items} mode={runtime.tabMode} />
  }

  it('màn xin hide thì cả provider ẩn thanh tab, gỡ ra thì trả lại', () => {
    act(() => root.render(<MobileRuntimeProvider><Probe mode="hide" /></MobileRuntimeProvider>))
    expect(host.querySelector('nav')?.getAttribute('aria-hidden')).toBe('true')
    expect(document.documentElement.dataset.tabMode).toBe('hide')
    act(() => root.render(<MobileRuntimeProvider><Probe /></MobileRuntimeProvider>))
    expect(host.querySelector('nav:not([aria-hidden="true"])')).not.toBeNull()
    expect(document.documentElement.dataset.tabMode).toBe('default')
  })

  it('pane ẩn không đẩy chế độ thanh tab', () => {
    act(() => root.render(<MobileRuntimeProvider><MobilePaneScope active={false}><Probe mode="hide" /></MobilePaneScope></MobileRuntimeProvider>))
    expect(host.querySelector('nav:not([aria-hidden="true"])')).not.toBeNull()
  })

  it('hidden (hết phiên) ẩn thanh tab bất kể màn', () => {
    act(() => root.render(<MobileRuntimeProvider hidden><Probe /></MobileRuntimeProvider>))
    expect(host.querySelector('nav')?.getAttribute('aria-hidden')).toBe('true')
  })

  it('bàn phím nhập chữ ẩn nav và blur trả lại nav mà không cần cuộn', async () => {
    vi.useFakeTimers()
    const viewport = Object.assign(new EventTarget(), { height: window.innerHeight, offsetTop: 0, scale: 1 })
    vi.stubGlobal('visualViewport', viewport)
    try {
      act(() => root.render(<MobileRuntimeProvider><Probe /><textarea /></MobileRuntimeProvider>))
      const input = host.querySelector('textarea')!
      await act(async () => {
        input.focus()
        viewport.height = window.innerHeight - 300
        viewport.dispatchEvent(new Event('resize'))
        await vi.advanceTimersByTimeAsync(20)
      })
      expect(document.documentElement.style.getPropertyValue('--kb')).toBe('300px')
      expect(host.querySelector('nav')?.getAttribute('aria-hidden')).toBe('true')
      expect(tabButtons().every((button) => button.tabIndex === -1)).toBe(true)
      await act(async () => {
        input.blur()
        await vi.advanceTimersByTimeAsync(20)
      })
      expect(document.documentElement.style.getPropertyValue('--kb')).toBe('0px')
      expect(host.querySelector('nav:not([aria-hidden="true"])')).not.toBeNull()
      expect(tabButtons().every((button) => button.tabIndex !== -1)).toBe(true)
    } finally {
      act(() => root.render(null))
      vi.useRealTimers()
      vi.unstubAllGlobals()
    }
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

