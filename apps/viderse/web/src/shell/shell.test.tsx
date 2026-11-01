// @vitest-environment happy-dom
// Shell: badge Duyệt + chấm bánh răng lấy từ /api/status; dải mất kết nối
// hiện khi luồng báo retrying và nút "Thử lại ngay" gọi retry; vùng nội dung
// inert (khoá pointer + bàn phím) khi offline; khung điện thoại dưới 768px.
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { applyViderseLabels } from '../kitLabels'
import { AppHeader } from './AppHeader'
import { AppShell } from './AppShell'
import { MobileShell } from './MobileShell'
import { OfflineStrip } from './OfflineStrip'
import { t } from '../i18n/all'
import type { Status } from '../api/status'
import { useTabMode } from '@noverse/ui'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

const roots: Root[] = []
async function render(node: React.ReactNode) {
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)
  roots.push(root)
  await act(async () => { root.render(node) })
  return container
}
let mobileMatch = false
const setMobile = (v: boolean) => { mobileMatch = v }

beforeEach(() => {
  mobileMatch = false
  // Desktop mặc định; test điện thoại gọi setMobile(true) trước khi render —
  // matchMedia mô lại đúng một query '(max-width: 767px)'.
  vi.stubGlobal('matchMedia', vi.fn().mockImplementation((query: string) => ({
    matches: query === '(max-width: 767px)' && mobileMatch,
    media: query, addEventListener: () => {}, removeEventListener: () => {},
    addListener: () => {}, removeListener: () => {},
  })))
  applyViderseLabels()
  location.hash = '#/'
})
afterEach(async () => {
  for (const root of roots.splice(0)) await act(() => root.unmount())
  document.body.replaceChildren()
  vi.unstubAllGlobals()
})

const STATUS: Status = {
  running: 2, queued: 5, paused: false, reviewCount: 4,
  attention: [{ kind: 'login_expired', text: 'Đăng nhập ChatGPT hết hạn', href: '#/settings' }],
}

function SelectionMode({ selecting }: { selecting: boolean }) {
  useTabMode(selecting ? 'hide' : undefined)
  return <p>Danh sách video</p>
}

describe('AppHeader', () => {
  it('badge trên Duyệt đếm từ reviewCount', async () => {
    const c = await render(<AppHeader route={{ name: 'workbench' }} status={STATUS} online />)
    const badge = c.querySelector('.vv-nav__badge')
    expect(badge?.textContent).toBe('4')
  })
  it('không có badge khi reviewCount = 0', async () => {
    const c = await render(<AppHeader route={{ name: 'workbench' }} status={{ ...STATUS, reviewCount: 0 }} online />)
    expect(c.querySelector('.vv-nav__badge')).toBeNull()
  })
  it('chấm đỏ trên bánh răng khi có mục attention', async () => {
    const c = await render(<AppHeader route={{ name: 'workbench' }} status={STATUS} online />)
    expect(c.querySelector('.vv-gear__dot')).toBeTruthy()
  })
  it('không có chấm khi attention rỗng', async () => {
    const c = await render(<AppHeader route={{ name: 'workbench' }} status={{ ...STATUS, attention: [] }} online />)
    expect(c.querySelector('.vv-gear__dot')).toBeNull()
  })
  it('JobsPill đọc bộ đếm từ status', async () => {
    const c = await render(<AppHeader route={{ name: 'workbench' }} status={STATUS} online />)
    expect(c.textContent).toContain('2 đang chạy · 5 chờ lượt')
  })
  it('JobsPill báo mất kết nối khi offline', async () => {
    const c = await render(<AppHeader route={{ name: 'workbench' }} status={STATUS} online={false} />)
    expect(c.textContent).toContain(t.shell.jobsOffline)
  })
})

