// Dữ liệu dùng chung cho test của Đề xuất và Quy tắc đã học (đúng hình dạng API đã chốt).
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { expect } from 'vitest'

export const json = (body: unknown, status = 200) => new Response(JSON.stringify(body), { status, headers: { 'content-type': 'application/json' } })

export const suggestion = (id: string, over: Record<string, unknown> = {}) => ({
  id, kind: 'hot_video', agent: 'scout', channelId: 'c1', channelName: 'Midnight Tales',
  title: `Đề xuất ${id}`, evidence: 'Midnight Tales · ×4,2 · đang thứ 6',
  reason: { agent: 'scout', text: 'Video này đang nổi.', facts: [{ label: 'Lượt xem', value: '×4,2', source: 'rival' }] },
  primary: { label: 'Lên đầu', op: 'prioritize', params: { jobId: 'j1' } },
  createdAt: '2026-11-01T00:00:00Z', expiresAt: '2026-11-08T00:00:00Z', state: 'open', ...over,
})

export const roots: Root[] = []
export const flush = (ms = 20) => act(async () => { const { promise, resolve } = Promise.withResolvers<void>(); setTimeout(resolve, ms); await promise })

export const mount = async (node: React.ReactNode) => {
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)
  roots.push(root)
  await act(async () => { root.render(node) })
  await flush()
  return container
}
export const unmountAll = async () => { await act(async () => { for (const root of roots.splice(0)) root.unmount() }); document.body.innerHTML = '' }

export const button = (el: ParentNode, text: string) => [...el.querySelectorAll('button')].find((b) => b.textContent?.trim().startsWith(text)) as HTMLButtonElement | undefined
export const click = async (target: Element | undefined | null) => { expect(target).toBeTruthy(); await act(async () => { (target as HTMLElement).click() }); await flush() }

export type Call = { url: string; method: string; body: unknown }
