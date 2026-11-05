// @vitest-environment happy-dom
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { cache } from '@noverse/ui/data'
import { WorkbenchScreen } from './WorkbenchScreen'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })
let root: Root, host: HTMLDivElement, admitted: string | null
const json = (body: unknown) => new Response(JSON.stringify(body), { headers: { 'Content-Type': 'application/json' } })
beforeEach(async () => {
  cache.clear(); vi.useFakeTimers(); admitted = null
  history.replaceState(null, '', '#/?queueItem=row')
  vi.stubGlobal('matchMedia', vi.fn(() => ({ matches: false, addEventListener() {}, removeEventListener() {} })))
  vi.stubGlobal('fetch', vi.fn(async (path: string) => {
    if (path === '/api/queue/items/row') return json({ id: 'row', channelId: 'mine', jobId: admitted, state: admitted ? 'admitted' : 'queued' })
    if (path === '/api/queue') return json({ items: [{ id: 'row', channelId: 'mine', title: 'Video thật đã chọn', source: 'Nguồn thật', templateName: 'Template của kênh', waitReason: 'queue_paused' }] })
    return json({})
  }))
  host = document.createElement('div'); document.body.append(host); root = createRoot(host)
  await act(async () => root.render(<WorkbenchScreen route={{ name: 'workbench', queueItem: 'row' }} />))
})
afterEach(async () => { await act(async () => root.unmount()); host.remove(); cache.clear(); vi.useRealTimers(); vi.unstubAllGlobals() })
it('reloads the selected real pending row rather than a generic queue screen', () => {
  expect(host.textContent).toContain('Video thật đã chọn')
  expect(host.textContent).toContain('Template của kênh')
  expect(host.textContent).toContain('Hàng chờ đang tạm dừng')
})
it('follows the persisted queue-item-to-job mapping after admission', async () => {
  admitted = 'real-job'
  await act(async () => { await vi.advanceTimersByTimeAsync(2500) })
  expect(location.hash).toBe('#/?channel=mine&video=real-job')
})
