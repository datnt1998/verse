// @vitest-environment happy-dom
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { invalidate } from '@noverse/ui/data'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import StorageSection from './StorageSection'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })
let root: Root
let el: HTMLDivElement
let role = 'admin'
let locked = false
let deliveryFromEnv = false
let issue: string | undefined
let direct = false
let mobile = false
const calls: { url: string; init?: RequestInit }[] = []
const steps = (probe = false) => ['write', 'read', 'range', 'presign', probe ? 'cors_probe' : 'app_range', 'delete'].map(id => ({ id, status: id === 'presign' && !probe ? 'na' : 'ok', ms: 2 }))
const flush = async () => { await act(async () => { await new Promise(resolve => setTimeout(resolve, 25)) }) }
const button = (text: string) => [...document.querySelectorAll('button')].find(x => x.textContent?.includes(text))!
const click = async (text: string) => { expect(button(text)).toBeTruthy(); await act(async () => button(text).click()); await flush() }
async function render() {
  await act(async () => root.render(<StorageSection />))
  await flush()
}
async function form() {
  await render(); await click('Chọn kho S3'); await click('Garage')
  for (const [name, value] of Object.entries({ endpoint: 'http://garage:3900', bucket: 'viderse-media', accessKeyId: 'key', secretAccessKey: 'secret' })) {
    const input = el.querySelector<HTMLInputElement>(`[name="${name}"]`)!
    expect(input).toBeTruthy()
    await act(async () => {
      Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')!.set!.call(input, value)
      input.dispatchEvent(new Event('input', { bubbles: true }))
    })
  }
}
beforeEach(async () => {
  await invalidate(); calls.length = 0; role = 'admin'; locked = false; deliveryFromEnv = false; issue = undefined; direct = false; mobile = false
  vi.stubGlobal('matchMedia', vi.fn(() => ({ get matches() { return mobile }, addEventListener() {}, removeEventListener() {} })))
  vi.stubGlobal('fetch', vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = String(input); calls.push({ url, init })
    if (url === '/api/storage/server') return Response.json({ role, storage: {
      kind: 'disk', delivery: 'proxy', sources: locked ? { provider: 'env' } : deliveryFromEnv ? { delivery: 'env', cache: 'env' } : {}, encryptionKey: { ok: !issue, path: '/data/config/secret.key', issue }, online: true, checkedAt: '2026-10-31T21:31:00Z', disk: { path: '/data/projects', freeBytes: 100e9 },
    }, usage: ['images', 'editor', 'library', 'sources', 'exports'].map(group => ({ group, files: group === 'library' ? 22 : null, bytes: group === 'library' ? 400e6 : null })), workCache: { usedBytes: 0, limitBytes: 20e9 }, ops: [] })
    if (url === '/api/storage/server/test') return Response.json({ testId: 'test1', steps: steps(direct), ...(direct ? { probeUrl: 'https://storage.example/probe' } : {}) })
    if (url === 'https://storage.example/probe') return new Response(new Uint8Array(1024), { status: 206 })
    if (url.endsWith('/test/test1/probe')) return Response.json({ testId: 'test1', steps: steps(true) })
    if (url.endsWith('/config')) return Response.json({ needsMigration: { files: 22, bytes: 400e6, groups: [] } })
    if (url.endsWith('/migrate')) return Response.json({ taskId: 'move1' })
    if (url === '/api/storage/cleanup/preview') return Response.json({ previewId: 'clean1', items: [{ kind: 'orphan_objects', label: 'old objects', count: 3, bytes: 30e6 }, { kind: 'work_cache', label: 'cache', count: 1, bytes: 20e6 }, { kind: 'pipeline', label: 'pipeline', count: 10, bytes: 100e6 }], freedBytes: 150e6, stoppedProcesses: 0 })
    if (url === '/api/storage/cleanup/run') return Response.json({ deleted: [], freedBytes: 30e6, stoppedProcesses: 0, finishedAt: '2026-10-31T00:00:00Z' })
    if (url === '/api/storage') return Response.json({ projects: { path: '/data/projects', volume: { name: 'Disk', kind: 'internal', freeBytes: 100e9 }, usedBytes: 400e6, online: true }, move: { active: false }, stayBytes: 0, suggestions: [], usage: [], clean: { lastRunAt: null } })
    if (url === '/api/storage/reclaim') return Response.json({ items: [] })
    if (url.startsWith('/api/activity')) return Response.json({ running: [] })
    return Response.json({}, { status: 404 })
  }))
  el = document.createElement('div'); document.body.append(el); root = createRoot(el)
})
afterEach(async () => { await act(async () => root.unmount()); el.remove(); await invalidate(); vi.unstubAllGlobals() })
it('transitions picker → form → tested → confirmation and starts migration only after confirmation', async () => {
  await form(); expect(button('Lưu…').disabled).toBe(true)
  await click('Kiểm tra kết nối'); expect(button('Lưu…').disabled).toBe(false)
  await click('Lưu…'); expect(document.querySelector('[role="alertdialog"]')).toBeTruthy()
  expect(calls.some(x => x.url.endsWith('/migrate'))).toBe(false)
  await click('Bắt đầu chuyển'); expect(calls.some(x => x.url.endsWith('/migrate'))).toBe(true)
})
it('clears a successful test when any form field changes', async () => {
  await form(); await click('Kiểm tra kết nối')
  const input = el.querySelector<HTMLInputElement>('[name="bucket"]')!
  await act(async () => { Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')!.set!.call(input, 'another'); input.dispatchEvent(new Event('input', { bubbles: true })) })
  expect(button('Lưu…').disabled).toBe(true)
  expect(el.querySelector('[data-test-success]')).toBeNull()
})
it('filters all admin controls even if a member response includes admin fields', async () => {
  role = 'member'; await render()
  expect(el.textContent).toContain('Dung lượng theo nhóm')
  expect(button('Chọn kho S3')).toBeUndefined()
  expect(el.textContent).not.toContain('Bộ nhớ đệm làm việc')
  expect(el.textContent).not.toContain('/data/projects')
})
it('makes environment configuration read-only with no provider mutation', async () => {
  locked = true; await render()
  expect(el.textContent).toContain('do máy chủ đặt · chỉ đọc')
  expect(button('Chọn kho S3')).toBeUndefined()
  expect(button('Đổi kho')).toBeUndefined()
  expect(el.querySelector('input')).toBeNull()
})
it('allows tested storage changes when only delivery and cache come from the environment', async () => {
  deliveryFromEnv = true
  await form()
  await click('Kiểm tra kết nối')
  expect(button('Lưu…').disabled).toBe(false)
  await click('Lưu…')
  expect(document.querySelector('[role="alertdialog"]')).toBeTruthy()
})
it('renders null groups as unknown instead of zero and totals only counted data', async () => {
  await render()
  expect(el.textContent).toContain('Đã đếm 22 tệp · 400 MB')
  expect([...el.querySelectorAll('[data-usage-gap]')]).toHaveLength(4)
})
it('runs the browser range probe before finalizing the redirect test', async () => {
  direct = true; await form(); await click('Trực tiếp từ kho'); await click('Kiểm tra kết nối')
  const probe = calls.find(x => x.url === 'https://storage.example/probe')!
  expect(probe?.init?.headers).toEqual({ Range: 'bytes=0-1023' })
  expect(probe?.init?.credentials).toBe('omit')
  expect(calls.find(x => x.url.endsWith('/test/test1/probe'))?.init?.body).toBe('{"ok":true}')
  expect(button('Lưu…').disabled).toBe(false)
})
it('keeps mobile read-only without configuration or migration controls', async () => {
  mobile = true; await render()
  expect(button('Chọn kho S3')).toBeUndefined()
  expect(el.textContent).toContain('máy tính')
})
it.each(['readonly_dir', 'bad_permissions', 'corrupt'])('blocks saving for encryption issue %s but allows testing', async value => {
  issue = value; await form(); await click('Kiểm tra kết nối')
  expect(button('Lưu…').disabled).toBe(true)
  expect(el.textContent).toContain('Máy chủ chưa có khoá mã hoá')
  expect(el.querySelector('details')?.textContent).toContain('/data/config/secret.key')
})
it('previews server-owned cleanup groups for the admin and runs the exact preview', async () => {
  role = 'admin'; await render(); await click('Xem trước những gì sẽ dọn')
  expect(el.textContent).toContain('Tệp trên kho của video đã xoá')
  expect(el.textContent?.includes('Bộ đệm trên máy chủ')).toBe(true)
  expect(el.textContent).not.toContain('pipeline')
  expect(el.textContent).not.toContain('old objects')
  expect(calls.find(call => call.url === '/api/storage/cleanup/preview')?.init?.method).toBe('POST')
  await click('Dọn 50 MB')
  expect(calls.find(call => call.url === '/api/storage/cleanup/run')?.init?.body).toBe('{\"previewId\":\"clean1\"}')
})
it('shows members no cleanup controls because the server refuses them', async () => {
  role = 'member'; await render()
  expect(button('Xem trước những gì sẽ dọn')).toBeUndefined()
  expect(calls.some(call => call.url.startsWith('/api/storage/cleanup/'))).toBe(false)
})
