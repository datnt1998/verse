// @vitest-environment happy-dom
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { MigrationCard } from './MigrationCard'
import type { Migration } from '../../../api/storageServer'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })
let root: Root
let el: HTMLDivElement
const calls: string[] = []
const migration: Migration = {
  taskId: 'move1', state: 'running', from: { kind: 'disk', label: 'Ổ máy chủ' }, to: { kind: 's3', label: 'Garage' },
  files: { done: 10, total: 20 }, bytes: { done: 500e6, total: 1e9 }, checked: 10, rateBps: 10e6, startedAt: '2026-10-31T00:00:00Z', groups: [], errors: [],
}
beforeEach(() => {
  calls.length = 0
  vi.stubGlobal('fetch', vi.fn(async (input: RequestInfo | URL) => { calls.push(String(input)); return new Response(null, { status: 204 }) }))
  el = document.createElement('div'); document.body.append(el); root = createRoot(el)
})
afterEach(async () => { await act(async () => root.unmount()); el.remove(); vi.unstubAllGlobals() })
async function render(value = migration, mobile = false) { await act(async () => root.render(<MigrationCard migration={value} mobile={mobile} />)) }
async function click(label: string) {
  const button = [...el.querySelectorAll('button')].find(item => item.textContent?.includes(label))!
  expect(button).toBeTruthy()
  await act(async () => button.click())
}
it('shows a compact single-progress mobile summary without desktop controls', async () => {
  await render(migration, true)
  expect(el.querySelectorAll('[role="progressbar"]')).toHaveLength(1)
  expect(el.querySelector('button')).toBeNull()
  expect(el.textContent).toContain('Tạm dừng hoặc huỷ trên máy tính')
})
it.each(['running', 'paused'] as const)('sends the correct control for %s migration', async state => {
  await render({ ...migration, state })
  await click(state === 'running' ? 'Tạm dừng' : 'Tiếp tục')
  expect(calls).toEqual([`/api/storage/server/migrate/move1/${state === 'running' ? 'pause' : 'resume'}`])
})
it('cancels only after confirmation and explains copied objects remain reusable', async () => {
  await render(); await click('Huỷ chuyển')
  expect(calls).toHaveLength(0)
  expect(el.querySelector('[role="alertdialog"]')?.textContent).toContain('lần chuyển sau dùng lại, không phải chép lại')
  const confirm = [...el.querySelectorAll('[role="alertdialog"] button')].find(button => button.textContent === 'Huỷ chuyển') as HTMLButtonElement
  await act(async () => confirm.click())
  expect(calls).toEqual(['/api/storage/server/migrate/move1/cancel'])
})
it('retries failed files without switching the active old location', async () => {
  await render({ ...migration, state: 'error', errors: [{ group: 'library', title: 'Ảnh lớp phủ', code: 'content_mismatch' }] })
  expect(el.textContent).toContain('Viderse vẫn dùng Ổ máy chủ')
  expect(el.textContent).toContain('đọc lại không khớp')
  await click('Thử lại 1 tệp lỗi')
  expect(calls).toEqual(['/api/storage/server/migrate/move1/retry'])
})
