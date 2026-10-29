// @vitest-environment happy-dom
// Invariant #7 (docs/design/v1/21-dev-prompt.md · Gói A): chữ kỹ thuật không bao giờ hiện ra ở UI thường.
// Dựng nguyên <App/> với API mock thật (web/mock/server.ts chạy trong tiến trình qua http loopback), mở từng route/viewport,
// gom chữ NHÌN THẤY (textContent trừ phần ẩn + aria-label/title/placeholder) rồi so với mẫu cấm.
// Ngoại lệ có chủ đích: Nâng cao (admin), khối "Chi tiết kỹ thuật" (<details> hoặc [data-tech]), trang OAuth from=cli (Go, không render ở đây).
import http from 'node:http'
import type { AddressInfo } from 'node:net'
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterAll, afterEach, beforeAll, describe, expect, it, vi } from 'vitest'
import App from '../App'
import { mockApiPlugin } from '../../mock/server'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

const FORBIDDEN = /(~\/|\.ainovel|\.json\b|output\/|kich_ban|y-tuong|tai-san|xuat-ban|ainovel-cli|\b[a-z]+_[a-z_]+\b)/

// ─── mock API qua http loopback ──────────────────────────────────────────────
type Handler = (req: http.IncomingMessage, res: http.ServerResponse, next: () => void) => unknown
const stack: { prefix: string; fn: Handler }[] = []
let server: http.Server
let port = 0
let inflight = 0
let jar = ''

beforeAll(async () => {
  const configure = mockApiPlugin().configureServer as (server: unknown) => void
  configure({ middlewares: { use: (prefix: string, fn: Handler) => { stack.push({ prefix, fn }) } }, httpServer: null })
  server = http.createServer((req, res) => {
    const original = req.url ?? '/'
    let index = 0
    const next = () => {
      const entry = stack[index++]
      if (!entry) { res.statusCode = 404; res.end('{"error":"Not found"}'); return }
      if (!original.startsWith(entry.prefix)) return next()
      req.url = original.slice(entry.prefix.length) || '/'
      void entry.fn(req, res, next)
    }
    next()
  })
  await new Promise<void>((done) => server.listen(0, '127.0.0.1', done))
  port = (server.address() as AddressInfo).port
})
afterAll(async () => { server.closeAllConnections(); await new Promise((done) => server.close(done)) })

function loopbackFetch(input: RequestInfo | URL, init?: RequestInit): Promise<Response> {
  const raw = typeof input === 'string' ? input : input instanceof URL ? input.href : input.url
  const target = new URL(raw, 'http://127.0.0.1')
  const headers: Record<string, string> = { ...(init?.headers as Record<string, string> | undefined) }
  if (jar) headers.cookie = jar
  const live = headers.Accept === 'text/event-stream' // luồng sự kiện sống mãi: không tính vào inflight
  let resolve!: (response: Response) => void
  let reject!: (reason: unknown) => void
  const promise = new Promise<Response>((ok, fail) => { resolve = ok; reject = fail })
  if (!live) inflight++
  const req = http.request({ host: '127.0.0.1', port, path: target.pathname + target.search, method: init?.method ?? 'GET', headers }, (res) => {
    for (const cookie of res.headers['set-cookie'] ?? []) jar = cookie.split(';')[0]
    const status = res.statusCode ?? 500
    const responseInit = { status, headers: { 'content-type': String(res.headers['content-type'] ?? 'application/json') } }
    if (live) {
      resolve(new Response(new ReadableStream<Uint8Array>({
        start(controller) {
          res.on('data', (chunk: Buffer) => controller.enqueue(new Uint8Array(chunk)))
          res.on('end', () => controller.close())
          res.on('error', (error) => controller.error(error))
        },
        cancel() { req.destroy() },
      }), responseInit))
      return
    }
    const chunks: Buffer[] = []
    res.on('data', (chunk: Buffer) => chunks.push(chunk))
    res.on('end', () => {
      inflight--
      resolve(new Response(status === 204 || status === 304 ? null : Buffer.concat(chunks), responseInit))
    })
  })
  req.on('error', (error) => { if (!live) inflight--; reject(error) })
  init?.signal?.addEventListener('abort', () => { req.destroy(); reject(new DOMException('aborted', 'AbortError')) })
  if (init?.body) req.write(init.body as string)
  req.end()
  return promise
}

