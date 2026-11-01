// @vitest-environment happy-dom
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cache } from '@noverse/ui/data'
import { YoutubeDataKeyCard } from './YoutubeDataKeyCard'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

let root: Root
let host: HTMLDivElement
beforeEach(() => { host = document.createElement('div'); document.body.append(host); root = createRoot(host) })
afterEach(() => { act(() => root.unmount()); host.remove(); vi.unstubAllGlobals(); cache.clear?.() })

const settle = () => act(async () => {
  const { promise, resolve } = Promise.withResolvers<void>()
  setTimeout(resolve, 20)
  await promise
})

function typeKey(value: string) {
  const input = host.querySelector('input')!
  Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')!.set!.call(input, value)
  input.dispatchEvent(new Event('input', { bubbles: true }))
}

describe('Kết nối AI › YouTube Data API', () => {
  it('Google từ chối khoá (400) → hiện lý do dưới ô, không đóng ô nhập; khoá đúng thì lưu bản đã cắt khoảng trắng', async () => {
    const puts: unknown[] = []
    let accept = false
    vi.stubGlobal('fetch', vi.fn((_url: string, init?: RequestInit) => {
      if (init?.method === 'PUT') {
        puts.push(JSON.parse(init.body as string))
        return Promise.resolve(accept ? Response.json({ configured: true, keyHint: '••••7c2e' }) : Response.json({ error: 'bị từ chối', field: 'key' }, { status: 400 }))
      }
      return Promise.resolve(Response.json({ configured: false }))
    }))
    await act(async () => { root.render(<YoutubeDataKeyCard />) })
    await settle()
    expect(host.textContent).toContain('Chưa có khoá')
    await act(async () => { buttonByText('Thêm khoá').click() })
    await act(async () => { typeKey('  bad-key ') })
    await act(async () => { buttonByText('Lưu khoá').click() })
    await settle()
    expect(host.textContent).toContain('Google không nhận khoá này')
    expect(host.querySelector('input')).not.toBeNull()
    accept = true
    await act(async () => { buttonByText('Lưu khoá').click() })
    await settle()
    expect(puts).toEqual([{ key: 'bad-key' }, { key: 'bad-key' }])
    expect(host.querySelector('input')).toBeNull()
  })
})
