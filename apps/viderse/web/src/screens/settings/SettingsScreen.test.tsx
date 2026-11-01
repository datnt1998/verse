import * as stylex from '@stylexjs/stylex'
import { styles } from './settings.stylex'
// @vitest-environment happy-dom
// Khung Cài đặt — chuyển mục bằng ?section=, alias cũ ?section=images → agents,
// chấm cam theo attention của /api/status, và mobile danh sách không có tab bar.
// Nguồn: design v1 artboard Settings + MSettings, 01-ia-flows W13.
import { act } from 'react'
import { invalidate } from '@noverse/ui/data'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { SettingsScreen } from './SettingsScreen'
import { parseHash } from '../../router'
import type { User, AppLink } from '../../api/types'

const session = vi.hoisted(() => ({ user: null as User | null, apps: [] as AppLink[] }))
vi.mock('../../api/session', () => ({ useSession: () => session }))

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

const roots: Root[] = []
let mobile = false

const respond = (body: unknown, status = 200) => Promise.resolve(Response.json(body, { status }))

function installFetch() {
  vi.stubGlobal('fetch', vi.fn(async (input: RequestInfo | URL) => {
    const url = typeof input === 'string' ? input : input instanceof URL ? input.href : input.url
    if (url.startsWith('/api/status')) {
      return respond({
        rev: 'r1', running: 0, queued: 0, reviewCount: 0, attention: [{ kind: 'tool_update', text: 'Có bản mới', href: '#/settings?section=tools' }],
      })
    }
    if (url.startsWith('/api/tools')) return respond({ tools: [], checkedAt: '2026-10-30T09:00:00Z' })
    if (url === '/api/storage/server') return respond({}, 404)
    if (url.startsWith('/api/storage')) return respond({ projects: { path: '/Users/dat/Viderse Projects', volume: { name: 'Macintosh HD', kind: 'internal', fs: 'apfs', freeBytes: 5e11 }, usedBytes: 8.6e10, online: true }, stayBytes: 5.3e9, suggestions: [], usage: [], clean: { lastRunAt: null } })
    if (url.startsWith('/api/activity')) return respond({ running: [], videoJobs: 0, queued: 0, unread: [] })
    if (url.startsWith('/api/errors')) return respond({ items: [], count7d: 0 })
    if (url.startsWith('/api/version')) return respond({ version: '1.4.0', latest: null, checkedAt: null })
    if (url.startsWith('/api/ai/providers')) return respond([{ id: 'chatgpt', kind: 'chatgpt', auth: 'subscription', label: 'ChatGPT', status: 'ready', catalogCount: 3, selected: [] }])
    if (url.startsWith('/api/ai/agents')) return respond({ director: { model: { provider: 'chatgpt', model: 'gpt-5' }, effort: 'medium', fallback: [] }, artist: { promptModel: { provider: 'chatgpt', model: 'gpt-5' }, draw: { provider: 'chatgpt', model: 'gpt-image-1' }, fallbackOn: false, drawFallback: [] } })
    if (url.startsWith('/api/platforms')) return respond(['youtube', 'twitch', 'bilibili'].map((id) => ({
      id, label: id, uses: [], kinds: [], targetHeight: 1080, state: 'ready', usage: { channels: 0, videos: 0 },
      login: { type: 'none', saved: false, renewable: false },
    })))
    return respond({})
  }))
}

async function render(section?: string) {
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)
  roots.push(root)
  const parsed = parseHash(`#/settings${section ? `?section=${section}` : ''}`)
  const s = parsed.route.name === 'settings' ? parsed.route.section : undefined
  await act(async () => { root.render(<SettingsScreen route={{ name: 'settings', section: s }} />) })
  for (let i = 0; i < 5; i++) await act(async () => { await new Promise((r) => setTimeout(r, 20)) })
  return container
}

/** Chờ mục lazy tải xong: mỗi nhịp trong act để React flush Suspense. */
async function settle(done: () => boolean) {
  for (let i = 0; i < 100 && !done(); i++) await act(async () => { await new Promise((r) => setTimeout(r, 30)) })
}

beforeEach(() => {
  void invalidate()
  mobile = false
  session.user = null
  session.apps = []
  vi.stubGlobal('matchMedia', vi.fn().mockImplementation((q: string) => ({
    matches: q.includes('767px') ? mobile : false, addEventListener: () => {}, removeEventListener: () => {},
  })))
  installFetch()
})

afterEach(async () => {
  await invalidate()
  for (const root of roots.splice(0)) await act(() => root.unmount())
  document.body.replaceChildren()
  vi.unstubAllGlobals()
})