/** EventSource tối giản chạy trên loopbackFetch: đủ cho useStoryEvents (open · event có tên · error · close). */
class MockEventSource extends EventTarget {
  readyState = 0
  onerror: ((event: Event) => void) | null = null
  private abort = new AbortController()
  constructor(public url: string) {
    super()
    void this.pump()
  }

  private async pump() {
    try {
      const response = await loopbackFetch(this.url, { headers: { Accept: 'text/event-stream' }, signal: this.abort.signal })
      if (!response.ok || !response.body) throw new Error('sse')
      this.readyState = 1
      this.dispatchEvent(new Event('open'))
      const reader = response.body.getReader()
      const decoder = new TextDecoder()
      let buffer = ''
      for (;;) {
        const { done, value } = await reader.read()
        if (done) break
        buffer += decoder.decode(value, { stream: true })
        let cut = buffer.indexOf('\n\n')
        while (cut >= 0) {
          this.dispatchFrame(buffer.slice(0, cut))
          buffer = buffer.slice(cut + 2)
          cut = buffer.indexOf('\n\n')
        }
      }
      throw new Error('sse closed')
    } catch {
      if (this.readyState === 2) return
      const event = new Event('error')
      this.onerror?.(event)
      this.dispatchEvent(event)
    }
  }

  private dispatchFrame(frame: string) {
    let type = 'message'
    const data: string[] = []
    for (const line of frame.split('\n')) {
      if (line.startsWith('event:')) type = line.slice(6).trim()
      else if (line.startsWith('data:')) data.push(line.slice(5).replace(/^ /, ''))
    }
    if (data.length) this.dispatchEvent(Object.assign(new Event(type), { data: data.join('\n') }))
  }

  close() { this.readyState = 2; this.abort.abort() }
}

async function signIn(username: string | null) {
  jar = ''
  if (!username) return
  const password = { dat: 'mat-khau-123', vien: 'vien-12345', lan: 'lan-123456' }[username]
  const response = await loopbackFetch('/api/login', { method: 'POST', headers: { 'content-type': 'application/json' }, body: JSON.stringify({ username, password }) })
  expect(response.ok).toBe(true)
}

function setViewport(width: number) {
  Object.defineProperty(window, 'innerWidth', { configurable: true, value: width })
  vi.stubGlobal('matchMedia', (query: string) => {
    const max = /max-width:\s*(\d+)px/.exec(query)?.[1]
    const min = /min-width:\s*(\d+)px/.exec(query)?.[1]
    const matches = (max === undefined || width <= Number(max)) && (min === undefined || width >= Number(min))
    return { matches, media: query, addEventListener() {}, removeEventListener() {}, addListener() {}, removeListener() {} }
  })
}

// ─── chữ nhìn thấy ───────────────────────────────────────────────────────────
const SKIP_TAGS = new Set(['SCRIPT', 'STYLE', 'TEMPLATE', 'NOSCRIPT', 'SVG', 'svg', 'HEAD'])

function isHidden(el: Element): boolean {
  if (el.hasAttribute('hidden')) return true
  const style = (el.getAttribute('style') ?? '').replace(/\s+/g, '')
  return /display:none|visibility:hidden/.test(style)
}

