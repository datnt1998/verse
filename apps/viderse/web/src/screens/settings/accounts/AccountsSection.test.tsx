// @vitest-environment happy-dom
// Tài khoản nền tảng — bố cục theo artboard Settings section=accounts và
// MSettings screen=accounts. Wire khớp internal/web/platforms.go; test giữ
// hành vi: chọn hồ sơ lưu ngay (Tắt = xoá có xác nhận), banner bot Thử lại,
// Gia hạn → login/refresh, Xoá đăng nhập có xác nhận, QR poll 2 s duy nhất khi
// mở và bốn trạng thái hộp mã (chờ quét · đã quét · hết hạn · từ chối · xong).
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { invalidate } from '@noverse/ui/data'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { AccountsSection } from './AccountsSection'
import type { Platform } from '../../../api/platforms'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

const youtube: Platform = {
  id: 'youtube', label: 'YouTube', uses: ['competitor'], kinds: ['video'], targetHeight: 360,
  state: 'needs_login',
  login: { type: 'browser_cookie', saved: false, renewable: false },
  usage: { channels: 0, videos: 0, rivals: 5 },
}
const twitch: Platform = {
  id: 'twitch', label: 'Twitch', uses: ['backdrop'], kinds: ['clip', 'highlight', 'vod'], targetHeight: 480,
  state: 'ready',
  login: { type: 'none', saved: false, renewable: false },
  usage: { channels: 0, videos: 3 },
}
const bilibili = (over: Partial<Platform> = {}, login: Partial<Platform['login']> = {}): Platform => ({
  id: 'bilibili', label: 'Bilibili', uses: ['backdrop'], kinds: ['video'], targetHeight: 1080,
  state: 'ready',
  login: { type: 'qr', saved: true, account: 'kenh-phu-01', method: 'qr', renewable: true, masked: 'SESSDATA=8f3a••••c21e', lastRefreshAt: new Date().toISOString(), ...login },
  usage: { channels: 1, videos: 2 },
  ...over,
})

const roots: Root[] = []
let calls: { url: string; method: string; body?: string }[] = []
let qrStatus: 'waiting' | 'scanned' | 'success' | 'expired' | 'rejected' = 'waiting'
let platforms: Platform[] = []
let qrExpiresMs = 180_000
/** Mã HTTP của POST tạo phiên QR (0 = lỗi mạng, 200 = tạo được). */
let qrCreateStatus = 200

const respond = (body: unknown, status = 200) => Promise.resolve(Response.json(body, { status }))
const qrGets = () => calls.filter((c) => c.method === 'GET' && c.url.includes('/login/qr/')).length

let mobile = false

async function render() {
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)
  roots.push(root)
  await act(async () => { root.render(<AccountsSection />) })
  await act(async () => { await vi.advanceTimersByTimeAsync(30) })
  return container
}

const choose = async (el: ParentNode, value: string) => {
  const select = el.querySelector('select')
  expect(select).toBeTruthy()
  await act(async () => {
    const setter = Object.getOwnPropertyDescriptor(window.HTMLSelectElement.prototype, 'value')?.set
    setter?.call(select, value)
    select!.dispatchEvent(new Event('change', { bubbles: true }))
  })
}

const click = async (el: Element | null | undefined) => {
  expect(el).toBeTruthy()
  await act(async () => { (el as HTMLElement).dispatchEvent(new MouseEvent('click', { bubbles: true })) })
}

const lastDialog = () => [...document.querySelectorAll('[role="dialog"],[role="alertdialog"]')].pop()

const byText = (root: ParentNode, sel: string, text: string) =>
  [...root.querySelectorAll(sel)].find((el) => el.textContent?.includes(text))

const cardOf = (root: ParentNode, label: string) =>
  [...root.querySelectorAll('.sacc-card')].find((c) => c.textContent?.includes(label))!

const buttons = (root: ParentNode) => [...root.querySelectorAll('button')]