describe('khung Cài đặt', () => {
  it('mục mặc định là Công cụ, tiêu đề lấy từ META', async () => {
    const el = await render()
    expect(el.querySelector('.vv-set__title')?.textContent).toBe('Công cụ trên máy')
    expect(el.querySelector('.vv-set__nav-it[aria-current="true"]')?.textContent).toContain('Công cụ trên máy')
  })

  it('?section=storage mở đúng mục Lưu trữ', async () => {
    const el = await render('storage')
    expect(el.querySelector('.vv-set__title')?.textContent).toBe('Lưu trữ')
  })

  it('alias cũ ?section=images rơi về Trợ lý AI', () => {
    const { route } = parseHash('#/settings?section=images')
    expect(route).toEqual({ name: 'settings', section: 'agents' })
  })

  it('chấm xuất hiện trên mục có attention trỏ tới, tông theo loại (tool_update → xanh)', async () => {
    const el = await render()
    const toolsNav = el.querySelector('.vv-set__nav-it[aria-current="true"]')
    const dot = toolsNav?.querySelector('.vv-dot')
    expect(dot).toBeTruthy()
    for (const atom of stylex.props(styles.dotInfo).className!.split(' ')) expect(dot?.classList.contains(atom)).toBe(true)
  })

  it('mỗi mục sidebar luôn giữ span số ở cuối (kể cả rỗng) để chấm không trôi sát mép', async () => {
    const el = await render()
    const its = [...el.querySelectorAll('.vv-set__nav-it')]
    for (const it of its) {
      const num = it.querySelector('.vv-set__nav-num')
      expect(num, it.textContent ?? '').toBeTruthy()
      // Span phải là phần tử cuối (sau chấm attention nếu có).
      expect(it.lastElementChild?.classList.contains('vv-set__nav-num')).toBe(true)
    }
  })

  it('thẻ phải “Khi nào có hiệu lực” đổi theo mục', async () => {
    const el = await render('storage')
    expect(el.querySelector('.vv-set__help-card h2')?.textContent).toBe('Không mất gì quan trọng')
  })

  it('mobile là danh sách phủ toàn màn, không thanh tab dưới', async () => {
    mobile = true
    const el = await render()
    expect(el.querySelector('.vv-mset')).toBeTruthy()
    expect(el.querySelector('[role="navigation"]')).toBeNull()
    expect(el.textContent).toContain('Báo lỗi')
  })

  it('mobile lấy giá trị thật cho Kết nối AI, Trợ lý AI, Tài khoản nền tảng', async () => {
    mobile = true
    const el = await render()
    const value = (label: string) =>
      [...el.querySelectorAll('.vv-mset__row')].find((r) => r.textContent?.includes(label))?.querySelector('.vv-mset__row-value')?.textContent
    expect(value('Kết nối AI')).toBe('1 nhà cung cấp')
    expect(value('Trợ lý AI')).toBe('Vẽ bằng ChatGPT')
    expect(value('Tài khoản nền tảng')).toBe('3 nền tảng')
  })

  it.each(['admin', 'member'] as const)('chỉ quản trị viên thấy lối Thành viên: %s', async (role) => {
    session.user = { username: 'linh', name: 'Linh', role }
    session.apps = [{ id: 'noverse', name: 'Noverse', origin: 'https://nov.example.test' }]
    const el = await render()
    const link = el.querySelector<HTMLAnchorElement>('a[href="https://nov.example.test/#/settings/members"]')
    expect(Boolean(link)).toBe(role === 'admin')
    if (link) expect(link.target).toBe('_blank')
  })

  it.each(['tools', 'connect', 'agents', 'accounts'] as const)('thành viên xem mục %s ở chế độ chỉ đọc, quản trị viên thì không', async (section) => {
    session.user = { username: 'linh', name: 'Linh', role: 'member' }
    const member = await render(section)
    await settle(() => !!member.querySelector('[role="note"]'))
    expect(member.querySelector('[role="note"]')?.textContent).toContain('chỉ đọc')
    expect(member.querySelector('[inert]')).toBeTruthy()
    for (const root of roots.splice(0)) await act(() => root.unmount())
    document.body.replaceChildren()
    session.user = { username: 'boss', name: 'Boss', role: 'admin' }
    const admin = await render(section)
    await settle(() => (admin.querySelector('.vv-set__body')?.textContent?.length ?? 0) > 20)
    expect(admin.querySelector('[role="note"]')).toBeNull()
    expect(admin.querySelector('[inert]')).toBeNull()
  })

  it('mục Lưu trữ không bị khoá chỉ đọc: thành viên vẫn xem và đọc được', async () => {
    session.user = { username: 'linh', name: 'Linh', role: 'member' }
    const el = await render('storage')
    expect(el.querySelector('[inert]')).toBeNull()
  })
})