/** textContent trừ phần ẩn / chi tiết kỹ thuật, cộng aria-label · title · placeholder (mỗi mục một dòng). */
function visibleText(root: Element): string[] {
  const out: string[] = []
  const walk = (node: Node) => {
    if (node.nodeType === 3) { const text = (node.textContent ?? '').trim(); if (text) out.push(text); return }
    if (node.nodeType !== 1) return
    const el = node as Element
    if (SKIP_TAGS.has(el.tagName) || isHidden(el) || el.hasAttribute('data-tech')) return
    for (const attr of ['aria-label', 'title', 'placeholder']) {
      const value = el.getAttribute(attr)?.trim()
      if (value) out.push(value)
    }
    for (const child of Array.from(el.childNodes)) {
      // <details>: chỉ <summary> là chữ thường; phần còn lại là khối chi tiết kỹ thuật.
      if (el.tagName === 'DETAILS' && !(child.nodeType === 1 && (child as Element).tagName === 'SUMMARY')) continue
      walk(child)
    }
  }
  walk(root)
  return out
}

function leaks(root: Element): string[] {
  const found: string[] = []
  for (const text of visibleText(root)) {
    const match = FORBIDDEN.exec(text)
    if (match) found.push(`«${match[0]}» trong: ${text.length > 140 ? text.slice(0, 140) + '…' : text}`)
  }
  return [...new Set(found)]
}

describe('bộ lọc chữ nhìn thấy', () => {
  const html = (markup: string) => { const el = document.createElement('div'); el.innerHTML = markup; return el }
  it('bắt từng dạng chữ kỹ thuật', () => {
    for (const text of ['kich_ban/01-x', 'y-tuong', 'tai-san', 'xuat-ban', '~/.ainovel/config.json', 'project.json', 'output/novel', 'ainovel-cli serve', 'commit_chapter', 'base_url']) {
      expect(FORBIDDEN.test(text), text).toBe(true)
    }
    for (const text of ['Ý tưởng · Tư liệu · Truyện · Bản xuất', 'Đít Đang Audio', 'Noverse tự sắp xếp tệp', 'Tuỳ chỉnh', 'chữ thường · không dấu', '/export', 'code#state']) {
      expect(FORBIDDEN.test(text), text).toBe(false)
    }
  })

  it('tính aria-label · title · placeholder, bỏ phần ẩn · <details> · [data-tech]', () => {
    const el = html(`
      <button aria-label='nhãn kich_ban' title='gợi ý y-tuong'>ok</button><input placeholder='vd: project.json'>
      <p hidden>output/novel</p><p style='display: none'>tai-san</p><span data-tech><code>commit_chapter</code></span>
      <details><summary>Chi tiết kỹ thuật</summary><code>base_url</code></details>`)
    expect(visibleText(el)).toEqual(['nhãn kich_ban', 'gợi ý y-tuong', 'ok', 'vd: project.json', 'Chi tiết kỹ thuật'])
    expect(leaks(el).map((leak) => /«([^»]+)»/.exec(leak)?.[1])).toEqual(['kich_ban', 'y-tuong', '.json'])
  })
})

// ─── render ──────────────────────────────────────────────────────────────────
const sleep = (ms: number) => new Promise((done) => setTimeout(done, ms))
const roots: Root[] = []

async function settle(container: HTMLElement) {
  let stable = 0
  let last = ''
  for (let i = 0; i < 80 && stable < 4; i++) {
    await act(async () => { await sleep(25) })
    const now = inflight === 0 ? container.innerHTML.length + ':' + document.body.innerHTML.length : '…' + i
    stable = now === last ? stable + 1 : 0
    last = now
  }
}

async function open(hash: string, opts: { user?: string | null; width?: number } = {}): Promise<HTMLElement> {
  vi.stubGlobal('fetch', loopbackFetch)
  vi.stubGlobal('EventSource', MockEventSource)
  await signIn(opts.user === undefined ? 'dat' : opts.user)
  setViewport(opts.width ?? 1440)
  location.hash = hash
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)
  roots.push(root)
  await act(async () => { root.render(<App />) })
  await settle(container)
  return document.body
}

afterEach(async () => {
  for (const root of roots.splice(0)) await act(async () => { root.unmount() })
  document.body.replaceChildren()
  localStorage.clear()
  sessionStorage.clear()
  vi.unstubAllGlobals()
})

const P = 'dit-dang-audio'
const S = '01-tra-xanh-cuop-kich-ban'
const work = `/p/${P}/s/${S}/work`