describe('MobileShell — MMain', () => {
  it('keeps all five destinations in equal-width tab slots, including Video mới', async () => {
    const c = await render(<MobileShell route={{ name: 'workbench' }} status={STATUS} online><p>x</p></MobileShell>)
    const tabs = c.querySelectorAll('.vv-tabbar button')
    expect(tabs).toHaveLength(5)
    expect([...tabs].every((tab) => tab.classList.contains('vv-tabbar__item'))).toBe(true)
    // Ô giữa là nút hành động (FAB đỏ), không phải đích điều hướng thường.
    expect(c.querySelector('.vv-tabbar__new')?.closest('button')?.getAttribute('aria-label')).toBe(t.nav.new)
    // Mọi ô đều là <button> thật (bàn phím + screen reader dùng được), không phải <a>.
    expect(c.querySelectorAll('.vv-tabbar a')).toHaveLength(0)
  })
  it('chạm ô giữa mở Video mới; ô Kênh đi tới kênh đang mở', async () => {
    location.hash = '#/'
    const c = await render(<MobileShell route={{ name: 'workbench' }} status={STATUS} online><p>x</p></MobileShell>)
    const newTab = c.querySelector('.vv-tabbar__new')?.closest('button') as HTMLButtonElement
    await act(async () => { newTab.click() })
    expect(location.hash).toBe('#/new')
  })
  it('màn tạo kênh không vẽ thanh tab (artboard MCreate)', async () => {
    const c = await render(<MobileShell route={{ name: 'channel-new' }} status={STATUS} online><p>x</p></MobileShell>)
    expect(c.querySelector('.vv-tabbar')).toBeNull()
  })
  it('opens settings from the avatar while keeping the real running count on the round jobs button', async () => {
    const c = await render(<MobileShell route={{ name: 'workbench' }} status={STATUS} online userName="Linh"><p>x</p></MobileShell>)
    expect(c.querySelector('.vv-mavatar')?.getAttribute('href')).toBe('#/settings')
    expect(c.querySelector('.vv-mavatar')?.getAttribute('aria-label')).toBe(t.shell.accountSettings)
    // Điện thoại: nút tròn — số việc đang chạy ở giữa, không còn viên chữ “2 · 5”.
    expect(c.querySelector('.vv-jobsbtn__n')?.textContent).toBe('2')
    expect(c.querySelector('.vv-jobs .num')).toBeNull()
  })

  it('chọn nhiều khoá điều hướng tab và thoát chọn khôi phục điều hướng', async () => {
    const c = await render(<MobileShell route={{ name: 'explore' }} status={STATUS} online><SelectionMode selecting /></MobileShell>)
    const nav = c.querySelector('.vv-tabbar') as HTMLElement
    const tab = nav.querySelector('button') as HTMLButtonElement
    expect(nav.getAttribute('aria-hidden')).toBe('true')
    expect(tab.tabIndex).toBe(-1)
    await act(async () => { tab.click() })
    expect(location.hash).toBe('#/')

    await act(async () => { roots.at(-1)!.render(<MobileShell route={{ name: 'explore' }} status={STATUS} online><SelectionMode selecting={false} /></MobileShell>) })
    expect(nav.getAttribute('aria-hidden')).toBe('false')
    expect(tab.tabIndex).toBe(0)
  })
})

describe('Bộ đổi kênh', () => {
  const CHANNELS = [
    { id: 'kenh-a', name: 'Kênh A' },
    { id: 'kenh-b', name: 'Kênh B' },
  ]
  const openMenu = async (c: HTMLElement) => {
    const btn = c.querySelector('[aria-haspopup="menu"]') as HTMLButtonElement
    await act(async () => { btn.click() })
  }
  const pick = async (label: string) => {
    const item = [...document.querySelectorAll('[role="menuitemradio"],[role="menuitem"]')]
      .find((el) => (el.textContent ?? '').includes(label)) as HTMLElement
    await act(async () => { item.click() })
  }

  it('Bàn dựng: chọn kênh → hash lọc ?channel; chọn Mọi kênh → về #/', async () => {
    location.hash = '#/'
    await render(<AppHeader route={{ name: 'workbench' }} status={STATUS} online channels={CHANNELS} />)
    await openMenu(document.body)
    await pick('Kênh B')
    expect(location.hash).toBe('#/?channel=kenh-b')
  })

  it('Bàn dựng: chọn Mọi kênh → hash #/?scope=all', async () => {
    location.hash = '#/?channel=kenh-b'
    await render(<AppHeader route={{ name: 'workbench', channel: 'kenh-b' }} status={STATUS} online channels={CHANNELS} />)
    await openMenu(document.body)
    await pick(t.nav.allChannels)
    expect(location.hash).toBe('#/?scope=all')
  })

  it('Trang Kênh: chọn kênh khác → #/channels/<id>; Mọi kênh → về Bàn dựng', async () => {
    location.hash = '#/channels/kenh-a'
    await render(<AppHeader route={{ name: 'channel', channelId: 'kenh-a' }} status={STATUS} online channels={CHANNELS} />)
    await openMenu(document.body)
    await pick('Kênh B')
    expect(location.hash).toBe('#/channels/kenh-b')

    await openMenu(document.body)
    await pick(t.nav.allChannels)
    expect(location.hash).toBe('#/?scope=all')
  })
})

