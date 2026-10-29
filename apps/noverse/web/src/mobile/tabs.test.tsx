// @vitest-environment happy-dom
import { act, useEffect, useState, type ReactNode } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, describe, expect, it, vi } from 'vitest'

// Chỉ cần context pane là hàm đồng nhất: test này kiểm tra DOM/điều hướng, không kiểm tra runtime.
vi.mock('./runtime', () => ({
  MobilePaneScope: ({ children }: { active: boolean; children: ReactNode }) => children,
}))

import { navigateNow, parseHash, useRoute } from '../router'
import { MobilePanes, MobileTabBar, keepPane, moreStart, paneKeyOf, phoneRedirect, type PaneSnapshot, type TabItem } from './tabs'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })
const roots: Root[] = []
afterEach(async () => {
  for (const root of roots.splice(0)) await act(() => root.unmount())
  document.body.replaceChildren()
  vi.clearAllMocks()
})

const render = async (ui: ReactNode) => {
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container); roots.push(root)
  await act(async () => { root.render(ui) })
  return { container, root }
}

const TABS: TabItem[] = [
  { key: 'work', label: 'Viết', icon: 'category', href: '/p/a/s/b/work' },
  { key: 'read', label: 'Đọc', icon: 'book-1', href: '/p/a/s/b/read' },
  { key: 'audio', label: 'Nghe', icon: 'headphone', href: '/p/a/s/b/audio/listen' },
  { key: 'more', label: 'Thêm', icon: 'setting-2', href: '/settings/mobile' },
]
const touch = (element: Element, type: string, init: MouseEventInit = {}) =>
  act(async () => {
    // `detail` (phân biệt Enter với chạm) chỉ có trên MouseEvent.
    const EventType = type === 'click' ? MouseEvent : Event
    element.dispatchEvent(new EventType(type, { bubbles: true, cancelable: true, ...init }))
  })

const bar = (onSelect: (href: string) => void, mode: 'default' | 'hide' = 'default') =>
  <MobileTabBar tabs={TABS} index={0} mode={mode} dot={false} onSelect={onSelect} />

describe('thanh tab điện thoại: nút trong app, không điều hướng <a>', () => {
  it('mỗi mục là <button type="button">, không phải link hash (iOS không còn cú chạm/điều hướng mặc định của <a>)', async () => {
    const { container } = await render(bar(vi.fn()))
    const tabs = container.querySelectorAll('.m-tab')
    expect(tabs).toHaveLength(4)
    for (const tab of tabs) {
      expect(tab.tagName).toBe('BUTTON')
      expect((tab as HTMLButtonElement).type).toBe('button')
      expect(tab.hasAttribute('href')).toBe(false)
    }
    expect(container.querySelector('.m-tabs a')).toBeNull()
  })

  it('bấm một lần gọi navigate đúng href, không có đường điều hướng nào khác', async () => {
    const onSelect = vi.fn()
    const { container } = await render(bar(onSelect))
    const tabs = container.querySelectorAll('.m-tab')
    await touch(tabs[1], 'click', { detail: 1 })
    expect(onSelect).toHaveBeenCalledTimes(1)
    expect(onSelect).toHaveBeenCalledWith('/p/a/s/b/read')
  })

  it('tab chưa vào được (chưa chọn truyện) bị tắt: không bấm được, không điều hướng, vạch không chạy', async () => {
    const onSelect = vi.fn()
    const tabs: TabItem[] = TABS.map((item) => item.key === 'more' ? item : { ...item, disabled: true })
    const { container } = await render(<MobileTabBar tabs={tabs} index={3} mode="default" dot={false} onSelect={onSelect} />)
    const buttons = container.querySelectorAll<HTMLButtonElement>('.m-tab')
    expect([...buttons].map((button) => button.disabled)).toEqual([true, true, true, false])
    await touch(buttons[1], 'click', { detail: 1 })
    expect(onSelect).not.toHaveBeenCalled()
    expect((container.querySelector('.m-tabs') as HTMLElement).style.getPropertyValue('--tab-index')).toBe('3')
  })

  it('vạch chỉ báo lạc quan đi theo cú chạm trong cùng click, rồi trả về tab thật khi điều hướng bị chặn', async () => {
    vi.useFakeTimers()
    try {
      const onSelect = vi.fn()
      const { container } = await render(bar(onSelect))
      const nav = container.querySelector('.m-tabs') as HTMLElement
      expect(nav.style.getPropertyValue('--tab-index')).toBe('0')
      await touch(container.querySelectorAll('.m-tab')[2], 'click', { detail: 1 })
      expect(nav.style.getPropertyValue('--tab-index')).toBe('2')
      // Route không đổi (điều hướng bị chặn) → sau RELEASE_MS vạch quay về tab đang mở.
      await act(async () => { vi.advanceTimersByTime(900) })
      expect(nav.style.getPropertyValue('--tab-index')).toBe('0')
    } finally {
      vi.useRealTimers()
    }
  })

  it('bàn phím (Enter/Space của button bắn click) vẫn chuyển tab', async () => {
    const onSelect = vi.fn()
    const { container } = await render(bar(onSelect))
    await touch(container.querySelectorAll('.m-tab')[3], 'click', { detail: 0 })
    expect(onSelect).toHaveBeenCalledTimes(1)
    expect(onSelect).toHaveBeenCalledWith('/settings/mobile')
  })

  it('thanh tab đang ẩn không nhận chạm', async () => {
    const onSelect = vi.fn()
    const { container } = await render(bar(onSelect, 'hide'))
    await touch(container.querySelectorAll('.m-tab')[1], 'click', { detail: 1 })
    expect(onSelect).not.toHaveBeenCalled()
  })
})