/**
 * name · hash · user (mặc định quản trị 'dat'; null = chưa đăng nhập) · width (viewport)
 * scope: chỉ kiểm tra phần tử khớp selector (hộp thoại — nền là màn khác, có test riêng)
 * has: một chữ BẮT BUỘC thấy (chống 'màn trống/đang tải' làm test xanh giả)
 * click: lần lượt bấm nút có chữ/aria-label này, kiểm tra lại sau MỖI bước ('?' đầu = bỏ qua nếu không có)
 * pending: chủ sở hữu còn nợ rò chữ → test khẳng định VẪN rò (đỏ khi chủ đã sửa xong → gỡ cờ, lúc đó test thành bảo vệ thường)
 */
type Screen = { name: string; hash: string; user?: string | null; width?: number; scope?: string; has: string; click?: string[]; pending?: string }
const DESKTOP = 1440
const PHONE = 390
const TABLET = 820
const DIALOG = '[role="dialog"], [role="alertdialog"]'

async function check(screen: Screen): Promise<string[]> {
  const body = await open(screen.hash, screen)
  const roots = () => (screen.scope ? [...body.querySelectorAll(screen.scope)] : [body])
  const found: string[] = []
  const scan = (step: string) => {
    const scoped = roots()
    expect(scoped.length, `${step}: không thấy vùng ${screen.scope}`).toBeGreaterThan(0)
    for (const root of scoped) found.push(...leaks(root).map((leak) => (step ? `[${step}] ` : '') + leak))
  }
  expect(visibleText(body).join(' '), 'màn hình trống').toContain(screen.has)
  scan('')
  for (const raw of screen.click ?? []) {
    const optional = raw.startsWith('?')
    const label = optional ? raw.slice(1) : raw
    const buttons = [...body.querySelectorAll('button, a, [role="button"], [role="tab"]')]
    const target = buttons.find((el) => (el.textContent ?? '').trim() === label || el.getAttribute('aria-label') === label)
      ?? buttons.find((el) => (el.textContent ?? '').includes(label))
    if (!target && optional) continue
    expect(target, `không thấy nút «${label}»`).toBeTruthy()
    const before = visibleText(body).join(' | ')
    await act(async () => { (target as HTMLElement).click() })
    await settle(body)
    expect(visibleText(body).join(' | '), `bấm «${label}» không đổi màn`).not.toBe(before)
    scan(label)
  }
  return [...new Set(found)]
}

function suite(title: string, screens: Screen[]) {
  describe(title, () => {
    for (const screen of screens) {
      const label = `${screen.pending ? `[chờ ${screen.pending}] ` : ''}${screen.name} (${screen.hash} @${screen.width ?? DESKTOP})`
      it(label, async () => {
        const found = await check(screen)
        if (screen.pending) expect(found.length, `${screen.pending} đã sửa xong? gỡ cờ pending của màn này`).toBeGreaterThan(0)
        else expect(found.length, '\nRò chữ kỹ thuật:\n' + found.join('\n')).toBe(0)
      }, 30_000)
    }
  })
}

const sc = (name: string, hash: string, has: string, extra: Partial<Screen> = {}): Screen => ({ name, hash, has, ...extra })
const proj = `#/p/${P}`
const story = `#/p/${P}/s/${S}`
const MORE_PICK = ['?Trà Xanh Cướp Kịch Bản'] // lần mở đầu của phiên rơi vào danh sách dự án; các lần sau thẳng vào trang chủ
const resultPage = (command: string) => `#/?result=new&project=${P}&story=${S}&command=${encodeURIComponent(command)}`

// ─── thư viện · dự án · truyện — Gói E (PkgE) đang gỡ cây thư mục kỹ thuật khỏi màn này ───
suite('thư viện / dự án', [
  sc('thư viện (gốc)', '#/', 'Đít Đang Audio'),
  sc('dự án', proj, 'Đít Đang Audio'),
  sc('truyện', story, 'Trà Xanh Cướp Kịch Bản'),
  sc('dự án (điện thoại)', proj, 'Dự án & truyện', { width: PHONE }),
  sc('dự án (máy tính bảng)', proj, 'Đít Đang Audio', { width: TABLET }),
])

