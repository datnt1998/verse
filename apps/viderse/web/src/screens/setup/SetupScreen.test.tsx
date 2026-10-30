// @vitest-environment happy-dom
// Chuẩn bị máy #/setup — khung riêng theo Onboard.dc.html (step=setup,
// setup=running|login|failed): header “Viderse · Lần đầu mở” + Cài đặt nâng
// cao, hero orb + tiêu đề, 4 chip trợ lý, thẻ tiến độ riêng rồi thẻ 6 bước,
// câu chân. Bước lỗi là lý do + Thử lại; bước ChatGPT luôn có nút đăng nhập
// (không có spinner thay nút); mobile chỉ nhắc mở máy tính (W1).
import { act } from 'react'
import { cache, invalidate } from '@noverse/ui/data'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import SetupScreen from './SetupScreen'
import { keys } from '../../data/keys'
import type { SetupStatus } from '../../api/setup'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

const roots: Root[] = []
let mobile = false
let state: SetupStatus
let calls: { url: string; method: string }[] = []
let startFails = false
/** Ảnh chụp server trả cho POST start (mặc định = `state`). */
let started: SetupStatus | null = null

const SIZES: Record<string, number> = {
  'yt-dlp': 50e6, ffmpeg: 400e6, transcription: 4e9, chrome: 1e9, codex: 300e6, 'codex-login': 0,
}
const step = (
  id: SetupStatus['steps'][number]['id'],
  st: SetupStatus['steps'][number]['state'],
  error: string | null = null,
) => ({
  id, state: st, error,
  sizeBytes: SIZES[id] ?? 0,
  provided: id === 'chrome',
  progress: st === 'running'
    ? { phase: 'đang tải mô hình nhận giọng nói', bytesDone: 806e6, bytesTotal: 1.3e9, pct: 62, startedAt: '2026-10-30T09:00:00Z' }
    : null,
})

function installFetch() {
  vi.stubGlobal('fetch', vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = typeof input === 'string' ? input : input instanceof URL ? input.href : input.url
    calls.push({ url, method: init?.method ?? 'GET' })
    if (url.startsWith('/api/setup/state')) return Promise.resolve(Response.json(state))
    if (url.startsWith('/api/setup/start')) {
      if (startFails) return Promise.resolve(new Response('{"error":"không gọi được service cài"}', { status: 500 }))
      return Promise.resolve(Response.json(started ?? state, { status: 202 }))
    }
    if (url.startsWith('/api/setup')) return Promise.resolve(Response.json(state))
    return Promise.resolve(Response.json({}))
  }))
}

async function render() {
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)
  roots.push(root)
  await act(async () => { root.render(<SetupScreen />) })
  for (let i = 0; i < 5; i++) await act(async () => { await new Promise((r) => setTimeout(r, 20)) })
  return container
}

/** Gỡ các root đang mount — cần khi một test dựng màn hai lần (root cũ vẫn
 *  sống sẽ nhận dữ liệu mới và tự gọi start lần nữa). */
const unmountAll = async () => {
  for (const root of roots.splice(0)) await act(() => root.unmount())
  document.body.replaceChildren()
}

const click = async (el: Element | null | undefined) => {
  expect(el).toBeTruthy()
  await act(async () => { (el as HTMLElement).dispatchEvent(new MouseEvent('click', { bubbles: true })) })
}

beforeEach(async () => {
  // clear() chứ không chỉ invalidate(): invalidate giữ dữ liệu cũ để
  // stale-while-revalidate, và dữ liệu `idle` còn sót sẽ khiến màn tự POST
  // start ngay từ khung hình đầu — đúng hành vi thật, sai với ý đồ test.
  cache.clear()
  await invalidate(keys.setup())
  mobile = false
  calls = []
  vi.stubGlobal('matchMedia', vi.fn().mockImplementation((q: string) => ({
    matches: q.includes('767px') ? mobile : false, addEventListener: () => {}, removeEventListener: () => {},
  })))
  startFails = false
  // Đồng hồ cố định đúng mốc startedAt của fixture: ETA (cần > 3 giây đã trôi)
  // không tự xuất hiện theo giờ thật lúc chạy test.
  vi.useFakeTimers({ toFake: ['Date'] })
  vi.setSystemTime(new Date('2026-10-30T09:00:00Z'))
  started = null
  state = {
    state: 'running', complete: false, running: true,
    disk: { freeBytes: 5e11, requiredBytes: 5.75e9, enough: true, approxBytes: 18e8 },
    steps: [
      step('yt-dlp', 'done'), step('ffmpeg', 'done'), step('transcription', 'running'),
      step('chrome', 'done'), step('codex', 'pending'), step('codex-login', 'needs_user'),
    ],
  }
  installFetch()
})

afterEach(async () => {
  await invalidate()
  for (const root of roots.splice(0)) await act(() => root.unmount())
  document.body.replaceChildren()
  vi.unstubAllGlobals()
  vi.useRealTimers()
})