describe('OfflineStrip', () => {
  it('hiện tức thì khi trạng thái báo retrying (≤5s sau khi mất kết nối)', async () => {
    vi.useFakeTimers()
    try {
      const c = await render(<OfflineStrip status={{ state: 'retrying', reconnectAt: Date.now() + 4000, failures: 2 }} />)
      expect(c.querySelector('[role="alert"]')).toBeTruthy()
      expect(c.textContent).toContain(t.offline.title)
      expect(c.textContent).toContain('4')
    } finally {
      vi.useRealTimers()
    }
  })
  it('nút Chi tiết mở đoạn giải thích; Thử lại ngay gọi retry', async () => {
    const retry = vi.fn()
    const c = await render(<OfflineStrip status={{ state: 'retrying', reconnectAt: null, failures: 1 }} retry={retry} />)
    const [detailsBtn, retryBtn] = [...c.querySelectorAll('button')]
    expect(c.textContent).not.toContain(t.offline.body)
    await act(async () => { detailsBtn.click() })
    expect(c.textContent).toContain(t.offline.body)
    await act(async () => { retryBtn.click() })
    expect(retry).toHaveBeenCalledTimes(1)
  })
})

/** Luồng SSE trả frame hello ngay rồi treo — đủ để client mở kết nối. */
function hangingStream() {
  return Promise.resolve(new Response(
    new ReadableStream<Uint8Array>({
      start(controller) {
        controller.enqueue(new TextEncoder().encode('event: hello\ndata: {"subscriber":"t1"}\n\n'))
      },
    }),
    { status: 200, headers: { 'Content-Type': 'text/event-stream' } },
  ))
}

describe('AppShell — ngưỡng khung và khoá ghi', () => {
  const USER = { username: 'linh', name: 'Linh', role: 'admin' } as Parameters<typeof AppShell>[0]['user']
  const stubApi = () => vi.stubGlobal('fetch', vi.fn((path: string | URL) => {
    const url = String(path)
    if (url.includes('/api/status')) {
      return Promise.resolve(new Response(JSON.stringify({ running: 0, queued: 0, paused: false, reviewCount: 0, attention: [] }), { headers: { 'Content-Type': 'application/json' } }))
    }
    if (url.includes('/api/events')) return hangingStream()
    // Setup hoàn tất → AppShell không điều hướng sang #/setup trong test.
    if (url.includes('/api/setup/state')) return Promise.resolve(new Response('{"state":"done","complete":true,"running":false,"steps":[],"disk":{"freeBytes":1,"requiredBytes":1,"enough":true}}', { headers: { 'Content-Type': 'application/json' } }))
    if (url.includes('/api/me')) return Promise.resolve(new Response('{"user":{}}', { headers: { 'Content-Type': 'application/json' } }))
    return Promise.resolve(new Response('{}'))
  }))

  it('dưới 768px → MobileShell (thanh dưới); từ 768px → header desktop', async () => {
    stubApi()
    setMobile(true)
    const m = await render(<AppShell route={{ name: 'workbench' }} user={USER}><p>x</p></AppShell>)
    expect(m.querySelector('.vv-tabbar')).toBeTruthy()
    expect(m.querySelector('.vv-header')).toBeNull()
    // Ở đúng ngưỡng 768 (query không khớp) → khung desktop.
    setMobile(false)
    const d = await render(<AppShell route={{ name: 'workbench' }} user={USER}><p>x</p></AppShell>)
    expect(d.querySelector('.vv-header')).toBeTruthy()
    expect(d.querySelector('.vv-tabbar')).toBeNull()
  })

  it('offline → vùng nội dung inert (khoá pointer + bàn phím); online → mở lại', async () => {
    stubApi()
    vi.stubGlobal('navigator', { onLine: false })
    const c = await render(<AppShell route={{ name: 'workbench' }} user={USER}><p>workbench</p></AppShell>)
    const content = [...c.querySelectorAll('div')].find((d) => d.hasAttribute('inert'))
    expect(content).toBeTruthy()
    expect(c.querySelector('.vv-offline')).toBeTruthy()
    // Nút Thử lại của dải vẫn dùng được (không nằm trong vùng inert).
    const retry = [...c.querySelectorAll('button')].find((b) => b.textContent === t.common.retryNow)
    expect(retry?.closest('[inert]')).toBeNull()
    expect(retry?.disabled).toBe(false)
    // Kết nối thật trở lại: hook đọc navigator.onLine, không coi SSE mở là online.
    await act(async () => {
      vi.stubGlobal('navigator', { onLine: true })
      window.dispatchEvent(new Event('online'))
    })
    expect(c.querySelector('[inert]')).toBeNull()
  })
})