suite('khu viết: workspace · nhật ký · trình đọc · truyện mới', [
  sc('workspace desktop', `#${work}`, 'Luồng viết trực tiếp'),
  sc('workspace desktop — người viết', `#${work}`, 'Luồng viết trực tiếp', { user: 'vien' }),
  sc('workspace mobile', `#${work}`, 'Trợ lý', { width: PHONE }),
  sc('workspace tablet', `#${work}`, 'Trà Xanh Cướp Kịch Bản', { width: TABLET }),
  sc('nhật ký desktop', `${story}/activity`, 'Nhật ký tiến độ'),
  sc('nhật ký mobile', `${story}/activity`, 'Nhật ký hoạt động', { width: PHONE }),
  sc('trình đọc desktop', `${story}/read/1`, 'Vai của tôi là đồ canh'),
  sc('trình đọc desktop · Xuất truyện', `${story}/read/1`, 'Vai của tôi là đồ canh', { click: ['Xuất truyện'] }),
  sc('trình đọc mobile', `${story}/read/1`, 'Vai của tôi là đồ canh', { width: PHONE }),
  sc('trình đọc — người đọc', `${story}/read/1`, 'Vai của tôi là đồ canh', { user: 'lan' }),
  sc('truyện mới', `${proj}/new`, 'Bạn muốn viết câu chuyện nào?'),
  sc('truyện mới (điện thoại)', `${proj}/new`, 'Thêm', { width: PHONE }),
])

suite('hộp thoại · lệnh · trợ giúp', [
  sc('Dự án mới', `#${work}?dlg=newProject`, 'Dự án mới', { scope: DIALOG }),
  sc('Nhập truyện', `#${work}?dlg=import`, 'Nhập tiểu thuyết', { scope: DIALOG }),
  sc('Mở lại truyện', `#${work}?dlg=reopen`, 'Viết tiếp tập mới', { scope: DIALOG }),
  sc('Bảng lệnh', `#${work}?dlg=palette`, 'Bảng lệnh', { scope: DIALOG }),
  sc('Bắt đầu từ tệp', `#${work}?dlg=startFile`, 'Bắt đầu từ tệp', { scope: DIALOG }),
  sc('Nhập hồ sơ mô phỏng', `#${work}?dlg=importsim`, 'Nhập hồ sơ mô phỏng', { scope: DIALOG }),
  sc('Thiết bị đăng nhập', `#${work}?dlg=devices`, 'Đăng xuất mọi thiết bị khác', { scope: DIALOG }),
  sc('Mã QR điện thoại', `#${work}?dlg=qr`, 'Đừng chia sẻ mã này', { scope: DIALOG }),
  sc('Thành viên', '#/settings/members', 'Đặt lại mật khẩu'),
  sc('Chẩn đoán', `#${work}?dlg=diag`, 'Sức khoẻ truyện'),
  sc('Model', `#${work}?dlg=model`, 'Nhà cung cấp'),
  sc('Đồng sáng tác', `#${work}?dlg=cocreate`, 'Hội thoại đồng sáng tác'),
  sc('trợ giúp: cách dùng lệnh', '#/help/commands', 'Hướng dẫn lệnh'),
  ...['export', 'import', 'diag', 'model', 'sync', 'start', 'review', 'next'].map((cmd) => sc(`trợ giúp: /${cmd}`, `#/help/commands/${cmd}`, 'Hướng dẫn lệnh')),
])

suite('kết quả lệnh', [
  sc('Chẩn đoán gần nhất (desktop)', `#${work}?dlg=diag:last`, 'Bằng chứng'),
  sc('Đồng sáng tác (điện thoại)', resultPage('/cocreate'), 'Bàn hướng đi với Kiến trúc sư', { width: PHONE }),
  sc('Nhập truyện (điện thoại)', resultPage('/import'), 'Nhập truyện có sẵn', { width: PHONE }),
  sc('Chẩn đoán (điện thoại)', resultPage('/diag'), 'Chẩn đoán truyện', { width: PHONE }),
  sc('Đồng bộ (điện thoại)', resultPage('/sync --check'), 'Đồng bộ chương sửa ngoài', { width: PHONE }),
  sc('Mở lại (điện thoại)', resultPage('/reopen'), 'Mở lại truyện đã hoàn', { width: PHONE }),
])

