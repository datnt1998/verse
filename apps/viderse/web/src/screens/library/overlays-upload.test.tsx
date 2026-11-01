// Lớp phủ: tệp vượt trần tải qua internet phải bị chặn ngay trên UI, kèm câu
// nêu trần — nếu để request đi, Cloudflare trả trang 413 HTML mà người dùng
// không hiểu.
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { OverlaysTab } from './OverlaysTab'
import { resetUploadLimitsCache } from '../../api/upload-limits'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

const roots: Root[] = []

function respond(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } })
}

const calls: { url: string; method: string }[] = []

function installFetch() {
  vi.stubGlobal('fetch', vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = typeof input === 'string' ? input : input instanceof URL ? input.href : input.url
    calls.push({ url, method: init?.method ?? 'GET' })
    if (url === '/api/upload-limits') {
      return respond({
        hosted: true,
        limits: { video: 90 << 20, audio: 90 << 20, image: 50 << 20, overlay: 90 << 20, template: 90 << 20 },
        message: 'Tải lên qua internet tối đa 90 MB',
      })
    }
    if (url === '/api/overlays') return respond({ items: [] })
    return respond({}, 404)
  }))
}

async function render(node: React.ReactNode) {
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)
  roots.push(root)
  await act(async () => { root.render(node) })
  await flush()
  return container
}

/** Nhường một nhịp cho effect và promise của React chạy xong. */
const tick = () => {
  const { promise, resolve } = Promise.withResolvers<void>()
  setTimeout(resolve, 10)
  return promise
}

const flush = async (rounds = 6) => {
  for (let i = 0; i < rounds; i++) await act(async () => { await tick() })
}

afterEach(() => {
  for (const root of roots.splice(0)) act(() => root.unmount())
  document.body.replaceChildren()
  calls.length = 0
  resetUploadLimitsCache()
  vi.unstubAllGlobals()
})

describe('tải lớp phủ qua internet', () => {
  it('tệp 91 MiB bị chặn trên UI và hiện trần, không gửi request', async () => {
    installFetch()
    const page = await render(<OverlaysTab />)
    const input = page.querySelector<HTMLInputElement>('input[type="file"]')!

    const file = new File(['x'], 'mua.mp4', { type: 'video/mp4' })
    Object.defineProperty(file, 'size', { value: 91 << 20 })
    Object.defineProperty(input, 'files', { value: [file], configurable: true })
    await act(async () => { input.dispatchEvent(new Event('change', { bubbles: true })) })
    await flush()

    expect(calls.filter((c) => c.method === 'POST')).toHaveLength(0)
    expect(page.textContent).toContain('Tải lên qua internet tối đa 90 MB')
  })

  it('tệp trong trần vẫn được gửi', async () => {
    installFetch()
    const page = await render(<OverlaysTab />)
    const input = page.querySelector<HTMLInputElement>('input[type="file"]')!

    const file = new File(['x'], 'mua.mp4', { type: 'video/mp4' })
    Object.defineProperty(file, 'size', { value: 8 << 20 })
    Object.defineProperty(input, 'files', { value: [file], configurable: true })
    await act(async () => { input.dispatchEvent(new Event('change', { bubbles: true })) })
    await flush()

    expect(calls.some((c) => c.method === 'POST' && c.url === '/api/overlays')).toBe(true)
  })
})