function stubFetch() {
  vi.stubGlobal('fetch', vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = typeof input === 'string' ? input : input instanceof URL ? input.href : input.url
    const method = init?.method ?? 'GET'
    calls.push({ url, method, body: typeof init?.body === 'string' ? init.body : undefined })
    if (url === '/api/platforms') return respond(platforms)
    if (url === '/api/platforms/youtube/login/profiles') {
      return respond([
        { browser: 'Chrome', profile: 'Kênh chính', name: 'Kênh chính' },
        { browser: 'Edge', profile: 'Kênh phụ', name: 'Kênh phụ' },
      ])
    }
    if (url === '/api/platforms/bilibili/login/qr' && method === 'POST') {
      if (qrCreateStatus !== 200) {
        if (qrCreateStatus === 0) return Promise.reject(new TypeError('Failed to fetch'))
        return respond({ error: 'upstream failed', reason: 'upstream_failed' }, qrCreateStatus)
      }
      return respond({ qrcodeKey: 'k1', qrImage: 'data:image/svg+xml;base64,PHN2Zy8+', loginUrl: 'https://passport.bilibili.com/h5-app/passport/login/scan', expiresAt: new Date(Date.now() + qrExpiresMs).toISOString() })
    }
    if (url === '/api/platforms/bilibili/login/refresh' && method === 'POST') {
      return respond({ type: 'qr', saved: true, account: 'kenh-phu-01', method: 'qr', renewable: true, status: 'ready' })
    }
    if (url.startsWith('/api/platforms/bilibili/login/qr/') && method === 'GET') {
      return respond({ status: qrStatus, account: qrStatus === 'success' ? 'kenh-phu-02' : undefined })
    }
    return respond({ error: 'not found', reason: 'not_found' }, 404)
  }))
}

function setup(rows: Platform[]) {
  platforms = rows
  calls = []
  qrStatus = 'waiting'
  mobile = false
  qrExpiresMs = 180_000
  qrCreateStatus = 200
  void invalidate()
  vi.useFakeTimers()
  vi.stubGlobal('matchMedia', vi.fn().mockImplementation(() => ({
    matches: mobile, addEventListener: () => {}, removeEventListener: () => {},
  })))
  stubFetch()
}

afterEach(async () => {
  await act(async () => { await vi.advanceTimersByTimeAsync(10) })
  for (const root of roots.splice(0)) await act(async () => { root.unmount() })
  document.body.innerHTML = ''
  vi.clearAllTimers()
  vi.useRealTimers()
  vi.unstubAllGlobals()
})