describe('Chuẩn bị máy', () => {
  it('header riêng, hero canh trái + đủ 4 chip trợ lý, không nav', async () => {
    const el = await render()
    expect(el.textContent).toContain('Lần đầu mở')
    expect(el.textContent).toContain('Cài đặt nâng cao')
    expect(el.querySelector('.vv-setup__hero h1')?.textContent).toBe('Chuẩn bị máy cho 4 trợ lý')
    expect(el.textContent).toContain('khoảng')
    expect(el.querySelectorAll('.vv-setup__chip').length).toBe(4)
    expect(el.querySelector('.vv-setup__hero-text')).toBeTruthy()
  })

  it('phần giới thiệu “khoảng X” dùng tổng cài đặt xấp xỉ, không phải chỗ trống cần của ổ', async () => {
    const el = await render()
    // approxBytes = 18e8 → “khoảng 1,8 GB”; requiredBytes 5.75e9 phải KHÔNG
    // xuất hiện (là chỗ dự phòng ổ, không hiện ra UI).
    expect(el.textContent).toContain('khoảng 1,8 GB')
    expect(el.textContent).not.toContain('5,8 GB')
    expect(el.textContent).not.toContain('khoảng 5')
  })

  it('thẻ tiến độ tách khỏi thẻ bước và đếm theo số bước xong', async () => {
    const el = await render()
    expect(el.querySelector('.vv-setup__card-head b')?.textContent).toBe('Đang cài')
    expect(el.querySelector('.vv-setup__card-head')?.textContent).toContain('3 / 6 xong')
    expect(el.querySelector('.vv-setup__card .vv-setup__steps')).toBeNull()
    expect(el.querySelector('ol.vv-setup__steps li')).toBeTruthy()
  })

  it('hàng xong hiện dung lượng, Chrome có sẵn hiện “Có sẵn” và câu không cần tải', async () => {
    const el = await render()
    const rows = [...el.querySelectorAll('.vv-setup__row')]
    expect(rows[0].textContent).toContain('Đã cài · 50 MB')
    expect(rows[3].textContent).toContain('Có sẵn')
    expect(rows[3].textContent).toContain('không cần tải thêm')
  })

  it('hàng xong chưa đo được dung lượng chỉ ghi “Đã cài”, không treo dấu “·”', async () => {
    state = { ...state, steps: state.steps.map((s, i) => (i === 0 ? { ...s, sizeBytes: 0 } : s)) }
    const el = await render()
    const side = el.querySelector('.vv-setup__row .vv-setup__row-side')
    expect(side?.textContent).toBe('Đã cài')
  })

  it('bước đang chạy có tiến độ byte và phần trăm', async () => {
    const el = await render()
    const running = el.querySelector('.vv-setup__row.is-running')
    expect(running?.textContent).toContain('đang tải mô hình nhận giọng nói')
    expect(running?.textContent).toContain('806 MB / 1,3 GB')
    expect(running?.textContent).toContain('62%')
  })

  it('pha chưa biết tổng (pct null) chỉ ghi tên pha, không vẽ 0% hay thanh đứng yên', async () => {
    state = {
      ...state,
      steps: state.steps.map((s) => s.state === 'running'
        ? { ...s, progress: { phase: 'downloading', phaseLabel: 'Đang tải mô hình nhận giọng nói', bytesDone: 0, bytesTotal: null, pct: null, startedAt: '2026-10-30T09:00:00Z' } }
        : s),
    }
    const el = await render()
    const running = el.querySelector('.vv-setup__row.is-running')
    expect(running?.textContent).toContain('Đang tải mô hình nhận giọng nói')
    expect(running?.textContent).not.toContain('%')
    expect(running?.querySelector('[role="progressbar"]')).toBeNull()
  })

  it('bước ChatGPT đang cài có nút “Đăng nhập ngay”, không thay bằng spinner', async () => {
    const el = await render()
    const loginRow = el.querySelector('.vv-setup__row.is-needs_user')
    const btn = [...(loginRow?.querySelectorAll('button') ?? [])].find((b) => b.textContent?.includes('Đăng nhập ngay'))
    expect(btn).toBeTruthy()
    expect(loginRow?.textContent).not.toContain('Chờ bạn đăng nhập')
  })

  it('setup=login: bước ChatGPT thành focus với nút primary “Đăng nhập ChatGPT”', async () => {
    state = {
      ...state, state: 'login', running: false,
      steps: state.steps.map((s) => (s.id === 'transcription' ? step('transcription', 'done') : s.id === 'codex' ? step('codex', 'done') : s)),
    }
    const el = await render()
    expect(el.querySelector('.vv-setup__card-head b')?.textContent).toBe('Còn một bước cần bạn')
    const loginRow = el.querySelector('.vv-setup__row.is-needs_user')
    expect(loginRow?.className).toContain('is-focus')
    const btn = [...(loginRow?.querySelectorAll('button') ?? [])].find((b) => b.textContent?.includes('Đăng nhập ChatGPT'))
    expect(btn).toBeTruthy()
  })

  it('bước lỗi nêu lý do mạng và Thử lại gọi POST retry', async () => {
    state = {
      ...state, state: 'failed', running: false,
      steps: state.steps.map((s) => (s.id === 'transcription' ? step('transcription', 'failed', 'dial tcp: i/o timeout') : s.id === 'codex' ? step('codex', 'done') : s)),
    }
    const el = await render()
    expect(el.querySelector('.vv-setup__card-head b')?.textContent).toBe('Một bước chưa cài được')
    const failRow = el.querySelector('.vv-setup__row.is-failed')
    expect(failRow?.textContent).toContain('Tải quá chậm nên bị ngắt')
    expect(el.querySelector('.vv-setup__card-sub')?.textContent).toContain('bước 3')
    await click([...(failRow?.querySelectorAll('button') ?? [])].find((b) => b.textContent?.includes('Thử lại')))
    expect(calls.some((c) => c.url === '/api/setup/retry' && c.method === 'POST')).toBe(true)
  })

  it('máy mới (idle + chưa xong + đủ chỗ) tự gọi đúng một POST start', async () => {
    state = {
      ...state, state: 'idle', running: false,
      steps: state.steps.map((s) => ({ ...s, state: s.id === 'chrome' ? 'done' as const : s.id === 'codex-login' ? 'needs_user' as const : 'pending' as const, progress: null })),
    }
    await render()
    const starts = calls.filter((c) => c.url === '/api/setup/start' && c.method === 'POST')
    expect(starts.length).toBe(1)
  })

  it('máy đã xong hoặc đang chạy thì không gọi lại POST start', async () => {
    await render() // mặc định running
    expect(calls.some((c) => c.url === '/api/setup/start')).toBe(false)
    await unmountAll()
    cache.clear()
    calls = []
    state = { ...state, state: 'done', complete: true, running: false }
    await render()
    expect(calls.some((c) => c.url === '/api/setup/start')).toBe(false)
  })

  it('mobile và máy thiếu chỗ đều không tự khởi lượt cài', async () => {
    mobile = true
    state = { ...state, state: 'idle', running: false }
    await render()
    expect(calls.some((c) => c.url === '/api/setup/start')).toBe(false)
    await unmountAll()
    cache.clear()
    calls = []
    mobile = false
    state = { ...state, state: 'idle', running: false, disk: { ...state.disk, enough: false } }
    await render()
    expect(calls.some((c) => c.url === '/api/setup/start')).toBe(false)
  })

  it('idle chưa gọi start xong thì thẻ nói đang mở lượt, không gọi sai “Đang cài”', async () => {
    state = {
      ...state, state: 'idle', running: false,
      steps: state.steps.map((s) => ({ ...s, state: s.id === 'chrome' ? 'done' as const : 'pending' as const, progress: null })),
    }
    const el = await render()
    const head = el.querySelector('.vv-setup__card-head b')?.textContent
    expect(head).not.toBe('Đang cài')
  })

  it('ảnh chụp POST start được ghi vào màn: thẻ chuyển sang Đang cài ngay', async () => {
    state = {
      ...state, state: 'idle', running: false,
      steps: state.steps.map((s) => ({ ...s, state: s.id === 'chrome' ? 'done' as const : s.id === 'codex-login' ? 'needs_user' as const : 'pending' as const, progress: null })),
    }
    started = {
      ...state, state: 'running', running: true,
      steps: state.steps.map((s) => (s.id === 'transcription'
        ? step('transcription', 'running')
        : s)),
    }
    const el = await render()
    expect(el.querySelector('.vv-setup__card-head b')?.textContent).toBe('Đang cài')
    expect(el.querySelector('.vv-setup__row.is-running')).toBeTruthy()
  })

  it('POST start lỗi thì thẻ báo lỗi chứ không giả chạy', async () => {
    startFails = true
    state = {
      ...state, state: 'idle', running: false,
      steps: state.steps.map((s) => ({ ...s, state: s.id === 'chrome' ? 'done' as const : 'pending' as const, progress: null })),
    }
    const el = await render()
    const head = el.querySelector('.vv-setup__card-head b')?.textContent
    expect(head).toBe('Không bắt đầu được')
    expect(el.querySelector('.vv-setup__card-sub')?.textContent?.toLowerCase()).toContain('service cài')
  })

  it('mobile chỉ nhắc mở trên máy tính', async () => {
    mobile = true
    const el = await render()
    expect(el.textContent).toContain('Chuẩn bị máy chỉ có trên máy tính')
    expect(el.querySelector('.vv-setup__card')).toBeNull()
  })
})
