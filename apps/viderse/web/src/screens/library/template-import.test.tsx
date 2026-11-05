// @vitest-environment happy-dom
// Hộp nhập template: tiến độ thật từng tệp, huỷ phần chưa nhập, mất kết nối giữ
// tệp và Thử lại chỉ nhập lại tệp chưa xong (không nhập trùng tệp đã xong).
import { act, StrictMode } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { TemplateImportDialog } from './TemplateImportDialog'
import { t } from '../../i18n/library'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

const roots: Root[] = []
const imported: string[] = []
let mode: (name: string) => Promise<Response>
let gate: (() => void) | null = null

const zip = (name: string) => new File(['PK'], name, { type: 'application/zip' })
const item = (name: string, ok = true, error?: string) => ({ file: name, name, ok, warnings: [], error, media: 2, mediaReused: 0 })

beforeEach(() => {
  imported.length = 0
  gate = null
  mode = async (name) => Response.json({ items: [item(name)], imported: 1, failed: 0 })
  vi.stubGlobal('fetch', vi.fn(async (path: string, init?: RequestInit) => {
    if (String(path).includes('upload-limits')) return Response.json({ hosted: false, limits: {}, message: '' })
    if (String(path).includes('/api/templates/import')) {
      const file = (init?.body as FormData).get('file') as File
      imported.push(file.name)
      return mode(file.name)
    }
    return Response.json({})
  }))
})

afterEach(async () => {
  for (const root of roots.splice(0)) await act(() => root.unmount())
  document.body.replaceChildren()
  vi.unstubAllGlobals()
})

async function mount() {
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)
  roots.push(root)
  await act(async () => { root.render(<StrictMode><TemplateImportDialog onClose={() => {}} /></StrictMode>) })
  return container
}

const choose = async (names: string[]) => {
  const input = document.querySelector<HTMLInputElement>('input[type="file"]')!
  Object.defineProperty(input, 'files', { value: names.map(zip), configurable: true })
  await act(async () => { input.dispatchEvent(new Event('change', { bubbles: true })) })
}

const button = (label: string) => Array.from(document.querySelectorAll('button')).find((node) => node.textContent?.includes(label))!

describe('TemplateImportDialog', () => {
  it('một tệp lỗi → một phần: mỗi tệp đúng một yêu cầu, tổng kết đúng', async () => {
    mode = async (name) => name === 'b.zip'
      ? Response.json({ items: [item(name, false, 'tệp hỏng')], imported: 0, failed: 1 })
      : Response.json({ items: [{ ...item(name), name: 'Tên mẫu video đã nhập' }], imported: 1, failed: 0 })
    await mount()
    await choose(['a.zip', 'b.zip'])
    await vi.waitFor(() => expect(document.body.textContent).toContain(t.library.templates.importSummary(1, 1)))
    expect(imported).toEqual(['a.zip', 'b.zip'])
    expect(document.body.textContent).toContain('tệp hỏng')
    expect(document.body.textContent).toContain('Tên mẫu video đã nhập')
    expect(document.body.textContent).toContain('b.zip')
  })

  it('huỷ phần chưa nhập: tệp đang chạy vẫn xong, tệp sau không gửi', async () => {
    mode = async (name) => {
      if (name !== 'a.zip') return Response.json({ items: [item(name)], imported: 1, failed: 0 })
      const pending = Promise.withResolvers<void>()
      gate = pending.resolve
      await pending.promise
      return Response.json({ items: [item(name)], imported: 1, failed: 0 })
    }
    await mount()
    await choose(['a.zip', 'b.zip', 'c.zip'])
    await vi.waitFor(() => expect(imported).toEqual(['a.zip']))
    await act(async () => { button(t.library.templates.importCancel).click() })
    await act(async () => { gate?.() })
    await vi.waitFor(() => expect(document.body.textContent).toContain(t.library.templates.importCancelSummary(1, 2)))
    expect(imported).toEqual(['a.zip'])
    expect(document.body.textContent).toContain(t.library.templates.importCancelled)
  })

  it('mất kết nối giữ tệp; Thử lại chỉ gửi lại tệp chưa xong', async () => {
    let offline = true
    mode = async (name) => {
      if (name === 'b.zip' && offline) throw new TypeError('Failed to fetch')
      return Response.json({ items: [item(name)], imported: 1, failed: 0 })
    }
    await mount()
    await choose(['a.zip', 'b.zip', 'c.zip'])
    await vi.waitFor(() => expect(document.body.textContent).toContain(t.library.templates.importOffline))
    expect(imported).toEqual(['a.zip', 'b.zip'])

    offline = false
    await act(async () => { button(t.library.templates.importRetry).click() })
    await vi.waitFor(() => expect(document.body.textContent).toContain(t.library.templates.importSummary(3, 0)))
    expect(imported).toEqual(['a.zip', 'b.zip', 'b.zip', 'c.zip'])
  })
})