describe('giữ pane đã mở', () => {
  it('đổi tab chỉ ẩn pane cũ, không unmount: DOM, state và vị trí cuộn còn nguyên', async () => {
    const Work = () => <div data-marker="work"><button type="button">nút Viết</button></div>
    const Read = () => <div data-marker="read">Đọc</div>
    const work = <Work />
    const read = <Read />
    const { container, root } = await render(<MobilePanes panes={[['work', work], ['read', read]]} active="work" />)
    const workPane = container.querySelector('[data-pane="work"]') as HTMLElement
    const inner = container.querySelector('[data-marker="work"]') as HTMLElement
    inner.dataset.kept = 'yes'
    workPane.scrollTop = 240
    await act(async () => { root.render(<MobilePanes panes={[['work', work], ['read', read]]} active="read" />) })
    const readPane = container.querySelector('[data-pane="read"]') as HTMLElement
    expect(readPane.className).toContain('on')
    expect(workPane.className).not.toContain('on')
    expect(workPane.getAttribute('aria-hidden')).toBe('true')
    expect(container.querySelector('[data-pane="work"]')).toBe(workPane)
    expect(container.querySelector('[data-marker="work"]')).toBe(inner)
    expect((container.querySelector('[data-marker="work"]') as HTMLElement).dataset.kept).toBe('yes')
    expect(workPane.scrollTop).toBe(240)
  })

  it('quay lại tab đã mở không mount lại nội dung (0 remount khi chuyển)', async () => {
    let mounts = 0
    const Counting = () => {
      const [state] = useState(() => { mounts++; return true })
      return <div data-marker="counting" data-alive={state ? 'yes' : 'no'}>đã mở</div>
    }
    const counting = <Counting />
    const other = <div data-marker="other" />
    const { container, root } = await render(<MobilePanes panes={[['read', counting], ['work', other]]} active="read" />)
    expect(mounts).toBe(1)
    const node = container.querySelector('[data-marker="counting"]')
    // Đi vòng qua tab khác rồi quay lại: pane đọc chỉ ẩn/hiện, số lần mount vẫn là 1.
    await act(async () => { root.render(<MobilePanes panes={[['read', counting], ['work', other]]} active="work" />) })
    await act(async () => { root.render(<MobilePanes panes={[['read', counting], ['work', other]]} active="read" />) })
    expect(mounts).toBe(1)
    expect(container.querySelector('[data-marker="counting"]')).toBe(node)
  })
})