describe('Tài khoản nền tảng', () => {
  it('thẻ theo artboard: ô chữ cái, câu dùng cho, chất lượng + số nguồn, link, nhãn trạng thái', async () => {
    setup([youtube, twitch, bilibili()])
    const el = await render()
    expect(cardOf(el, 'YouTube').textContent).toContain('YT')
    expect(cardOf(el, 'YouTube').textContent).toContain('Video đối thủ: tải video gốc, phụ đề, tiêu đề, lượt xem')
    expect(cardOf(el, 'YouTube').textContent).toContain('Tải 360p + phụ đề · 5 đối thủ · Mở Khám phá')
    expect(cardOf(el, 'Twitch').textContent).toContain('TW')
    expect(cardOf(el, 'Twitch').textContent).toContain('Không cần đăng nhập: app chỉ đọc clip, highlight và video phát lại công khai.')
    expect(cardOf(el, 'Bilibili').textContent).toContain('Tải 1080p · 1 kênh, 2 video lẻ')
    expect(el.textContent).toContain('Sẵn sàng')
    expect(byText(el, '.sacc-more', 'Nền tảng khác')).toBeTruthy()
    expect(el.textContent).toContain('Chỉ dùng tài khoản phụ, không dùng tài khoản đang kiếm tiền.')
  })

  it('YouTube: đổi hồ sơ lưu ngay bằng PUT (không có nút Lưu riêng)', async () => {
    setup([{ ...youtube, state: 'ready', login: { type: 'browser_cookie', saved: false, renewable: false } }])
    const el = await render()
    await choose(cardOf(el, 'YouTube'), 'Edge:Kênh phụ')
    await act(async () => { await vi.advanceTimersByTimeAsync(50) })
    const put = calls.find((c) => c.method === 'PUT' && c.url === '/api/platforms/youtube/login')
    expect(put?.body).toContain('"browser":"Edge"')
    expect(buttons(el).some((b) => b.textContent === 'Lưu hồ sơ')).toBe(false)
  })

  it('YouTube: chọn "Tắt" hỏi xác nhận cam rồi DELETE', async () => {
    setup([{ ...youtube, state: 'ready', login: { type: 'browser_cookie', saved: true, browserProfile: 'Chrome:Kênh chính', renewable: false } }])
    const el = await render()
    expect(el.textContent).toContain('Đang dùng · lần tải gần nhất không lỗi')
    await choose(cardOf(el, 'YouTube'), 'off')
    await act(async () => { await vi.advanceTimersByTimeAsync(20) })
    const dialog = lastDialog()
    expect(dialog?.textContent).toContain('Tắt cookie YouTube?')
    await click(byText(dialog!, 'button', 'Tắt'))
    await act(async () => { await vi.advanceTimersByTimeAsync(50) })
    expect(calls.some((c) => c.method === 'DELETE' && c.url === '/api/platforms/youtube/login')).toBe(true)
  })

  it('YouTube bot: banner “xác nhận bạn không phải bot” + Thử lại → login/check', async () => {
    setup([{
      ...youtube,
      state: 'ready',
      login: { type: 'browser_cookie', saved: true, browserProfile: 'Chrome:Kênh chính', renewable: false },
      lastError: { code: 'cookies_failed', at: new Date().toISOString() },
    }])
    const el = await render()
    expect(el.textContent).toContain('YouTube báo “xác nhận bạn không phải bot”')
    expect(el.textContent).toContain('Cần cookie')
    await click(byText(el, 'button', 'Thử lại'))
    await act(async () => { await vi.advanceTimersByTimeAsync(50) })
    expect(calls.some((c) => c.method === 'POST' && c.url === '/api/platforms/youtube/login/check')).toBe(true)
  })

  it('Bilibili chưa đăng nhập: câu lợi ích + nút chính, không có nút xoá', async () => {
    setup([bilibili({ state: 'needs_login' }, { saved: false, account: undefined, method: undefined, renewable: false })])
    const el = await render()
    const card = cardOf(el, 'Bilibili')
    expect(card.textContent).toContain('Đăng nhập tài khoản phụ')
    expect(card.textContent).toContain('· tuỳ chọn')
    expect(card.textContent).toContain('Quét mã một lần bằng app 哔哩哔哩')
    expect(buttons(card).some((b) => b.textContent === 'Xoá đăng nhập')).toBe(false)
    expect(buttons(card).some((b) => b.textContent === 'Đăng nhập bằng mã QR')).toBe(true)
  })

  it('Bilibili mã QR: nhãn Mã QR, dòng tự gia hạn, Gia hạn ngay → login/refresh, Xoá có xác nhận', async () => {
    setup([bilibili()])
    const el = await render()
    const card = cardOf(el, 'Bilibili')
    expect(card.textContent).toContain('Mã QR')
    expect(card.textContent).toContain('Tự gia hạn mỗi ngày')
    expect(card.textContent).toContain('SESSDATA=8f3a••••c21e')
    await click(byText(card, 'button', 'Gia hạn ngay'))
    await act(async () => { await vi.advanceTimersByTimeAsync(50) })
    expect(calls.some((c) => c.method === 'POST' && c.url === '/api/platforms/bilibili/login/refresh')).toBe(true)
    expect(cardOf(el, 'Bilibili').textContent).toContain('Vừa kiểm tra lúc')
  })

  it('Bilibili cookie dán tay: nhãn xám, nhắc không tự gia hạn, Quét mã QR; thiếu SESSDATA không gửi', async () => {
    setup([bilibili({}, { method: 'manual', renewable: false })])
    const el = await render()
    const card = cardOf(el, 'Bilibili')
    expect(card.textContent).toContain('Cookie dán tay')
    expect(card.textContent).toContain('Cookie dán tay không tự gia hạn')
    expect(buttons(card).some((b) => b.textContent === 'Đăng nhập bằng mã QR')).toBe(true)
    await click(byText(card, 'button', 'Dán cookie khác'))
    await act(async () => { await vi.advanceTimersByTimeAsync(20) })
    const area = card.querySelector('textarea')!
    expect(area).toBeTruthy()
    await act(async () => {
      Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype, 'value')!.set!.call(area, 'buvid3=abc')
      area.dispatchEvent(new Event('input', { bubbles: true }))
    })
    await click(byText(card, 'button', 'Lưu cookie'))
    await act(async () => { await vi.advanceTimersByTimeAsync(50) })
    expect(calls.some((c) => c.method === 'PUT' && c.url === '/api/platforms/bilibili/login')).toBe(false)
  })

  it('Bilibili hết hạn: nhãn Mã QR · hết hạn, banner hết hạn + Quét mã QR, chỉ còn Xoá', async () => {
    setup([bilibili({ state: 'needs_login' }, { refreshError: 'expired' })])
    const el = await render()
    const card = cardOf(el, 'Bilibili')
    expect(card.textContent).toContain('Mã QR · hết hạn')
    expect(card.textContent).toContain('Đăng nhập Bilibili đã hết hạn')
    expect(buttons(card).some((b) => b.textContent === 'Gia hạn ngay')).toBe(false)
    expect(buttons(card).some((b) => b.textContent === 'Quét mã QR')).toBe(true)
    expect(buttons(card).some((b) => b.textContent === 'Xoá đăng nhập')).toBe(true)
  })

  it('Bilibili tạm chặn: nhãn Tạm chặn tới HH:MM + banner giới hạn yêu cầu', async () => {
    setup([bilibili({ state: 'paused', pausedUntil: new Date(Date.now() + 7_200_000).toISOString(), failureCount: 4 })])
    const el = await render()
    const card = cardOf(el, 'Bilibili')
    expect(card.textContent).toMatch(/Tạm chặn tới \d\d:\d\d/)
    expect(card.textContent).toContain('Bilibili đang giới hạn yêu cầu từ mạng này')
    expect(card.textContent).toContain('Bị chặn 4 lần liên tiếp')
  })

  it('QR: mở hộp → tạo phiên, poll 2 s không chồng request; đã quét → success → Xong', async () => {
    setup([bilibili({ state: 'needs_login' }, { saved: false, account: undefined, method: undefined, renewable: false })])
    const el = await render()
    await click(byText(el, 'button', 'Đăng nhập bằng mã QR'))
    await act(async () => { await vi.advanceTimersByTimeAsync(20) })
    const dialog = lastDialog()!
    expect(dialog.textContent).toContain('Đăng nhập Bilibili bằng mã QR')
    expect(dialog.textContent).toContain('Đang chờ quét')
    expect(calls.filter((c) => c.method === 'POST' && c.url === '/api/platforms/bilibili/login/qr')).toHaveLength(1)
    qrStatus = 'scanned'
    await act(async () => { await vi.advanceTimersByTimeAsync(2100) })
    expect(lastDialog()!.textContent).toContain('Đã quét. Bấm xác nhận trên điện thoại để xong.')
    qrStatus = 'success'
    await act(async () => { await vi.advanceTimersByTimeAsync(2100) })
    expect(lastDialog()!.textContent).toContain('Đã đăng nhập: kenh-phu-02')
    expect(lastDialog()!.textContent).toContain('App tự gia hạn đăng nhập mỗi ngày khi quét kênh.')
    await click(byText(lastDialog()!, 'button', 'Xong'))
    await act(async () => { await vi.advanceTimersByTimeAsync(30) })
    expect(qrGets()).toBeGreaterThanOrEqual(2)
    expect(lastDialog()?.textContent ?? '').not.toContain('Đăng nhập Bilibili bằng mã QR')
  })

  it('QR: mã hết hạn → “Mã đã hết hạn sau 3 phút.” + Tạo mã mới → phiên mới', async () => {
    setup([bilibili({ state: 'needs_login' }, { saved: false, account: undefined, method: undefined, renewable: false })])
    const el = await render()
    qrExpiresMs = 1000
    await click(byText(el, 'button', 'Đăng nhập bằng mã QR'))
    await act(async () => { await vi.advanceTimersByTimeAsync(20) })
    await act(async () => { await vi.advanceTimersByTimeAsync(1200) })
    expect(lastDialog()!.textContent).toContain('Mã đã hết hạn sau 3 phút.')
    qrExpiresMs = 180_000
    await click(byText(lastDialog()!, 'button', 'Tạo mã mới'))
    await act(async () => { await vi.advanceTimersByTimeAsync(30) })
    expect(calls.filter((c) => c.method === 'POST' && c.url === '/api/platforms/bilibili/login/qr')).toHaveLength(2)
    expect(lastDialog()!.textContent).toContain('Đang chờ quét')
  })

  it('QR: lỗi mạng khi tạo mã → banner “Chưa kết nối được”, KHÔNG phải lời từ chối', async () => {
    setup([bilibili({ state: 'needs_login' }, { saved: false, account: undefined, method: undefined, renewable: false })])
    qrCreateStatus = 502
    const el = await render()
    await click(byText(el, 'button', 'Đăng nhập bằng mã QR'))
    await act(async () => { await vi.advanceTimersByTimeAsync(30) })
    const dialog = lastDialog()!
    expect(dialog.textContent).toContain('Chưa có mã')
    expect(dialog.textContent).toContain('Chưa kết nối được Bilibili')
    expect(dialog.textContent).toContain('Máy chạy Viderse chưa gọi được Bilibili. Kiểm tra mạng của máy đó rồi tạo mã mới.')
    expect(dialog.textContent).not.toContain('Bilibili chưa chấp nhận đăng nhập')
    expect(dialog.textContent).not.toContain('Huỷ trên điện thoại')
    expect(dialog.querySelector('.sacc-qr__skel')).toBeNull()
    expect(byText(dialog, 'button', 'Tạo mã mới')).toBeTruthy()
  })

  it('QR: mất mạng hẳn (không có HTTP) cũng là lỗi kết nối, không phải từ chối', async () => {
    setup([bilibili({ state: 'needs_login' }, { saved: false, account: undefined, method: undefined, renewable: false })])
    qrCreateStatus = 0
    const el = await render()
    await click(byText(el, 'button', 'Đăng nhập bằng mã QR'))
    await act(async () => { await vi.advanceTimersByTimeAsync(30) })
    const dialog = lastDialog()!
    expect(dialog.textContent).toContain('Chưa kết nối được Bilibili')
    expect(dialog.textContent).not.toContain('Bilibili chưa chấp nhận đăng nhập')
  })

  it('QR: 409 → “Bilibili đang bận”, không phải từ chối', async () => {
    setup([bilibili({ state: 'needs_login' }, { saved: false, account: undefined, method: undefined, renewable: false })])
    qrCreateStatus = 409
    const el = await render()
    await click(byText(el, 'button', 'Đăng nhập bằng mã QR'))
    await act(async () => { await vi.advanceTimersByTimeAsync(30) })
    const dialog = lastDialog()!
    expect(dialog.textContent).toContain('Bilibili đang bận')
    expect(dialog.textContent).not.toContain('Bilibili chưa chấp nhận đăng nhập')
  })

  it('QR: bị từ chối giữ bước 1–2 đã xong (artboard không reset bước)', async () => {
    setup([bilibili({ state: 'needs_login' }, { saved: false, account: undefined, method: undefined, renewable: false })])
    qrStatus = 'rejected'
    const el = await render()
    await click(byText(el, 'button', 'Đăng nhập bằng mã QR'))
    await act(async () => { await vi.advanceTimersByTimeAsync(2100) })
    const dialog = lastDialog()!
    expect(dialog.textContent).toContain('Bilibili chưa chấp nhận đăng nhập')
    expect(dialog.querySelectorAll('.sacc-qr__step--done')).toHaveLength(2)
    expect(dialog.textContent).toContain('Chưa đăng nhập được')
  })

  it('QR: bị từ chối → banner + Tạo mã mới', async () => {
    setup([bilibili({ state: 'needs_login' }, { saved: false, account: undefined, method: undefined, renewable: false })])
    const el = await render()
    await click(byText(el, 'button', 'Đăng nhập bằng mã QR'))
    await act(async () => { await vi.advanceTimersByTimeAsync(20) })
    qrStatus = 'rejected'
    await act(async () => { await vi.advanceTimersByTimeAsync(2100) })
    const dialog = lastDialog()!
    expect(dialog.textContent).toContain('Bilibili chưa chấp nhận đăng nhập')
    expect(dialog.textContent).toContain('Chưa đăng nhập được')
    expect(buttons(dialog).some((b) => b.textContent === 'Tạo mã mới')).toBe(true)
  })

  it('điện thoại QR: thẻ “trên chính máy này?” chỉ hiện khi chờ quét, ẩn sau khi đã quét', async () => {
    setup([bilibili({ state: 'needs_login' }, { saved: false, account: undefined, method: undefined, renewable: false })])
    mobile = true
    const el = await render()
    await click(byText(el, 'button', 'Đăng nhập'))
    await act(async () => { await vi.advanceTimersByTimeAsync(30) })
    expect(lastDialog()!.querySelector('.sacc-qr__same')).toBeTruthy()
    qrStatus = 'scanned'
    await act(async () => { await vi.advanceTimersByTimeAsync(2100) })
    const sheet = lastDialog()!
    expect(sheet.querySelector('.sacc-qr__same')).toBeNull()
    expect(sheet.textContent).toContain('Đã quét. Bấm xác nhận trên điện thoại.')
    expect(sheet.textContent).not.toContain('để xong')
    expect(sheet.querySelectorAll('.sacc-qr__step--done')).toHaveLength(2)
  })

  it('điện thoại QR: nút cuối sheet chỉ có chữ, không icon', async () => {
    setup([bilibili({ state: 'needs_login' }, { saved: false, account: undefined, method: undefined, renewable: false })])
    mobile = true
    const el = await render()
    await click(byText(el, 'button', 'Đăng nhập'))
    await act(async () => { await vi.advanceTimersByTimeAsync(30) })
    qrStatus = 'rejected'
    await act(async () => { await vi.advanceTimersByTimeAsync(2100) })
    const retry = byText(lastDialog()!, 'button', 'Tạo mã mới') as HTMLElement | undefined
    expect(retry).toBeTruthy()
    expect(retry?.querySelector('span[role="img"], svg')).toBeFalsy()
    expect(lastDialog()!.querySelectorAll('.sacc-qr__step--done')).toHaveLength(2)
  })

  it('điện thoại: câu dẫn + thẻ gọn không select, không thẻ “Nền tảng khác”, cuối có ghi chú', async () => {
    mobile = true
    setup([youtube, twitch, bilibili()])
    vi.stubGlobal('matchMedia', vi.fn().mockImplementation(() => ({ matches: true, addEventListener: () => {}, removeEventListener: () => {} })))
    const el = await render()
    expect(el.textContent).toContain('Các nền tảng app lấy video từ đó. Đăng nhập chỉ cần khi nền tảng đòi; chỉ dùng tài khoản phụ.')
    expect(cardOf(el, 'YouTube').textContent).toContain('Cookie từ trình duyệt')
    expect(cardOf(el, 'YouTube').textContent).toContain('Chưa chọn · đổi trên máy tính')
    expect(cardOf(el, 'YouTube').querySelector('[role="combobox"],select,button[aria-haspopup]')).toBeNull()
    expect(cardOf(el, 'Twitch').textContent).toContain('Chỉ đọc clip và video công khai')
    expect(cardOf(el, 'Bilibili').textContent).toContain('kenh-phu-01 · mã QR')
    expect(cardOf(el, 'Bilibili').textContent).toContain('Tự gia hạn mỗi ngày')
    expect(byText(el, '.sacc-more', 'Nền tảng khác')).toBeFalsy()
    expect(el.textContent).toContain('Thêm kênh hoặc video lẻ ở Thư viện › Video nền trên máy tính.')
    expect(el.textContent).toContain('Gia hạn')
  })
})

