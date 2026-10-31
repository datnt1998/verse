// @vitest-environment happy-dom
import { act } from 'react'
import { createRoot } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { confirm } from '@noverse/ui'
import type { CloneJobView } from '../../api/workbench'
import { requestMarkPosted } from './MarkPosted'

vi.mock('@noverse/ui', async (importOriginal) => ({ ...(await importOriginal<object>()), confirm: vi.fn(), toast: { ok: vi.fn(), err: vi.fn() } }))
vi.mock('@noverse/ui/data', () => ({ invalidate: vi.fn() }))
beforeEach(() => vi.clearAllMocks())

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })
afterEach(() => { vi.unstubAllGlobals(); document.body.innerHTML = '' })

const job = (exportBytes: number | null) => ({ id: 'j', exportBytes }) as CloneJobView
type Options = Parameters<typeof confirm>[0]

/** Vẽ thân hộp rồi gõ link vào ô, như người dùng. */
async function typeLink(options: Options, link: string) {
  const host = document.createElement('div')
  document.body.append(host)
  await act(async () => { createRoot(host).render(options.body) })
  const input = host.querySelector('input')!
  await act(async () => {
    Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')!.set!.call(input, link)
    input.dispatchEvent(new Event('input', { bubbles: true }))
  })
}

/** Hộp xác nhận giả theo kit: chạy onConfirm, lỗi ném ra → hộp không đóng (false). */
async function acceptDialog(options: Options) {
  try { await options.onConfirm(''); return true } catch { return false }
}

describe('Đánh dấu đã đăng kèm link', () => {
  it('gửi link YouTube tới /posted rồi mới hỏi xoá MP4 đúng dung lượng', async () => {
    const calls: Array<{ url: string; body: unknown }> = []
    vi.stubGlobal('fetch', vi.fn((url: string, init: RequestInit) => {
      calls.push({ url, body: JSON.parse(init.body as string) })
      return Promise.resolve(Response.json({ youtubeId: 'abc123', results: [{ ok: true }] }))
    }))
    const titles: string[] = []
    vi.mocked(confirm).mockImplementation(async (options) => {
      titles.push(options.title)
      if (titles.length === 1) await typeLink(options, 'https://youtu.be/abc123')
      return acceptDialog(options)
    })
    const done = vi.fn()
    await requestMarkPosted(job(1048577), done)
    expect(titles).toEqual(['Đánh dấu đã đăng?', 'Xoá MP4 để lấy lại 2 MB?'])
    expect(calls.map((call) => call.url)).toEqual(['/api/clone/j/posted', '/api/clone/bulk'])
    expect(calls[0].body).toEqual({ youtubeUrl: 'https://youtu.be/abc123' })
    expect(done).toHaveBeenCalledTimes(2)
  })
  it('link sai thì lỗi hiện trong hộp, không gọi máy chủ và không hỏi xoá MP4', async () => {
    const fetchMock = vi.fn()
    vi.stubGlobal('fetch', fetchMock)
    vi.mocked(confirm).mockImplementationOnce(async (options) => {
      await typeLink(options, 'https://example.com/x')
      await expect(options.onConfirm('')).rejects.toThrow('Link này không phải video YouTube')
      return false
    })
    await requestMarkPosted(job(10), vi.fn())
    expect(fetchMock).not.toHaveBeenCalled()
    expect(confirm).toHaveBeenCalledTimes(1)
  })
})