suite('đăng nhập & thiết lập lần đầu', [
  sc('đăng nhập', '#/login', 'Quên mật khẩu?', { user: null }),
  sc('đăng nhập (điện thoại)', '#/login', 'Đăng nhập Noverse', { user: null, width: PHONE }),
  sc('onboard mặc định (quản trị)', '#/welcome', 'Thiết lập lần đầu'),
  sc('onboard mặc định (người viết)', '#/welcome', 'Thiết lập lần đầu', { user: 'vien' }),
  ...['language', 'password', 'provider', 'model', 'library', 'project', 'mobile', 'done'].map((step) => sc(`onboard bước ${step}`, `#/welcome/${step}`, 'Thiết lập lần đầu')),
  ...['password', 'mobile', 'done'].map((step) => sc(`onboard bước ${step} (người viết)`, `#/welcome/${step}`, 'Thiết lập lần đầu', { user: 'vien' })),
  sc('onboard bước library (máy tính bảng)', '#/welcome/library', 'Thiết lập lần đầu', { width: TABLET }),
])

// ─── Tuỳ chỉnh: 'Nâng cao' (#/settings/adv) được miễn (xem docs 20-settings-rules §1: chuỗi kế thừa ~/.ainovel/…, đường dẫn thư mục) nên không có trong danh sách; các ngăn còn lại phải sạch ───
const SETTINGS = 'Mục tuỳ chỉnh'
suite('thêm (điện thoại) · thông báo · tuỳ chỉnh', [
  sc('Thêm — dự án & truyện', '#/more', 'Dự án & truyện', { width: PHONE }),
  sc('Thêm — trang chủ', '#/more', 'Dự án & truyện', { width: PHONE, click: MORE_PICK }),
  sc('Thêm → Model', '#/more', 'Dự án & truyện', { width: PHONE, click: [...MORE_PICK, 'Model cho từng trợ lý'] }),
  sc('Thêm → Thiết bị', '#/more', 'Dự án & truyện', { width: PHONE, click: [...MORE_PICK, 'Thiết bị đang đăng nhập'] }),
  sc('Thêm → Đổi mật khẩu', '#/more', 'Dự án & truyện', { width: PHONE, click: [...MORE_PICK, 'Đổi mật khẩu'] }),
  sc('Thêm → Hướng dẫn lệnh', '#/more', 'Dự án & truyện', { width: PHONE, click: [...MORE_PICK, 'Hướng dẫn lệnh'] }),
  sc('thông báo (desktop)', '#/notifications', 'Chương 25 chờ bạn duyệt'),
  sc('thông báo (điện thoại)', '#/notifications', 'Chương 25 chờ bạn duyệt', { width: PHONE }),
  sc('cài đặt thông báo (điện thoại)', '#/notifications?view=settings', 'Thông báo', { width: PHONE }),
  ...['writing', 'models', 'budget', 'info', 'connect', 'export', 'notify', 'app'].map((section) => sc(`tuỳ chỉnh: ${section}`, `#/settings/${section}`, SETTINGS)),
  ...['writing', 'models', 'budget', 'info'].map((section) => sc(`tuỳ chỉnh: ${section} (cấp truyện)`, `#/settings/${section}?scope=story&project=${P}&story=${S}`, SETTINGS)),
  sc('tuỳ chỉnh: quy tắc viết', '#/settings/rules', SETTINGS),
  sc('tuỳ chỉnh: thông báo (người viết)', '#/settings/notify', SETTINGS, { user: 'vien' }),
  sc('tuỳ chỉnh: Trợ lý AI (người viết)', '#/settings/models', SETTINGS, { user: 'vien' }),
])