describe('Hợp đồng CSS theo artboard', () => {
  const css = readFileSync(resolve('src/screens/settings/accounts/accounts.css'), 'utf8')

  it('ô chọn trình duyệt YouTube nằm một hàng riêng, rộng đầy nội dung (vv-select width:100% + wrap)', () => {
    vi.useFakeTimers()
    const rule = css.match(/\.sacc-select\s*\{[^}]*\}/)?.[0] ?? ''
    expect(rule).toContain('width: 100%')
    const row = css.match(/\.sacc-yt\s*\{[^}]*\}/)?.[0] ?? ''
    expect(row).toContain('flex-wrap: wrap')
  })

  it('badge bước QR (xong · đang làm) chỉ dùng token có định nghĩa', () => {
    vi.useFakeTimers()
    const defined = new Set([
      readFileSync(resolve('src/styles/viderse.css'), 'utf8'),
      readFileSync(resolve('../../../platform/ui/src/tokenDefaults.ts'), 'utf8'),
      css,
    ].flatMap((src) => [...src.matchAll(/["']?(--[\w-]+)["']?\s*:/g)].map((m) => m[1])))
    for (const state of ['done', 'now']) {
      const rule = css.match(new RegExp(`\\.sacc-qr__step--${state} \\.sacc-qr__stepnum\\s*\\{[^}]*\\}`))?.[0] ?? ''
      const refs = [...rule.matchAll(/var\((--[\w-]+)\)/g)].map((m) => m[1])
      expect(refs.length, state).toBeGreaterThan(0)
      expect(refs.filter((r) => !defined.has(r)), state).toEqual([])
    }
  })
})
