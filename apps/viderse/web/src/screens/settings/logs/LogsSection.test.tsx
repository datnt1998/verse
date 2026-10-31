// @vitest-environment happy-dom
// Cài đặt › Báo lỗi — Settings.dc.html (section=logs): thẻ xuất .zip, “Lỗi gần
// đây” 7 ngày từ /api/errors, thẻ phiên bản gọi POST /api/version/check. Không
// có bảng log trong trang.
import { act } from 'react'
import { invalidate } from '@noverse/ui/data'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import LogsSection from './LogsSection'
import * as stylex from '@stylexjs/stylex'
import { styles } from '../settings.stylex'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

const roots: Root[] = []
let errors: unknown
let latest: string | null
let calls: { url: string; method: string }[] = []

const respond = (body: unknown) => Promise.resolve(Response.json(body))

function installFetch() {
  vi.stubGlobal('fetch', vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = typeof input === 'string' ? input : input instanceof URL ? input.href : input.url
    calls.push({ url, method: init?.method ?? 'GET' })
    if (url.startsWith('/api/errors')) return respond(errors)
    if (url === '/api/version/check') return respond({ version: '1.4.0', latest, checkedAt: '2026-10-30T09:00:00Z' })
    if (url.startsWith('/api/version')) return respond({ version: '1.4.0', latest: null, checkedAt: null })
    return respond({})
  }))
}

async function render() {
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)
  roots.push(root)
  await act(async () => { root.render(<LogsSection />) })
  for (let i = 0; i < 5; i++) await act(async () => { await new Promise((r) => setTimeout(r, 20)) })
  return container
}

const click = async (el: Element | null | undefined) => {
  expect(el).toBeTruthy()
  await act(async () => { (el as HTMLElement).dispatchEvent(new MouseEvent('click', { bubbles: true })) })
}

beforeEach(() => {
  void invalidate()
  calls = []
  latest = null
  errors = {
    count7d: 4,
    items: [
      { id: 'e1', time: '23:15', what: 'Tải video 12/18 lỗi 403', why: 'YouTube chặn yt-dlp cũ.', status: 'Đang chờ', tone: 'warn', href: '#/channels/c1' },
      { id: 'e2', time: '22:40', what: 'Tạo phụ đề 1/18 lỗi', why: 'Mô hình chưa tải xong.', status: 'App đã xử lý', tone: 'muted' },
    ],
  }
  installFetch()
})

afterEach(async () => {
  await invalidate()
  for (const root of roots.splice(0)) await act(() => root.unmount())
  document.body.replaceChildren()
  vi.unstubAllGlobals()
})

describe('Báo lỗi', () => {
  it('thẻ xuất nói rõ tệp .zip và nút là liên kết tải', async () => {
    const el = await render()
    expect(el.querySelector('.vv-logs__export h2')?.textContent).toBe('Xuất log để báo lỗi')
    expect(el.querySelector('.vv-logs__export a, .vv-logs__export button')).toBeTruthy()
    expect(el.textContent).toContain('.zip')
    expect(el.textContent).toContain('5 video gần nhất')
  })

  it('lỗi gần đây hiện giờ, việc, cách xử lý và số lỗi 7 ngày', async () => {
    const el = await render()
    expect(el.textContent).toContain('Lỗi gần đây')
    expect(el.textContent).toContain('Tải video 12/18 lỗi 403')
    expect(el.textContent).toContain('23:15')
    expect(el.querySelectorAll('.vv-logs__err-it').length).toBe(2)
  })

  it('lỗi chưa xử lý dùng chấm cam lỗi và nhãn cam (artboard logs), không phải vàng', async () => {
    const el = await render()
    const warnRow = [...el.querySelectorAll('.vv-logs__err-it')].find((r) => r.textContent?.includes('Đang chờ'))
    for (const atom of stylex.props(styles.dotErr).className!.split(' ')) expect(warnRow?.querySelector('.vv-dot')?.classList.contains(atom)).toBe(true)
    for (const atom of stylex.props(styles.logsErrStIsErr).className!.split(' ')) expect(warnRow?.querySelector('.vv-logs__err-st')?.classList.contains(atom)).toBe(true)
    const mutedRow = [...el.querySelectorAll('.vv-logs__err-it')].find((r) => r.textContent?.includes('App đã xử lý'))
    for (const atom of stylex.props(styles.dotMuted).className!.split(' ')) expect(mutedRow?.querySelector('.vv-dot')?.classList.contains(atom)).toBe(true)
    for (const atom of stylex.props(styles.logsErrStIsMuted).className!.split(' ')) expect(mutedRow?.querySelector('.vv-logs__err-st')?.classList.contains(atom)).toBe(true)
  })

  it('không có lỗi thì hiện câu trống thay vì bảng rỗng', async () => {
    errors = { count7d: 0, items: [] }
    const el = await render()
    expect(el.textContent).toContain('Không có lỗi nào trong 7 ngày')
  })

  it('Kiểm tra bản mới gọi POST thật và hiện bản mới', async () => {
    latest = '1.5.0'
    const el = await render()
    const btn = [...el.querySelectorAll('button')].find((b) => b.textContent?.includes('Kiểm tra bản mới'))
    await click(btn)
    expect(calls.some((c) => c.url === '/api/version/check' && c.method === 'POST')).toBe(true)
    expect(el.textContent).toContain('Có bản 1.5.0')
  })
})