describe('route → pane', () => {
  it('mỗi tab một pane; màn riêng không chiếm pane để tab cũ vẫn nằm dưới', () => {
    expect(paneKeyOf(parseHash('#/p/a/s/b/work'))).toBe('work')
    expect(paneKeyOf(parseHash('#/p/a/s/b'))).toBe('work')
    expect(paneKeyOf(parseHash('#/p/a/s/b/read/3'))).toBe('read')
    expect(paneKeyOf(parseHash('#/p/a/s/b/audio/listen'))).toBe('audio')
    expect(paneKeyOf(parseHash('#/p/a/s/b/audio/review'))).toBe('audio')
    expect(paneKeyOf(parseHash('#/p/a/s/b/audio/2/make'))).toBe('audio')
    expect(paneKeyOf(parseHash('#/settings/mobile'))).toBe('more:mobile:')
    expect(paneKeyOf(parseHash('#/settings/mobile?page=devices'))).toBe('more:mobile:devices')
    expect(paneKeyOf(parseHash('#/settings/rules'))).toBe('more:rules:')
    expect(paneKeyOf(parseHash('#/settings/notify'))).toBeNull()
    expect(paneKeyOf(parseHash('#/p/a/s/b/activity'))).toBeNull()
    expect(paneKeyOf(parseHash('#/p/a/s/b/brain'))).toBeNull()
    expect(paneKeyOf(parseHash('#/'))).toBe('more:projects')
  })

  it('route chỉ có ở bản máy tính vẫn thuộc tab Thêm — không còn màn “Trên máy tính” dưới tab vừa bấm', () => {
    // Mọi hash mà điện thoại có thể đang mở (kể cả hash cũ do bản máy tính ghi lại).
    for (const hash of [
      '#/settings', '#/settings/providers', '#/settings/roles', '#/settings/writing', '#/settings/voice', '#/settings/library',
      '#/p/a/brain', '#/p/a/new', '#/p/a/s/b/quality', '#/system/brain', '#/khong-co', '#/notifications',
    ]) {
      const key = paneKeyOf(parseHash(hash))
      if (hash === '#/notifications') { expect(key, hash).toBeNull(); continue }
      expect(key, hash).toBe('more:mobile:')
    }
  })

  it('route máy tính được thay bằng URL điện thoại ngay (replace), không để URL lạ nằm lại', () => {
    expect(phoneRedirect(parseHash('#/settings'))).toBe('/settings/mobile')
    expect(phoneRedirect(parseHash('#/settings/providers'))).toBe('/settings/mobile')
    expect(phoneRedirect(parseHash('#/p/a/brain'))).toBe('/settings/mobile')
    expect(phoneRedirect(parseHash('#/p/a/new'))).toBe('/settings/mobile')
    expect(phoneRedirect(parseHash('#/system/brain'))).toBe('/settings/mobile')
    expect(phoneRedirect(parseHash('#/khong-co'))).toBe('/settings/mobile')
    expect(phoneRedirect(parseHash('#/p/a/s/b/quality'))).toBe('/p/a/s/b/work')
    // Route điện thoại thật thì không chuyển.
    for (const hash of ['#/settings/mobile', '#/settings/mobile?page=devices', '#/settings/rules', '#/settings/notify', '#/p/a/s/b/work', '#/p/a/s/b/read/2', '#/p/a/s/b/audio/listen', '#/p/a/s/b/brain', '#/p/a/s/b/activity', '#/help/commands/export', '#/']) {
      expect(phoneRedirect(parseHash(hash)), hash).toBeNull()
    }
  })

  it('ảnh chụp pane giữ nguyên tham chiếu khi route không đổi — memo của PaneView không bị phá', () => {
    const panes = new Map<string, PaneSnapshot>()
    const first = keepPane(panes, 'read', parseHash('#/p/a/s/b/read/3'), 'a', 'b')
    // Router parse lại cùng một hash sau mỗi hashchange: object mới, nội dung y hệt.
    const again = keepPane(panes, 'read', parseHash('#/p/a/s/b/read/3'), 'a', 'b')
    expect(again).toBe(first)
    const other = keepPane(panes, 'read', parseHash('#/p/a/s/b/read/4'), 'a', 'b')
    expect(other).not.toBe(first)
    expect(panes.get('read')).toBe(other)
  })

  it('pane “Thêm” giữ trang đầu theo route lúc mở', () => {
    expect(moreStart(parseHash('#/settings/rules')).initialPage).toBe('rules')
    expect(moreStart(parseHash('#/settings/mobile?page=devices')).initialPage).toBe('devices')
    expect(moreStart(parseHash('#/settings/mobile?page=projects')).initialPage).toBe('projects')
    expect(moreStart(parseHash('#/')).initialPage).toBe('projects')
    expect(moreStart(parseHash('#/help/commands/export'))).toEqual({ initialPage: 'guide', initialCommand: 'export' })
  })
})

describe('đổi tab áp dụng ngay trong khung hình của cú chạm', () => {
  it('hashchange sau navigateNow cập nhật DOM đồng bộ, không chờ transition', async () => {
    location.hash = '#/p/a/s/b/work'
    const Probe = () => <output>{useRoute().route.name}</output>
    const { container } = await render(<Probe />)
    expect(container.textContent).toBe('workspace')
    // Listener thêm sau khi mount chạy sau listener của router: đọc DOM ngay trong sự kiện.
    let during = ''
    const spy = () => { during = container.textContent ?? '' }
    addEventListener('hashchange', spy)
    try {
      act(() => { navigateNow('/settings/mobile'); dispatchEvent(new HashChangeEvent('hashchange')) })
    } finally {
      removeEventListener('hashchange', spy)
    }
    expect(during).toBe('settings')
    expect(container.textContent).toBe('settings')
  })
})

describe('pane giữ bằng <Activity>', () => {
  it('pane ẩn dừng effect (SSE/poll/timer) nhưng giữ state, DOM và vị trí cuộn', async () => {
    let effectRuns = 0
    let effectStops = 0
    const Counting = () => {
      const [state] = useState(() => 'đã mở')
      useEffect(() => { effectRuns++; return () => { effectStops++ } }, [])
      return <div data-marker="counting">{state}</div>
    }
    const counting = <Counting />
    const other = <div data-marker="other" />
    const { container, root } = await render(<MobilePanes panes={[['read', counting], ['work', other]]} active="read" />)
    expect(effectRuns).toBe(1)
    await act(async () => { root.render(<MobilePanes panes={[['read', counting], ['work', other]]} active="work" />) })
    expect(effectStops).toBeGreaterThanOrEqual(1)
    expect(container.querySelector('[data-marker="counting"]')?.textContent).toBe('đã mở')
    expect(container.querySelector('[data-pane="read"]')?.getAttribute('aria-hidden')).toBe('true')
    await act(async () => { root.render(<MobilePanes panes={[['read', counting], ['work', other]]} active="read" />) })
    expect(effectRuns).toBe(2)
    expect(container.querySelector('[data-marker="counting"]')?.textContent).toBe('đã mở')
  })
})
